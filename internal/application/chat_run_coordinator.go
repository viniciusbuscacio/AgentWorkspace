package application

import (
	"context"
	"errors"
	"sync"
)

// ErrChatAlreadyRunning is returned by ChatRunCoordinator.Begin when a chat
// already has an in-flight run. The interface layer surfaces its message
// verbatim, so keep the wording stable.
var ErrChatAlreadyRunning = errors.New("chat is already sending")

// ChatRunCoordinator owns the per-chat run lifecycle: it enforces one in-flight
// run per chat, holds each run's cancel func, and tracks which chat is
// currently streaming. This is application coordination policy — extracted from
// the interface layer, which now only translates runs into Wails events.
//
// The zero value is ready to use (Begin lazily allocates), mirroring the
// original map+mutex it replaced. Methods are safe for concurrent use; do not
// copy a coordinator after first use.
type ChatRunCoordinator struct {
	mu          sync.Mutex
	runs        map[string]chatRunState
	runSeq      int64
	streamMu    sync.Mutex
	streamID    string
	streamToken int64
}

type chatRunState struct {
	cancel context.CancelFunc
	token  int64
}

// Begin registers cancel as the in-flight run for chatID. It returns
// ErrChatAlreadyRunning if a run is already in flight for that chat (one run
// per chat).
func (c *ChatRunCoordinator) Begin(chatID string, cancel context.CancelFunc) error {
	_, err := c.BeginRun(chatID, cancel)
	return err
}

// BeginRun is Begin plus a generation token. Call EndRun with this token so a
// stopped/old goroutine cannot clear a newer run for the same chat.
func (c *ChatRunCoordinator) BeginRun(chatID string, cancel context.CancelFunc) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.runs == nil {
		c.runs = map[string]chatRunState{}
	}
	if _, exists := c.runs[chatID]; exists {
		return 0, ErrChatAlreadyRunning
	}
	c.runSeq++
	c.runs[chatID] = chatRunState{cancel: cancel, token: c.runSeq}
	return c.runSeq, nil
}

// End clears the in-flight run for chatID.
func (c *ChatRunCoordinator) End(chatID string) {
	c.EndRun(chatID, 0)
}

// EndRun clears the in-flight run for chatID. When token is non-zero, it only
// clears if the token still matches the active run.
func (c *ChatRunCoordinator) EndRun(chatID string, token int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if token != 0 {
		state, ok := c.runs[chatID]
		if !ok || state.token != token {
			return
		}
	}
	delete(c.runs, chatID)
}

// IsRunning reports whether a run is in flight for chatID.
func (c *ChatRunCoordinator) IsRunning(chatID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, running := c.runs[chatID]
	return running
}

// Stop cancels the in-flight run for chatID, if any. Cancellation runs outside
// the lock so a slow CancelFunc cannot block other coordinator calls.
func (c *ChatRunCoordinator) Stop(chatID string) {
	c.mu.Lock()
	state := c.runs[chatID]
	delete(c.runs, chatID)
	c.mu.Unlock()
	if state.cancel != nil {
		state.cancel()
	}
}

// ActiveCount returns how many chats have a run in flight.
func (c *ChatRunCoordinator) ActiveCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.runs)
}

// SetStreaming records which chat is currently streaming (empty clears it) so
// out-of-band side channels (e.g. an inline screenshot) can target it.
func (c *ChatRunCoordinator) SetStreaming(chatID string) {
	c.SetStreamingRun(chatID, 0)
}

// SetStreamingRun records which run owns the current streaming side channel.
func (c *ChatRunCoordinator) SetStreamingRun(chatID string, token int64) {
	c.streamMu.Lock()
	c.streamID = chatID
	c.streamToken = token
	c.streamMu.Unlock()
}

// ClearStreamingRun clears streaming only if the active streaming owner still
// matches token. A stopped/old run must not clear a newer queued run.
func (c *ChatRunCoordinator) ClearStreamingRun(token int64) {
	c.streamMu.Lock()
	if token == 0 || c.streamToken == token {
		c.streamID = ""
		c.streamToken = 0
	}
	c.streamMu.Unlock()
}

// Streaming returns the chat id that is currently streaming, or "".
func (c *ChatRunCoordinator) Streaming() string {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()
	return c.streamID
}
