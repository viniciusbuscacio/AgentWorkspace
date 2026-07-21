package webserver

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DefaultPort is the default web-mode listen port (next free after MCP 9300 /
// REST 9301).
const DefaultPort = 9302

// maxBodyBytes bounds /login, /setup and /bridge request bodies. Chat sends go
// through the bridge and carry base64 attachments, so it is generous but finite.
const maxBodyBytes = 32 << 20 // 32 MiB

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// decodeLimited reads at most maxBodyBytes and JSON-decodes into dst.
func decodeLimited(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	return json.NewDecoder(r.Body).Decode(dst)
}

// bearerToken extracts the token from an Authorization: Bearer header.
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if strings.HasPrefix(h, prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

// clientKey identifies a caller for rate limiting (peer IP, port-stripped).
func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// rateLimiter is a tiny fixed-window per-key limiter for login/setup endpoints.
type rateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string]*windowCount
}

type windowCount struct {
	count int
	start time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, hits: map[string]*windowCount{}}
}

// allow reports whether key may make another attempt now.
func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	wc := rl.hits[key]
	if wc == nil || now.Sub(wc.start) > rl.window {
		rl.hits[key] = &windowCount{count: 1, start: now}
		return true
	}
	if wc.count >= rl.limit {
		return false
	}
	wc.count++
	return true
}
