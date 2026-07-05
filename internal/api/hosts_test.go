package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/t0mer/bothan/internal/model"
	"github.com/t0mer/bothan/internal/store"
)

// fakeRepo is an in-memory HostRepo for handler tests.
type fakeRepo struct {
	hosts  map[int64]*model.Host
	nextID int64
	names  map[string]int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{hosts: map[int64]*model.Host{}, names: map[string]int64{}, nextID: 0}
}

func (f *fakeRepo) Create(_ context.Context, h *model.Host) error {
	if _, ok := f.names[h.Hostname]; ok {
		return store.ErrConflict
	}
	f.nextID++
	h.ID = f.nextID
	f.hosts[h.ID] = h
	f.names[h.Hostname] = h.ID
	return nil
}
func (f *fakeRepo) Get(_ context.Context, id int64) (*model.Host, error) {
	h, ok := f.hosts[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *h
	return &cp, nil
}
func (f *fakeRepo) List(_ context.Context) ([]model.Host, error) {
	out := []model.Host{}
	for _, h := range f.hosts {
		out = append(out, *h)
	}
	return out, nil
}
func (f *fakeRepo) Update(_ context.Context, h *model.Host) error {
	if _, ok := f.hosts[h.ID]; !ok {
		return store.ErrNotFound
	}
	if other, ok := f.names[h.Hostname]; ok && other != h.ID {
		return store.ErrConflict
	}
	f.hosts[h.ID] = h
	f.names[h.Hostname] = h.ID
	return nil
}
func (f *fakeRepo) SetEnabled(_ context.Context, id int64, enabled bool) error {
	h, ok := f.hosts[id]
	if !ok {
		return store.ErrNotFound
	}
	h.Enabled = enabled
	return nil
}
func (f *fakeRepo) Delete(_ context.Context, id int64) error {
	if _, ok := f.hosts[id]; !ok {
		return store.ErrNotFound
	}
	delete(f.hosts, id)
	return nil
}

func newRouter(repo HostRepo) http.Handler {
	r := chi.NewRouter()
	r.Route("/hosts", NewHosts(HostsDeps{Repo: repo, DefaultPublish: func() bool { return false }}).Routes)
	return r
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestCreateHost_Defaults(t *testing.T) {
	h := newRouter(newFakeRepo())
	rec := do(t, h, http.MethodPost, "/hosts", `{"hostname":"example.com"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var got model.Host
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got.ID == 0 || got.Hostname != "example.com" {
		t.Errorf("bad create response: %+v", got)
	}
	if !got.Enabled {
		t.Error("enabled should default to true")
	}
	if got.Publish {
		t.Error("publish should default to false (private)")
	}
}

func TestCreateHost_ValidationErrors(t *testing.T) {
	h := newRouter(newFakeRepo())
	cases := map[string]string{
		"empty hostname": `{"hostname":"  "}`,
		"has scheme":     `{"hostname":"https://example.com"}`,
		"has port":       `{"hostname":"example.com:443"}`,
		"unknown field":  `{"hostname":"example.com","bogus":1}`,
		"bad max age":    `{"hostname":"example.com","from_cache":true,"max_age_hours":0}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := do(t, h, http.MethodPost, "/hosts", body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400; body=%s", rec.Code, rec.Body)
			}
		})
	}
}

func TestCreateHost_Duplicate(t *testing.T) {
	h := newRouter(newFakeRepo())
	do(t, h, http.MethodPost, "/hosts", `{"hostname":"dup.com"}`)
	rec := do(t, h, http.MethodPost, "/hosts", `{"hostname":"dup.com"}`)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestGetHost_NotFound(t *testing.T) {
	h := newRouter(newFakeRepo())
	rec := do(t, h, http.MethodGet, "/hosts/999", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestEnableDisableLifecycle(t *testing.T) {
	repo := newFakeRepo()
	h := newRouter(repo)
	do(t, h, http.MethodPost, "/hosts", `{"hostname":"toggle.com"}`)

	rec := do(t, h, http.MethodPost, "/hosts/1/disable", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("disable status = %d", rec.Code)
	}
	var got model.Host
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Enabled {
		t.Error("host should be disabled")
	}

	rec = do(t, h, http.MethodPost, "/hosts/1/enable", "")
	json.Unmarshal(rec.Body.Bytes(), &got)
	if !got.Enabled {
		t.Error("host should be enabled")
	}
}

func TestUpdateAndDelete(t *testing.T) {
	h := newRouter(newFakeRepo())
	do(t, h, http.MethodPost, "/hosts", `{"hostname":"old.com"}`)

	rec := do(t, h, http.MethodPut, "/hosts/1", `{"hostname":"new.com","publish":true,"notes":"x"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d; body=%s", rec.Code, rec.Body)
	}
	var got model.Host
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Hostname != "new.com" || !got.Publish || got.Notes != "x" {
		t.Errorf("update not applied: %+v", got)
	}

	rec = do(t, h, http.MethodDelete, "/hosts/1", "")
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", rec.Code)
	}
	rec = do(t, h, http.MethodGet, "/hosts/1", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", rec.Code)
	}
}
