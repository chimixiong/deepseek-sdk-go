package deepseek

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
	// Truncated reports that Body was larger than the cap.
	Truncated bool
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
	if e.Truncated {
		b.WriteString(" [body truncated]")
	}
	return b.String()
}

// TransportError wraps a failure below HTTP: dial, TLS, timeout or a cancelled
// context.
type TransportError struct{ Err error }

// ResponseError is the error object a 200 response can carry when generation
// fails after the status line was sent. Chat Completions and Messages return
// it with the billed usage; OCR returns it without usage.
type ResponseError struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    string  `json:"code"`
}

func (e *ResponseError) Error() string {
	if e == nil {
		return "deepseek: generation failed"
	}
	var b strings.Builder
	b.WriteString("deepseek: generation failed")
	if e.Type != "" {
		b.WriteString(" " + e.Type)
	}
	if e.Code != "" && e.Code != e.Type {
		fmt.Fprintf(&b, " (%s)", e.Code)
	}
	if e.Message != "" {
		b.WriteString(": " + e.Message)
	}
	return b.String()
}

func (e *TransportError) Error() string { return "deepseek: transport: " + e.Err.Error() }
func (e *TransportError) Unwrap() error { return e.Err }

func readAPIError(resp *http.Response) *APIError {
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody+1))
	truncated := len(body) > maxErrorBody
	if truncated {
		body = body[:maxErrorBody]
	}
	err := &APIError{
		StatusCode: resp.StatusCode,
		RequestID: firstNonEmpty(
			resp.Header.Get("x-request-id"),
			resp.Header.Get("x-ds-trace-id"),
			resp.Header.Get("request-id"),
		),
		Body:      body,
		Truncated: truncated,
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
	if truncated {
		err.Message += " (body truncated)"
	}
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
	case http.StatusRequestTimeout,
		http.StatusTooManyRequests,
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
	if seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
		if seconds < 0 {
			return 0, false
		}
		if seconds > int64(maxRetryDelay/time.Second) {
			return maxRetryDelay, true
		}
		return time.Duration(seconds) * time.Second, true
	}
	if when, err := http.ParseTime(value); err == nil {
		d := when.Sub(at)
		if d <= 0 {
			return 0, true
		}
		if d > maxRetryDelay {
			d = maxRetryDelay
		}
		return d, true
	}
	return 0, false
}

var now = time.Now
