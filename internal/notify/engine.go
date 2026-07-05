// Package notify implements notification channels and the rules engine that
// dispatches messages after each scan.
package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/t0mer/bothan/internal/crypto"
	"github.com/t0mer/bothan/internal/model"
	"github.com/t0mer/bothan/internal/ssllabs"
	"github.com/t0mer/bothan/internal/store"
)

// Engine evaluates rules after each scan and dispatches matched notifications.
type Engine struct {
	hosts      *store.HostRepo
	scans      *store.ScanRepo
	rules      *store.RuleRepo
	channels   *store.ChannelRepo
	cipher     *crypto.Cipher
	dispatcher *Dispatcher
	logger     *slog.Logger
	observe    func(channelType, result string)

	// sendHook, when set, replaces the real provider send (used in tests).
	sendHook func(channelType string, configJSON []byte, message string) error
}

// EngineOptions configures the Engine.
type EngineOptions struct {
	Store      *store.Store
	Cipher     *crypto.Cipher // may be nil when no key is configured
	Dispatcher *Dispatcher
	Logger     *slog.Logger
	Observe    func(channelType, result string) // metrics hook (Phase 10)
}

// NewEngine builds a rules Engine.
func NewEngine(o EngineOptions) *Engine {
	logger := o.Logger
	if logger == nil {
		logger = slog.Default()
	}
	d := o.Dispatcher
	if d == nil {
		d = NewDispatcher(nil)
	}
	return &Engine{
		hosts: o.Store.Hosts(), scans: o.Store.Scans(), rules: o.Store.Rules(),
		channels: o.Store.Channels(), cipher: o.Cipher, dispatcher: d,
		logger: logger, observe: o.Observe,
	}
}

// Evaluate runs after a scan completes: it matches the union of global and
// host rules and dispatches a message per matched rule to the host's enabled
// channels. It is best-effort and never blocks the scan.
func (e *Engine) Evaluate(ctx context.Context, scan *model.Scan) {
	host, err := e.hosts.Get(ctx, scan.HostID)
	if err != nil {
		e.logger.Error("notify: loading host", slog.Int64("host", scan.HostID), slog.String("error", err.Error()))
		return
	}

	var prev *model.Scan
	if p, err := e.scans.PreviousReady(ctx, scan.HostID, scan.ID); err == nil {
		prev = p
	}

	rules, err := e.rules.RulesForHost(ctx, host.ID)
	if err != nil {
		e.logger.Error("notify: loading rules", slog.String("error", err.Error()))
		return
	}

	matched := make([]model.Rule, 0, len(rules))
	for _, rule := range rules {
		if e.matches(ctx, rule, scan, prev) {
			matched = append(matched, rule)
		}
	}
	if len(matched) == 0 {
		return
	}

	channels, err := e.channels.EnabledChannelsForHost(ctx, host.ID)
	if err != nil {
		e.logger.Error("notify: loading channels", slog.String("error", err.Error()))
		return
	}
	if len(channels) == 0 {
		return
	}

	for _, rule := range matched {
		message := formatMessage(host, scan, prev, rule)
		for _, ch := range channels {
			e.deliver(ctx, ch, message)
		}
	}
}

func (e *Engine) deliver(ctx context.Context, ch model.Channel, message string) {
	result := "sent"
	if err := e.send(ctx, ch, message); err != nil {
		result = "failed"
		e.logger.Warn("notify: send failed",
			slog.String("channel", ch.Name), slog.String("type", ch.Type), slog.String("error", err.Error()))
	} else {
		e.logger.Info("notify: sent", slog.String("channel", ch.Name), slog.String("type", ch.Type))
	}
	if e.observe != nil {
		e.observe(ch.Type, result)
	}
}

func (e *Engine) send(ctx context.Context, ch model.Channel, message string) error {
	if e.cipher == nil {
		return errors.New("encryption key not configured")
	}
	configJSON, err := e.cipher.Decrypt(ch.ConfigEncrypted)
	if err != nil {
		return fmt.Errorf("decrypting channel config: %w", err)
	}
	if e.sendHook != nil {
		return e.sendHook(ch.Type, configJSON, message)
	}
	return e.dispatcher.Send(ctx, ch.Type, configJSON, message)
}

// matches reports whether a rule fires for this scan given the previous scan.
func (e *Engine) matches(ctx context.Context, rule model.Rule, scan, prev *model.Scan) bool {
	switch rule.ConditionType {
	case model.CondScanFailed:
		return scan.Status == model.ScanStatusError

	case model.CondScanCompleted:
		return scan.Status == model.ScanStatusReady
	}

	// Remaining conditions require a completed scan.
	if scan.Status != model.ScanStatusReady {
		return false
	}

	switch rule.ConditionType {
	case model.CondGradeBelow:
		if model.GradeRank(scan.OverallGrade) >= model.GradeRank(rule.ThresholdGrade) {
			return false
		}
		// Suppress repeat alerts while the failing grade is unchanged.
		if prev != nil && prev.OverallGrade == scan.OverallGrade {
			return false
		}
		return true

	case model.CondGradeChanged:
		return prev != nil && prev.OverallGrade != scan.OverallGrade

	case model.CondGradeDowngraded:
		return prev != nil && model.GradeRank(scan.OverallGrade) < model.GradeRank(prev.OverallGrade)

	case model.CondGradeImproved:
		return prev != nil && model.GradeRank(scan.OverallGrade) > model.GradeRank(prev.OverallGrade)

	case model.CondCertExpiry:
		days := 30
		if rule.ExpiryDays != nil {
			days = *rule.ExpiryDays
		}
		return anyCertExpiringWithin(scan, days)

	case model.CondVulnDetected:
		return e.hasVulnerabilities(ctx, scan)
	}
	return false
}

func anyCertExpiringWithin(scan *model.Scan, days int) bool {
	threshold := nowUTC().Add(time.Duration(days) * 24 * time.Hour)
	for _, ep := range scan.Endpoints {
		if ep.CertNotAfter != nil && ep.CertNotAfter.Before(threshold) {
			return true
		}
	}
	return false
}

func (e *Engine) hasVulnerabilities(ctx context.Context, scan *model.Scan) bool {
	raw, err := e.scans.GetRaw(ctx, scan.ID)
	if err != nil || len(raw) == 0 {
		return false
	}
	var host ssllabs.Host
	if err := json.Unmarshal(raw, &host); err != nil {
		return false
	}
	for i := range host.Endpoints {
		if len(host.Endpoints[i].Vulnerabilities()) > 0 {
			return true
		}
	}
	return false
}

// nowUTC is overridable in tests.
var nowUTC = func() time.Time { return time.Now().UTC() }
