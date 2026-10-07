package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is TypeSafe's own host; BaseURL exists for a proxy or a
	// test server.
	DefaultBaseURL = "https://api.typesafe.ai"

	// DefaultModel is the current System One model name. JEV is early access, so
	// a caller may override it without a code change.
	DefaultModel = "jev-latest"

	// endpoint is the TypeSafe System One surface. It is the path a client
	// speaks when its Config names no endpoint, so a client built before the
	// Decisions surface existed keeps hitting it with no code change.
	endpoint = "/v1/systemone"

	// decisionsPath is the OpenRouter Decisions surface, the route that also
	// serves Cloudflare's clef.
	decisionsPath = "/api/alpha/decisions"

	defaultTimeout  = 10 * time.Second
	defaultAttempts = 4
	defaultBackoff  = 250 * time.Millisecond

	// maxBody caps what one answer may be read into memory; a JEV response is
	// small, so anything larger is a proxy error page, not an answer.
	maxBody = 1 << 20
)

// Endpoint names the API surface a client speaks. A zero value is System One,
// so a Config that never mentions it talks to TypeSafe exactly as before.
type Endpoint string

const (
	// EndpointSystemOne speaks POST {base}/v1/systemone, TypeSafe's own
	// surface. It is the default.
	EndpointSystemOne Endpoint = "systemone"

	// EndpointDecisions speaks POST {base}/api/alpha/decisions, the OpenRouter
	// surface that also serves Cloudflare's clef.
	EndpointDecisions Endpoint = "decisions"
)

// Config is one client's settings. Empty fields take the defaults; APIKey falls
// back to TYPESAFE_API_KEY when it is unset, so a key already exported for the
// vendor needs no second copy. Endpoint selects the API surface; BaseURL is the
// host the chosen surface is reached at, so a Decisions client points at
// https://openrouter.ai and a System One client at https://api.typesafe.ai.
type Config struct {
	BaseURL  string
	APIKey   string
	Model    string
	Endpoint Endpoint
	Timeout  time.Duration
	Attempts int
	Backoff  time.Duration
}

// Client is a decision client. It is safe for concurrent use; the judge only
// ever calls it from one pass goroutine.
type Client struct {
	baseURL  string
	apiKey   string
	model    string
	endpoint Endpoint
	http     *http.Client
	attempts int
	backoff  time.Duration
}

// applyDefaults fills unset client settings from defaults.
func applyDefaults(c *Config) {
	switch {
	case c.BaseURL != "":
	case c.Endpoint == EndpointDecisions:
		c.BaseURL = "https://openrouter.ai"
	default:
		c.BaseURL = DefaultBaseURL
	}
	if c.Model == "" {
		c.Model = DefaultModel
	}
	switch {
	case c.APIKey != "":
	case c.Endpoint == EndpointDecisions:
		c.APIKey = os.Getenv("OPENROUTER_API_KEY")
	default:
		c.APIKey = os.Getenv("TYPESAFE_API_KEY")
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
	if c.Attempts <= 0 {
		c.Attempts = defaultAttempts
	}
	if c.Backoff <= 0 {
		c.Backoff = defaultBackoff
	}
}

// New builds a client, filling every unset field from the defaults. A Config
// that carries no Endpoint speaks System One on DefaultBaseURL, byte-identical
// to a client built before the second surface existed.
func New(c Config) *Client {
	applyDefaults(&c)
	return &Client{
		baseURL:  strings.TrimRight(c.BaseURL, "/"),
		apiKey:   c.APIKey,
		model:    c.Model,
		endpoint: c.Endpoint,
		http:     &http.Client{Timeout: c.Timeout},
		attempts: c.Attempts,
		backoff:  c.Backoff,
	}
}

// Evaluate sends one state and its questions and returns every answer, all of
// them computed against the same state in a single call. A transient failure —
// a 429, a 529, a gateway that never reached the endpoint, or a transport that
// never answered — is retried with exponential backoff up to Attempts times;
// anything else comes back as its typed error. The caller's context bounds all
// of it: once it is spent the loop stops rather than spending what is left of
// the attempt budget on requests that are already out of time.
//
// At least one attempt always happens, whatever Attempts says: a zero-value
// Client must come back with a transport error, never with an empty response
// that a caller would read as a successful call with no answers.
func (c *Client) Evaluate(ctx context.Context, state any, questions map[string]Question) (Response, error) {
	payload, err := json.Marshal(request{State: state, Model: c.model, Questions: questions})
	if err != nil {
		return Response{}, err
	}
	attempts := max(c.attempts, 1)
	var last error
	for attempt := range attempts {
		response, err := c.post(ctx, payload)
		if err == nil {
			return response, nil
		}
		last = err
		if !retryable(err) || attempt == attempts-1 || ctx.Err() != nil {
			break
		}
		if err := c.pause(ctx, attempt); err != nil {
			return Response{}, err
		}
	}
	return Response{}, last
}

// post is one attempt: one request, one read, one decoded response or typed
// error. The bearer key is the only credential this package ever touches and is
// never logged.
func (c *Client) post(ctx context.Context, payload []byte) (Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url(), bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	res, err := c.doer().Do(req)
	if err != nil {
		return Response{}, err
	}
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
	if err != nil {
		return Response{}, err
	}
	if res.StatusCode != http.StatusOK {
		return Response{}, errorFor(c.endpoint, res.StatusCode, strings.TrimSpace(string(body)))
	}

	var out Response
	if err := json.Unmarshal(body, &out); err != nil {
		return Response{}, &HTTPError{Status: res.StatusCode, Detail: "malformed response: " + err.Error()}
	}
	return out, nil
}

// doer is the HTTP client one attempt goes through: the configured one, or the
// default for a Client that was not built by New. Without the fallback a
// zero-value Client is a nil dereference rather than an error, which is the
// worst way for a caller to find out it skipped the constructor.
func (c *Client) doer() *http.Client {
	if c.http == nil {
		return http.DefaultClient
	}
	return c.http
}

// path returns the request path for this client's chosen surface. A zero-value
// endpoint is System One, so a Client built before the second surface existed
// keeps hitting the old path with no code change.
func (c *Client) path() string {
	if c.endpoint == EndpointDecisions {
		return decisionsPath
	}
	return endpoint
}

// url builds the request URL, ensuring that a BaseURL ending with /api does not
// duplicate the /api prefix of the Decisions route.
func (c *Client) url() string {
	base := strings.TrimRight(c.baseURL, "/")
	p := c.path()
	if strings.HasSuffix(base, "/api") && strings.HasPrefix(p, "/api/") {
		base = strings.TrimSuffix(base, "/api")
	}
	return base + p
}

// pause waits out one backoff interval, doubling per attempt, and gives up the
// moment the caller's context does.
func (c *Client) pause(ctx context.Context, attempt int) error {
	shift := min(attempt, 30)
	timer := time.NewTimer(c.backoff << shift)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
