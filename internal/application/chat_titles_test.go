package application

import (
	"testing"

	"aw/internal/domain"
)

func TestNextNumberedChatTitle(t *testing.T) {
	cases := []struct {
		name  string
		chats []domain.Chat
		want  string
	}{
		{"empty", nil, "Chat 1"},
		{"sequential fills next", []domain.Chat{{Title: "Chat 1"}, {Title: "Chat 2"}}, "Chat 3"},
		{"fills smallest gap", []domain.Chat{{Title: "Chat 1"}, {Title: "Chat 3"}}, "Chat 2"},
		{"high numbers free low ones", []domain.Chat{{Title: "Chat 5"}, {Title: "Chat 123"}}, "Chat 1"},
		{"ignores custom and New Chat", []domain.Chat{{Title: "Capital do Brasil"}, {Title: "New Chat"}, {Title: "Chat 1"}}, "Chat 2"},
		{"case insensitive used", []domain.Chat{{Title: "chat 1"}, {Title: "Chat 2"}}, "Chat 3"},
		{"ignores archived", []domain.Chat{{Title: "Chat 1", Archived: true}, {Title: "Chat 2"}}, "Chat 1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NextNumberedChatTitle(tc.chats); got != tc.want {
				t.Fatalf("NextNumberedChatTitle() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveInitialChatTitle(t *testing.T) {
	existing := []domain.Chat{{Title: "Chat 1"}, {Title: "Chat 2"}}
	if got := ResolveInitialChatTitle("", existing); got != "Chat 3" {
		t.Fatalf("ResolveInitialChatTitle(empty) = %q, want Chat 3", got)
	}
	if got := ResolveInitialChatTitle(" Planejamento ", existing); got != "Planejamento" {
		t.Fatalf("ResolveInitialChatTitle(custom) = %q, want Planejamento", got)
	}
}
