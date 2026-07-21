package application

import (
	"testing"

	"aw/internal/domain"
)

type fakeLLMTurnStore struct {
	turns []domain.LLMTurn
}

func (s *fakeLLMTurnStore) InsertLLMTurn(turn domain.LLMTurn) (domain.LLMTurn, error) {
	if turn.TurnIndex == 0 {
		turn.TurnIndex = len(s.turns) + 1
	}
	s.turns = append(s.turns, turn)
	return turn, nil
}

func (s *fakeLLMTurnStore) ListLLMTurns(sessionID string) ([]domain.LLMTurn, error) {
	var result []domain.LLMTurn
	for _, turn := range s.turns {
		if turn.SessionID == sessionID {
			result = append(result, turn)
		}
	}
	return result, nil
}

func TestRecordLLMTurnValidatesAndPersists(t *testing.T) {
	store := &fakeLLMTurnStore{}
	turn, err := RecordLLMTurn(store, domain.LLMTurn{
		SessionID:   " chat-1 ",
		RequestJSON: `{"messages":[]}`,
	})
	if err != nil {
		t.Fatalf("RecordLLMTurn() error = %v", err)
	}
	if turn.SessionID != "chat-1" || turn.ToolCallsJSON != "[]" || turn.TurnIndex != 1 {
		t.Fatalf("turn = %+v", turn)
	}

	listed, err := ListLLMTurns(store, "chat-1")
	if err != nil {
		t.Fatalf("ListLLMTurns() error = %v", err)
	}
	if len(listed) != 1 || listed[0].RequestJSON != `{"messages":[]}` {
		t.Fatalf("ListLLMTurns() = %+v", listed)
	}
}

func TestRecordLLMTurnRejectsInvalidInput(t *testing.T) {
	store := &fakeLLMTurnStore{}
	if _, err := RecordLLMTurn(nil, domain.LLMTurn{SessionID: "chat", RequestJSON: "{}"}); err == nil {
		t.Fatalf("RecordLLMTurn(nil) error = nil")
	}
	if _, err := RecordLLMTurn(store, domain.LLMTurn{RequestJSON: "{}"}); err == nil {
		t.Fatalf("RecordLLMTurn(empty session) error = nil")
	}
	if _, err := RecordLLMTurn(store, domain.LLMTurn{SessionID: "chat"}); err == nil {
		t.Fatalf("RecordLLMTurn(empty request) error = nil")
	}
	if _, err := ListLLMTurns(nil, "chat"); err == nil {
		t.Fatalf("ListLLMTurns(nil) error = nil")
	}
	if _, err := ListLLMTurns(store, " "); err == nil {
		t.Fatalf("ListLLMTurns(empty) error = nil")
	}
}
