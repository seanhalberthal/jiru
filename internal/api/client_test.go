package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_BasicAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			t.Errorf("expected Basic auth, got %q", auth)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept: application/json, got %q", r.Header.Get("Accept"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"displayName":"test"}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Username: "user", Token: "token", Auth: AuthBasic})
	resp, err := c.Get(context.Background(), V2("/myself"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := DecodeResponse[MeResponse](resp)
	if err != nil {
		t.Fatal(err)
	}
	if result.DisplayName != "test" {
		t.Errorf("got %q, want %q", result.DisplayName, "test")
	}
}

func TestClient_BearerAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer my-pat" {
			t.Errorf("expected 'Bearer my-pat', got %q", auth)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"displayName":"bearer-user"}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Token: "my-pat", Auth: AuthBearer})
	resp, err := c.Get(context.Background(), V2("/myself"))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
}

func TestClient_PostSetsContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type: application/json, got %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Username: "u", Token: "t", Auth: AuthBasic})
	resp, err := c.Post(context.Background(), V2("/issue/TEST-1/comment"), map[string]string{"body": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
}

func TestClient_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid credentials"}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Username: "u", Token: "bad", Auth: AuthBasic})
	resp, err := c.Get(context.Background(), V2("/myself"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = DecodeResponse[MeResponse](resp)
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should contain status code, got: %v", err)
	}
}

func TestCheckResponse_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Username: "u", Token: "t", Auth: AuthBasic})
	resp, err := c.Get(context.Background(), "/test")
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckResponse(resp); err != nil {
		t.Errorf("expected no error for 204, got: %v", err)
	}
}

func TestCheckResponse_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`forbidden`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Username: "u", Token: "t", Auth: AuthBasic})
	resp, err := c.Get(context.Background(), "/test")
	if err != nil {
		t.Fatal(err)
	}
	err = CheckResponse(resp)
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should contain status code, got: %v", err)
	}
}

func TestClient_Put(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify correct HTTP method.
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		// Verify auth header is present.
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			t.Errorf("expected Basic auth header, got %q", auth)
		}
		// Verify Content-Type is set for JSON body.
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type: application/json, got %q", r.Header.Get("Content-Type"))
		}
		// Verify Accept header.
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept: application/json, got %q", r.Header.Get("Accept"))
		}
		// Read and verify the body was serialised correctly.
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body["summary"] != "updated title" {
			t.Errorf("expected summary 'updated title', got %q", body["summary"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"key":"TEST-1"}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Username: "user", Token: "token", Auth: AuthBasic})
	resp, err := c.Put(context.Background(), V2("/issue/TEST-1"), map[string]string{"summary": "updated title"})
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckResponse(resp); err != nil {
		t.Errorf("expected successful response, got: %v", err)
	}
}

func TestClient_Put_BearerAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer my-pat" {
			t.Errorf("expected 'Bearer my-pat', got %q", auth)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Token: "my-pat", Auth: AuthBearer})
	resp, err := c.Put(context.Background(), V2("/issue/TEST-1"), map[string]string{"summary": "test"})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
}

func TestClient_Put_ErrorStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"bad request", http.StatusBadRequest},
		{"not found", http.StatusNotFound},
		{"internal server error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(`{"message":"something went wrong"}`))
			}))
			defer srv.Close()

			c := New(Config{BaseURL: srv.URL, Username: "u", Token: "t", Auth: AuthBasic})
			resp, err := c.Put(context.Background(), V2("/issue/TEST-1"), map[string]string{"summary": "fail"})
			if err != nil {
				t.Fatal(err)
			}
			err = CheckResponse(resp)
			if err == nil {
				t.Fatalf("expected error for status %d", tt.statusCode)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("%d", tt.statusCode)) {
				t.Errorf("error should contain status code %d, got: %v", tt.statusCode, err)
			}
		})
	}
}

func TestClient_Delete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify correct HTTP method.
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		// Verify auth header is present.
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			t.Errorf("expected Basic auth header, got %q", auth)
		}
		// Verify Accept header.
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept: application/json, got %q", r.Header.Get("Accept"))
		}
		// Delete should not send Content-Type (no body).
		if ct := r.Header.Get("Content-Type"); ct != "" {
			t.Errorf("expected no Content-Type for DELETE, got %q", ct)
		}
		// Verify the body is empty.
		if r.ContentLength > 0 {
			t.Errorf("expected empty body for DELETE, got ContentLength %d", r.ContentLength)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Username: "user", Token: "token", Auth: AuthBasic})
	resp, err := c.Delete(context.Background(), V2("/issue/TEST-1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckResponse(resp); err != nil {
		t.Errorf("expected successful response, got: %v", err)
	}
}

func TestClient_Delete_BearerAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer my-pat" {
			t.Errorf("expected 'Bearer my-pat', got %q", auth)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Token: "my-pat", Auth: AuthBearer})
	resp, err := c.Delete(context.Background(), V2("/issue/TEST-1"))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
}

func TestClient_Delete_ErrorStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"not found", http.StatusNotFound},
		{"forbidden", http.StatusForbidden},
		{"internal server error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(`{"message":"deletion failed"}`))
			}))
			defer srv.Close()

			c := New(Config{BaseURL: srv.URL, Username: "u", Token: "t", Auth: AuthBasic})
			resp, err := c.Delete(context.Background(), V2("/issue/TEST-1"))
			if err != nil {
				t.Fatal(err)
			}
			err = CheckResponse(resp)
			if err == nil {
				t.Fatalf("expected error for status %d", tt.statusCode)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("%d", tt.statusCode)) {
				t.Errorf("error should contain status code %d, got: %v", tt.statusCode, err)
			}
		})
	}
}

func TestPathConstructors(t *testing.T) {
	if got := V1("/board/1"); got != "/rest/agile/1.0/board/1" {
		t.Errorf("V1 = %q", got)
	}
	if got := V2("/myself"); got != "/rest/api/2/myself" {
		t.Errorf("V2 = %q", got)
	}
	if got := V3("/search/jql"); got != "/rest/api/3/search/jql" {
		t.Errorf("V3 = %q", got)
	}
}
