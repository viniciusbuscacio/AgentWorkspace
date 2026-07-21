package application

import (
	"fmt"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

const (
	CompactionThresholdPercent = 90
	defaultContextWindowTokens = 100000
)

type ChatSessionInfo = domain.ChatSessionInfo

type ChatSessionInfoInput struct {
	ChatID   string
	AgentErr error
	PlanMode bool
	// RunActive reports whether a backend run is in flight for THIS chat
	// (the chatRuns registry). The frontend rehydrates its thinking
	// indicator and queue routing from it after a remount — it must be
	// per-chat truth, never a capability flag or a global "anything
	// running" answer (a hardcoded true here put a thinking bubble on
	// every freshly opened chat, 2026-06-12).
	RunActive       bool
	LastUsage       *domain.TokenUsage
	SupportsRuntime func(domain.ModelConfig) bool
	// ProviderOverride / ModelOverride reflect this chat's per-chat model
	// choice so session-info agrees with what the run will actually use.
	ProviderOverride string
	ModelOverride    string
}

func BuildChatSessionInfo(store ports.ChatSessionInfoStore, providerStore ports.ProviderSecretStore, input ChatSessionInfoInput) domain.ChatSessionInfo {
	info := domain.ChatSessionInfo{
		PlanMode:     input.PlanMode,
		Streaming:    input.RunActive,
		GoalReady:    false,
		MessageCount: 0,
	}
	if input.LastUsage != nil {
		usage := *input.LastUsage
		info.TokenUsage = &usage
		info.LastUsage = &usage
	}

	if input.AgentErr != nil {
		info.Error = input.AgentErr.Error()
	}

	count := 0
	var countErr error
	if store == nil {
		countErr = fmt.Errorf("chat session info store is required")
	} else {
		count, countErr = store.CountMessages(input.ChatID)
		info.MessageCount = count
	}

	runtimeConfig, providerErr := ResolveEffectiveProviderRuntimeConfig(providerStore, input.ProviderOverride, input.ModelOverride)
	if providerErr == nil {
		info.Provider = runtimeConfig.ProviderID
		info.Model = runtimeConfig.Model
		info.ContextWindow = ContextWindowForModel(runtimeConfig.ProviderID, runtimeConfig.Model)
		info.CompactionThreshold = CompactionThresholdPercent
		if input.LastUsage != nil {
			info.ContextUsedPercent = ContextUsedPercent(*input.LastUsage, info.ContextWindow)
		}
	}

	supports := true
	if providerErr == nil && input.SupportsRuntime != nil {
		supports = input.SupportsRuntime(ModelConfigFromProviderRuntimeConfig(runtimeConfig))
	}
	unlocked := store != nil && store.IsUnlocked()
	info.Ready = input.AgentErr == nil && providerErr == nil && countErr == nil && unlocked && supports
	info.CompactReady = info.Ready

	if providerErr != nil {
		info.Error = providerErr.Error()
	} else if !supports {
		info.Error = fmt.Sprintf("%s is configured, but aw does not have a Vault-only OAuth runtime adapter for %s yet", runtimeConfig.ProviderName, runtimeConfig.AuthType)
	}
	if countErr != nil {
		info.Error = countErr.Error()
	}
	return info
}

func ShouldAutoCompactForUsage(usage *domain.TokenUsage, providerID string, model string) bool {
	if usage == nil {
		return false
	}
	window := ContextWindowForModel(providerID, model)
	return ContextUsedPercent(*usage, window) >= CompactionThresholdPercent
}

func ContextUsedPercent(usage domain.TokenUsage, window int) int {
	if window <= 0 {
		return 0
	}
	used := usage.Input
	if used <= 0 {
		used = usage.Total
	}
	if used <= 0 {
		return 0
	}
	percent := (used * 100) / window
	if percent > 100 {
		return 100
	}
	return percent
}

func ContextWindowForModel(providerID string, model string) int {
	model = strings.ToLower(strings.TrimSpace(model))
	providerID = strings.ToLower(strings.TrimSpace(providerID))
	switch {
	case strings.Contains(model, "gemini"):
		return 1000000
	case strings.Contains(model, "claude"):
		return 200000
	case strings.Contains(model, "gpt-4.1"):
		return 1000000
	case strings.Contains(model, "gpt-5"):
		return 400000
	case strings.Contains(model, "deepseek"):
		return 128000
	case strings.Contains(model, "qwen"):
		return 128000
	case strings.Contains(model, "llama"):
		return 128000
	case strings.Contains(model, "mistral") || strings.Contains(model, "mixtral"):
		return 128000
	case providerID == "custom-openai":
		return defaultContextWindowTokens
	default:
		return defaultContextWindowTokens
	}
}
