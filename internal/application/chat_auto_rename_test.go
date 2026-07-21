package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"aw/internal/domain"
)

func chatTurns(count int) []domain.Message {
	messages := make([]domain.Message, 0, count*2)
	for i := 1; i <= count; i++ {
		messages = append(messages,
			domain.Message{ID: fmt.Sprintf("user-%d", i), Role: "user", Content: fmt.Sprintf("Pergunta %d", i)},
			domain.Message{ID: fmt.Sprintf("assistant-%d", i), Role: "assistant", Content: fmt.Sprintf("Resposta %d", i)},
		)
	}
	return messages
}

func TestMaybeAutoRenameChatRenamesGenericChatOnThirdTurn(t *testing.T) {
	store := newFakeChatConversationStore()
	store.chats = []domain.Chat{{ID: "chat-1", Title: "New Chat"}}
	store.messages = chatTurns(3)
	runtime := &fakeCompactionRuntime{reply: domain.AgentReply{Text: "TITLE: Capital do Brasil\nSUMMARY: O usuario perguntou a capital do Brasil; o assistente respondeu Brasilia."}}

	result, err := MaybeAutoRenameChat(context.Background(), store, runtime, AutoRenameChatInput{
		ChatID:         "chat-1",
		UserMessage:    "Qual e a capital do Brasil?",
		AssistantReply: "Brasilia.",
		ModelConfig:    domain.ModelConfig{ProviderID: "openrouter", Model: "demo-model", AuthType: "api-key"},
	})
	if err != nil {
		t.Fatalf("MaybeAutoRenameChat() error = %v", err)
	}
	if !result.Renamed || result.Title != "Capital do Brasil" || result.Chat.ID != "chat-1" {
		t.Fatalf("result = %+v, want renamed Capital do Brasil", result)
	}
	if runtime.generateCallCount != 1 {
		t.Fatalf("generateCallCount = %d, want 1", runtime.generateCallCount)
	}
	if got := store.secrets[ChatAutoRenameCountPrefix+"chat-1"]; got != "1" {
		t.Fatalf("auto rename count = %q", got)
	}
	if got := store.secrets[ChatAutoRenameLastTurnPrefix+"chat-1"]; got != "3" {
		t.Fatalf("last turn = %q", got)
	}
	if got := store.secrets[ChatAutoRenamedPrefix+"chat-1"]; got != "true" {
		t.Fatalf("legacy renamed flag = %q", got)
	}
	titles, err := store.ListChatTitles("chat-1")
	if err != nil {
		t.Fatalf("ListChatTitles() error = %v", err)
	}
	if len(titles) != 1 || titles[0].Title != "Capital do Brasil" || titles[0].Source != "auto" || titles[0].Turn != 3 {
		t.Fatalf("title history = %+v, want one auto entry at turn 3", titles)
	}
	if got := store.summaries["chat-1"]; got.summary == "" || got.turn != 3 {
		t.Fatalf("session summary = %+v, want non-empty summary at turn 3", got)
	}
}

func TestMaybeAutoRenameChatScrubsSecretsFromPromptAndPersistedValues(t *testing.T) {
	store := newFakeChatConversationStore()
	store.chats = []domain.Chat{{ID: "chat-1", Title: "New Chat"}}
	store.messages = chatTurns(3)
	secret := "sk-test_abcdefghijklmnopqrstuvwxyz1234567890"
	runtime := &fakeCompactionRuntime{reply: domain.AgentReply{Text: "TITLE: Token Debug\nSUMMARY: O segredo era " + secret + " e password=supersecretvalue."}}

	result, err := MaybeAutoRenameChat(context.Background(), store, runtime, AutoRenameChatInput{
		ChatID:         "chat-1",
		UserMessage:    "Meu token e " + secret,
		AssistantReply: "Use Bearer abcdefghijklmnopqrstuvwxyz123456 para autenticar.",
		ModelConfig:    domain.ModelConfig{ProviderID: "openrouter", Model: "demo-model", AuthType: "api-key"},
	})
	if err != nil {
		t.Fatalf("MaybeAutoRenameChat() error = %v", err)
	}
	if !result.Renamed {
		t.Fatalf("result = %+v, want renamed", result)
	}
	persistedSummary := store.summaries["chat-1"].summary
	for _, value := range []string{runtime.prompt, result.Title, persistedSummary} {
		if strings.Contains(value, secret) || strings.Contains(value, "supersecretvalue") || strings.Contains(value, "abcdefghijklmnopqrstuvwxyz123456") {
			t.Fatalf("secret survived scrub in %q", value)
		}
	}
	if !strings.Contains(runtime.prompt, "[redacted]") || !strings.Contains(persistedSummary, "[redacted]") {
		t.Fatalf("expected redaction markers in prompt and summary, prompt=%q summary=%q", runtime.prompt, persistedSummary)
	}
}

func TestMaybeAutoRenameChatSkipsManualAndSpecialChats(t *testing.T) {
	tests := []struct {
		name   string
		title  string
		manual bool
		reason string
	}{
		{name: "manual", title: "New Chat", manual: true, reason: "manual-rename"},
		{name: "special", title: "Spec: Billing", reason: "special-prefix"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeChatConversationStore()
			store.chats = []domain.Chat{{ID: "chat-1", Title: tt.title}}
			store.messages = chatTurns(3)
			if tt.manual {
				store.secrets[ChatManualRenamedKey("chat-1")] = "true"
			}
			runtime := &fakeCompactionRuntime{reply: domain.AgentReply{Text: "Should Not Happen"}}

			result, err := MaybeAutoRenameChat(context.Background(), store, runtime, AutoRenameChatInput{
				ChatID:         "chat-1",
				UserMessage:    "Pergunta",
				AssistantReply: "Resposta",
				ModelConfig:    domain.ModelConfig{ProviderID: "openrouter", Model: "demo-model", AuthType: "api-key"},
			})
			if err != nil {
				t.Fatalf("MaybeAutoRenameChat() error = %v", err)
			}
			if result.Renamed || result.Reason != tt.reason {
				t.Fatalf("result = %+v, want no rename reason %q", result, tt.reason)
			}
			if runtime.generateCallCount != 0 {
				t.Fatalf("generateCallCount = %d, want 0", runtime.generateCallCount)
			}
		})
	}
}

func TestMaybeAutoRenameChatHonorsCadenceAndPeriodicRetitle(t *testing.T) {
	store := newFakeChatConversationStore()
	store.chats = []domain.Chat{{ID: "chat-1", Title: "Capital do Brasil"}}
	store.messages = chatTurns(10)
	store.secrets[ChatAutoRenameCountPrefix+"chat-1"] = "1"
	store.secrets[ChatAutoRenameLastTurnPrefix+"chat-1"] = "3"
	runtime := &fakeCompactionRuntime{reply: domain.AgentReply{Text: "Capital Federal"}}

	result, err := MaybeAutoRenameChat(context.Background(), store, runtime, AutoRenameChatInput{
		ChatID:         "chat-1",
		UserMessage:    "Atualize o titulo",
		AssistantReply: "Falamos da capital federal.",
		ModelConfig:    domain.ModelConfig{ProviderID: "openrouter", Model: "demo-model", AuthType: "api-key"},
	})
	if err != nil {
		t.Fatalf("MaybeAutoRenameChat() error = %v", err)
	}
	if !result.Renamed || result.Title != "Capital Federal" {
		t.Fatalf("result = %+v, want periodic rename", result)
	}
	if got := store.secrets[ChatAutoRenameCountPrefix+"chat-1"]; got != "2" {
		t.Fatalf("auto rename count = %q", got)
	}
	if got := store.secrets[ChatAutoRenameLastTurnPrefix+"chat-1"]; got != strconv.Itoa(10) {
		t.Fatalf("last turn = %q", got)
	}
}

func TestMaybeAutoRenameChatUsesFallbackWhenModelTitleFails(t *testing.T) {
	store := newFakeChatConversationStore()
	store.chats = []domain.Chat{{ID: "chat-1", Title: "New Chat"}}
	store.messages = chatTurns(3)
	runtime := &fakeCompactionRuntime{err: fmt.Errorf("title generation failed")}

	result, err := MaybeAutoRenameChat(context.Background(), store, runtime, AutoRenameChatInput{
		ChatID:         "chat-1",
		UserMessage:    "Explique em uma frase o que e fotossintese.",
		AssistantReply: "",
		ModelConfig:    domain.ModelConfig{ProviderID: "openrouter", Model: "demo-model", AuthType: "api-key"},
	})
	if err != nil {
		t.Fatalf("MaybeAutoRenameChat() error = %v", err)
	}
	if !result.Renamed || result.Title != "Fotossintese" {
		t.Fatalf("result = %+v, want fallback title Fotossintese", result)
	}
}

func TestMaybeAutoRenameChatKeepsTitlesUnique(t *testing.T) {
	store := newFakeChatConversationStore()
	store.chats = []domain.Chat{
		{ID: "chat-1", Title: "New Chat"},
		{ID: "chat-2", Title: "Capital do Brasil"},
	}
	store.messages = chatTurns(3)
	runtime := &fakeCompactionRuntime{reply: domain.AgentReply{Text: "Capital do Brasil"}}

	result, err := MaybeAutoRenameChat(context.Background(), store, runtime, AutoRenameChatInput{
		ChatID:         "chat-1",
		UserMessage:    "Qual e a capital do Brasil?",
		AssistantReply: "Brasilia.",
		ModelConfig:    domain.ModelConfig{ProviderID: "openrouter", Model: "demo-model", AuthType: "api-key"},
	})
	if err != nil {
		t.Fatalf("MaybeAutoRenameChat() error = %v", err)
	}
	if !result.Renamed || result.Title != "Capital do Brasil 2" {
		t.Fatalf("result = %+v, want unique suffix", result)
	}
}

func TestMarkChatManuallyRenamedRecordsTitleAndStopsAutoRename(t *testing.T) {
	store := newFakeChatConversationStore()

	if err := MarkChatManuallyRenamed(store, "chat-1", "My Custom Title"); err != nil {
		t.Fatalf("MarkChatManuallyRenamed() error = %v", err)
	}

	if got := store.secrets[ChatManualRenamedKey("chat-1")]; got != "true" {
		t.Fatalf("manual flag = %q, want true", got)
	}
	titles, err := store.ListChatTitles("chat-1")
	if err != nil {
		t.Fatalf("ListChatTitles() error = %v", err)
	}
	if len(titles) != 1 || titles[0].Title != "My Custom Title" || titles[0].Source != "manual" {
		t.Fatalf("title history = %+v, want one manual entry", titles)
	}
}

func TestParseTitleAndSummary(t *testing.T) {
	title, summary := ParseTitleAndSummary("TITLE: Capital do Brasil\nSUMMARY: O usuario perguntou a capital.\nÉ Brasilia.", 40, 240)
	if title != "Capital do Brasil" {
		t.Fatalf("title = %q", title)
	}
	if summary != "O usuario perguntou a capital. É Brasilia." {
		t.Fatalf("summary = %q", summary)
	}

	// Backward compatible: a plain title (no labels) is treated as the title,
	// and the summary stays empty without error.
	title, summary = ParseTitleAndSummary("\"Capital do Brasil.\"", 40, 240)
	if title != "Capital do Brasil" || summary != "" {
		t.Fatalf("legacy parse: title = %q, summary = %q", title, summary)
	}
}

func TestCleanChatSummaryTrimsAndCaps(t *testing.T) {
	if got := CleanChatSummary("  SUMMARY:   hello    world  ", 240); got != "hello world" {
		t.Fatalf("clean = %q", got)
	}
	long := CleanChatSummary("abcdefghij", 4)
	if len([]rune(long)) != 4 || long != "abcd" {
		t.Fatalf("capped = %q", long)
	}
}
