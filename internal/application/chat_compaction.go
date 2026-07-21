package application

import (
	"context"
	"fmt"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

const (
	CompactSummaryPrefix      = "[aw compacted context]"
	CompactRecentMessages     = 6
	CompactSummaryMaxRunes    = 1800
	CompactSummaryMaxTokens   = 700
	compactSummaryTargetRunes = 1200
)

type CompactChatHistoryInput struct {
	ChatID          string
	RecentMessages  int
	SummaryMaxRunes int
	MaxOutputTokens int32
}

type CompactChatHistoryResult struct {
	ChatID      string
	Summary     string
	Messages    []domain.Message
	ModelConfig domain.ModelConfig
}

func CompactChatHistory(ctx context.Context, store ports.ChatCompactionStore, providerStore ports.ProviderSecretStore, runtime ports.ChatCompactionRuntime, input CompactChatHistoryInput) (CompactChatHistoryResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	result := CompactChatHistoryResult{ChatID: input.ChatID}
	if store == nil {
		return result, fmt.Errorf("chat compaction store is required")
	}
	if providerStore == nil {
		return result, fmt.Errorf("provider store is required")
	}
	if runtime == nil {
		return result, fmt.Errorf("agent runtime is not available")
	}
	if !store.IsUnlocked() {
		return result, fmt.Errorf("vault is locked")
	}

	runtimeConfig, err := ResolveProviderRuntimeConfig(providerStore)
	if err != nil {
		return result, err
	}
	modelConfig := ModelConfigFromProviderRuntimeConfig(runtimeConfig)
	result.ModelConfig = modelConfig

	messages, err := store.ListMessages(input.ChatID)
	if err != nil {
		return result, err
	}
	previousSummaries, conversation := compactableConversation(messages)
	if len(conversation) < 4 {
		return result, fmt.Errorf("not enough chat history to compact")
	}

	recentCount := compactRecentCount(len(conversation), input.RecentMessages)
	if recentCount < 2 || recentCount >= len(conversation) {
		return result, fmt.Errorf("not enough older chat history to compact")
	}
	older := conversation[:len(conversation)-recentCount]
	recent := conversation[len(conversation)-recentCount:]

	maxOutputTokens := input.MaxOutputTokens
	if maxOutputTokens <= 0 {
		maxOutputTokens = CompactSummaryMaxTokens
	}
	reply, err := runtime.GenerateOneShot(ctx, modelConfig, CompactPrompt(previousSummaries, older, recent), maxOutputTokens)
	if err != nil {
		return result, err
	}
	summary := SanitizeCompactSummary(ScrubChatSecrets(reply.Text), input.SummaryMaxRunes)
	if summary == "" {
		return result, fmt.Errorf("model returned an empty compaction summary")
	}

	compacted := make([]domain.Message, 0, len(recent)+1)
	compacted = append(compacted, domain.Message{
		Role:    "system",
		Content: CompactSummaryPrefix + "\n" + summary,
	})
	compacted = append(compacted, recent...)

	persisted, err := store.ReplaceMessages(input.ChatID, compacted)
	if err != nil {
		return result, err
	}
	history := make([]domain.HistoryMessage, 0, len(persisted))
	for _, message := range persisted {
		history = append(history, domain.HistoryMessage{Role: message.Role, Content: message.Content})
	}
	if err := runtime.ResetSessionWithHistory(ctx, modelConfig, input.ChatID, history); err != nil {
		return result, err
	}

	result.Summary = summary
	result.Messages = persisted
	return result, nil
}

func compactableConversation(messages []domain.Message) ([]string, []domain.Message) {
	var previousSummaries []string
	conversation := make([]domain.Message, 0, len(messages))
	for _, message := range messages {
		if strings.TrimSpace(message.Content) == "" {
			continue
		}
		if IsCompactionSummaryMessage(message) {
			previousSummaries = append(previousSummaries, CompactSummaryBody(message.Content))
			continue
		}
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		conversation = append(conversation, message)
	}
	return previousSummaries, conversation
}

func compactRecentCount(conversationCount int, requested int) int {
	recentCount := requested
	if recentCount <= 0 {
		recentCount = CompactRecentMessages
	}
	if conversationCount-recentCount < 2 {
		recentCount = conversationCount - 2
	}
	if recentCount > 2 && recentCount%2 == 1 {
		recentCount--
	}
	return recentCount
}

func IsCompactionSummaryMessage(message domain.Message) bool {
	return strings.EqualFold(strings.TrimSpace(message.Role), "system") && strings.HasPrefix(strings.TrimSpace(message.Content), CompactSummaryPrefix)
}

func CompactSummaryBody(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, CompactSummaryPrefix)
	return strings.TrimSpace(content)
}

func CompactPrompt(previousSummaries []string, older []domain.Message, recent []domain.Message) string {
	var b strings.Builder
	b.WriteString("Create a compact searchable summary that will replace older chat history in aw.\n")
	b.WriteString(fmt.Sprintf("Use the main language of the conversation. Preserve concrete projects, files, bugs, decisions, constraints, open questions, and user preferences. Do not invent facts. Do not give instructions to the assistant. Keep it under %d characters.\n\n", compactSummaryTargetRunes))
	if len(previousSummaries) > 0 {
		b.WriteString("Previous compacted summary:\n")
		for _, summary := range previousSummaries {
			if text := strings.TrimSpace(ScrubChatSecrets(summary)); text != "" {
				b.WriteString("- ")
				b.WriteString(text)
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("Older messages to summarize:\n")
	for _, message := range older {
		b.WriteString(compactMessageLabel(message.Role))
		b.WriteString(": ")
		b.WriteString(strings.TrimSpace(ScrubChatSecrets(message.Content)))
		b.WriteString("\n")
	}
	if len(recent) > 0 {
		b.WriteString("\nRecent messages that will remain outside the summary; use them only for continuity:\n")
		for _, message := range recent {
			b.WriteString(compactMessageLabel(message.Role))
			b.WriteString(": ")
			b.WriteString(strings.TrimSpace(ScrubChatSecrets(message.Content)))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func compactMessageLabel(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant":
		return "Assistant"
	default:
		return "User"
	}
}

func SanitizeCompactSummary(summary string, maxRunes int) string {
	summary = strings.TrimSpace(summary)
	summary = strings.Trim(summary, "`")
	summary = strings.TrimSpace(summary)
	if maxRunes <= 0 {
		maxRunes = CompactSummaryMaxRunes
	}
	runes := []rune(summary)
	if len(runes) > maxRunes {
		summary = strings.TrimSpace(string(runes[:maxRunes])) + "..."
	}
	return summary
}
