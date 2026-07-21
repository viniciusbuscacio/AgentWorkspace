package application

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"aw/internal/domain"
)

func (s *fakeChatConversationStore) ReplaceMessages(chatID string, messages []domain.Message) ([]domain.Message, error) {
	s.messages = s.messages[:0]
	persisted := make([]domain.Message, 0, len(messages))
	for i, message := range messages {
		message.ID = fmt.Sprintf("compacted-%d", i+1)
		message.SessionID = chatID
		persisted = append(persisted, message)
		s.messages = append(s.messages, message)
	}
	return persisted, nil
}

type fakeCompactionRuntime struct {
	reply             domain.AgentReply
	err               error
	resetErr          error
	prompt            string
	maxOutputTokens   int32
	resetChatID       string
	resetHistory      []domain.HistoryMessage
	receivedModel     string
	receivedProvider  string
	generateCallCount int
	resetCallCount    int
}

func (r *fakeCompactionRuntime) GenerateOneShot(_ context.Context, cfg domain.ModelConfig, text string, maxOutputTokens int32) (domain.AgentReply, error) {
	r.generateCallCount++
	r.prompt = text
	r.maxOutputTokens = maxOutputTokens
	r.receivedModel = cfg.Model
	r.receivedProvider = cfg.ProviderID
	if r.err != nil {
		return domain.AgentReply{}, r.err
	}
	return r.reply, nil
}

func (r *fakeCompactionRuntime) ResetSessionWithHistory(_ context.Context, _ domain.ModelConfig, chatID string, messages []domain.HistoryMessage) error {
	r.resetCallCount++
	r.resetChatID = chatID
	r.resetHistory = append([]domain.HistoryMessage(nil), messages...)
	return r.resetErr
}

func TestCompactChatHistorySummarizesOlderMessagesAndSeedsRuntime(t *testing.T) {
	store := newFakeChatConversationStore()
	store.messages = []domain.Message{
		{ID: "prev", SessionID: "chat-1", Role: "system", Content: CompactSummaryPrefix + "\nEarlier compacted facts."},
		{ID: "old-1", SessionID: "chat-1", Role: "user", Content: "RAW_OLD_ALPHA: analisar sidebar do aw."},
		{ID: "old-2", SessionID: "chat-1", Role: "assistant", Content: "RAW_OLD_ALPHA_RESULT: sidebar precisa de botao de fechar."},
		{ID: "old-3", SessionID: "chat-1", Role: "user", Content: "RAW_OLD_BETA: decidir icones Google Material."},
		{ID: "old-4", SessionID: "chat-1", Role: "assistant", Content: "RAW_OLD_BETA_RESULT: todos os icones devem vir do Google Icons."},
		{ID: "recent-1", SessionID: "chat-1", Role: "user", Content: "Recent 1: validar OpenRouter funcionando."},
		{ID: "recent-2", SessionID: "chat-1", Role: "assistant", Content: "Recent 2: OpenRouter responde e persiste mensagens."},
		{ID: "recent-3", SessionID: "chat-1", Role: "user", Content: "Recent 3: ajustar session-info com usage real."},
		{ID: "recent-4", SessionID: "chat-1", Role: "assistant", Content: "Recent 4: token usage capturado."},
		{ID: "recent-5", SessionID: "chat-1", Role: "user", Content: "Recent 5: preparar compact real."},
		{ID: "recent-6", SessionID: "chat-1", Role: "assistant", Content: "Recent 6: compact deve manter o fio."},
	}
	runtime := &fakeCompactionRuntime{
		reply: domain.AgentReply{Text: "`Alpha and Beta decisions were compacted; keep Gamma follow-up in mind.`"},
	}

	result, err := CompactChatHistory(context.Background(), store, store, runtime, CompactChatHistoryInput{ChatID: "chat-1"})
	if err != nil {
		t.Fatalf("CompactChatHistory() error = %v", err)
	}
	if result.Summary != "Alpha and Beta decisions were compacted; keep Gamma follow-up in mind." {
		t.Fatalf("summary = %q", result.Summary)
	}
	if len(result.Messages) != 7 {
		t.Fatalf("message count = %d, want summary + 6 recent", len(result.Messages))
	}
	if result.Messages[0].Role != "system" || !strings.Contains(result.Messages[0].Content, result.Summary) {
		t.Fatalf("first message = %+v, want compacted system summary", result.Messages[0])
	}
	if result.Messages[1].Content != "Recent 1: validar OpenRouter funcionando." || result.Messages[len(result.Messages)-1].Content != "Recent 6: compact deve manter o fio." {
		t.Fatalf("recent messages not preserved: %+v", result.Messages)
	}
	for _, message := range result.Messages {
		if strings.Contains(message.Content, "RAW_OLD_ALPHA") || strings.Contains(message.Content, "RAW_OLD_BETA") {
			t.Fatalf("old raw message survived compaction: %q", message.Content)
		}
	}
	if !strings.Contains(runtime.prompt, "Previous compacted summary:\n- Earlier compacted facts.") {
		t.Fatalf("prompt did not include previous summary: %q", runtime.prompt)
	}
	if !strings.Contains(runtime.prompt, "RAW_OLD_ALPHA") || strings.Contains(runtime.prompt, "Recent 6: compact deve manter o fio.\nOlder") {
		t.Fatalf("prompt has unexpected history sections: %q", runtime.prompt)
	}
	if runtime.maxOutputTokens != CompactSummaryMaxTokens {
		t.Fatalf("maxOutputTokens = %d", runtime.maxOutputTokens)
	}
	if runtime.receivedProvider != "openrouter" || runtime.receivedModel != "deepseek/deepseek-r1" {
		t.Fatalf("runtime provider/model = %q/%q", runtime.receivedProvider, runtime.receivedModel)
	}
	if runtime.resetChatID != "chat-1" || len(runtime.resetHistory) != 7 {
		t.Fatalf("reset chat/history = %q/%d", runtime.resetChatID, len(runtime.resetHistory))
	}
	if runtime.generateCallCount != 1 || runtime.resetCallCount != 1 {
		t.Fatalf("runtime calls generate=%d reset=%d", runtime.generateCallCount, runtime.resetCallCount)
	}
}

func TestCompactChatHistoryScrubsSecretsFromPromptAndPersistedSummary(t *testing.T) {
	store := newFakeChatConversationStore()
	secret := "sk-test_abcdefghijklmnopqrstuvwxyz1234567890"
	store.messages = []domain.Message{
		{Role: "user", Content: "Older user token " + secret},
		{Role: "assistant", Content: "Noted Bearer abcdefghijklmnopqrstuvwxyz123456"},
		{Role: "user", Content: "Older password=supersecretvalue"},
		{Role: "assistant", Content: "Older AWS AKIA1234567890ABCDEF"},
		{Role: "user", Content: "Recent 1"},
		{Role: "assistant", Content: "Recent 2"},
	}
	runtime := &fakeCompactionRuntime{
		reply: domain.AgentReply{Text: "Summary mentions " + secret + " and password=supersecretvalue"},
	}

	result, err := CompactChatHistory(context.Background(), store, store, runtime, CompactChatHistoryInput{ChatID: "chat-1", RecentMessages: 2})
	if err != nil {
		t.Fatalf("CompactChatHistory() error = %v", err)
	}
	for _, value := range []string{runtime.prompt, result.Summary, result.Messages[0].Content} {
		if strings.Contains(value, secret) || strings.Contains(value, "supersecretvalue") || strings.Contains(value, "AKIA1234567890ABCDEF") {
			t.Fatalf("secret survived scrub in %q", value)
		}
	}
	if !strings.Contains(runtime.prompt, "[redacted]") || !strings.Contains(result.Summary, "[redacted]") {
		t.Fatalf("expected redaction markers in prompt and summary, prompt=%q summary=%q", runtime.prompt, result.Summary)
	}
}

func TestCompactChatHistoryRejectsInvalidState(t *testing.T) {
	store := newFakeChatConversationStore()
	runtime := &fakeCompactionRuntime{reply: domain.AgentReply{Text: "summary"}}

	store.unlocked = false
	if _, err := CompactChatHistory(context.Background(), store, store, runtime, CompactChatHistoryInput{ChatID: "chat-1"}); err == nil || err.Error() != "vault is locked" {
		t.Fatalf("locked error = %v", err)
	}
	store.unlocked = true

	store.messages = []domain.Message{
		{Role: "user", Content: "one"},
		{Role: "assistant", Content: "two"},
		{Role: "system", Content: "ignored"},
	}
	if _, err := CompactChatHistory(context.Background(), store, store, runtime, CompactChatHistoryInput{ChatID: "chat-1"}); err == nil || err.Error() != "not enough chat history to compact" {
		t.Fatalf("short history error = %v", err)
	}

	if _, err := CompactChatHistory(context.Background(), store, store, nil, CompactChatHistoryInput{ChatID: "chat-1"}); err == nil || err.Error() != "agent runtime is not available" {
		t.Fatalf("nil runtime error = %v", err)
	}

	delete(store.secrets, activeProviderSecret)
	store.messages = []domain.Message{
		{Role: "user", Content: "one"},
		{Role: "assistant", Content: "two"},
		{Role: "user", Content: "three"},
		{Role: "assistant", Content: "four"},
	}
	if _, err := CompactChatHistory(context.Background(), store, store, runtime, CompactChatHistoryInput{ChatID: "chat-1"}); err == nil || !strings.Contains(strings.ToLower(err.Error()), "no provider") {
		t.Fatalf("provider error = %v", err)
	}
}

func TestCompactChatHistoryPropagatesRuntimeAndResetErrors(t *testing.T) {
	store := newFakeChatConversationStore()
	store.messages = []domain.Message{
		{Role: "user", Content: "one"},
		{Role: "assistant", Content: "two"},
		{Role: "user", Content: "three"},
		{Role: "assistant", Content: "four"},
	}

	_, err := CompactChatHistory(context.Background(), store, store, &fakeCompactionRuntime{err: fmt.Errorf("summary failed")}, CompactChatHistoryInput{ChatID: "chat-1"})
	if err == nil || err.Error() != "summary failed" {
		t.Fatalf("generate error = %v", err)
	}

	_, err = CompactChatHistory(context.Background(), store, store, &fakeCompactionRuntime{
		reply:    domain.AgentReply{Text: "summary"},
		resetErr: fmt.Errorf("reset failed"),
	}, CompactChatHistoryInput{ChatID: "chat-1"})
	if err == nil || err.Error() != "reset failed" {
		t.Fatalf("reset error = %v", err)
	}
}
