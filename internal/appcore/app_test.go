package appcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/infrastructure/agent"
	"aw/internal/infrastructure/appconfig"
	"aw/internal/infrastructure/profile"
	vaultpkg "aw/internal/infrastructure/vault"
)

func TestGetChatSessionInfoIncludesLastTokenUsage(t *testing.T) {
	v := vaultpkg.New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() { _ = v.Lock() }()
	if err := v.SetSecret("_config_provider", "openrouter"); err != nil {
		t.Fatalf("SetSecret(provider) error = %v", err)
	}
	if err := v.SetSecret("_config_model_openrouter", "deepseek/deepseek-r1"); err != nil {
		t.Fatalf("SetSecret(model) error = %v", err)
	}
	if err := v.SetSecret("openrouter_api_key", "sk-or-test"); err != nil {
		t.Fatalf("SetSecret(key) error = %v", err)
	}
	chat, err := v.CreateChat("Usage test")
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}
	if _, err := v.AddMessage(chat.ID, "user", "Qual e a capital do Brasil?"); err != nil {
		t.Fatalf("AddMessage() error = %v", err)
	}

	usageTracker := application.NewChatUsageTracker()
	usageTracker.Remember(chat.ID, &agent.TokenUsage{Input: 21, Output: 3, Total: 24})
	app := &App{
		agentErr:  nil,
		vault:     v,
		planModes: map[string]bool{},
		chatUsage: usageTracker,
	}

	info := app.GetChatSessionInfo(chat.ID)
	if !info.Ready {
		t.Fatalf("info.Ready = false, error = %q", info.Error)
	}
	if info.Provider != "openrouter" || info.Model != "deepseek/deepseek-r1" {
		t.Fatalf("provider/model = %q/%q", info.Provider, info.Model)
	}
	if info.MessageCount != 1 {
		t.Fatalf("MessageCount = %d, want 1", info.MessageCount)
	}
	if info.TokenUsage == nil || info.TokenUsage.Input != 21 || info.TokenUsage.Output != 3 || info.TokenUsage.Total != 24 {
		t.Fatalf("TokenUsage = %+v, want input=21 output=3 total=24", info.TokenUsage)
	}
	if info.LastUsage == nil || *info.LastUsage != *info.TokenUsage {
		t.Fatalf("LastUsage = %+v, TokenUsage = %+v; want same usage", info.LastUsage, info.TokenUsage)
	}
}

func TestCompactedRetryUserMessageUsesPersistedCompactedID(t *testing.T) {
	original := domain.Message{ID: "old-user-id", Role: "user", Content: "delete esse email da Claro"}
	messages := []domain.Message{
		{ID: "compacted-1", Role: "system", Content: application.CompactSummaryPrefix + "\nResumo"},
		{ID: "compacted-2", Role: "assistant", Content: "ok"},
		{ID: "compacted-3", Role: "user", Content: "delete esse email da Claro"},
	}

	got := compactedRetryUserMessage(messages, original)
	if got == nil || got.ID != "compacted-3" {
		t.Fatalf("compactedRetryUserMessage() = %+v, want compacted user id", got)
	}
}

func TestShouldAutoCompactBeforeSendAtNinetyPercent(t *testing.T) {
	v := vaultpkg.New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() { _ = v.Lock() }()
	if err := v.SetSecret("_config_provider", "openrouter"); err != nil {
		t.Fatalf("SetSecret(provider) error = %v", err)
	}
	if err := v.SetSecret("_config_model_openrouter", "deepseek/deepseek-r1"); err != nil {
		t.Fatalf("SetSecret(model) error = %v", err)
	}
	if err := v.SetSecret("openrouter_api_key", "sk-or-test"); err != nil {
		t.Fatalf("SetSecret(key) error = %v", err)
	}
	tracker := application.NewChatUsageTracker()
	tracker.Remember("chat-1", &domain.TokenUsage{Input: 116000, Total: 117000})
	app := &App{vault: v, chatUsage: tracker}
	if !app.shouldAutoCompactBeforeSend("chat-1") {
		t.Fatal("shouldAutoCompactBeforeSend = false at 90%+ context usage")
	}
}

func TestAutoRenameGenericChatAfterThirdTurn(t *testing.T) {
	// Isolate from the developer's real appconfig (added modules feed the
	// agent prompt through RefreshAgentContext).
	t.Setenv("aw_DATA_DIR", t.TempDir())
	var mu sync.Mutex
	var prompts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPrompt := decodeLastPrompt(t, r)
		mu.Lock()
		prompts = append(prompts, lastPrompt)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(lastPrompt, "Generate a concise chat title") {
			_, _ = w.Write([]byte(`{"model":"demo-model","choices":[{"message":{"role":"assistant","content":"\"Capital do Brasil.\""},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}`))
			return
		}
		_, _ = w.Write([]byte(`{"model":"demo-model","choices":[{"message":{"role":"assistant","content":"Brasilia."},"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}}`))
	}))
	defer server.Close()

	v := newConfiguredVault(t, server.URL)
	runtime, err := agent.NewRuntime()
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	app := &App{
		agent:     runtime,
		agentErr:  nil,
		vault:     v,
		planModes: map[string]bool{},
		chatUsage: application.NewChatUsageTracker(),
	}
	chat, err := v.CreateChat("New Chat")
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}

	// Turns 1 and 2 must NOT trigger an auto-rename (cadence starts on turn 3).
	for i := 0; i < 2; i++ {
		result := app.SendChatMessage(chat.ID, "Qual e a capital do Brasil?", nil)
		if !result.Success {
			t.Fatalf("SendChatMessage() error = %s", result.Error)
		}
	}
	time.Sleep(100 * time.Millisecond)
	if got := chatByID(t, v, chat.ID).Title; got != "New Chat" {
		t.Fatalf("after 2 turns title = %q, want still generic", got)
	}

	// Turn 3 triggers the initial auto-rename.
	result := app.SendChatMessage(chat.ID, "Qual e a capital do Brasil?", nil)
	if !result.Success {
		t.Fatalf("SendChatMessage() error = %s", result.Error)
	}

	waitForTestCondition(t, func() bool {
		return chatByID(t, v, chat.ID).Title == "Capital do Brasil"
	})

	mu.Lock()
	defer mu.Unlock()
	titlePrompts := 0
	titlePrompt := ""
	for _, p := range prompts {
		if strings.Contains(p, "Generate a concise chat title") {
			titlePrompts++
			titlePrompt = p
		}
	}
	if titlePrompts != 1 {
		t.Fatalf("title generation count = %d, want exactly 1", titlePrompts)
	}
	if !strings.Contains(titlePrompt, "Assistant reply:\nBrasilia.") {
		t.Fatalf("title prompt = %q, want assistant reply context", titlePrompt)
	}
}

func TestCreateChatNumbersSequentially(t *testing.T) {
	v := vaultpkg.New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() { _ = v.Lock() }()
	app := &App{vault: v, planModes: map[string]bool{}, chatUsage: application.NewChatUsageTracker()}

	// Start from a clean slate (dev vault seeds sample chats).
	existing, err := v.ListChats()
	if err != nil {
		t.Fatalf("ListChats() error = %v", err)
	}
	for _, c := range existing {
		if err := v.DeleteChat(c.ID); err != nil {
			t.Fatalf("DeleteChat() error = %v", err)
		}
	}

	first, err := app.CreateChat()
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}
	if first.Title != "Chat 1" {
		t.Fatalf("first chat title = %q, want %q", first.Title, "Chat 1")
	}
	second, err := app.CreateChat()
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}
	if second.Title != "Chat 2" {
		t.Fatalf("second chat title = %q, want %q", second.Title, "Chat 2")
	}

	// Custom title via CreateChatWithTitle is preserved verbatim.
	custom, err := app.CreateChatWithTitle("Planejamento")
	if err != nil {
		t.Fatalf("CreateChatWithTitle() error = %v", err)
	}
	if custom.Title != "Planejamento" {
		t.Fatalf("custom chat title = %q, want %q", custom.Title, "Planejamento")
	}

	// Numbering uses the smallest available number; the custom name is ignored.
	third, err := app.CreateChat()
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}
	if third.Title != "Chat 3" {
		t.Fatalf("third chat title = %q, want %q", third.Title, "Chat 3")
	}
}

func TestManualRenamePreventsAutoRename(t *testing.T) {
	t.Setenv("aw_DATA_DIR", t.TempDir())
	var mu sync.Mutex
	var titleRequests int
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		lastPrompt := decodeLastPrompt(t, r)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(lastPrompt, "Generate a concise chat title") {
			mu.Lock()
			titleRequests++
			mu.Unlock()
			_, _ = w.Write([]byte(`{"model":"demo-model","choices":[{"message":{"role":"assistant","content":"Should Not Happen"},"finish_reason":"stop"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"model":"demo-model","choices":[{"message":{"role":"assistant","content":"Done."},"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}}`))
	}))
	defer server.Close()

	v := newConfiguredVault(t, server.URL)
	runtime, err := agent.NewRuntime()
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	app := &App{
		agent:     runtime,
		agentErr:  nil,
		vault:     v,
		planModes: map[string]bool{},
		chatUsage: application.NewChatUsageTracker(),
	}
	chat, err := v.CreateChat("New Chat")
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}
	renameResult := app.RenameChat(chat.ID, "New Chat")
	if !renameResult.Success {
		t.Fatalf("RenameChat() error = %s", renameResult.Error)
	}

	// Seed preferred-language so the deterministic language backfill no-ops —
	// this test counts LLM requests and only tolerates the main turn.
	if _, err := v.SetUserMemoryDoc(domain.UserMemoryDoc{Content: "- [preference] preferred-language: Português"}); err != nil {
		t.Fatalf("seed memory doc: %v", err)
	}
	result := app.SendChatMessage(chat.ID, "Nao renomeie este chat.", nil)
	if !result.Success {
		t.Fatalf("SendChatMessage() error = %s", result.Error)
	}

	renamed := chatByID(t, v, chat.ID)
	if renamed.Title != "New Chat" {
		t.Fatalf("chat title = %q, want manual title preserved", renamed.Title)
	}
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if requests != 1 || titleRequests != 0 {
		t.Fatalf("requests = %d, titleRequests = %d; want one main request and no title generation", requests, titleRequests)
	}
}

func TestCompactChatSummarizesHistoryAndSeedsRuntime(t *testing.T) {
	t.Setenv("aw_DATA_DIR", t.TempDir())
	var mu sync.Mutex
	var normalRequests [][]string
	var summaryRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prompts := decodePromptMessages(t, r)
		lastPrompt := prompts[len(prompts)-1]
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(lastPrompt, "Create a compact searchable summary") {
			mu.Lock()
			summaryRequests++
			mu.Unlock()
			_, _ = w.Write([]byte(`{"model":"demo-model","choices":[{"message":{"role":"assistant","content":"Alpha and Beta decisions were compacted; keep the Gamma follow-up in mind."},"finish_reason":"stop"}],"usage":{"prompt_tokens":120,"completion_tokens":20,"total_tokens":140}}`))
			return
		}
		mu.Lock()
		normalRequests = append(normalRequests, prompts)
		mu.Unlock()
		_, _ = w.Write([]byte(`{"model":"demo-model","choices":[{"message":{"role":"assistant","content":"Continuing from the compacted context."},"finish_reason":"stop"}],"usage":{"prompt_tokens":50,"completion_tokens":8,"total_tokens":58}}`))
	}))
	defer server.Close()

	v := newConfiguredVault(t, server.URL)
	runtime, err := agent.NewRuntime()
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	app := &App{
		agent:     runtime,
		agentErr:  nil,
		vault:     v,
		planModes: map[string]bool{},
		chatUsage: application.NewChatUsageTracker(),
	}
	chat, err := v.CreateChat("Compaction Test")
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}
	seed := []struct {
		role    string
		content string
	}{
		{"user", "RAW_OLD_ALPHA: analisar sidebar do aw."},
		{"assistant", "RAW_OLD_ALPHA_RESULT: sidebar precisa de botao de fechar."},
		{"user", "RAW_OLD_BETA: decidir icones Google Material."},
		{"assistant", "RAW_OLD_BETA_RESULT: todos os icones devem vir do Google Icons."},
		{"user", "Recent 1: validar OpenRouter funcionando."},
		{"assistant", "Recent 2: OpenRouter responde e persiste mensagens."},
		{"user", "Recent 3: ajustar session-info com usage real."},
		{"assistant", "Recent 4: token usage capturado."},
		{"user", "Recent 5: preparar compact real."},
		{"assistant", "Recent 6: compact deve manter o fio."},
	}
	for _, message := range seed {
		if _, err := v.AddMessage(chat.ID, message.role, message.content); err != nil {
			t.Fatalf("AddMessage(%s) error = %v", message.content, err)
		}
	}

	result := app.CompactChat(chat.ID)
	if !result.Success {
		t.Fatalf("CompactChat() error = %s", result.Error)
	}
	messages, err := v.ListMessages(chat.ID)
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(messages) != 7 {
		t.Fatalf("compacted message count = %d, want summary + 6 recent messages", len(messages))
	}
	if messages[0].Role != "system" || !strings.Contains(messages[0].Content, "Alpha and Beta decisions were compacted") {
		t.Fatalf("first compacted message = %#v, want system summary", messages[0])
	}
	for _, message := range messages {
		if strings.Contains(message.Content, "RAW_OLD_ALPHA") || strings.Contains(message.Content, "RAW_OLD_BETA") {
			t.Fatalf("old raw message survived compaction: %q", message.Content)
		}
	}
	if messages[1].Content != "Recent 1: validar OpenRouter funcionando." || messages[len(messages)-1].Content != "Recent 6: compact deve manter o fio." {
		t.Fatalf("recent messages not preserved in order: %#v", messages)
	}

	send := app.SendChatMessage(chat.ID, "Continue a partir do contexto compactado.", nil)
	if !send.Success {
		t.Fatalf("SendChatMessage() error = %s", send.Error)
	}
	mu.Lock()
	defer mu.Unlock()
	if summaryRequests != 1 {
		t.Fatalf("summaryRequests = %d, want 1", summaryRequests)
	}
	if len(normalRequests) != 1 {
		t.Fatalf("normalRequests = %d, want 1", len(normalRequests))
	}
	joined := strings.Join(normalRequests[0], "\n")
	if !strings.Contains(joined, "Alpha and Beta decisions were compacted") {
		t.Fatalf("follow-up request did not include compacted summary: %q", joined)
	}
	if !strings.Contains(joined, "Recent 6: compact deve manter o fio.") {
		t.Fatalf("follow-up request did not include recent context: %q", joined)
	}
	if strings.Contains(joined, "RAW_OLD_ALPHA") || strings.Contains(joined, "RAW_OLD_BETA") {
		t.Fatalf("follow-up request included raw pre-compaction history: %q", joined)
	}
}

func TestFallbackChatTitleDoesNotCutKeywordBeforeFiltering(t *testing.T) {
	title := application.FallbackChatTitle("Explique em uma frase o que e fotossintese.", "", 0)
	if title != "Fotossintese" {
		t.Fatalf("FallbackChatTitle() = %q, want Fotossintese", title)
	}
}

func newConfiguredVault(t *testing.T, baseURL string) *vaultpkg.Vault {
	t.Helper()
	v := vaultpkg.New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	secrets := map[string]string{
		"_config_provider":               "custom-openai",
		"_config_model_custom_openai":    "demo-model",
		"custom_openai_api_key":          "test-key",
		"_config_base_url_custom_openai": baseURL,
	}
	for name, value := range secrets {
		if err := v.SetSecret(name, value); err != nil {
			t.Fatalf("SetSecret(%s) error = %v", name, err)
		}
	}
	return v
}

func decodeLastPrompt(t *testing.T, r *http.Request) string {
	t.Helper()
	messages := decodePromptMessages(t, r)
	return messages[len(messages)-1]
}

func decodePromptMessages(t *testing.T, r *http.Request) []string {
	t.Helper()
	var req struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if len(req.Messages) == 0 {
		t.Fatalf("request messages empty")
	}
	messages := make([]string, 0, len(req.Messages))
	for _, message := range req.Messages {
		messages = append(messages, message.Content)
	}
	return messages
}

func chatByID(t *testing.T, v *vaultpkg.Vault, id string) vaultpkg.Chat {
	t.Helper()
	chats, err := v.ListChats()
	if err != nil {
		t.Fatalf("ListChats() error = %v", err)
	}
	for _, chat := range chats {
		if chat.ID == id {
			return chat
		}
	}
	t.Fatalf("chat %s not found", id)
	return vaultpkg.Chat{}
}

func waitForTestCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	if condition() {
		return
	}
	t.Fatalf("condition did not become true before deadline")
}

func TestAppZoomPercentPersistsAndClamps(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("aw_DATA_DIR", t.TempDir())

	app := &App{}
	if got := app.GetAppZoomPercent(); got != application.AppZoomDefaultPercent {
		t.Fatalf("GetAppZoomPercent() = %d, want default %d", got, application.AppZoomDefaultPercent)
	}

	if result := app.SetAppZoomPercent(500); !result.Success {
		t.Fatalf("SetAppZoomPercent(high) failed: %s", result.Error)
	}
	if got := app.GetAppZoomPercent(); got != application.AppZoomMaxPercent {
		t.Fatalf("GetAppZoomPercent() after high clamp = %d, want %d", got, application.AppZoomMaxPercent)
	}

	if result := app.SetAppZoomPercent(10); !result.Success {
		t.Fatalf("SetAppZoomPercent(low) failed: %s", result.Error)
	}
	if got := app.GetAppZoomPercent(); got != application.AppZoomMinPercent {
		t.Fatalf("GetAppZoomPercent() after low clamp = %d, want %d", got, application.AppZoomMinPercent)
	}
}

func TestToolOptionsSelfDevMode(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module aw\n"), 0o644); err != nil {
		t.Fatalf("seed go.mod: %v", err)
	}
	workspace := t.TempDir()
	app := &App{workspaceRoot: workspace}

	normal, extra, err := app.toolOptions(appconfig.Config{})
	if err != nil {
		t.Fatalf("toolOptions(normal) error = %v", err)
	}
	// AutoApprove is intentionally on even in normal mode: the Permissions
	// sandbox (permit_list by default) is the real fence, so the redundant
	// per-action confirmation popup is skipped. Shell is on in every chat —
	// the sandbox gates each call — while self-manage stays self-dev-only.
	if normal.Root != workspace || !normal.AutoApprove || !normal.AllowShell || normal.SelfManage {
		t.Fatalf("normal options = %+v, want confined workspace with auto-approve, shell on, no self-manage", normal)
	}
	if normal.SandboxPolicyFn == nil {
		t.Fatal("normal SandboxPolicyFn = nil, want per-dispatch permissions policy")
	}
	normalPolicy := normal.SandboxPolicyFn()
	if normalPolicy.Config.Mode != domain.SandboxPermitList {
		t.Fatalf("normal sandbox mode = %q, want permit_list (fail-closed default)", normalPolicy.Config.Mode)
	}
	if normalPolicy.WorkspaceRoot != workspace || normalPolicy.DataDir == "" {
		t.Fatalf("normal sandbox policy = %+v, want workspace root and data dir set", normalPolicy)
	}
	if extra != "" {
		t.Fatalf("normal instruction extra = %q, want empty", extra)
	}

	selfDev, extra, err := app.toolOptions(appconfig.Config{
		SelfDev: appconfig.SelfDevConfig{Enabled: true, RepoRoot: repo},
	})
	if err != nil {
		t.Fatalf("toolOptions(self-dev) error = %v", err)
	}
	if selfDev.Root != repo {
		t.Fatalf("selfDev.Root = %q, want %q", selfDev.Root, repo)
	}
	if !selfDev.AutoApprove || !selfDev.AllowShell || !selfDev.SelfManage {
		t.Fatalf("self-dev options = %+v, want auto-approve shell self-manage", selfDev)
	}
	if selfDev.SandboxPolicyFn == nil {
		t.Fatal("self-dev SandboxPolicyFn = nil")
	}
	selfDevPolicy := selfDev.SandboxPolicyFn()
	if selfDevPolicy.Config.Mode != domain.SandboxPermitAll {
		t.Fatalf("self-dev sandbox mode = %q, want permit_all (replaces Unconfined)", selfDevPolicy.Config.Mode)
	}
	if selfDevPolicy.WorkspaceRoot != repo {
		t.Fatalf("self-dev sandbox workspace = %q, want repo %q", selfDevPolicy.WorkspaceRoot, repo)
	}
	if selfDev.StateFn == nil {
		t.Fatalf("self-dev StateFn = nil, want system.state callback")
	}
	if !strings.Contains(extra, repo) || !strings.Contains(extra, "shell.exec") || !strings.Contains(extra, "aw tool") {
		t.Fatalf("instruction extra = %q, want repo root, shell.exec and aw tool", extra)
	}
}

func TestApplyProfileLocksBeforeSwitchAndIsIdempotent(t *testing.T) {
	// Isolate appconfig.Save writes to a throwaway HOME.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("aw_DATA_DIR", t.TempDir())

	manager := profile.NewManager(t.TempDir())
	parent := t.TempDir()
	first, err := manager.CreateAtLocation(parent, "VaultOne", "database_upload")
	if err != nil {
		t.Fatalf("CreateAtLocation(first) error = %v", err)
	}
	second, err := manager.CreateAtLocation(parent, "VaultTwo", "database_upload")
	if err != nil {
		t.Fatalf("CreateAtLocation(second) error = %v", err)
	}

	v := vaultpkg.New(first.VaultDir)
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })

	app := &App{
		vault:            v,
		profileManager:   manager,
		currentProfileID: first.ID,
	}

	// Re-selecting the already-open vault must succeed and keep it unlocked.
	result, err := application.ApplyProfile(v, appconfig.Store{}, app.currentProfileID, first)
	app.applyCurrentProfileID(result.CurrentProfileID)
	if err != nil {
		t.Fatalf("applyProfile(same) error = %v, want nil", err)
	}
	if !v.IsUnlocked() {
		t.Fatalf("vault should remain unlocked when reselecting the current profile")
	}

	// Switching to a different vault while unlocked must lock + switch, not error.
	result, err = application.ApplyProfile(v, appconfig.Store{}, app.currentProfileID, second)
	app.applyCurrentProfileID(result.CurrentProfileID)
	if err != nil {
		t.Fatalf("applyProfile(other) error = %v, want nil", err)
	}
	if v.IsUnlocked() {
		t.Fatalf("vault should be locked after switching to a different profile")
	}
	if app.currentProfileID != second.ID {
		t.Fatalf("currentProfileID = %q, want %q", app.currentProfileID, second.ID)
	}
	if v.Dir() != second.VaultDir {
		t.Fatalf("vault dir = %q, want %q", v.Dir(), second.VaultDir)
	}
}

func TestDialogAffirmative(t *testing.T) {
	cases := []struct {
		choice string
		label  string
		want   bool
	}{
		// macOS/Linux return the custom label verbatim.
		{"Delete", "Delete", true},
		{"delete", "Delete", true},
		{"  Change location  ", "Change location", true},
		// Windows ignores custom Buttons and returns "Yes"/"No" for a
		// QuestionDialog; "Yes" must count as approval (the reported bug).
		{"Yes", "Delete", true},
		{"yes", "Remove", true},
		// Anything else declines.
		{"No", "Delete", false},
		{"Cancel", "Delete", false},
		{"", "Delete", false},
	}
	for _, tc := range cases {
		if got := dialogAffirmative(tc.choice, tc.label); got != tc.want {
			t.Errorf("dialogAffirmative(%q, %q) = %v, want %v", tc.choice, tc.label, got, tc.want)
		}
	}
}
