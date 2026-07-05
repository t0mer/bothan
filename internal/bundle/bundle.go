// Package bundle implements Bothan's portable configuration export/import: a
// versioned JSON bundle of hosts, schedules, channels, rules, and their links,
// with three secret-handling modes (none / instance_key / passphrase).
package bundle

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/t0mer/bothan/internal/crypto"
	"github.com/t0mer/bothan/internal/model"
	"github.com/t0mer/bothan/internal/scheduler"
	"github.com/t0mer/bothan/internal/store"
)

// SchemaVersion is the current bundle format version.
const SchemaVersion = 1

// Secret-handling modes.
const (
	SecretNone        = "none"
	SecretInstanceKey = "instance_key"
	SecretPassphrase  = "passphrase"
)

// Bundle is the portable configuration document.
type Bundle struct {
	SchemaVersion    int               `json:"schema_version"`
	App              string            `json:"app"`
	AppVersion       string            `json:"app_version"`
	ExportedAt       string            `json:"exported_at"`
	SecretEncryption string            `json:"secret_encryption"`
	KeyFingerprint   string            `json:"key_fingerprint,omitempty"`
	KDF              *crypto.KDFParams `json:"kdf,omitempty"`
	Hosts            []Host            `json:"hosts"`
	Schedules        []Schedule        `json:"schedules"`
	Channels         []Channel         `json:"channels"`
	Rules            []Rule            `json:"rules"`
	Links            Links             `json:"links"`
}

// Host is a host in the bundle.
type Host struct {
	Hostname       string `json:"hostname"`
	Enabled        bool   `json:"enabled"`
	Publish        bool   `json:"publish"`
	IgnoreMismatch bool   `json:"ignore_mismatch"`
	FromCache      bool   `json:"from_cache"`
	MaxAgeHours    *int   `json:"max_age_hours,omitempty"`
	Notes          string `json:"notes,omitempty"`
}

// Schedule is a schedule in the bundle.
type Schedule struct {
	Name    string `json:"name"`
	Spec    string `json:"spec"`
	Enabled bool   `json:"enabled"`
}

// Channel is a channel in the bundle. Ciphertext is base64 and present only for
// instance_key / passphrase modes.
type Channel struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Enabled    bool   `json:"enabled"`
	Ciphertext string `json:"ciphertext,omitempty"`
}

// Rule is a rule in the bundle. Host is the hostname, empty for a global rule.
type Rule struct {
	Name           string `json:"name"`
	Host           string `json:"host,omitempty"`
	ConditionType  string `json:"condition_type"`
	ThresholdGrade string `json:"threshold_grade,omitempty"`
	ExpiryDays     *int   `json:"expiry_days,omitempty"`
	Enabled        bool   `json:"enabled"`
}

// Links records host↔schedule and host↔channel links by natural key.
type Links struct {
	HostSchedules []HostSchedule `json:"host_schedules"`
	HostChannels  []HostChannel  `json:"host_channels"`
}

// HostSchedule links a host to a schedule by name.
type HostSchedule struct {
	Host     string `json:"host"`
	Schedule string `json:"schedule"`
}

// HostChannel links a host to a channel by name.
type HostChannel struct {
	Host    string `json:"host"`
	Channel string `json:"channel"`
}

// ExportOptions configures an export.
type ExportOptions struct {
	Mode       string // none | instance_key | passphrase
	Passphrase string // required for passphrase mode
	AppVersion string
	Now        time.Time
}

// Export builds a bundle from the current configuration. cipher may be nil only
// for mode none.
func Export(ctx context.Context, st *store.Store, cipher *crypto.Cipher, opts ExportOptions) (*Bundle, error) {
	mode := opts.Mode
	if mode == "" {
		mode = SecretNone
	}
	b := &Bundle{
		SchemaVersion:    SchemaVersion,
		App:              "bothan",
		AppVersion:       opts.AppVersion,
		ExportedAt:       opts.Now.UTC().Format(time.RFC3339),
		SecretEncryption: mode,
		Links:            Links{HostSchedules: []HostSchedule{}, HostChannels: []HostChannel{}},
	}

	// A passphrase cipher for re-encrypting secrets, when requested.
	var passCipher *crypto.Cipher
	if mode == SecretInstanceKey || mode == SecretPassphrase {
		if cipher == nil {
			return nil, fmt.Errorf("encryption key is required to export secrets")
		}
	}
	switch mode {
	case SecretInstanceKey:
		b.KeyFingerprint = cipher.Fingerprint()
	case SecretPassphrase:
		if opts.Passphrase == "" {
			return nil, fmt.Errorf("passphrase is required for passphrase mode")
		}
		params, err := crypto.DefaultKDFParams()
		if err != nil {
			return nil, err
		}
		passCipher, err = crypto.NewFromPassphrase(opts.Passphrase, params)
		if err != nil {
			return nil, err
		}
		b.KDF = &params
	}

	hosts, err := st.Hosts().List(ctx)
	if err != nil {
		return nil, err
	}
	for _, h := range hosts {
		b.Hosts = append(b.Hosts, Host{
			Hostname: h.Hostname, Enabled: h.Enabled, Publish: h.Publish,
			IgnoreMismatch: h.IgnoreMismatch, FromCache: h.FromCache,
			MaxAgeHours: h.MaxAgeHours, Notes: h.Notes,
		})
		scheds, err := st.Schedules().SchedulesForHost(ctx, h.ID)
		if err != nil {
			return nil, err
		}
		for _, sc := range scheds {
			b.Links.HostSchedules = append(b.Links.HostSchedules, HostSchedule{Host: h.Hostname, Schedule: sc.Name})
		}
		chans, err := st.Channels().ChannelsForHost(ctx, h.ID)
		if err != nil {
			return nil, err
		}
		for _, ch := range chans {
			b.Links.HostChannels = append(b.Links.HostChannels, HostChannel{Host: h.Hostname, Channel: ch.Name})
		}
	}

	scheds, err := st.Schedules().List(ctx)
	if err != nil {
		return nil, err
	}
	for _, sc := range scheds {
		b.Schedules = append(b.Schedules, Schedule{Name: sc.Name, Spec: sc.Spec, Enabled: sc.Enabled})
	}

	chans, err := st.Channels().List(ctx)
	if err != nil {
		return nil, err
	}
	for _, ch := range chans {
		bc := Channel{Name: ch.Name, Type: ch.Type, Enabled: ch.Enabled}
		if mode == SecretInstanceKey {
			bc.Ciphertext = base64.StdEncoding.EncodeToString(ch.ConfigEncrypted)
		} else if mode == SecretPassphrase {
			plain, err := cipher.Decrypt(ch.ConfigEncrypted)
			if err != nil {
				return nil, fmt.Errorf("decrypting channel %q for re-encryption: %w", ch.Name, err)
			}
			reenc, err := passCipher.Encrypt(plain)
			if err != nil {
				return nil, err
			}
			bc.Ciphertext = base64.StdEncoding.EncodeToString(reenc)
		}
		b.Channels = append(b.Channels, bc)
	}

	rules, err := st.Rules().List(ctx)
	if err != nil {
		return nil, err
	}
	hostByID := map[int64]string{}
	for _, h := range hosts {
		hostByID[h.ID] = h.Hostname
	}
	for _, ru := range rules {
		br := Rule{
			Name: ru.Name, ConditionType: ru.ConditionType, ThresholdGrade: ru.ThresholdGrade,
			ExpiryDays: ru.ExpiryDays, Enabled: ru.Enabled,
		}
		if ru.HostID != nil {
			br.Host = hostByID[*ru.HostID]
		}
		b.Rules = append(b.Rules, br)
	}
	return b, nil
}

// ImportOptions configures an import.
type ImportOptions struct {
	Mode       string // merge | replace
	DryRun     bool
	Passphrase string // required for passphrase bundles
}

// Import validates a bundle, resolves its secrets for the destination key, and
// applies it (or, for a dry run, reports what would happen).
func Import(ctx context.Context, st *store.Store, cipher *crypto.Cipher, b *Bundle, opts ImportOptions) (*store.ImportReport, error) {
	if b.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("unsupported bundle schema_version %d (expected %d)", b.SchemaVersion, SchemaVersion)
	}
	mode := opts.Mode
	if mode == "" {
		mode = store.ImportMerge
	}
	if mode != store.ImportMerge && mode != store.ImportReplace {
		return nil, fmt.Errorf("invalid import mode %q", mode)
	}
	if err := validate(b); err != nil {
		return nil, err
	}

	channels, err := resolveChannels(b, cipher, opts.Passphrase)
	if err != nil {
		return nil, err
	}

	data := store.ImportData{Channels: channels}
	for _, h := range b.Hosts {
		data.Hosts = append(data.Hosts, model.Host{
			Hostname: h.Hostname, Enabled: h.Enabled, Publish: h.Publish,
			IgnoreMismatch: h.IgnoreMismatch, FromCache: h.FromCache,
			MaxAgeHours: h.MaxAgeHours, Notes: h.Notes,
		})
	}
	for _, sc := range b.Schedules {
		data.Schedules = append(data.Schedules, model.Schedule{Name: sc.Name, Spec: sc.Spec, Enabled: sc.Enabled})
	}
	for _, ru := range b.Rules {
		data.Rules = append(data.Rules, store.ImportRule{
			Name: ru.Name, Hostname: ru.Host, ConditionType: ru.ConditionType,
			ThresholdGrade: ru.ThresholdGrade, ExpiryDays: ru.ExpiryDays, Enabled: ru.Enabled,
		})
	}
	for _, l := range b.Links.HostSchedules {
		data.HostSchedules = append(data.HostSchedules, store.LinkPair{Host: l.Host, Target: l.Schedule})
	}
	for _, l := range b.Links.HostChannels {
		data.HostChannels = append(data.HostChannels, store.LinkPair{Host: l.Host, Target: l.Channel})
	}

	return st.ApplyImport(ctx, data, mode, opts.DryRun)
}

// resolveChannels turns bundle channels into store channels with ciphertext
// re-keyed to the destination instance key.
func resolveChannels(b *Bundle, cipher *crypto.Cipher, passphrase string) ([]store.ImportChannel, error) {
	out := make([]store.ImportChannel, 0, len(b.Channels))

	switch b.SecretEncryption {
	case SecretNone, "":
		for _, ch := range b.Channels {
			out = append(out, store.ImportChannel{
				Name: ch.Name, Type: ch.Type, Enabled: false, NeedsCredentials: true,
			})
		}
		return out, nil

	case SecretInstanceKey:
		if cipher == nil {
			return nil, fmt.Errorf("bundle carries secrets but no encryption key is configured")
		}
		if b.KeyFingerprint != cipher.Fingerprint() {
			return nil, fmt.Errorf("bundle was encrypted with a different key — provide the matching encryption_key, or re-export with a passphrase")
		}
		for _, ch := range b.Channels {
			blob, err := base64.StdEncoding.DecodeString(ch.Ciphertext)
			if err != nil {
				return nil, fmt.Errorf("channel %q: bad ciphertext: %w", ch.Name, err)
			}
			out = append(out, store.ImportChannel{Name: ch.Name, Type: ch.Type, Enabled: ch.Enabled, ConfigEncrypted: blob})
		}
		return out, nil

	case SecretPassphrase:
		if passphrase == "" {
			return nil, fmt.Errorf("this bundle requires a passphrase")
		}
		if b.KDF == nil {
			return nil, fmt.Errorf("passphrase bundle is missing KDF parameters")
		}
		if cipher == nil {
			return nil, fmt.Errorf("an encryption key is required to import secrets")
		}
		passCipher, err := crypto.NewFromPassphrase(passphrase, *b.KDF)
		if err != nil {
			return nil, err
		}
		for _, ch := range b.Channels {
			blob, err := base64.StdEncoding.DecodeString(ch.Ciphertext)
			if err != nil {
				return nil, fmt.Errorf("channel %q: bad ciphertext: %w", ch.Name, err)
			}
			plain, err := passCipher.Decrypt(blob)
			if err != nil {
				return nil, fmt.Errorf("channel %q: wrong passphrase or corrupt data", ch.Name)
			}
			reenc, err := cipher.Encrypt(plain)
			if err != nil {
				return nil, err
			}
			out = append(out, store.ImportChannel{Name: ch.Name, Type: ch.Type, Enabled: ch.Enabled, ConfigEncrypted: reenc})
		}
		return out, nil

	default:
		return nil, fmt.Errorf("unknown secret_encryption %q", b.SecretEncryption)
	}
}

// validate checks natural-key uniqueness, spec/grade validity, and link
// referential integrity before anything is applied.
func validate(b *Bundle) error {
	hosts := map[string]bool{}
	for _, h := range b.Hosts {
		if h.Hostname == "" {
			return fmt.Errorf("host with empty hostname")
		}
		if hosts[h.Hostname] {
			return fmt.Errorf("duplicate host %q", h.Hostname)
		}
		hosts[h.Hostname] = true
	}

	schedules := map[string]bool{}
	for _, sc := range b.Schedules {
		if schedules[sc.Name] {
			return fmt.Errorf("duplicate schedule %q", sc.Name)
		}
		schedules[sc.Name] = true
		if _, err := scheduler.NormalizeSpec(sc.Spec); err != nil {
			return fmt.Errorf("schedule %q: %w", sc.Name, err)
		}
	}

	channels := map[string]bool{}
	for _, ch := range b.Channels {
		if channels[ch.Name] {
			return fmt.Errorf("duplicate channel %q", ch.Name)
		}
		channels[ch.Name] = true
		if !validChannelType(ch.Type) {
			return fmt.Errorf("channel %q: invalid type %q", ch.Name, ch.Type)
		}
	}

	rules := map[string]bool{}
	for _, ru := range b.Rules {
		if rules[ru.Name] {
			return fmt.Errorf("duplicate rule %q", ru.Name)
		}
		rules[ru.Name] = true
		if !model.ConditionTypes[ru.ConditionType] {
			return fmt.Errorf("rule %q: invalid condition_type %q", ru.Name, ru.ConditionType)
		}
		if ru.ConditionType == model.CondGradeBelow && model.GradeRank(ru.ThresholdGrade) == model.GradeRankUnknown {
			return fmt.Errorf("rule %q: grade_below needs a valid threshold_grade", ru.Name)
		}
		if ru.Host != "" && !hosts[ru.Host] {
			return fmt.Errorf("rule %q references unknown host %q", ru.Name, ru.Host)
		}
	}

	for _, l := range b.Links.HostSchedules {
		if !hosts[l.Host] {
			return fmt.Errorf("host_schedule link references unknown host %q", l.Host)
		}
		if !schedules[l.Schedule] {
			return fmt.Errorf("host_schedule link references unknown schedule %q", l.Schedule)
		}
	}
	for _, l := range b.Links.HostChannels {
		if !hosts[l.Host] {
			return fmt.Errorf("host_channel link references unknown host %q", l.Host)
		}
		if !channels[l.Channel] {
			return fmt.Errorf("host_channel link references unknown channel %q", l.Channel)
		}
	}
	return nil
}

func validChannelType(t string) bool {
	switch t {
	case model.ChannelShoutrrr, model.ChannelWhatsAppGreenAPI, model.ChannelWhatsAppMultidevice:
		return true
	}
	return false
}
