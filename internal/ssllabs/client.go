package ssllabs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Base URLs for the supported API versions.
const (
	baseV4 = "https://api.ssllabs.com/api/v4"
	baseV3 = "https://api.ssllabs.com/api/v3"
)

// Client talks to the SSL Labs Assessment API.
type Client struct {
	baseURL    string
	apiVersion string
	email      string
	http       *http.Client
	observe    func(statusCode int)
}

// Options configures a Client.
type Options struct {
	APIVersion string       // "v4" (default) or "v3"
	Email      string       // registered email, required for v4
	BaseURL    string       // overrides the default base (for testing)
	HTTPClient *http.Client // defaults to a client with a sane timeout
	Observe    func(int)    // optional per-request status-code observer (metrics)
}

// New builds a Client from Options.
func New(opts Options) *Client {
	base := opts.BaseURL
	if base == "" {
		if opts.APIVersion == "v3" {
			base = baseV3
		} else {
			base = baseV4
		}
	}
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	version := opts.APIVersion
	if version == "" {
		version = "v4"
	}
	return &Client{
		baseURL:    base,
		apiVersion: version,
		email:      opts.Email,
		http:       hc,
		observe:    opts.Observe,
	}
}

// APIError is returned for non-2xx responses so callers can back off by code.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("ssllabs: HTTP %d: %s", e.StatusCode, e.Body)
}

// IsRateLimited reports HTTP 429 (going too fast / too many concurrent).
func (e *APIError) IsRateLimited() bool { return e.StatusCode == http.StatusTooManyRequests }

// IsUnavailable reports HTTP 503 (service not available).
func (e *APIError) IsUnavailable() bool { return e.StatusCode == http.StatusServiceUnavailable }

// IsOverloaded reports HTTP 529 (service overloaded).
func (e *APIError) IsOverloaded() bool { return e.StatusCode == 529 }

// Info fetches server availability and capacity from /info.
func (c *Client) Info(ctx context.Context) (*Info, error) {
	var info Info
	if err := c.get(ctx, "/info", nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// AnalyzeParams maps a host's configuration to /analyze query parameters.
type AnalyzeParams struct {
	Host           string
	StartNew       bool // force a fresh assessment (first call only)
	FromCache      bool // use cached result; mutually exclusive with StartNew
	MaxAgeHours    int  // with FromCache
	All            bool // include full EndpointDetails (all=done)
	Publish        bool
	IgnoreMismatch bool
}

// Analyze performs a single /analyze call and returns the Host object. The
// caller polls (StartNew on the first call, omitted thereafter) until the Host
// reaches READY or ERROR.
func (c *Client) Analyze(ctx context.Context, p AnalyzeParams) (*Host, error) {
	host, _, err := c.analyze(ctx, p.query())
	return host, err
}

// AnalyzeRaw is like Analyze but also returns the raw response body, so callers
// can persist the full Host object (including fields Bothan does not type).
func (c *Client) AnalyzeRaw(ctx context.Context, p AnalyzeParams) (*Host, []byte, error) {
	return c.analyze(ctx, p.query())
}

func (c *Client) analyze(ctx context.Context, q url.Values) (*Host, []byte, error) {
	body, err := c.getBytes(ctx, "/analyze", q)
	if err != nil {
		return nil, nil, err
	}
	var host Host
	if err := json.Unmarshal(body, &host); err != nil {
		return nil, nil, fmt.Errorf("decoding ssllabs response: %w", err)
	}
	return &host, body, nil
}

func (p AnalyzeParams) query() url.Values {
	q := url.Values{}
	q.Set("host", p.Host)
	if p.StartNew {
		q.Set("startNew", "on")
	} else if p.FromCache {
		q.Set("fromCache", "on")
		if p.MaxAgeHours > 0 {
			q.Set("maxAge", strconv.Itoa(p.MaxAgeHours))
		}
	}
	if p.All {
		q.Set("all", "done")
	}
	q.Set("publish", onOff(p.Publish))
	q.Set("ignoreMismatch", onOff(p.IgnoreMismatch))
	return q
}

// RegisterRequest is the v4 registration payload.
type RegisterRequest struct {
	FirstName    string `json:"firstName"`
	LastName     string `json:"lastName"`
	Email        string `json:"email"`
	Organization string `json:"organization"`
}

// Register performs SSL Labs v4 email registration.
func (c *Client) Register(ctx context.Context, req RegisterRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling registration: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/register", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return c.do(httpReq, nil)
}

// get issues a GET request, decoding a JSON response into out.
func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	body, err := c.getBytes(ctx, path, q)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decoding ssllabs response: %w", err)
	}
	return nil
}

// getBytes issues a GET request and returns the raw response body.
func (c *Client) getBytes(ctx context.Context, path string, q url.Values) ([]byte, error) {
	u := c.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	return c.doBytes(req)
}

// do issues a request that expects no response body (e.g. registration).
func (c *Client) do(req *http.Request, out any) error {
	body, err := c.doBytes(req)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

func (c *Client) doBytes(req *http.Request) ([]byte, error) {
	if c.apiVersion == "v4" && c.email != "" {
		req.Header.Set("email", c.email)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ssllabs request: %w", err)
	}
	defer resp.Body.Close()
	if c.observe != nil {
		c.observe(resp.StatusCode)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(body)}
	}
	return body, nil
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
