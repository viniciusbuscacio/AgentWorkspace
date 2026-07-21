package application

import (
	"fmt"
	"strings"
	"testing"

	"aw/internal/domain"
)

type fakeSessionInfoStore struct {
	unlocked bool
	count    int
	err      error
}

func (s fakeSessionInfoStore) IsUnlocked() bool {
	return s.unlocked
}

func (s fakeSessionInfoStore) CountMessages(_ string) (int, error) {
	return s.count, s.err
}

func TestChatUsageTrackerRemembersLastUsage(t *testing.T) {
	tracker := NewChatUsageTracker()
	tracker.Remember("chat-1", &domain.TokenUsage{Input: 10, Output: 5, Total: 15})

	usage := tracker.Last("chat-1")
	if usage == nil || usage.Input != 10 || usage.Output != 5 || usage.Total != 15 {
		t.Fatalf("usage = %+v, want 10/5/15", usage)
	}

	usage.Total = 99
	again := tracker.Last("chat-1")
	if again == nil || again.Total != 15 {
		t.Fatalf("tracker leaked internal usage pointer, got %+v", again)
	}
}

func TestBuildChatSessionInfoReadyWithUsage(t *testing.T) {
	store := fakeSessionInfoStore{unlocked: true, count: 7}
	providerStore := newMemoryProviderStore()
	providerStore.values[activeProviderSecret] = "openrouter"
	providerStore.values["_config_model_openrouter"] = "deepseek/deepseek-r1"
	providerStore.values["openrouter_api_key"] = "sk-or-test"
	usage := &domain.TokenUsage{Input: 21, Output: 3, Total: 24}

	info := BuildChatSessionInfo(store, providerStore, ChatSessionInfoInput{
		ChatID:    "chat-1",
		PlanMode:  true,
		RunActive: true,
		LastUsage: usage,
		SupportsRuntime: func(cfg domain.ModelConfig) bool {
			return cfg.AuthType == "api-key"
		},
	})

	if !info.Ready || !info.CompactReady || info.Error != "" {
		t.Fatalf("info ready/error = %v/%q", info.Ready, info.Error)
	}
	if info.Provider != "openrouter" || info.Model != "deepseek/deepseek-r1" {
		t.Fatalf("provider/model = %q/%q", info.Provider, info.Model)
	}
	if !info.PlanMode || !info.Streaming || info.GoalReady {
		t.Fatalf("mode flags = plan:%v streaming:%v goal:%v", info.PlanMode, info.Streaming, info.GoalReady)
	}
	// Streaming is PER-CHAT run truth, never a capability flag: the frontend
	// rehydrates its thinking indicator from it after a remount, and a
	// hardcoded true put a thinking bubble on every freshly opened chat.
	idle := BuildChatSessionInfo(store, providerStore, ChatSessionInfoInput{ChatID: "chat-1", RunActive: false, SupportsRuntime: func(domain.ModelConfig) bool { return true }})
	if idle.Streaming {
		t.Fatal("Streaming must be false when no run is in flight for the chat")
	}
	if info.MessageCount != 7 {
		t.Fatalf("message count = %d", info.MessageCount)
	}
	if info.TokenUsage == nil || info.LastUsage == nil || *info.TokenUsage != *usage || *info.LastUsage != *usage {
		t.Fatalf("usage = %+v last = %+v, want %+v", info.TokenUsage, info.LastUsage, usage)
	}
	if info.ContextWindow != 128000 || info.ContextUsedPercent != 0 || info.CompactionThreshold != CompactionThresholdPercent {
		t.Fatalf("context fields = window:%d used:%d threshold:%d", info.ContextWindow, info.ContextUsedPercent, info.CompactionThreshold)
	}
}

func TestShouldAutoCompactForUsageAtNinetyPercent(t *testing.T) {
	if !ShouldAutoCompactForUsage(&domain.TokenUsage{Input: 116000, Total: 117000}, "openrouter", "deepseek/deepseek-r1") {
		t.Fatal("deepseek 116k input tokens should auto-compact at 90% of 128k")
	}
	if ShouldAutoCompactForUsage(&domain.TokenUsage{Input: 100000, Total: 101000}, "openrouter", "deepseek/deepseek-r1") {
		t.Fatal("deepseek 100k input tokens should stay below 90% of 128k")
	}
	if got := ContextUsedPercent(domain.TokenUsage{Input: 450000}, 400000); got != 100 {
		t.Fatalf("ContextUsedPercent caps at 100, got %d", got)
	}
}

func TestBuildChatSessionInfoReportsProviderAndSupportErrors(t *testing.T) {
	store := fakeSessionInfoStore{unlocked: true, count: 1}
	providerStore := newMemoryProviderStore()

	info := BuildChatSessionInfo(store, providerStore, ChatSessionInfoInput{})
	if info.Ready || !strings.Contains(strings.ToLower(info.Error), "no provider") {
		t.Fatalf("missing provider info = %+v", info)
	}

	providerStore.values[activeProviderSecret] = "github-copilot"
	providerStore.values["_config_model_copilot"] = "claude-sonnet-4"
	providerStore.values["github_copilot_auth_json"] = `{"access_token":"token"}`
	info = BuildChatSessionInfo(store, providerStore, ChatSessionInfoInput{
		SupportsRuntime: func(domain.ModelConfig) bool { return false },
	})
	if info.Ready || !strings.Contains(info.Error, "does not have a Vault-only OAuth runtime adapter") {
		t.Fatalf("unsupported runtime info = %+v", info)
	}
	if info.Provider != "github-copilot" || info.Model != "claude-sonnet-4" {
		t.Fatalf("provider/model = %q/%q", info.Provider, info.Model)
	}
}

func TestBuildChatSessionInfoCountErrorWinsLikeLegacyHandler(t *testing.T) {
	store := fakeSessionInfoStore{unlocked: true, err: fmt.Errorf("count failed")}
	providerStore := newMemoryProviderStore()
	providerStore.values[activeProviderSecret] = "openrouter"
	providerStore.values["_config_model_openrouter"] = "deepseek/deepseek-r1"
	providerStore.values["openrouter_api_key"] = "sk-or-test"

	info := BuildChatSessionInfo(store, providerStore, ChatSessionInfoInput{
		AgentErr: fmt.Errorf("agent failed"),
		SupportsRuntime: func(domain.ModelConfig) bool {
			return true
		},
	})
	if info.Ready || info.Error != "count failed" {
		t.Fatalf("info = %+v, want count error to match legacy precedence", info)
	}
}
