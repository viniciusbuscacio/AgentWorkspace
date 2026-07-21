package tools

import (
	"context"
	"fmt"
)

func registerChatActions(reg map[string]AwActionHandler) {
	reg["chat.list"] = func(_ context.Context, _ map[string]any, w *workspace) (string, error) {
		chats, err := w.control.ListChats()
		if err != nil {
			return "", err
		}
		return awJSON(chats)
	}

	reg["chat.create"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		title, _, err := awStringArg(args, "title")
		if err != nil {
			return "", err
		}
		open, err := awBoolArg(args, "open", true)
		if err != nil {
			return "", err
		}
		chat, err := w.control.CreateChat(title, open)
		if err != nil {
			return "", err
		}
		return awJSON(chat)
	}

	reg["chat.open"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		chatID, err := awRequiredStringArg(args, "chatId")
		if err != nil {
			return "", err
		}
		if err := w.control.Navigate("chat", chatID); err != nil {
			return "", err
		}
		return awOK(map[string]any{"chatId": chatID})
	}

	reg["chat.send"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		chatID, err := awRequiredStringArg(args, "chatId")
		if err != nil {
			return "", err
		}
		text, err := awRequiredStringArg(args, "text")
		if err != nil {
			return "", err
		}
		if err := w.control.SendChatMessage(chatID, text); err != nil {
			return "", err
		}
		return awOK(map[string]any{"chatId": chatID, "accepted": true})
	}

	reg["chat.stop"] = chatIDAction(func(w *workspace, chatID string) error {
		return w.control.StopChat(chatID)
	})

	reg["chat.rename"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		chatID, err := awRequiredStringArg(args, "chatId")
		if err != nil {
			return "", err
		}
		title, err := awRequiredStringArg(args, "title")
		if err != nil {
			return "", err
		}
		if err := w.control.RenameChat(chatID, title); err != nil {
			return "", err
		}
		return awOK(map[string]any{"chatId": chatID, "title": title})
	}

	reg["chat.archive"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		chatID, err := awRequiredStringArg(args, "chatId")
		if err != nil {
			return "", err
		}
		archived, err := awBoolArg(args, "archived", true)
		if err != nil {
			return "", err
		}
		if err := w.control.SetChatArchived(chatID, archived); err != nil {
			return "", err
		}
		return awOK(map[string]any{"chatId": chatID, "archived": archived})
	}

	// chat.delete is the trash-bin verb: it ARCHIVES (recoverable from the
	// Archived list), so the agent can "delete" without an irreversible
	// loss and without a confirmation prompt. True removal is a separate,
	// confirmed action below.
	reg["chat.delete"] = chatIDAction(func(w *workspace, chatID string) error {
		return w.control.SetChatArchived(chatID, true)
	})

	// chat.delete_permanent empties the trash: it removes the chat and every
	// message/turn for good. Irreversible, so it keeps the human confirmation.
	reg["chat.delete_permanent"] = confirmedChatIDAction("Delete chat permanently (cannot be undone)", func(w *workspace, chatID string) error {
		return w.control.DeleteChat(chatID)
	})

	reg["chat.clear"] = confirmedChatIDAction("Clear all chat messages", func(w *workspace, chatID string) error {
		return w.control.ClearChat(chatID)
	})

	reg["chat.session.new"] = chatIDAction(func(w *workspace, chatID string) error {
		return w.control.NewChatSession(chatID)
	})

	reg["chat.compact"] = chatIDAction(func(w *workspace, chatID string) error {
		return w.control.CompactChat(chatID)
	})

	reg["chat.messages"] = func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		chatID, err := awRequiredStringArg(args, "chatId")
		if err != nil {
			return "", err
		}
		limit, _, err := awIntArg(args, "limit")
		if err != nil {
			return "", err
		}
		messages, err := w.control.ChatMessages(chatID, limit)
		if err != nil {
			return "", err
		}
		return awJSON(messages)
	}
}

func chatIDAction(run func(w *workspace, chatID string) error) AwActionHandler {
	return func(_ context.Context, args map[string]any, w *workspace) (string, error) {
		chatID, err := awRequiredStringArg(args, "chatId")
		if err != nil {
			return "", err
		}
		if err := run(w, chatID); err != nil {
			return "", err
		}
		return awOK(map[string]any{"chatId": chatID})
	}
}

// confirmedChatIDAction gates destructive chat operations behind the same
// human confirmation flow used by mutating file tools.
func confirmedChatIDAction(summary string, run func(w *workspace, chatID string) error) AwActionHandler {
	return func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		chatID, err := awRequiredStringArg(args, "chatId")
		if err != nil {
			return "", err
		}
		approved, err := w.requireConfirmation(contextOrBackground(ctx), ConfirmRequest{
			Tool:    "aw",
			Summary: fmt.Sprintf("%s %q", summary, chatID),
			Args:    map[string]any{"chatId": chatID},
		})
		if err != nil {
			return "", err
		}
		if !approved {
			return "", ErrConfirmationDenied
		}
		if err := run(w, chatID); err != nil {
			return "", err
		}
		return awOK(map[string]any{"chatId": chatID})
	}
}
