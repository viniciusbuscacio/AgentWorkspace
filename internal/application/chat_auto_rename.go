package application

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

const (
	ChatManualRenamedPrefix      = "chat-manual-renamed-"
	ChatAutoRenamedPrefix        = "chat-auto-renamed-"
	ChatAutoRenameCountPrefix    = "chat-auto-rename-count-"
	ChatAutoRenameLastTurnPrefix = "chat-auto-rename-last-turn-"
	ChatTitleMaxRunes            = 40
	InitialAutoRenameTurn        = 3
	AutoRenameTurnInterval       = 10
	ChatTitleMaxOutputTokens     = 320
	ChatSummaryMaxRunes          = 240
)

var specialChatTitlePrefixRe = regexp.MustCompile(`(?i)^(run|spec):`)

type AutoRenameChatInput struct {
	ChatID              string
	UserMessage         string
	AssistantReply      string
	ModelConfig         domain.ModelConfig
	InitialTurn         int
	TurnInterval        int
	TitleMaxRunes       int
	MaxOutputTokens     int32
	UniqueFallbackClock func() time.Time
}

type AutoRenameChatResult struct {
	Renamed bool
	Chat    domain.Chat
	Title   string
	Turns   int
	Reason  string
}

func MarkChatManuallyRenamed(store ports.ChatAutoRenameStore, chatID string, title string) error {
	if store == nil {
		return fmt.Errorf("chat auto-rename store is required")
	}
	recordChatTitle(store, chatID, title, 0, "manual")
	return store.SetSecret(ChatManualRenamedKey(chatID), "true")
}

// recordChatTitle appends a title-history entry best-effort; failures here must
// never block a rename.
func recordChatTitle(store ports.ChatTitleWriter, sessionID string, title string, turn int, source string) {
	if store == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	title = strings.TrimSpace(title)
	if sessionID == "" || title == "" {
		return
	}
	_, _ = store.InsertChatTitle(domain.ChatTitleEntry{
		SessionID: sessionID,
		Title:     title,
		Turn:      turn,
		Source:    source,
	})
}

func MaybeAutoRenameChat(ctx context.Context, store ports.ChatAutoRenameStore, runtime ports.ChatTitleRuntime, input AutoRenameChatInput) (AutoRenameChatResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	result := AutoRenameChatResult{}
	chatID := strings.TrimSpace(input.ChatID)
	if chatID == "" {
		result.Reason = "missing-chat-id"
		return result, nil
	}
	if store == nil {
		return result, fmt.Errorf("chat auto-rename store is required")
	}
	if runtime == nil {
		return result, fmt.Errorf("agent runtime is not available")
	}
	if manual, exists, err := store.GetSecret(ChatManualRenamedKey(chatID)); err == nil && exists && manual == "true" {
		result.Reason = "manual-rename"
		return result, nil
	} else if err != nil {
		return result, err
	}

	chats, err := store.ListChats()
	if err != nil {
		return result, err
	}
	currentTitle := ""
	for _, chat := range chats {
		if chat.ID == chatID {
			currentTitle = chat.Title
			break
		}
	}
	if specialChatTitlePrefixRe.MatchString(strings.TrimSpace(currentTitle)) {
		result.Reason = "special-prefix"
		return result, nil
	}

	messages, err := store.ListMessages(chatID)
	if err != nil {
		return result, err
	}
	turns := UserTurnCount(messages)
	result.Turns = turns
	if turns < 1 {
		result.Reason = "no-user-turns"
		return result, nil
	}

	initialTurn := input.InitialTurn
	if initialTurn <= 0 {
		initialTurn = InitialAutoRenameTurn
	}
	interval := input.TurnInterval
	if interval <= 0 {
		interval = AutoRenameTurnInterval
	}

	autoCount := secretCount(store, ChatAutoRenameCountPrefix+chatID)
	if autoCount == 0 {
		if legacy, exists, gerr := store.GetSecret(ChatAutoRenamedPrefix + chatID); gerr == nil && exists && legacy == "true" {
			autoCount = 1
		} else if gerr != nil {
			return result, gerr
		}
	}
	lastTurn := secretCount(store, ChatAutoRenameLastTurnPrefix+chatID)
	isInitial := IsGenericChatTitle(currentTitle) && autoCount == 0 && turns != lastTurn && turns >= initialTurn
	isPeriodic := autoCount > 0 && turns != lastTurn && turns%interval == 0
	if !isInitial && !isPeriodic {
		result.Reason = "cadence"
		return result, nil
	}

	titleMaxRunes := input.TitleMaxRunes
	if titleMaxRunes <= 0 {
		titleMaxRunes = ChatTitleMaxRunes
	}
	maxOutputTokens := input.MaxOutputTokens
	if maxOutputTokens <= 0 {
		maxOutputTokens = ChatTitleMaxOutputTokens
	}

	title := ""
	summary := ""
	if reply, err := runtime.GenerateOneShot(ctx, input.ModelConfig, ChatTitlePrompt(input.UserMessage, input.AssistantReply, titleMaxRunes), maxOutputTokens); err == nil {
		title, summary = ParseTitleAndSummary(ScrubChatSecrets(reply.Text), titleMaxRunes, ChatSummaryMaxRunes)
	}
	if title == "" {
		title = FallbackChatTitle(ScrubChatSecrets(input.UserMessage), ScrubChatSecrets(input.AssistantReply), titleMaxRunes)
	}
	if title == "" || strings.EqualFold(title, strings.TrimSpace(currentTitle)) {
		_ = store.SetSecret(ChatAutoRenameLastTurnPrefix+chatID, strconv.Itoa(turns))
		result.Reason = "same-title"
		return result, nil
	}

	title = UniqueChatTitle(title, chatID, chats, input.UniqueFallbackClock)
	chat, err := store.RenameChat(chatID, title)
	if err != nil {
		return result, err
	}
	_ = store.SetSecret(ChatAutoRenameCountPrefix+chatID, strconv.Itoa(autoCount+1))
	_ = store.SetSecret(ChatAutoRenameLastTurnPrefix+chatID, strconv.Itoa(turns))
	_ = store.SetSecret(ChatAutoRenamedPrefix+chatID, "true")
	recordChatTitle(store, chatID, chat.Title, turns, "auto")
	if summary != "" {
		_ = store.SetSessionSummary(chatID, summary, turns)
	}

	result.Renamed = true
	result.Chat = chat
	result.Title = chat.Title
	return result, nil
}

func ChatManualRenamedKey(chatID string) string {
	return ChatManualRenamedPrefix + strings.TrimSpace(chatID)
}

func secretCount(store ports.ChatAutoRenameStore, key string) int {
	val, exists, err := store.GetSecret(key)
	if err != nil || !exists {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func IsGenericChatTitle(title string) bool {
	title = strings.TrimSpace(title)
	if title == "" || strings.EqualFold(title, "New Chat") {
		return true
	}
	if !strings.HasPrefix(title, "Chat ") {
		return false
	}
	suffix := strings.TrimSpace(strings.TrimPrefix(title, "Chat "))
	if suffix == "" {
		return false
	}
	for _, ch := range suffix {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func UserTurnCount(messages []domain.Message) int {
	count := 0
	for _, message := range messages {
		if strings.EqualFold(strings.TrimSpace(message.Role), "user") {
			count++
		}
	}
	return count
}

func ChatTitlePrompt(userMessage string, assistantReply string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = ChatTitleMaxRunes
	}
	userMessage = ScrubChatSecrets(userMessage)
	assistantReply = ScrubChatSecrets(assistantReply)
	return strings.TrimSpace(fmt.Sprintf(`Generate a concise chat title and a short summary in the same language as the conversation.
Return exactly two lines in this format:
TITLE: <2 to 4 words, maximum %d characters, no quotes, markdown, or final punctuation>
SUMMARY: <one or two sentences describing what the conversation is about>

User message:
%s

Assistant reply:
%s`, maxRunes, truncateForTitlePrompt(userMessage), truncateForTitlePrompt(assistantReply)))
}

// ParseTitleAndSummary extracts a title and an optional summary from a model
// reply. It understands the TITLE:/SUMMARY: format but stays backward
// compatible: a reply without those labels is treated entirely as the title.
func ParseTitleAndSummary(raw string, titleMaxRunes int, summaryMaxRunes int) (string, string) {
	lines := strings.Split(raw, "\n")
	titleRaw := ""
	summaryRaw := ""
	inSummary := false
	sawTitleLabel := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "title:"):
			titleRaw = strings.TrimSpace(trimmed[len("title:"):])
			sawTitleLabel = true
			inSummary = false
		case strings.HasPrefix(lower, "summary:"):
			summaryRaw = strings.TrimSpace(trimmed[len("summary:"):])
			inSummary = true
		case inSummary && trimmed != "":
			summaryRaw = strings.TrimSpace(summaryRaw + " " + trimmed)
		}
	}
	if !sawTitleLabel {
		titleRaw = raw
	}
	return CleanChatTitle(titleRaw, titleMaxRunes), CleanChatSummary(summaryRaw, summaryMaxRunes)
}

// CleanChatSummary normalizes a model summary into a single trimmed line capped
// at maxRunes characters.
func CleanChatSummary(raw string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = ChatSummaryMaxRunes
	}
	summary := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(summary), "summary:") {
		summary = strings.TrimSpace(summary[len("summary:"):])
	}
	summary = strings.Join(strings.Fields(summary), " ")
	summary = strings.Trim(summary, " \t\r\n\"'`")
	runes := []rune(summary)
	if len(runes) > maxRunes {
		summary = strings.TrimSpace(string(runes[:maxRunes]))
	}
	return summary
}

func truncateForTitlePrompt(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= 280 {
		return value
	}
	return string(runes[:280])
}

func CleanChatTitle(raw string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = ChatTitleMaxRunes
	}
	title := strings.TrimSpace(raw)
	title = strings.Trim(title, " \t\r\n\"'`.,!?:;-")
	title = strings.Join(strings.Fields(title), " ")
	if strings.Contains(title, ":") {
		parts := strings.SplitN(title, ":", 2)
		prefix := strings.ToLower(strings.TrimSpace(parts[0]))
		if prefix == "title" || prefix == "titulo" || prefix == "chat" {
			title = strings.TrimSpace(parts[1])
		}
	}
	title = strings.Trim(title, " \t\r\n\"'`.,!?:;-")
	runes := []rune(title)
	if len(runes) > maxRunes {
		title = strings.TrimSpace(string(runes[:maxRunes]))
		title = strings.Trim(title, " \t\r\n\"'`.,!?:;-")
	}
	if len([]rune(title)) < 2 {
		return ""
	}
	return title
}

func UniqueChatTitle(title string, chatID string, chats []domain.Chat, fallbackClock func() time.Time) string {
	base := strings.TrimSpace(title)
	if base == "" {
		return ""
	}
	existing := map[string]bool{}
	for _, chat := range chats {
		if chat.ID == chatID {
			continue
		}
		existing[strings.ToLower(strings.TrimSpace(chat.Title))] = true
	}
	if !existing[strings.ToLower(base)] {
		return base
	}
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s %d", base, i)
		if !existing[strings.ToLower(candidate)] {
			return candidate
		}
	}
	now := time.Now()
	if fallbackClock != nil {
		now = fallbackClock()
	}
	return fmt.Sprintf("%s %d", base, now.Unix())
}

func FallbackChatTitle(userMessage string, assistantReply string, maxRunes int) string {
	source := strings.TrimSpace(userMessage)
	if source == "" {
		source = assistantReply
	}
	source = strings.Join(strings.Fields(strings.TrimSpace(source)), " ")
	if source == "" {
		return ""
	}
	words := strings.Fields(source)
	filtered := make([]string, 0, len(words))
	for _, word := range words {
		normalized := strings.ToLower(strings.Trim(word, " \t\r\n\"'`.,!?:;-()[]{}"))
		if normalized == "" || isChatTitleStopWord(normalized) {
			continue
		}
		filtered = append(filtered, normalized)
		if len(filtered) == 4 {
			break
		}
	}
	if len(filtered) == 0 {
		for _, word := range words {
			cleaned := strings.Trim(word, " \t\r\n\"'`.,!?:;-()[]{}")
			if cleaned != "" {
				filtered = append(filtered, strings.ToLower(cleaned))
			}
			if len(filtered) == 4 {
				break
			}
		}
	}
	return CleanChatTitle(titleCaseWords(strings.Join(filtered, " ")), maxRunes)
}

func isChatTitleStopWord(word string) bool {
	switch word {
	case "a", "as", "o", "os", "um", "uma", "uns", "umas", "de", "da", "das", "do", "dos", "e", "em", "no", "na", "nos", "nas", "por", "para", "com", "que", "qual", "quais", "como", "sobre", "explique", "explica", "diga", "me", "frase", "resuma", "resumir", "the", "an", "and", "or", "in", "on", "for", "to", "of", "what", "how", "why", "please", "explain", "summarize":
		return true
	default:
		return false
	}
}

func titleCaseWords(value string) string {
	words := strings.Fields(value)
	for i, word := range words {
		runes := []rune(word)
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}
