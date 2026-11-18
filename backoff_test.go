package deepseek

import (
	"net/http"
	"testing"
	"time"
)

func TestBackoffCapsWithoutOverflow(t *testing.T) {
	client := &Client{retryBaseDelay: 500 * time.Millisecond}
	if got := client.backoff(0); got != 500*time.Millisecond {
		t.Fatalf("backoff(0) = %v", got)
	}
	if got := client.backoff(2); got != 2*time.Second {
		t.Fatalf("backoff(2) = %v", got)
	}
	// Shift overflow used to make late attempts negative; the cap must hold for
	// every attempt count.
	for _, attempt := range []int{10, 35, 62, 63, 200, 1 << 20} {
		if got := client.backoff(attempt); got <= 0 || got > maxRetryDelay {
			t.Fatalf("backoff(%d) = %v, want (0, %v]", attempt, got, maxRetryDelay)
		}
	}
	if got := client.backoff(40); got != maxRetryDelay {
		t.Fatalf("backoff(40) = %v, want %v", got, maxRetryDelay)
	}
}

func TestBackoffZeroDelay(t *testing.T) {
	client := &Client{}
	if got := client.backoff(30); got != 0 {
		t.Fatalf("backoff(30) = %v, want 0", got)
	}
}

func TestRetryableStatus(t *testing.T) {
	for _, code := range []int{408, 429, 500, 502, 503, 504} {
		if !retryableStatus(code) {
			t.Errorf("retryableStatus(%d) = false, want true", code)
		}
	}
	for _, code := range []int{400, 401, 402, 404, 413, 422, 501} {
		if retryableStatus(code) {
			t.Errorf("retryableStatus(%d) = true, want false", code)
		}
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		value string
		want  time.Duration
		ok    bool
	}{
		{"0", 0, true},
		{"3", 3 * time.Second, true},
		{" 5 ", 5 * time.Second, true},
		{"-5", 0, false},
		{"3600", maxRetryDelay, true},
		{"9223372036854775807", maxRetryDelay, true},
		{"1.5", 0, false},
		{"garbage", 0, false},
	}
	for _, tc := range tests {
		got, ok := parseRetryAfter(tc.value, now)
		if got != tc.want || ok != tc.ok {
			t.Errorf("parseRetryAfter(%q) = %v, %v; want %v, %v", tc.value, got, ok, tc.want, tc.ok)
		}
	}
	when := now.Add(30 * time.Minute).Format(http.TimeFormat)
	if got, ok := parseRetryAfter(when, now); !ok || got != maxRetryDelay {
		t.Errorf("HTTP date = %v, %v; want %v, true", got, ok, maxRetryDelay)
	}
}
