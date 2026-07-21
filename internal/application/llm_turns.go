package application

import (
	"fmt"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

func RecordLLMTurn(store ports.LLMTurnStore, turn domain.LLMTurn) (domain.LLMTurn, error) {
	if store == nil {
		return domain.LLMTurn{}, fmt.Errorf("llm turn store is required")
	}
	turn.SessionID = strings.TrimSpace(turn.SessionID)
	if turn.SessionID == "" {
		return domain.LLMTurn{}, fmt.Errorf("chat id is required")
	}
	turn.RequestJSON = strings.TrimSpace(turn.RequestJSON)
	if turn.RequestJSON == "" {
		return domain.LLMTurn{}, fmt.Errorf("request json is required")
	}
	if strings.TrimSpace(turn.ToolCallsJSON) == "" {
		turn.ToolCallsJSON = "[]"
	}
	return store.InsertLLMTurn(turn)
}

func ListLLMTurns(store ports.LLMTurnStore, sessionID string) ([]domain.LLMTurn, error) {
	if store == nil {
		return nil, fmt.Errorf("llm turn store is required")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("chat id is required")
	}
	return store.ListLLMTurns(sessionID)
}
