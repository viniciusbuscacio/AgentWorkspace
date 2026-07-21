package domain

import (
	"context"
	"errors"
	"net"
	"strings"
)

// ProviderHTTPError is returned by provider adapters when an upstream LLM
// endpoint responds with a non-2xx status. It carries the HTTP status code so
// the fallback loop can decide whether to advance to the next provider instead
// of string-matching the message.
type ProviderHTTPError struct {
	StatusCode int
	Status     string
	Message    string
}

func (e *ProviderHTTPError) Error() string {
	switch {
	case e == nil:
		return ""
	case strings.TrimSpace(e.Message) == "" && strings.TrimSpace(e.Status) == "":
		return "provider request failed"
	case strings.TrimSpace(e.Message) == "":
		return e.Status
	case strings.TrimSpace(e.Status) == "":
		return e.Message
	default:
		return e.Status + ": " + e.Message
	}
}

// NewProviderHTTPError builds a typed HTTP error from a status code, a status
// line (e.g. "429 Too Many Requests") and a best-effort message extracted from
// the response body.
func NewProviderHTTPError(statusCode int, status, message string) *ProviderHTTPError {
	return &ProviderHTTPError{StatusCode: statusCode, Status: status, Message: message}
}

// IsFailoverError reports whether err is the kind of failure that should make
// the runtime try the next provider in the fallback chain. These are
// account/endpoint-level problems: auth (401/403), payment/quota (402),
// rate limit (429), request timeout (408), server errors (5xx) and
// network-level failures (DNS, refused, reset, transport timeout).
//
// User/request errors (HTTP 400), caller cancellation (user pressed stop) and
// our own turn deadline do NOT trigger failover, because switching providers
// would not help.
func IsFailoverError(err error) bool {
	if err == nil {
		return false
	}
	// Never fail over on caller cancellation or a turn-level deadline: the user
	// stopped, or our own timeout fired. Another provider won't fix that.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var httpErr *ProviderHTTPError
	if errors.As(err, &httpErr) {
		return isFailoverStatus(httpErr.StatusCode)
	}

	// Transport-level failures (connection refused/reset, DNS, client timeout)
	// are endpoint problems worth failing over.
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Conservative string match for transport errors that don't implement
	// net.Error (wrapped by libraries, etc.).
	msg := strings.ToLower(err.Error())
	for _, frag := range []string{
		"connection refused", "connection reset", "no such host",
		"network is unreachable", "timed out", "i/o timeout",
		"tls handshake", "unexpected eof",
		// Model gone at the provider (adapters normally type this as a 404
		// ProviderHTTPError, but wrappers can flatten it to a string).
		"model not found", "not deployed", "404 not found",
	} {
		if strings.Contains(msg, frag) {
			return true
		}
	}
	return false
}

// IsContextWindowError reports whether the provider rejected the request
// because the prompt/input was larger than the model context window. That is
// recoverable by compacting chat history and retrying the same user turn once.
func IsContextWindowError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, frag := range []string{
		"context window",
		"context length",
		"maximum context",
		"max context",
		"input exceeds",
		"input is too long",
		"prompt is too long",
		"too many tokens",
		"exceeds the context",
		"exceeded token limit",
		"reduce the length",
	} {
		if strings.Contains(msg, frag) {
			return true
		}
	}
	return false
}

func isFailoverStatus(code int) bool {
	switch code {
	// 404: "model not found / not deployed" — the provider pulled the model
	// or the deployment is gone; another provider in the chain can still
	// answer, so switching (with the user notice) beats a dead turn.
	case 401, 402, 403, 404, 408, 409, 425, 429:
		return true
	}
	return code >= 500 && code <= 599
}
