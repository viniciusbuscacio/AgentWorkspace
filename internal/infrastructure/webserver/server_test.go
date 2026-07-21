package webserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeAuth struct {
	unlocked    bool
	validToken  string
	invalidated int
	activity    int
}

func (f *fakeAuth) VerifyOrUnlock(password string) (string, error) {
	if password == "right" {
		f.unlocked = true
		return f.validToken, nil
	}
	return "", errBoom
}
func (f *fakeAuth) VaultUnlocked() bool        { return f.unlocked }
func (f *fakeAuth) ValidSession(t string) bool { return t != "" && t == f.validToken }
func (f *fakeAuth) InvalidateSessions(string)  { f.invalidated++ }
func (f *fakeAuth) RecordActivity()            { f.activity++ }

func newTestServer(auth *fakeAuth) *Server {
	return &Server{
		opts:       Options{Authenticator: auth, App: fakeApp{}, BindMode: BindModeManual},
		bridge:     newBridge(fakeApp{}),
		loginLimit: newRateLimiter(2, time.Minute),
	}
}

func TestHealthz(t *testing.T) {
	auth := &fakeAuth{unlocked: false}
	s := newTestServer(auth)
	rec := httptest.NewRecorder()
	s.handleHealthz(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["running"] != true || body["locked"] != true {
		t.Fatalf("got %+v", body)
	}
}

func TestBridgeGating(t *testing.T) {
	auth := &fakeAuth{unlocked: false, validToken: "good"}
	s := newTestServer(auth)

	// Locked vault -> "locked".
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bridge", strings.NewReader(`{"method":"Echo","args":["hi"]}`))
	s.handleBridge(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "locked") {
		t.Fatalf("locked gate: code=%d body=%s", rec.Code, rec.Body.String())
	}

	// Unlocked + bad token -> "invalid_session".
	auth.unlocked = true
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/bridge", strings.NewReader(`{"method":"Echo","args":["hi"]}`))
	req.Header.Set("Authorization", "Bearer wrong")
	s.handleBridge(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "invalid_session") {
		t.Fatalf("invalid gate: code=%d body=%s", rec.Code, rec.Body.String())
	}

	// Unlocked + good token -> dispatch and record activity.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/bridge", strings.NewReader(`{"method":"Echo","args":["hi"]}`))
	req.Header.Set("Authorization", "Bearer good")
	s.handleBridge(rec, req)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `"hi"` {
		t.Fatalf("dispatch: code=%d body=%s", rec.Code, rec.Body.String())
	}
	if auth.activity == 0 {
		t.Fatal("expected RecordActivity on authenticated bridge call")
	}
}

func TestLoginRateLimit(t *testing.T) {
	auth := &fakeAuth{validToken: "good"}
	s := newTestServer(auth) // limiter allows 2 per window

	post := func() int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"password":"wrong"}`))
		req.RemoteAddr = "203.0.113.5:5000"
		s.handleLogin(rec, req)
		return rec.Code
	}
	if c := post(); c != http.StatusUnauthorized {
		t.Fatalf("attempt 1: %d", c)
	}
	if c := post(); c != http.StatusUnauthorized {
		t.Fatalf("attempt 2: %d", c)
	}
	if c := post(); c != http.StatusTooManyRequests {
		t.Fatalf("attempt 3 should be rate-limited, got %d", c)
	}
}

func TestPeerGuardTailscale(t *testing.T) {
	s := &Server{opts: Options{Authenticator: &fakeAuth{}, BindMode: BindModeTailscale}}
	guarded := s.peerGuard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Non-tailnet peer rejected.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "192.0.2.50:1111"
	guarded.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-tailnet peer should be 403, got %d", rec.Code)
	}

	// Tailnet peer allowed.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "100.100.1.1:2222"
	guarded.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("tailnet peer should pass, got %d", rec.Code)
	}
}
