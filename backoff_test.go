package deepseek

import (
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
