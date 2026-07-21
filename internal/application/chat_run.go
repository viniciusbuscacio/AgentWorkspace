package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// interruptedPartialMinWords is the preservation threshold for a failed
// mid-stream run: below this the streamed fragment is not worth keeping and
// the turn behaves as before (nothing persisted).
const interruptedPartialMinWords = 10

// interruptionMarker is the line appended (after a blank line) to a preserved
// partial reply so both the user and the next model see why it was cut short.
func interruptionMarker(cause error) string {
	if cause == nil {
		return ""
	}
	if errors.Is(cause, context.Canceled) || strings.Contains(strings.ToLower(cause.Error()), "context canceled") {
		return "Interrupted by user (stop button)"
	}
	return cause.Error()
}

// persistInterruptedPartial stores the text a failed run had already streamed
// so it survives in the chat and enters the history the next turn (possibly on
// another provider) sees. Best-effort: a store failure just falls back to the
// old drop-everything behavior. The ADK session append keeps the live
// in-memory history in step with the vault (the runner itself only records
// completed events, never a partial stream).
func persistInterruptedPartial(ctx context.Context, store ports.ChatConversationStore, runtime ports.ChatRuntime, chatID string, partial string, cause error) (domain.Message, bool) {
	text := strings.TrimSpace(partial)
	if len(strings.Fields(text)) < interruptedPartialMinWords {
		return domain.Message{}, false
	}
	persisted := text + "\n\n" + interruptionMarker(cause)
	message, err := store.AddMessage(chatID, "assistant", persisted)
	if err != nil {
		return domain.Message{}, false
	}
	if appender, ok := runtime.(ports.ChatHistoryAppender); ok {
		_ = appender.AppendAssistantHistory(ctx, chatID, persisted)
	}
	return message, true
}

type RunChatMessageInput struct {
	ChatID      string
	Text        string
	Attachments []domain.Attachment
	Stream      bool
	// ExistingUserMessage skips persisting the user message again. It is used
	// after an automatic compaction retry: the first attempt already stored the
	// user's turn before the provider rejected the oversized context.
	ExistingUserMessage *domain.Message
	LLMTurnStore        ports.LLMTurnStore
	// AttachmentReader converts image/PDF attachments to sanitized untrusted
	// text injected into the prompt. nil = metadata-only (legacy behavior).
	AttachmentReader ports.SafeAttachmentReader
	// AttachmentTaintRecorder records suspicious attachment content into the
	// per-turn taint so later sensitive tool actions are gated (Stage C). nil =
	// label-only (Stage A).
	AttachmentTaintRecorder ports.ExternalTaintRecorder
	// TurnScope scopes this turn's external-content taint; it is propagated via
	// ctx so the tools layer shares it. Empty = tools fall back to the ADK
	// invocation id (attachment taint then does not reach tools).
	TurnScope string
	// FallbackOrder is the user-defined provider priority (index 0 = primary).
	// Empty means "use the active provider" (legacy single-provider behavior).
	FallbackOrder []string
	// ProviderOverride / ModelOverride pin THIS chat to a specific provider and
	// (optionally) model, placed at the head of the fallback chain. Empty =
	// use the global active provider.
	ProviderOverride string
	ModelOverride    string
	// Cooldown is the in-memory circuit breaker. nil disables benching.
	Cooldown *ProviderCooldown
	// DecorateReply transforms the final reply text right before it is
	// persisted as the assistant message (e.g. embedding spawn results at
	// their ::spawn markers). nil = persist as-is. The runtime reply in the
	// result stays undecorated.
	DecorateReply func(text string) string
	OnStart       func(chatID string)
	OnDelta       func(chatID string, seq int, delta string)
	OnDone        func(chatID string, messageID string)
	OnError       func(chatID string, err error)
	// OnFallback fires when a provider fails with a failover-class error and the
	// runtime advances to the next provider in the chain. Used to surface a chat
	// system-message and a desktop notification.
	OnFallback func(from domain.ProviderRuntimeConfig, to domain.ProviderRuntimeConfig, reason error)
}

type RunChatMessageResult struct {
	ChatID           string
	UserMessage      domain.Message
	AssistantMessage domain.Message
	Reply            domain.AgentReply
	ModelConfig      domain.ModelConfig
	// TurnRecordErrors holds best-effort persistence failures for LLM debug
	// turns; the chat reply itself succeeded.
	TurnRecordErrors []error
}

func PersistChatUserMessage(store ports.ChatConversationStore, chatID string, text string, attachments []domain.Attachment) (domain.Message, error) {
	if store == nil {
		return domain.Message{}, fmt.Errorf("chat store is required")
	}
	if !store.IsUnlocked() {
		return domain.Message{}, fmt.Errorf("vault is locked")
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" && len(attachments) == 0 {
		return domain.Message{}, fmt.Errorf("message is empty")
	}
	return store.AddMessageWithAttachments(chatID, "user", trimmed, attachments)
}

func RunChatMessage(ctx context.Context, store ports.ChatConversationStore, providerStore ports.ProviderSecretStore, runtime ports.ChatRuntime, input RunChatMessageInput) (RunChatMessageResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// Scope this turn's external-content taint. ADK preserves context values into
	// tool execution, so attachment taint recorded now and tool taint recorded
	// later in the turn share this scope.
	ctx = domain.WithExternalTaintScope(ctx, input.TurnScope)
	result := RunChatMessageResult{ChatID: input.ChatID}
	if store == nil {
		return result, fmt.Errorf("chat store is required")
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

	text := strings.TrimSpace(input.Text)
	if text == "" && len(input.Attachments) == 0 {
		return result, fmt.Errorf("message is empty")
	}

	var userMessage domain.Message
	if input.ExistingUserMessage != nil && strings.TrimSpace(input.ExistingUserMessage.ID) != "" {
		userMessage = *input.ExistingUserMessage
	} else {
		var err error
		userMessage, err = PersistChatUserMessage(store, input.ChatID, text, input.Attachments)
		if err != nil {
			return result, err
		}
	}
	result.UserMessage = userMessage

	chain, err := ResolveChatFallbackChain(providerStore, input.FallbackOrder, input.ProviderOverride, input.ModelOverride)
	if err != nil {
		return result, err
	}
	// Attachments enter as sanitized, untrusted text (extracted via the isolated
	// reader) instead of raw bytes. OCR uses the primary provider's model.
	var attachmentCfg domain.ModelConfig
	if len(chain) > 0 {
		attachmentCfg = ModelConfigFromProviderRuntimeConfig(chain[0])
	}
	runtimeText := text + attachmentContext(ctx, input.AttachmentReader, input.AttachmentTaintRecorder, input.Attachments, attachmentCfg)

	// Build the attempt order: skip providers currently benched by the circuit
	// breaker. If every provider is benched, fall back to the full chain so the
	// user still gets a response (the breaker is advisory, not a hard block).
	candidates := chain
	if input.Cooldown != nil {
		var live []domain.ProviderRuntimeConfig
		for _, cfg := range chain {
			if !input.Cooldown.Active(cfg.ProviderID) {
				live = append(live, cfg)
			}
		}
		if len(live) > 0 {
			candidates = live
		}
	}

	if input.Stream && input.OnStart != nil {
		input.OnStart(input.ChatID)
	}

	var (
		reply         domain.AgentReply
		lastErr       error
		succeeded     bool
		seq           int
		attemptErrors []string
		partial       strings.Builder
	)
	for i, cfg := range candidates {
		modelConfig := ModelConfigFromProviderRuntimeConfig(cfg)
		result.ModelConfig = modelConfig

		// Only a zero-delta attempt ever fails over, so at most one attempt
		// streams — the builder always holds exactly that attempt's text.
		partial.Reset()
		attemptDeltas := 0
		if input.Stream {
			reply, lastErr = runtime.StreamMessage(ctx, modelConfig, input.ChatID, runtimeText, func(delta string) {
				partial.WriteString(delta)
				if input.OnDelta != nil {
					input.OnDelta(input.ChatID, seq, delta)
				}
				seq++
				attemptDeltas++
			})
		} else {
			reply, lastErr = runtime.SendMessage(ctx, modelConfig, input.ChatID, runtimeText)
		}
		if lastErr == nil {
			succeeded = true
			break
		}

		attemptErrors = append(attemptErrors, fmt.Sprintf("%s: %s", cfg.ProviderName, lastErr.Error()))

		// Mid-stream failure: tokens were already shown for this turn, so we
		// cannot cleanly restart on another provider. Surface the error.
		midStream := input.Stream && attemptDeltas > 0
		failoverClass := domain.IsFailoverError(lastErr)

		// Bench the failing provider so subsequent turns skip it for a while —
		// a provider that died mid-stream is just as rate-limited/broken as one
		// that died before the first token, so benching ignores midStream even
		// though the advance below does not.
		if failoverClass && input.Cooldown != nil {
			input.Cooldown.Penalize(cfg.ProviderID)
		}

		if !midStream && failoverClass && i < len(candidates)-1 {
			if input.OnFallback != nil {
				input.OnFallback(cfg, candidates[i+1], lastErr)
			}
			continue
		}
		break
	}

	if !succeeded {
		finalErr := lastErr
		if len(attemptErrors) > 1 {
			finalErr = fmt.Errorf("all providers failed: %s", strings.Join(attemptErrors, "; "))
		}
		if message, persisted := persistInterruptedPartial(ctx, store, runtime, input.ChatID, partial.String(), lastErr); persisted {
			result.AssistantMessage = message
		}
		if input.Stream && input.OnError != nil {
			input.OnError(input.ChatID, finalErr)
		}
		return result, finalErr
	}
	result.Reply = reply

	persistedText := reply.Text
	if input.DecorateReply != nil {
		persistedText = input.DecorateReply(persistedText)
	}
	assistantMessage, err := store.AddMessage(input.ChatID, "assistant", persistedText)
	if err != nil {
		return result, err
	}
	result.AssistantMessage = assistantMessage
	if input.LLMTurnStore != nil {
		for _, turn := range reply.LLMTurns {
			turn.SessionID = input.ChatID
			if _, err := RecordLLMTurn(input.LLMTurnStore, turn); err != nil {
				result.TurnRecordErrors = append(result.TurnRecordErrors, err)
			}
		}
	}
	if input.Stream && input.OnDone != nil {
		input.OnDone(input.ChatID, assistantMessage.ID)
	}
	return result, nil
}
