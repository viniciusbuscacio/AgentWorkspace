package vault

import "testing"

func TestVaultChatOperationsPersistAttachmentsAndIndexesMessages(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)

	if _, err := v.Create("senha-chat"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() { _ = v.Lock() }()

	chat, err := v.CreateChat("Chat de teste")
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}

	attachment := Attachment{
		Name:    "screenshot.png",
		Type:    "image/png",
		DataURI: "data:image/png;base64,aGVsbG8=",
	}
	if _, err := v.AddMessageWithAttachments(chat.ID, "user", "mensagem com anexo", []Attachment{attachment}); err != nil {
		t.Fatalf("AddMessageWithAttachments() error = %v", err)
	}
	if _, err := v.AddMessage(chat.ID, "assistant", "resposta markdown **ok**"); err != nil {
		t.Fatalf("AddMessage() error = %v", err)
	}
	if _, err := v.InsertLLMTurn(LLMTurn{
		SessionID:        chat.ID,
		RequestJSON:      `{"model":"demo","messages":[{"role":"user","content":"oi"}]}`,
		ResponseText:     "resposta markdown **ok**",
		ToolCallsJSON:    "[]",
		Model:            "demo",
		PromptTokens:     11,
		CompletionTokens: 7,
		FinishReason:     "stop",
	}); err != nil {
		t.Fatalf("InsertLLMTurn() error = %v", err)
	}
	if _, err := v.InsertLLMTurn(LLMTurn{
		SessionID:    chat.ID,
		RequestJSON:  `{"model":"demo","messages":[{"role":"user","content":"follow up"}]}`,
		ResponseText: "segunda resposta",
	}); err != nil {
		t.Fatalf("InsertLLMTurn(second) error = %v", err)
	}

	count, err := v.CountMessages(chat.ID)
	if err != nil {
		t.Fatalf("CountMessages() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("CountMessages() = %d, want 2", count)
	}

	messages, err := v.ListMessages(chat.ID)
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("ListMessages() length = %d, want 2", len(messages))
	}
	if got := messages[0].Attachments; len(got) != 1 || got[0] != attachment {
		t.Fatalf("ListMessages()[0].Attachments = %#v, want %#v", got, []Attachment{attachment})
	}

	turns, err := v.ListLLMTurns(chat.ID)
	if err != nil {
		t.Fatalf("ListLLMTurns() error = %v", err)
	}
	if len(turns) != 2 {
		t.Fatalf("ListLLMTurns() length = %d, want 2", len(turns))
	}
	if turns[0].TurnIndex != 1 || turns[1].TurnIndex != 2 {
		t.Fatalf("turn indexes = %d/%d, want 1/2", turns[0].TurnIndex, turns[1].TurnIndex)
	}
	if turns[0].RequestJSON == "" || turns[0].ResponseText != "resposta markdown **ok**" || turns[0].PromptTokens != 11 || turns[0].CompletionTokens != 7 {
		t.Fatalf("first llm turn = %+v", turns[0])
	}

	recent, err := v.RecentMessages(chat.ID, 1)
	if err != nil {
		t.Fatalf("RecentMessages() error = %v", err)
	}
	if len(recent) != 1 || recent[0].Role != "assistant" {
		t.Fatalf("RecentMessages() = %#v, want latest assistant message", recent)
	}

	var indexedID string
	if err := v.db.QueryRow(`SELECT message_id FROM messages_fts WHERE session_id = ? AND content MATCH 'markdown'`, chat.ID).Scan(&indexedID); err != nil {
		t.Fatalf("messages_fts did not index assistant message: %v", err)
	}

	renamed, err := v.RenameChat(chat.ID, "Chat renomeado")
	if err != nil {
		t.Fatalf("RenameChat() error = %v", err)
	}
	if renamed.Title != "Chat renomeado" {
		t.Fatalf("RenameChat().Title = %q", renamed.Title)
	}

	archived, err := v.SetChatArchived(chat.ID, true)
	if err != nil {
		t.Fatalf("SetChatArchived() error = %v", err)
	}
	if !archived.Archived {
		t.Fatal("SetChatArchived() did not archive chat")
	}

	if err := v.ClearChat(chat.ID); err != nil {
		t.Fatalf("ClearChat() error = %v", err)
	}
	count, err = v.CountMessages(chat.ID)
	if err != nil {
		t.Fatalf("CountMessages() after clear error = %v", err)
	}
	if count != 0 {
		t.Fatalf("CountMessages() after clear = %d, want 0", count)
	}

	if err := v.DeleteChat(chat.ID); err != nil {
		t.Fatalf("DeleteChat() error = %v", err)
	}
	chats, err := v.ListChats()
	if err != nil {
		t.Fatalf("ListChats() after delete error = %v", err)
	}
	for _, existing := range chats {
		if existing.ID == chat.ID {
			t.Fatalf("deleted chat %q still listed", chat.ID)
		}
	}
}

func TestVaultChatTitleHistoryRoundTrips(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)

	if _, err := v.Create("senha-titles"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() { _ = v.Lock() }()

	chat, err := v.CreateChat("New Chat")
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}

	if _, err := v.InsertChatTitle(ChatTitleEntry{
		SessionID: chat.ID,
		Title:     "Capital do Brasil",
		Turn:      3,
		Source:    "auto",
	}); err != nil {
		t.Fatalf("InsertChatTitle(auto) error = %v", err)
	}
	if _, err := v.InsertChatTitle(ChatTitleEntry{
		SessionID: chat.ID,
		Title:     "Meu titulo",
		Source:    "manual",
	}); err != nil {
		t.Fatalf("InsertChatTitle(manual) error = %v", err)
	}

	entries, err := v.ListChatTitles(chat.ID)
	if err != nil {
		t.Fatalf("ListChatTitles() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ListChatTitles() length = %d, want 2", len(entries))
	}
	if entries[0].Title != "Capital do Brasil" || entries[0].Source != "auto" || entries[0].Turn != 3 {
		t.Fatalf("first entry = %+v", entries[0])
	}
	if entries[1].Title != "Meu titulo" || entries[1].Source != "manual" {
		t.Fatalf("second entry = %+v", entries[1])
	}
	if entries[0].ID == "" || entries[0].CreatedAt == "" {
		t.Fatalf("entry missing generated fields: %+v", entries[0])
	}
}

func TestVaultSessionSummaryRoundTrips(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)

	if _, err := v.Create("senha-summary"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() { _ = v.Lock() }()

	chat, err := v.CreateChat("New Chat")
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}

	// No summary yet.
	if summary, turn, err := v.SessionSummary(chat.ID); err != nil || summary != "" || turn != 0 {
		t.Fatalf("initial summary = %q/%d/%v", summary, turn, err)
	}

	if err := v.SetSessionSummary(chat.ID, "Resumo curto da sessao.", 3); err != nil {
		t.Fatalf("SetSessionSummary() error = %v", err)
	}
	summary, turn, err := v.SessionSummary(chat.ID)
	if err != nil {
		t.Fatalf("SessionSummary() error = %v", err)
	}
	if summary != "Resumo curto da sessao." || turn != 3 {
		t.Fatalf("summary = %q, turn = %d", summary, turn)
	}
}

func TestVaultSearchMessagesFindsSessionByTerm(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)

	if _, err := v.Create("senha-search"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() { _ = v.Lock() }()

	chatA, err := v.CreateChat("Deploy")
	if err != nil {
		t.Fatalf("CreateChat(A) error = %v", err)
	}
	chatB, err := v.CreateChat("Viagem")
	if err != nil {
		t.Fatalf("CreateChat(B) error = %v", err)
	}
	if _, err := v.AddMessage(chatA.ID, "user", "como faco o deploy do figurinhashop"); err != nil {
		t.Fatalf("AddMessage(A) error = %v", err)
	}
	if _, err := v.AddMessage(chatB.ID, "user", "quero um roteiro de viagem para Portugal"); err != nil {
		t.Fatalf("AddMessage(B) error = %v", err)
	}

	hits, err := v.SearchMessages("deploy", 10)
	if err != nil {
		t.Fatalf("SearchMessages() error = %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(hits))
	}
	if hits[0].SessionID != chatA.ID {
		t.Fatalf("hit session = %q, want %q", hits[0].SessionID, chatA.ID)
	}

	// Empty/whitespace query returns nothing without error.
	if hits, err := v.SearchMessages("   ", 10); err != nil || len(hits) != 0 {
		t.Fatalf("empty query: hits=%d err=%v", len(hits), err)
	}

	// Limit is honored.
	if _, err := v.AddMessage(chatB.ID, "user", "outro deploy mencionado aqui"); err != nil {
		t.Fatalf("AddMessage(B2) error = %v", err)
	}
	limited, err := v.SearchMessages("deploy", 1)
	if err != nil {
		t.Fatalf("SearchMessages(limit) error = %v", err)
	}
	if len(limited) != 1 {
		t.Fatalf("limited hits = %d, want 1", len(limited))
	}
}
