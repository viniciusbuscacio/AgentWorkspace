package application

import (
	"strings"
	"sync"

	"aw/internal/domain"
)

type ChatUsageTracker struct {
	mu    sync.Mutex
	usage map[string]domain.TokenUsage
}

func NewChatUsageTracker() *ChatUsageTracker {
	return &ChatUsageTracker{usage: map[string]domain.TokenUsage{}}
}

func (tracker *ChatUsageTracker) Remember(chatID string, usage *domain.TokenUsage) {
	if tracker == nil || usage == nil {
		return
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if tracker.usage == nil {
		tracker.usage = map[string]domain.TokenUsage{}
	}
	tracker.usage[chatID] = *usage
}

func (tracker *ChatUsageTracker) Last(chatID string) *domain.TokenUsage {
	if tracker == nil {
		return nil
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return nil
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if tracker.usage == nil {
		return nil
	}
	usage, ok := tracker.usage[chatID]
	if !ok {
		return nil
	}
	return &usage
}
