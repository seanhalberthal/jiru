package client

import (
	"net/http/httptest"

	"github.com/seanhalberthal/jiru/internal/api"
	"github.com/seanhalberthal/jiru/internal/config"
)

// newTestClient creates a Client that sends requests to the given httptest server.
// This bypasses config.ServerURL() which prepends "https://" — httptest uses plain HTTP.
func newTestClient(srv *httptest.Server, authType string) *Client {
	auth := api.AuthBasic
	if authType == "bearer" {
		auth = api.AuthBearer
	}

	cfg := &config.Config{
		Domain:   "test.atlassian.net",
		User:     "test@example.com",
		APIToken: "test-token",
		AuthType: authType,
		Project:  "TEST",
	}

	return &Client{
		http: api.New(api.Config{
			BaseURL:  srv.URL, // httptest URL (http://127.0.0.1:PORT)
			Username: cfg.User,
			Token:    cfg.APIToken,
			Auth:     auth,
		}),
		config: cfg,
	}
}
