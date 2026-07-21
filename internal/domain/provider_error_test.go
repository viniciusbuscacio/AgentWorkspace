package domain

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestProviderHTTPErrorMessage(t *testing.T) {
	cases := []struct {
		name string
		err  *ProviderHTTPError
		want string
	}{
		{"status+message", &ProviderHTTPError{StatusCode: 429, Status: "429 Too Many Requests", Message: "rate limited"}, "429 Too Many Requests: rate limited"},
		{"status only", &ProviderHTTPError{StatusCode: 500, Status: "500 Internal Server Error"}, "500 Internal Server Error"},
		{"message only", &ProviderHTTPError{StatusCode: 402, Message: "insufficient credits"}, "insufficient credits"},
		{"empty", &ProviderHTTPError{StatusCode: 0}, "provider request failed"},
	}
	for _, tc := range cases {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("%s: Error() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestIsFailoverErrorHTTPStatus(t *testing.T) {
	failover := []int{401, 402, 403, 404, 408, 409, 425, 429, 500, 502, 503, 504, 529}
	for _, code := range failover {
		err := NewProviderHTTPError(code, fmt.Sprintf("%d", code), "boom")
		if !IsFailoverError(err) {
			t.Errorf("status %d: IsFailoverError = false, want true", code)
		}
	}
	noFailover := []int{400, 410, 422}
	for _, code := range noFailover {
		err := NewProviderHTTPError(code, fmt.Sprintf("%d", code), "bad request")
		if IsFailoverError(err) {
			t.Errorf("status %d: IsFailoverError = true, want false", code)
		}
	}
}

func TestIsFailoverErrorContext(t *testing.T) {
	if IsFailoverError(context.Canceled) {
		t.Error("context.Canceled should not trigger failover (user stopped)")
	}
	if IsFailoverError(context.DeadlineExceeded) {
		t.Error("context.DeadlineExceeded should not trigger failover (turn deadline)")
	}
	// Wrapped cancellation must also be respected.
	wrapped := fmt.Errorf("stream aborted: %w", context.Canceled)
	if IsFailoverError(wrapped) {
		t.Error("wrapped context.Canceled should not trigger failover")
	}
}

type fakeTimeoutErr struct{}

func (fakeTimeoutErr) Error() string   { return "dial tcp: operation timed out" }
func (fakeTimeoutErr) Timeout() bool   { return true }
func (fakeTimeoutErr) Temporary() bool { return true }

func TestIsFailoverErrorNetwork(t *testing.T) {
	var _ net.Error = fakeTimeoutErr{}
	if !IsFailoverError(fakeTimeoutErr{}) {
		t.Error("net.Error timeout should trigger failover")
	}
	if !IsFailoverError(errors.New("dial tcp 1.2.3.4:443: connection refused")) {
		t.Error("connection refused should trigger failover")
	}
	if !IsFailoverError(errors.New("read: connection reset by peer")) {
		t.Error("connection reset should trigger failover")
	}
}

func TestIsFailoverErrorNil(t *testing.T) {
	if IsFailoverError(nil) {
		t.Error("nil error should not trigger failover")
	}
	if IsFailoverError(errors.New("chat completion returned no text")) {
		t.Error("a plain content error should not trigger failover")
	}
}

// ensure deadlineExceeded test isn't flaky on slow CI: deadline already passed.
func TestIsFailoverErrorExpiredDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if IsFailoverError(ctx.Err()) {
		t.Error("expired deadline should not trigger failover")
	}
}

func TestIsContextWindowError(t *testing.T) {
	positives := []error{
		fmt.Errorf("your input exceeds the context window of this model; please adjust your input and try again"),
		fmt.Errorf("maximum context length exceeded"),
		fmt.Errorf("prompt is too long: too many tokens"),
	}
	for _, err := range positives {
		if !IsContextWindowError(err) {
			t.Fatalf("IsContextWindowError(%q) = false", err.Error())
		}
	}
	if IsContextWindowError(fmt.Errorf("connection refused")) {
		t.Fatal("transport errors must not be treated as context-window errors")
	}
}
