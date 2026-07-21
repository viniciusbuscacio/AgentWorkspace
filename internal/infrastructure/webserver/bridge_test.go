package webserver

import (
	"context"
	"encoding/json"
	"testing"
)

// fakeApp exercises every bridge return shape: 0 results, 1 result, (T,error)
// both nil and non-nil, multi-result, a context.Context parameter, and a panic.
type fakeApp struct{}

func (fakeApp) NoResults()           {}
func (fakeApp) Echo(s string) string { return s }
func (fakeApp) Sum(a, b int) int     { return a + b }
func (fakeApp) Pair() (string, int)  { return "x", 7 }
func (fakeApp) WithCtx(ctx context.Context, s string) string {
	if ctx == nil {
		return "nil-ctx"
	}
	return s
}
func (fakeApp) MayFail(fail bool) (string, error) {
	if fail {
		return "", errBoom
	}
	return "ok", nil
}
func (fakeApp) Boom() { panic("kaboom") }

var errBoom = &boomError{}

type boomError struct{}

func (*boomError) Error() string { return "boom" }

func args(vals ...any) []json.RawMessage {
	out := make([]json.RawMessage, len(vals))
	for i, v := range vals {
		b, _ := json.Marshal(v)
		out[i] = b
	}
	return out
}

func TestBridgeDispatch(t *testing.T) {
	b := newBridge(fakeApp{})

	t.Run("zero results -> nil body", func(t *testing.T) {
		res := b.dispatch(bridgeRequest{Method: "NoResults"})
		if res.badReq != "" || res.appErr != "" || res.body != nil {
			t.Fatalf("got %+v", res)
		}
	})

	t.Run("single result", func(t *testing.T) {
		res := b.dispatch(bridgeRequest{Method: "Echo", Args: args("hi")})
		if res.body != "hi" {
			t.Fatalf("got %+v", res)
		}
	})

	t.Run("two int args", func(t *testing.T) {
		res := b.dispatch(bridgeRequest{Method: "Sum", Args: args(2, 3)})
		if res.body != 5 {
			t.Fatalf("got %+v", res)
		}
	})

	t.Run("multi result -> array", func(t *testing.T) {
		res := b.dispatch(bridgeRequest{Method: "Pair"})
		arr, ok := res.body.([]any)
		if !ok || len(arr) != 2 || arr[0] != "x" || arr[1] != 7 {
			t.Fatalf("got %+v", res)
		}
	})

	t.Run("context param injected", func(t *testing.T) {
		res := b.dispatch(bridgeRequest{Method: "WithCtx", Args: args("payload")})
		if res.body != "payload" {
			t.Fatalf("got %+v", res)
		}
	})

	t.Run("(T,error) nil error", func(t *testing.T) {
		res := b.dispatch(bridgeRequest{Method: "MayFail", Args: args(false)})
		if res.body != "ok" || res.appErr != "" {
			t.Fatalf("got %+v", res)
		}
	})

	t.Run("(T,error) non-nil error", func(t *testing.T) {
		res := b.dispatch(bridgeRequest{Method: "MayFail", Args: args(true)})
		if res.appErr != "boom" {
			t.Fatalf("got %+v", res)
		}
	})

	t.Run("arg count mismatch -> bad request", func(t *testing.T) {
		res := b.dispatch(bridgeRequest{Method: "Sum", Args: args(1)})
		if res.badReq == "" {
			t.Fatalf("expected bad request, got %+v", res)
		}
	})

	t.Run("unknown method -> bad request", func(t *testing.T) {
		res := b.dispatch(bridgeRequest{Method: "Nope"})
		if res.badReq == "" {
			t.Fatalf("expected bad request, got %+v", res)
		}
	})

	t.Run("denied method", func(t *testing.T) {
		// SelectFolder is on the denylist; fakeApp doesn't even have it, but the
		// denylist check precedes method lookup.
		res := b.dispatch(bridgeRequest{Method: "SelectFolder"})
		if !res.denied {
			t.Fatalf("expected denied, got %+v", res)
		}
	})
}
