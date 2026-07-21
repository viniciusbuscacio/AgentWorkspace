package application

import "sync"

// ConfirmationCoordinator owns the set of in-flight tool-confirmation requests:
// it hands each request a one-shot channel and routes the user's later decision
// back to the waiter. This is the coordination state behind the confirm flow —
// extracted from the interface layer, which keeps only the Wails event emission
// and logging.
//
// The zero value is ready to use (Register lazily allocates). Methods are safe
// for concurrent use; do not copy a coordinator after first use.
type ConfirmationCoordinator struct {
	mu      sync.Mutex
	pending map[string]chan bool
}

// Register creates and registers a buffered decision channel for id, returning
// it so the caller can block on the user's response.
func (c *ConfirmationCoordinator) Register(id string) chan bool {
	ch := make(chan bool, 1)
	c.mu.Lock()
	if c.pending == nil {
		c.pending = map[string]chan bool{}
	}
	c.pending[id] = ch
	c.mu.Unlock()
	return ch
}

// Unregister drops the pending request for id (call on completion/timeout).
func (c *ConfirmationCoordinator) Unregister(id string) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

// Resolve delivers approved to the waiter for id. It reports false when id is
// unknown or expired. The non-blocking send mirrors the original behavior:
// a duplicate resolution is dropped rather than blocking.
func (c *ConfirmationCoordinator) Resolve(id string, approved bool) bool {
	c.mu.Lock()
	ch := c.pending[id]
	c.mu.Unlock()
	if ch == nil {
		return false
	}
	select {
	case ch <- approved:
	default:
	}
	return true
}
