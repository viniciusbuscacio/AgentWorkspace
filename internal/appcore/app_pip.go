package appcore

import (
	"context"
	"errors"
	"strings"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/infrastructure/pip"
)

// pipBackend adapts the App (interface/composition root) to the pip.Backend
// port consumed by the infrastructure PiP server. It keeps the PiP subsystem
// decoupled from the concrete App type.
type pipBackend struct {
	app *App
}

func (b pipBackend) ChatSnapshot(chatID string) (pip.Snapshot, error) {
	result, err := application.ChatSnapshot(b.app.vault, chatID)
	if err != nil {
		return pip.Snapshot{}, err
	}
	return pip.Snapshot{Chat: result.Chat, Messages: result.Messages}, nil
}

func (b pipBackend) SendMessage(chatID string, text string, attachments []domain.Attachment) error {
	result := b.app.StreamChatMessage(chatID, text, attachments)
	if !result.Success {
		return errors.New(result.Error)
	}
	return nil
}

func (b pipBackend) StopChat(chatID string) error {
	result := b.app.StopChat(chatID)
	if !result.Success {
		return errors.New(result.Error)
	}
	return nil
}

func (b pipBackend) StartVoiceCapture() pip.VoiceResult {
	result := b.app.StartVoiceCapture()
	return pip.VoiceResult{Success: result.Success, Error: result.Error}
}

func (b pipBackend) StopVoiceCapture() pip.VoiceResult {
	result := b.app.StopVoiceCapture()
	return pip.VoiceResult{Success: result.Success, Error: result.Error, Text: result.Text}
}

func (b pipBackend) CancelVoiceCapture() pip.VoiceResult {
	result := b.app.CancelVoiceCapture()
	return pip.VoiceResult{Success: result.Success, Error: result.Error}
}

func (b pipBackend) TranscribeAudio(fileName string, mimeType string, dataURI string) pip.VoiceResult {
	result := b.app.TranscribeAudio(fileName, mimeType, dataURI)
	return pip.VoiceResult{Success: result.Success, Error: result.Error, Text: result.Text}
}

func (b pipBackend) SessionInfo(chatID string) any {
	return b.app.GetChatSessionInfo(chatID)
}

func (b pipBackend) GetPlanMode(chatID string) any {
	return b.app.GetPlanMode(chatID)
}

func (b pipBackend) SetPlanMode(chatID string, enabled bool) any {
	return b.app.SetPlanMode(chatID, enabled)
}

func (b pipBackend) SetChatModel(chatID string, provider string, model string) any {
	return b.app.SetChatModel(chatID, provider, model)
}

func (b pipBackend) ResolveToolConfirmation(id string, approved bool) any {
	return b.app.ResolveToolConfirmation(id, approved)
}

func (b pipBackend) ChatOp(action string, chatID string) any {
	switch action {
	case "new":
		return b.app.NewChatSession(chatID)
	case "compact":
		return b.app.CompactChat(chatID)
	case "clear":
		return b.app.ClearChat(chatID)
	default:
		return map[string]any{"success": false, "error": "unknown chat operation"}
	}
}

// broadcastPipEvent forwards a chat event to all open PiP windows.
func (a *App) broadcastPipEvent(name string, payload map[string]any) {
	if a == nil || a.pipHub == nil {
		return
	}
	a.pipHub.Broadcast(pip.Event{Name: name, Payload: payload})
}

// OpenPipWindow opens the current chat in a separate always-on-top process.
// Wails v2 does not expose Electron-style BrowserWindow creation from one app
// process, so aw spawns the same binary in a small PiP mode and serves it a
// token-scoped localhost API backed by the already-unlocked main app.
func (a *App) OpenPipWindow(chatID string) OperationResult {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return OperationResult{Success: false, Error: "chat is required"}
	}
	if _, err := application.ChatSnapshot(a.vault, chatID); err != nil {
		return OperationResult{Success: false, Error: err.Error()}
	}
	server, err := a.ensurePipServer()
	if err != nil {
		return OperationResult{Success: false, Error: err.Error()}
	}
	if err := application.LaunchPipProcess(pip.NewLauncher(), pip.BuildURL(server.Addr(), server.Token(), chatID)); err != nil {
		return OperationResult{Success: false, Error: err.Error()}
	}
	return OperationResult{Success: true}
}

func (a *App) ensurePipServer() (*pip.Server, error) {
	a.pipMu.Lock()
	defer a.pipMu.Unlock()
	if a.pipServer != nil {
		return a.pipServer, nil
	}
	server, err := pip.NewServer(pipBackend{app: a}, a.pipHub)
	if err != nil {
		return nil, err
	}
	a.pipServer = server
	return server, nil
}

func (a *App) closePipServer(ctx context.Context) {
	a.pipMu.Lock()
	server := a.pipServer
	a.pipServer = nil
	a.pipMu.Unlock()
	server.Shutdown(ctx)
}
