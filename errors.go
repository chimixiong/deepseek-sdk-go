package deepseek

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxErrorBody = 64 << 10

// APIError is a non-2xx response. DeepSeek returns OpenAI-shaped errors on most
// endpoints and Anthropic-shaped errors on the Messages endpoint; both expose
// message and type.
type APIError struct {
	StatusCode int
	Type       string
	Code       string
	Param      string
	Message    string
	RequestID  string
	// Body is the raw response body, capped at 64 KiB.
	Body []byte
}

func (e *APIError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "deepseek: HTTP %d", e.StatusCode)
	if e.Type != "" {
		b.WriteString(" " + e.Type)
	}
	if e.Code != "" {
		fmt.Fprintf(&b, " (%s)", e.Code)
	}
	if e.Message != "" {
		b.WriteString(": " + e.Message)
	}
	if e.RequestID != "" {
		fmt.Fprintf(&b, " [request_id=%s]", e.RequestID)
	}
	return b.String()
}

// TransportError wraps a failure below HTTP: dial, TLS, timeout or a cancelled
// context.
type TransportError struct{ Err error }

func (e *TransportError) Error() string { return "deepseek: transport: " + e.Err.Error() }
func (e *TransportError) Unwrap() error { return e.Err }

func readAPIError(resp *http.Response) *APIError {
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	err := &APIError{
		StatusCode: resp.StatusCode,
		RequestID: firstNonEmpty(
			resp.Header.Get("x-request-id"),
			resp.Header.Get("x-ds-trace-id"),
			resp.Header.Get("request-id"),
		),
		Body: body,
	}
	var wire struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
			Param   string `json:"param"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &wire) == nil && (wire.Error.Message != "" || wire.Error.Type != "") {
		err.Message = wire.Error.Message
		err.Type = wire.Error.Type
		err.Code = wire.Error.Code
		err.Param = wire.Error.Param
		return err
	}
	err.Message = strings.TrimSpace(string(body))
	return err
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func retryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

func (c *Client) backoff(attempt int) time.Duration {
	delay := c.retryBaseDelay
	for range attempt {
		if delay >= maxRetryDelay || delay <= 0 {
			break
		}
		delay *= 2
	}
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}
	return delay
}

func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func parseRetryAfter(value string, at time.Time) (time.Duration, bool) {
	if seconds, err := time.ParseDuration(value + "s"); err == nil {
		return seconds, true
	}
	if when, err := http.ParseTime(value); err == nil {
		if d := when.Sub(at); d > 0 {
			return d, true
		}
		return 0, true
	}
	return 0, false
}

var now = time.Now
