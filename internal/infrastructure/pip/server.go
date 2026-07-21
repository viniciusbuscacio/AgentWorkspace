// Package pip implements aw's picture-in-picture chat subsystem: a small
// token-scoped localhost HTTP API plus an SSE event hub that backs the separate
// always-on-top PiP window process. It lives in the infrastructure layer and
// depends only on the domain layer; the host app supplies behavior through the
// Backend port.
package pip

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"aw/internal/domain"
)

// Snapshot is the read-only chat view served to the PiP window.
type Snapshot struct {
	Chat     *domain.Chat     `json:"chat,omitempty"`
	Messages []domain.Message `json:"messages"`
}

// VoiceResult mirrors the host app's voice operation results so the PiP
// composer behaves exactly like the main chat composer.
type VoiceResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Text    string `json:"text,omitempty"`
}

// Backend is the set of operations the PiP server needs from the host app.
// It is implemented in the interface layer (composition root).
type Backend interface {
	ChatSnapshot(chatID string) (Snapshot, error)
	SendMessage(chatID string, text string, attachments []domain.Attachment) error
	StopChat(chatID string) error
	StartVoiceCapture() VoiceResult
	StopVoiceCapture() VoiceResult
	CancelVoiceCapture() VoiceResult
	TranscribeAudio(fileName string, mimeType string, dataURI string) VoiceResult
	// SessionInfo, plan mode, tool confirmation and chat ops return the host
	// app's result DTOs as-is (success/error carried inside) so the PiP
	// frontend behaves exactly like the main window.
	SessionInfo(chatID string) any
	GetPlanMode(chatID string) any
	SetPlanMode(chatID string, enabled bool) any
	SetChatModel(chatID string, provider string, model string) any
	ResolveToolConfirmation(id string, approved bool) any
	ChatOp(action string, chatID string) any
}

// Event is a server-sent event broadcast to PiP windows.
type Event struct {
	Name    string `json:"name"`
	Payload any    `json:"payload"`
}

// Hub is a simple in-memory pub/sub for PiP events. It is shared between the
// host app (which broadcasts chat events) and the server (which streams them).
type Hub struct {
	mu      sync.Mutex
	clients map[chan Event]struct{}
}

// NewHub creates an empty Hub.
func NewHub() *Hub {
	return &Hub{clients: map[chan Event]struct{}{}}
}

func (h *Hub) subscribe() (chan Event, func()) {
	ch := make(chan Event, 32)
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

// Broadcast delivers an event to every subscribed PiP window, dropping events
// for slow clients rather than blocking.
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

// Server is the token-scoped localhost API backing the PiP window.
type Server struct {
	addr    string
	token   string
	server  *http.Server
	hub     *Hub
	backend Backend
}

// NewServer starts a PiP HTTP server on a random localhost port and returns it.
// The server runs until Shutdown is called.
func NewServer(backend Backend, hub *Hub) (*Server, error) {
	token, err := randomHex(24)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &Server{
		addr:    listener.Addr().String(),
		token:   token,
		hub:     hub,
		backend: backend,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat", s.withAuth(s.handleChat))
	mux.HandleFunc("/api/send", s.withAuth(s.handleSend))
	mux.HandleFunc("/api/stop", s.withAuth(s.handleStop))
	mux.HandleFunc("/api/events", s.withAuth(s.handleEvents))
	mux.HandleFunc("/api/voice/start", s.withAuth(s.handleVoiceStart))
	mux.HandleFunc("/api/voice/stop", s.withAuth(s.handleVoiceStop))
	mux.HandleFunc("/api/voice/cancel", s.withAuth(s.handleVoiceCancel))
	mux.HandleFunc("/api/transcribe", s.withAuth(s.handleTranscribe))
	mux.HandleFunc("/api/session-info", s.withAuth(s.handleSessionInfo))
	mux.HandleFunc("/api/plan", s.withAuth(s.handlePlan))
	mux.HandleFunc("/api/set-chat-model", s.withAuth(s.handleSetChatModel))
	mux.HandleFunc("/api/confirm", s.withAuth(s.handleConfirm))
	mux.HandleFunc("/api/chat-op", s.withAuth(s.handleChatOp))
	s.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		_ = s.server.Serve(listener)
	}()
	return s, nil
}

// Addr returns the localhost address the server is listening on.
func (s *Server) Addr() string { return s.addr }

// Token returns the per-server auth token required by PiP requests.
func (s *Server) Token() string { return s.token }

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) {
	if s == nil || s.server == nil {
		return
	}
	_ = s.server.Shutdown(ctx)
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-aw-PiP-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		token := r.URL.Query().Get("token")
		if token == "" {
			token = r.Header.Get("X-aw-PiP-Token")
		}
		if token == "" || token != s.token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	snapshot, err := s.backend.ChatSnapshot(r.URL.Query().Get("chatId"))
	writeJSON(w, snapshot, err)
}

type sendRequest struct {
	ChatID      string              `json:"chatId"`
	Text        string              `json:"text"`
	Attachments []domain.Attachment `json:"attachments"`
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, nil, err)
		return
	}
	req.ChatID = strings.TrimSpace(req.ChatID)
	if req.ChatID == "" {
		writeJSON(w, nil, fmt.Errorf("chat is required"))
		return
	}
	go func() {
		if err := s.backend.SendMessage(req.ChatID, req.Text, req.Attachments); err != nil {
			s.hub.Broadcast(Event{Name: "chat:error", Payload: map[string]any{"chatId": req.ChatID, "error": err.Error()}})
		}
		s.hub.Broadcast(Event{Name: "chat:refresh", Payload: map[string]any{"chatId": req.ChatID}})
	}()
	writeJSON(w, map[string]bool{"accepted": true}, nil)
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ChatID string `json:"chatId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, nil, err)
		return
	}
	if err := s.backend.StopChat(req.ChatID); err != nil {
		writeJSON(w, nil, err)
		return
	}
	s.hub.Broadcast(Event{Name: "chat:refresh", Payload: map[string]any{"chatId": req.ChatID}})
	writeJSON(w, map[string]bool{"stopped": true}, nil)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	events, unsubscribe := s.hub.subscribe()
	defer unsubscribe()
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case event := <-events:
			bytes, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", bytes)
			flusher.Flush()
		}
	}
}

func (s *Server) handleVoiceStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.backend.StartVoiceCapture(), nil)
}

func (s *Server) handleVoiceStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.backend.StopVoiceCapture(), nil)
}

func (s *Server) handleVoiceCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.backend.CancelVoiceCapture(), nil)
}

type transcribeRequest struct {
	FileName string `json:"fileName"`
	MimeType string `json:"mimeType"`
	DataURI  string `json:"dataUri"`
}

func (s *Server) handleTranscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req transcribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, nil, err)
		return
	}
	writeJSON(w, s.backend.TranscribeAudio(req.FileName, req.MimeType, req.DataURI), nil)
}

func (s *Server) handleSessionInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.backend.SessionInfo(r.URL.Query().Get("chatId")), nil)
}

func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.backend.GetPlanMode(r.URL.Query().Get("chatId")), nil)
	case http.MethodPost:
		var req struct {
			ChatID  string `json:"chatId"`
			Enabled bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, nil, err)
			return
		}
		writeJSON(w, s.backend.SetPlanMode(req.ChatID, req.Enabled), nil)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSetChatModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ChatID   string `json:"chatId"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, nil, err)
		return
	}
	writeJSON(w, s.backend.SetChatModel(req.ChatID, req.Provider, req.Model), nil)
}

func (s *Server) handleConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID       string `json:"id"`
		Approved bool   `json:"approved"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, nil, err)
		return
	}
	writeJSON(w, s.backend.ResolveToolConfirmation(req.ID, req.Approved), nil)
}

func (s *Server) handleChatOp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Action string `json:"action"`
		ChatID string `json:"chatId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, nil, err)
		return
	}
	writeJSON(w, s.backend.ChatOp(req.Action, req.ChatID), nil)
}

// BuildURL constructs the localhost URL the PiP window loads.
func BuildURL(addr, token, chatID string) string {
	return fmt.Sprintf(
		"http://%s/?api=%s&token=%s&chatId=%s",
		addr,
		url.QueryEscape("http://"+addr),
		url.QueryEscape(token),
		url.QueryEscape(chatID),
	)
}

// StartProcess spawns the current binary in PiP mode pointed at pipURL. It must
// NOT hide the child window: the PiP child is aw's own GUI (windows-GUI
// subsystem) exe, so there is no console to suppress, and SysProcAttr's
// HideWindow flag would set STARTUPINFO.wShowWindow=SW_HIDE, which Wails honors
// as the initial show state — leaving the PiP window invisible.
func StartProcess(pipURL string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "--aw-pip", "--pip-url", pipURL)
	cmd.Env = os.Environ()
	return cmd.Start()
}

func writeJSON(w http.ResponseWriter, value any, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(value)
}

func randomHex(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
