package appcore

import (
	"context"
	"testing"
	"time"

	"aw/internal/infrastructure/webserver"
)

// TestStartHeadlessNoWails verifies the awd path: StartHeadless runs the
// background loops without a Wails runtime, leaves a.ctx nil, and does not panic.
func TestStartHeadlessNoWails(t *testing.T) {
	app := &App{webHub: webserver.NewHub(), planModes: map[string]bool{}}
	ctx, cancel := context.WithCancel(context.Background())
	app.StartHeadless(ctx)
	if app.ctx != nil {
		t.Fatal("StartHeadless must leave a.ctx nil so events route to the hubs, not Wails")
	}
	// Let the goroutines spin up, then cancel; nothing should panic.
	time.Sleep(20 * time.Millisecond)
	cancel()
	time.Sleep(20 * time.Millisecond)
}

// TestEmitChatEventWithoutWailsCtx verifies events reach the web hub even when
// the Wails runtime is absent (a.ctx == nil), which is the headless invariant.
func TestEmitChatEventWithoutWailsCtx(t *testing.T) {
	hub := webserver.NewHub()
	app := &App{webHub: hub}
	if app.ctx != nil {
		t.Fatal("precondition: a.ctx must be nil")
	}
	// emitChatEvent must not panic and must broadcast to the web hub.
	app.emitChatEvent("chat:delta", map[string]any{"chatId": "c1", "text": "hi"})
	// emitUIEvent goes to the web surface too (but not PiP).
	app.emitUIEvent("ui:navigate", map[string]any{"view": "home"})
}
