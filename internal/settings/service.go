package settings

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"sync"
	"sync/atomic"

	"github.com/t0mer/bothan/internal/config"
)

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Repo is the persistence contract the service needs.
type Repo interface {
	GetAll(ctx context.Context) (map[string]string, error)
	SeedDefaults(ctx context.Context, defaults map[string]string) error
	SetMany(ctx context.Context, values map[string]string) error
}

// Service owns the current settings snapshot and mediates updates. The snapshot
// is swapped atomically so concurrent readers always see a consistent value.
type Service struct {
	repo      Repo
	bootstrap config.Bootstrap

	mu   sync.Mutex        // guards raw + persistence
	raw  map[string]string // last-known key/value state
	snap atomic.Pointer[Settings]

	onChange []func(*Settings)
}

// New seeds defaults, loads the current settings, and builds the initial
// snapshot.
func New(ctx context.Context, repo Repo, bootstrap config.Bootstrap) (*Service, error) {
	if err := repo.SeedDefaults(ctx, Defaults()); err != nil {
		return nil, fmt.Errorf("seeding settings: %w", err)
	}
	raw, err := repo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading settings: %w", err)
	}

	// Generate a persistent session-signing secret on first boot so sessions
	// survive restarts. It is never returned by the API.
	if raw[KeyAuthSessionSecret] == "" {
		secret, err := randomHex(32)
		if err != nil {
			return nil, err
		}
		if err := repo.SetMany(ctx, map[string]string{KeyAuthSessionSecret: secret}); err != nil {
			return nil, fmt.Errorf("persisting session secret: %w", err)
		}
		raw[KeyAuthSessionSecret] = secret
	}

	snap, err := build(raw)
	if err != nil {
		return nil, fmt.Errorf("building settings snapshot: %w", err)
	}

	s := &Service{repo: repo, bootstrap: bootstrap, raw: raw}
	s.snap.Store(snap)
	return s, nil
}

// Current returns the active settings snapshot (never nil after New).
func (s *Service) Current() *Settings { return s.snap.Load() }

// Raw returns the last-known raw string value for a key (e.g. to echo a
// user-typed duration back unchanged). Empty if unset.
func (s *Service) Raw(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.raw[key]
}

// Bootstrap returns the env/flag bootstrap configuration.
func (s *Service) Bootstrap() config.Bootstrap { return s.bootstrap }

// OnChange registers a callback invoked with the new snapshot after each
// successful update (e.g. to re-apply the log level live).
func (s *Service) OnChange(fn func(*Settings)) { s.onChange = append(s.onChange, fn) }

// Update validates a partial key/value patch against the current state,
// persists the changed keys, and swaps in the new snapshot. It is all-or-nothing.
func (s *Service) Update(ctx context.Context, patch map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	merged := maps.Clone(s.raw)
	for k, v := range patch {
		merged[k] = v
	}
	snap, err := build(merged)
	if err != nil {
		return err
	}
	if err := s.repo.SetMany(ctx, patch); err != nil {
		return fmt.Errorf("persisting settings: %w", err)
	}
	s.raw = merged
	s.snap.Store(snap)

	for _, fn := range s.onChange {
		fn(snap)
	}
	return nil
}

// EffectiveBind returns the host and port to listen on, applying the bootstrap
// overrides (which win over the stored settings so a container can pin its bind).
func (s *Service) EffectiveBind() (host string, port int) {
	cur := s.Current()
	host, port = cur.Server.Host, cur.Server.Port
	if s.bootstrap.ServerHostSet {
		host = s.bootstrap.ServerHost
	}
	if s.bootstrap.ServerPortSet {
		port = s.bootstrap.ServerPort
	}
	return host, port
}

// EnvOverriddenBind reports which server bind fields are pinned by the
// environment (and therefore not effective from the Settings page until unset).
func (s *Service) EnvOverriddenBind() []string {
	var out []string
	if s.bootstrap.ServerHostSet {
		out = append(out, "host")
	}
	if s.bootstrap.ServerPortSet {
		out = append(out, "port")
	}
	return out
}
