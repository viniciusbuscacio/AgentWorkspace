package pip

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"aw/internal/domain"
)

type fakeBackend struct {
	voiceStarted bool
	voiceStopped bool
	transcribed  string
	ops          []string
}

func (f *fakeBackend) ChatSnapshot(chatID string) (Snapshot, error) {
	return Snapshot{Chat: &domain.Chat{ID: chatID, Title: "Chat"}, Messages: []domain.Message{}}, nil
}

func (f *fakeBackend) SendMessage(string, string, []domain.Attachment) error { return nil }
func (f *fakeBackend) StopChat(string) error                                 { return nil }

func (f *fakeBackend) StartVoiceCapture() VoiceResult {
	f.voiceStarted = true
	return VoiceResult{Success: true}
}

func (f *fakeBackend) StopVoiceCapture() VoiceResult {
	f.voiceStopped = true
	return VoiceResult{Success: true, Text: "ola"}
}

func (f *fakeBackend) CancelVoiceCapture() VoiceResult { return VoiceResult{Success: true} }

func (f *fakeBackend) TranscribeAudio(fileName, _, _ string) VoiceResult {
	f.transcribed = fileName
	return VoiceResult{Success: true, Text: "fala-ok"}
}

func (f *fakeBackend) SessionInfo(string) any { return map[string]any{"ready": true} }
func (f *fakeBackend) GetPlanMode(string) any { return map[string]any{"success": true} }
func (f *fakeBackend) SetPlanMode(string, bool) any {
	return map[string]any{"success": true}
}

func (f *fakeBackend) SetChatModel(_ string, provider string, model string) any {
	return map[string]any{"success": true, "provider": provider, "model": model}
}

func (f *fakeBackend) ResolveToolConfirmation(string, bool) any {
	return map[string]any{"success": true}
}

func (f *fakeBackend) ChatOp(action string, _ string) any {
	f.ops = append(f.ops, action)
	return map[string]any{"success": true}
}

func postJSON(t *testing.T, url string, body string) map[string]any {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s = HTTP %d", url, resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestServerVoiceAndOpEndpoints(t *testing.T) {
	backend := &fakeBackend{}
	server, err := NewServer(backend, NewHub())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Shutdown(t.Context()) })
	base := fmt.Sprintf("http://%s", server.Addr())
	auth := "token=" + server.Token()

	if out := postJSON(t, base+"/api/voice/start?"+auth, "{}"); out["success"] != true {
		t.Fatalf("voice/start = %v", out)
	}
	out := postJSON(t, base+"/api/voice/stop?"+auth, "{}")
	if out["success"] != true || out["text"] != "ola" {
		t.Fatalf("voice/stop = %v", out)
	}
	out = postJSON(t, base+"/api/transcribe?"+auth, `{"fileName":"a.webm","mimeType":"audio/webm","dataUri":"data:;base64,AA=="}`)
	if out["text"] != "fala-ok" || backend.transcribed != "a.webm" {
		t.Fatalf("transcribe = %v (file %q)", out, backend.transcribed)
	}
	if out := postJSON(t, base+"/api/chat-op?"+auth, `{"action":"compact","chatId":"c1"}`); out["success"] != true {
		t.Fatalf("chat-op = %v", out)
	}
	if !backend.voiceStarted || !backend.voiceStopped || len(backend.ops) != 1 || backend.ops[0] != "compact" {
		t.Fatalf("backend calls: started=%v stopped=%v ops=%v", backend.voiceStarted, backend.voiceStopped, backend.ops)
	}

	resp, err := http.Post(base+"/api/voice/start?token=wrong", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong token = HTTP %d, want 401", resp.StatusCode)
	}
}

func TestParseConfigAndScript(t *testing.T) {
	url := BuildURL("127.0.0.1:9999", "tok-1", "chat-1")
	config := ParseConfig(url)
	if config.API != "http://127.0.0.1:9999" || config.Token != "tok-1" || config.ChatID != "chat-1" {
		t.Fatalf("ParseConfig = %+v", config)
	}
	script := ConfigScript(url)
	if !strings.Contains(script, `"chatId":"chat-1"`) || !strings.Contains(script, "window.aw_PIP") {
		t.Fatalf("ConfigScript = %s", script)
	}
}
