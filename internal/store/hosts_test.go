package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/t0mer/bothan/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "bothan.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestHostRepo_CreateAndGet(t *testing.T) {
	repo := newTestStore(t).Hosts()
	ctx := context.Background()

	maxAge := 24
	h := &model.Host{
		Hostname:       "example.com",
		Enabled:        true,
		Publish:        false,
		IgnoreMismatch: true,
		FromCache:      true,
		MaxAgeHours:    &maxAge,
		Notes:          "primary site",
	}
	if err := repo.Create(ctx, h); err != nil {
		t.Fatalf("create: %v", err)
	}
	if h.ID == 0 {
		t.Fatal("create did not set ID")
	}
	if h.CreatedAt.IsZero() || h.UpdatedAt.IsZero() {
		t.Errorf("timestamps not populated: created=%v updated=%v", h.CreatedAt, h.UpdatedAt)
	}

	got, err := repo.Get(ctx, h.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Hostname != "example.com" || !got.Enabled || got.Publish ||
		!got.IgnoreMismatch || !got.FromCache || got.Notes != "primary site" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if got.MaxAgeHours == nil || *got.MaxAgeHours != 24 {
		t.Errorf("max_age_hours = %v, want 24", got.MaxAgeHours)
	}
}

func TestHostRepo_DuplicateHostname(t *testing.T) {
	repo := newTestStore(t).Hosts()
	ctx := context.Background()

	if err := repo.Create(ctx, &model.Host{Hostname: "dup.com"}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	err := repo.Create(ctx, &model.Host{Hostname: "dup.com"})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("second create err = %v, want ErrConflict", err)
	}
}

func TestHostRepo_GetNotFound(t *testing.T) {
	repo := newTestStore(t).Hosts()
	_, err := repo.Get(context.Background(), 12345)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("get missing err = %v, want ErrNotFound", err)
	}
}

func TestHostRepo_ListOrdered(t *testing.T) {
	repo := newTestStore(t).Hosts()
	ctx := context.Background()
	for _, name := range []string{"zeta.com", "alpha.com", "mid.com"} {
		if err := repo.Create(ctx, &model.Host{Hostname: name}); err != nil {
			t.Fatal(err)
		}
	}
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []string{"alpha.com", "mid.com", "zeta.com"}
	if len(list) != len(want) {
		t.Fatalf("got %d hosts, want %d", len(list), len(want))
	}
	for i, h := range list {
		if h.Hostname != want[i] {
			t.Errorf("list[%d] = %q, want %q", i, h.Hostname, want[i])
		}
	}
}

func TestHostRepo_Update(t *testing.T) {
	repo := newTestStore(t).Hosts()
	ctx := context.Background()

	h := &model.Host{Hostname: "old.com", Enabled: true}
	if err := repo.Create(ctx, h); err != nil {
		t.Fatal(err)
	}

	h.Hostname = "new.com"
	h.Notes = "renamed"
	h.Publish = true
	if err := repo.Update(ctx, h); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.Get(ctx, h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Hostname != "new.com" || got.Notes != "renamed" || !got.Publish {
		t.Errorf("update not persisted: %+v", got)
	}
}

func TestHostRepo_UpdateMissing(t *testing.T) {
	repo := newTestStore(t).Hosts()
	err := repo.Update(context.Background(), &model.Host{ID: 999, Hostname: "x.com"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("update missing err = %v, want ErrNotFound", err)
	}
}

func TestHostRepo_SetEnabledAndDelete(t *testing.T) {
	repo := newTestStore(t).Hosts()
	ctx := context.Background()

	h := &model.Host{Hostname: "toggle.com", Enabled: true}
	if err := repo.Create(ctx, h); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetEnabled(ctx, h.ID, false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	got, _ := repo.Get(ctx, h.ID)
	if got.Enabled {
		t.Error("host still enabled after SetEnabled(false)")
	}

	if err := repo.Delete(ctx, h.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(ctx, h.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("host still present after delete: %v", err)
	}
	if err := repo.Delete(ctx, h.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete missing err = %v, want ErrNotFound", err)
	}
}
