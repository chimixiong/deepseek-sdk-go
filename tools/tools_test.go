package tools

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chimixiong/deepseek-sdk-go"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return New(deepseek.New(deepseek.Config{
		APIKey:     "test-key",
		BaseURL:    srv.URL,
		MaxRetries: -1,
	}))
}

func requireValidationError(t *testing.T, err error, field string) {
	t.Helper()
	var validation *deepseek.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v (%T), want *deepseek.ValidationError", err, err)
	}
	if validation.Field != field {
		t.Fatalf("validation field = %q, want %q (problem: %s)", validation.Field, field, validation.Problem)
	}
}

func requireAPIError(t *testing.T, err error, status int) *deepseek.APIError {
	t.Helper()
	var apiErr *deepseek.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v (%T), want *deepseek.APIError", err, err)
	}
	if apiErr.StatusCode != status {
		t.Fatalf("status = %d, want %d", apiErr.StatusCode, status)
	}
	return apiErr
}

func noRequest(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
	}
}

func TestNewNilClient(t *testing.T) {
	c := New(nil)
	if c == nil || c.DeepSeek() == nil {
		t.Fatal("New(nil) did not install a default transport client")
	}
}
