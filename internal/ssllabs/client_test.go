package ssllabs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/info" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(Info{MaxAssessments: 25, CurrentAssessments: 2, NewAssessmentCoolOff: 1000})
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	info, err := c.Info(context.Background())
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.MaxAssessments != 25 || info.CurrentAssessments != 2 {
		t.Errorf("info = %+v", info)
	}
}

func TestAnalyze_QueryParams(t *testing.T) {
	var gotQuery string
	var gotEmail string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		gotEmail = r.Header.Get("email")
		json.NewEncoder(w).Encode(Host{Status: "READY", Endpoints: []Endpoint{{Grade: "A+"}}})
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, APIVersion: "v4", Email: "ops@example.com"})
	host, err := c.Analyze(context.Background(), AnalyzeParams{
		Host: "example.com", StartNew: true, All: true, Publish: false, IgnoreMismatch: true,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if !host.IsReady() || host.Endpoints[0].Grade != "A+" {
		t.Errorf("host = %+v", host)
	}
	for _, want := range []string{"host=example.com", "startNew=on", "all=done", "publish=off", "ignoreMismatch=on"} {
		if !contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
	if gotEmail != "ops@example.com" {
		t.Errorf("email header = %q, want ops@example.com", gotEmail)
	}
}

func TestAnalyze_FromCacheExcludesStartNew(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		json.NewEncoder(w).Encode(Host{Status: "READY"})
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	_, err := c.Analyze(context.Background(), AnalyzeParams{Host: "x.com", FromCache: true, MaxAgeHours: 12})
	if err != nil {
		t.Fatal(err)
	}
	if contains(gotQuery, "startNew") {
		t.Errorf("fromCache query should not include startNew: %q", gotQuery)
	}
	if !contains(gotQuery, "fromCache=on") || !contains(gotQuery, "maxAge=12") {
		t.Errorf("query %q missing fromCache/maxAge", gotQuery)
	}
}

func TestAPIError_Classification(t *testing.T) {
	for code, check := range map[int]func(*APIError) bool{
		429: (*APIError).IsRateLimited,
		503: (*APIError).IsUnavailable,
		529: (*APIError).IsOverloaded,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
			w.Write([]byte("nope"))
		}))
		c := New(Options{BaseURL: srv.URL})
		_, err := c.Info(context.Background())
		srv.Close()
		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("code %d: expected *APIError, got %T", code, err)
		}
		if apiErr.StatusCode != code || !check(apiErr) {
			t.Errorf("code %d: classification failed (%+v)", code, apiErr)
		}
	}
}

func TestRegister(t *testing.T) {
	var got RegisterRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/register" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, APIVersion: "v4"})
	err := c.Register(context.Background(), RegisterRequest{FirstName: "Ada", Email: "ada@example.com", Organization: "ACME"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if got.Email != "ada@example.com" || got.FirstName != "Ada" {
		t.Errorf("register body = %+v", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
