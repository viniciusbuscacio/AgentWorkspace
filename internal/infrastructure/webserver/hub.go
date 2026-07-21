package webserver

import "sync"

// Event is a server-sent event broadcast to connected web clients. It mirrors
// pip.Event so the frontend SSE parser is identical between PiP and web.
type Event struct {
	Name    string `json:"name"`
	Payload any    `json:"payload"`
}

// Hub is an in-memory pub/sub for web events. It is owned by the host app
// (which broadcasts chat/ui/provider events) and read by the server (which
// streams them to browsers over SSE). It is the web surface of the event bus:
// keeping it separate from pip.Hub means PiP windows never receive ui:* or
// vault:* traffic they don't subscribe to.
type Hub struct {
	mu      sync.Mutex
	clients map[chan Event]struct{}
}

// NewHub creates an empty Hub.
func NewHub() *Hub {
	return &Hub{clients: map[chan Event]struct{}{}}
}

func (h *Hub) subscribe() (chan Event, func()) {
	ch := make(chan Event, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.clients, ch)
		close(ch)
		h.mu.Unlock()
	}
}

// Broadcast delivers an event to every connected web client, dropping events
// for slow clients rather than blocking the caller.
func (h *Hub) Broadcast(event Event) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- event:
		default:
		}
	}
}
