package webserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// Session tokens are signed in this infrastructure package (pure HMAC over
// in-memory bytes — no I/O) so the composition root does not inline crypto. The
// host owns the signing key (vault) and the epoch (in-memory), passing them in:
// SignSession mints, VerifySession checks signature + expiry and returns the
// embedded epoch for the host to compare against the current one.

type sessionPayload struct {
	Epoch int64 `json:"e"`
	Iat   int64 `json:"i"`
	Exp   int64 `json:"x,omitempty"`
}

// SignSession produces a "<base64url(payload)>.<base64url(hmac)>" token. exp of
// 0 means no time-based expiry.
func SignSession(key []byte, epoch, iat, exp int64) string {
	data, err := json.Marshal(sessionPayload{Epoch: epoch, Iat: iat, Exp: exp})
	if err != nil {
		return ""
	}
	body := base64.RawURLEncoding.EncodeToString(data)
	sig := base64.RawURLEncoding.EncodeToString(sign(key, body))
	return body + "." + sig
}

// VerifySession validates the signature and (when set) the expiry of token,
// returning the embedded epoch. ok is false on any tampering or expiry; the
// caller still compares epoch against the current generation to reject sessions
// invalidated by lock/autolock/regenerate.
func VerifySession(key []byte, token string) (epoch int64, ok bool) {
	body, sig, found := strings.Cut(token, ".")
	if !found || body == "" || sig == "" {
		return 0, false
	}
	want := base64.RawURLEncoding.EncodeToString(sign(key, body))
	if subtle.ConstantTimeCompare([]byte(want), []byte(sig)) != 1 {
		return 0, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return 0, false
	}
	var payload sessionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0, false
	}
	if payload.Exp != 0 && time.Now().Unix() > payload.Exp {
		return 0, false
	}
	return payload.Epoch, true
}

func sign(key []byte, msg string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))
	return mac.Sum(nil)
}
