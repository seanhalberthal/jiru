package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AuthType represents the authentication method.
type AuthType int

const (
	AuthBasic  AuthType = iota // Basic auth (user:token)
	AuthBearer                 // Bearer token (PAT)
)

// Config holds the configuration for creating an API client.
type Config struct {
	BaseURL  string        // e.g., "https://my-domain.atlassian.net"
	Username string        // Jira username (for Basic auth)
	Token    string        // API token or PAT
	Auth     AuthType      // Authentication method
	Timeout  time.Duration // HTTP timeout (default: 30s)
}

// Client is a thin HTTP client for the Jira REST API.
type Client struct {
	http    *http.Client
	baseURL string
	auth    string // Pre-computed Authorization header value
}

// New creates a new API client.
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	var auth string
	switch cfg.Auth {
	case AuthBearer:
		auth = "Bearer " + cfg.Token
	default:
		encoded := base64.StdEncoding.EncodeToString(
			[]byte(cfg.Username + ":" + cfg.Token),
		)
		auth = "Basic " + encoded
	}

	return &Client{
		http:    &http.Client{Timeout: timeout},
		baseURL: cfg.BaseURL,
		auth:    auth,
	}
}

// Get performs a GET request and returns the raw response.
// The caller is responsible for closing the response body.
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	return c.do(ctx, http.MethodGet, path, nil)
}

// Post performs a POST request with a JSON body.
func (c *Client) Post(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.do(ctx, http.MethodPost, path, body)
}

// Put performs a PUT request with a JSON body.
func (c *Client) Put(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.do(ctx, http.MethodPut, path, body)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string) (*http.Response, error) {
	return c.do(ctx, http.MethodDelete, path, nil)
}

// V1 returns the Agile v1 base path prefix.
func V1(path string) string { return "/rest/agile/1.0" + path }

// V2 returns the REST API v2 base path prefix.
func V2(path string) string { return "/rest/api/2" + path }

// V3 returns the REST API v3 base path prefix.
func V3(path string) string { return "/rest/api/3" + path }

// Wiki returns the Confluence Cloud v2 API base path prefix.
func Wiki(path string) string { return "/wiki/api/v2" + path }

// WikiV1 returns the Confluence Cloud v1 API base path prefix (needed for CQL search).
func WikiV1(path string) string { return "/wiki/rest/api" + path }

func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}

	return resp, nil
}

const maxResponseBody = 50 << 20 // 50 MiB

// DecodeResponse reads and decodes a JSON response body into the target.
// Returns an error if the status code is not in the 2xx range.
// Closes the response body. The body is capped at 50 MiB to guard against
// unbounded responses from misconfigured proxies.
func DecodeResponse[T any](resp *http.Response) (*T, error) {
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result T
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBody)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// CheckResponse verifies the response has a 2xx status code.
// Closes the response body. Use for requests where the body is not needed.
func CheckResponse(resp *http.Response) error {
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
