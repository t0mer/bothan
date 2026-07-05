// Package scanner runs SSL Labs assessments: it triggers a scan for a host,
// polls until the assessment is ready (respecting rate limits and cool-off),
// persists the result and per-endpoint grades, and computes the overall grade.
//
// Phase 3 provides manual/API-triggered scans over a bounded executor; the
// cron scheduler and full worker pool build on this in Phase 4.
package scanner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/t0mer/bothan/internal/model"
	"github.com/t0mer/bothan/internal/settings"
	"github.com/t0mer/bothan/internal/ssllabs"
	"github.com/t0mer/bothan/internal/store"
)

// ErrScanInProgress is returned when a host already has a pending/running scan.
var ErrScanInProgress = errors.New("a scan is already in progress for this host")

// ErrNoEmail is returned when v4 is selected but no registered email is set.
var ErrNoEmail = errors.New("SSL Labs v4 requires a registered email; register or switch to v3")

// Analyzer is the SSL Labs behaviour the scanner needs (satisfied by
// *ssllabs.Client; faked in tests).
type Analyzer interface {
	Info(ctx context.Context) (*ssllabs.Info, error)
	AnalyzeRaw(ctx context.Context, p ssllabs.AnalyzeParams) (*ssllabs.Host, []byte, error)
}

// Factory builds an Analyzer from the current settings, or errors if the
// configuration forbids scanning (e.g. v4 without a registered email).
type Factory func(*settings.Settings) (Analyzer, error)

// DefaultFactory builds a real SSL Labs client from settings. baseURL overrides
// the API base (empty uses the default for the configured version).
func DefaultFactory(baseURL string, observe func(int)) Factory {
	return func(s *settings.Settings) (Analyzer, error) {
		sl := s.SSLLabs
		if sl.APIVersion == "v4" && sl.Email == "" {
			return nil, ErrNoEmail
		}
		return ssllabs.New(ssllabs.Options{
			APIVersion: sl.APIVersion,
			Email:      sl.Email,
			BaseURL:    baseURL,
			Observe:    observe,
		}), nil
	}
}

// Backoff configures sleeps for SSL Labs HTTP status handling (§5).
type Backoff struct {
	RateLimited        time.Duration // 429 base (exponential)
	Unavailable        time.Duration // 503
	Overloaded         time.Duration // 529
	Max                time.Duration // cap
	ServerErrorRetries int           // 500 bounded retries
}

// DefaultBackoff returns the contract's discipline.
func DefaultBackoff() Backoff {
	return Backoff{
		RateLimited:        5 * time.Second,
		Unavailable:        15 * time.Minute,
		Overloaded:         30 * time.Minute,
		Max:                30 * time.Minute,
		ServerErrorRetries: 3,
	}
}

// Service orchestrates scans.
type Service struct {
	scans    *store.ScanRepo
	hosts    *store.HostRepo
	settings *settings.Service
	factory  Factory
	logger   *slog.Logger
	backoff  Backoff

	sem chan struct{}
	wg  sync.WaitGroup

	mu         sync.Mutex
	rng        *rand.Rand
	onComplete func(context.Context, *model.Scan)
}

// Options configures the scanner Service.
type Options struct {
	Store    *store.Store
	Settings *settings.Service
	Factory  Factory
	Logger   *slog.Logger
	Backoff  *Backoff
	MaxSlots int // hard cap on concurrent executions (default 16)
}

// New builds a scanner Service.
func New(opts Options) *Service {
	slots := opts.MaxSlots
	if slots <= 0 {
		slots = 16
	}
	bo := DefaultBackoff()
	if opts.Backoff != nil {
		bo = *opts.Backoff
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		scans:    opts.Store.Scans(),
		hosts:    opts.Store.Hosts(),
		settings: opts.Settings,
		factory:  opts.Factory,
		logger:   logger,
		backoff:  bo,
		sem:      make(chan struct{}, slots),
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// OnComplete registers a callback fired after a scan finishes (ready or error),
// used by the rules engine in Phase 5.
func (s *Service) OnComplete(fn func(context.Context, *model.Scan)) { s.onComplete = fn }

// Trigger creates a pending scan for a host and starts it asynchronously. It
// returns ErrScanInProgress if one is already active, or store.ErrNotFound if
// the host does not exist.
func (s *Service) Trigger(ctx context.Context, hostID int64, trigger string) (*model.Scan, error) {
	host, err := s.hosts.Get(ctx, hostID)
	if err != nil {
		return nil, err
	}
	// Validate the configuration up front so callers get a synchronous error
	// (e.g. v4 without a registered email) instead of an async scan failure.
	if _, err := s.factory(s.settings.Current()); err != nil {
		return nil, err
	}
	active, err := s.scans.HasActive(ctx, hostID)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, ErrScanInProgress
	}
	sc := &model.Scan{HostID: hostID, Status: model.ScanStatusPending, Trigger: trigger}
	if err := s.scans.Create(ctx, sc); err != nil {
		return nil, err
	}
	s.dispatch(*host, sc.ID)
	return sc, nil
}

// Resume re-dispatches an already-persisted pending scan (restart recovery).
func (s *Service) Resume(host model.Host, scanID int64) { s.dispatch(host, scanID) }

// Wait blocks until all in-flight scans finish (used during shutdown).
func (s *Service) Wait() { s.wg.Wait() }

func (s *Service) dispatch(host model.Host, scanID int64) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.sem <- struct{}{}
		defer func() { <-s.sem }()
		s.execute(host, scanID)
	}()
}

func (s *Service) execute(host model.Host, scanID int64) {
	cfg := s.settings.Current()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.SSLLabs.ScanTimeout)
	defer cancel()

	analyzer, err := s.factory(cfg)
	if err != nil {
		s.fail(ctx, scanID, err)
		return
	}
	if err := s.scans.SetStatus(ctx, scanID, model.ScanStatusRunning); err != nil {
		s.logger.Error("marking scan running", slog.Int64("scan", scanID), slog.String("error", err.Error()))
	}

	host2, raw, err := s.assess(ctx, analyzer, host, cfg)
	if err != nil {
		s.fail(ctx, scanID, err)
		return
	}
	s.persist(ctx, scanID, host2, raw)
}

// assess runs the start + poll loop until READY/ERROR, timeout, or a fatal error.
func (s *Service) assess(ctx context.Context, a Analyzer, host model.Host, cfg *settings.Settings) (*ssllabs.Host, []byte, error) {
	params := ssllabs.AnalyzeParams{
		Host:           host.Hostname,
		StartNew:       !host.FromCache,
		FromCache:      host.FromCache,
		MaxAgeHours:    derefInt(host.MaxAgeHours),
		All:            true,
		Publish:        host.Publish,
		IgnoreMismatch: host.IgnoreMismatch,
	}

	if params.StartNew {
		if err := s.awaitCapacity(ctx, a); err != nil {
			return nil, nil, err
		}
	}

	result, raw, err := s.analyzeWithBackoff(ctx, a, params)
	if err != nil {
		return nil, nil, err
	}

	// Subsequent polls: plain analyze (no startNew/fromCache).
	params.StartNew = false
	params.FromCache = false

	ticker := time.NewTicker(cfg.SSLLabs.PollInterval)
	defer ticker.Stop()

	for !result.IsReady() {
		if result.IsError() {
			return nil, nil, fmt.Errorf("assessment error: %s", statusOr(result.StatusMessage, "ERROR"))
		}
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("scan timed out: %w", ctx.Err())
		case <-ticker.C:
		}
		result, raw, err = s.analyzeWithBackoff(ctx, a, params)
		if err != nil {
			return nil, nil, err
		}
	}
	return result, raw, nil
}

// awaitCapacity blocks until SSL Labs reports free assessment slots.
func (s *Service) awaitCapacity(ctx context.Context, a Analyzer) error {
	for {
		info, err := a.Info(ctx)
		if err != nil {
			// Treat info failures as transient; back off briefly and retry.
			if !s.sleep(ctx, s.jitter(s.backoff.RateLimited)) {
				return ctx.Err()
			}
			continue
		}
		if info.CurrentAssessments < info.MaxAssessments || info.MaxAssessments == 0 {
			// Honour cool-off between starting new assessments.
			if info.NewAssessmentCoolOff > 0 {
				_ = s.sleep(ctx, time.Duration(info.NewAssessmentCoolOff)*time.Millisecond)
			}
			return nil
		}
		if !s.sleep(ctx, s.jitter(s.backoff.RateLimited)) {
			return ctx.Err()
		}
	}
}

// analyzeWithBackoff performs one analyze call, retrying on transient HTTP
// statuses with the configured back-off.
func (s *Service) analyzeWithBackoff(ctx context.Context, a Analyzer, p ssllabs.AnalyzeParams) (*ssllabs.Host, []byte, error) {
	attempt := 0
	serverErrors := 0
	for {
		host, raw, err := a.AnalyzeRaw(ctx, p)
		if err == nil {
			return host, raw, nil
		}

		var apiErr *ssllabs.APIError
		if !errors.As(err, &apiErr) {
			return nil, nil, err
		}

		var wait time.Duration
		switch {
		case apiErr.IsRateLimited():
			attempt++
			wait = s.expBackoff(attempt)
		case apiErr.IsUnavailable():
			wait = s.jitter(s.backoff.Unavailable)
		case apiErr.IsOverloaded():
			wait = s.jitter(s.backoff.Overloaded)
		case apiErr.StatusCode >= 500:
			serverErrors++
			if serverErrors > s.backoff.ServerErrorRetries {
				return nil, nil, fmt.Errorf("giving up after %d server errors: %w", serverErrors, err)
			}
			attempt++
			wait = s.expBackoff(attempt)
		default:
			return nil, nil, err
		}
		s.logger.Warn("ssllabs back-off",
			slog.Int("status", apiErr.StatusCode),
			slog.Duration("wait", wait),
			slog.String("host", p.Host))
		if !s.sleep(ctx, wait) {
			return nil, nil, ctx.Err()
		}
	}
}

func (s *Service) persist(ctx context.Context, scanID int64, host *ssllabs.Host, raw []byte) {
	sc := &model.Scan{
		ID:              scanID,
		Status:          model.ScanStatusReady,
		OverallGrade:    overallGrade(host),
		EngineVersion:   host.EngineVersion,
		CriteriaVersion: host.CriteriaVersion,
		Endpoints:       mapEndpoints(host),
	}
	if err := s.scans.SaveResult(ctx, sc, raw); err != nil {
		s.logger.Error("saving scan result", slog.Int64("scan", scanID), slog.String("error", err.Error()))
		s.fail(ctx, scanID, fmt.Errorf("persisting result: %w", err))
		return
	}
	s.logger.Info("scan complete",
		slog.Int64("scan", scanID),
		slog.String("host", host.Host),
		slog.String("grade", sc.OverallGrade))
	s.notifyComplete(ctx, scanID)
}

func (s *Service) fail(ctx context.Context, scanID int64, cause error) {
	s.logger.Warn("scan failed", slog.Int64("scan", scanID), slog.String("error", cause.Error()))
	if err := s.scans.Fail(ctx, scanID, cause.Error()); err != nil {
		s.logger.Error("recording scan failure", slog.Int64("scan", scanID), slog.String("error", err.Error()))
	}
	s.notifyComplete(ctx, scanID)
}

func (s *Service) notifyComplete(ctx context.Context, scanID int64) {
	if s.onComplete == nil {
		return
	}
	sc, err := s.scans.Get(ctx, scanID)
	if err != nil {
		return
	}
	s.onComplete(ctx, sc)
}

// --- back-off helpers ---

func (s *Service) expBackoff(attempt int) time.Duration {
	d := s.backoff.RateLimited << (attempt - 1)
	if d > s.backoff.Max || d <= 0 {
		d = s.backoff.Max
	}
	return s.jitter(d)
}

func (s *Service) jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	s.mu.Lock()
	f := 0.5 + s.rng.Float64() // 0.5x .. 1.5x
	s.mu.Unlock()
	return time.Duration(float64(d) * f)
}

// sleep returns false if the context is cancelled before the delay elapses.
func (s *Service) sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func statusOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
