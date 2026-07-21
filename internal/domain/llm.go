package domain

type ModelConfig struct {
	ProviderID        string
	ProviderName      string
	AuthType          string
	Model             string
	APIKey            string
	Credential        string
	BaseURL           string
	CredentialUpdater func(string) error `json:"-"`
}

type AgentReply struct {
	Text       string      `json:"text"`
	Provider   string      `json:"provider,omitempty"`
	Model      string      `json:"model,omitempty"`
	TokenUsage *TokenUsage `json:"tokenUsage,omitempty"`
	LLMTurns   []LLMTurn   `json:"-"`
}

type TokenUsage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
	Total  int `json:"total"`
}

type HistoryMessage struct {
	Role    string
	Content string
}

type LLMTurn struct {
	ID               string `json:"id"`
	SessionID        string `json:"sessionId"`
	TurnIndex        int    `json:"turnIndex"`
	RequestJSON      string `json:"requestJson"`
	ResponseText     string `json:"responseText,omitempty"`
	ToolCallsJSON    string `json:"toolCallsJson,omitempty"`
	Model            string `json:"model,omitempty"`
	PromptTokens     int    `json:"promptTokens,omitempty"`
	CompletionTokens int    `json:"completionTokens,omitempty"`
	FinishReason     string `json:"finishReason,omitempty"`
	CreatedAt        string `json:"createdAt"`
}

type ChatSessionInfo struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Ready     bool   `json:"ready"`
	PlanMode  bool   `json:"planMode"`
	Streaming bool   `json:"streaming"`
	// PartialReply is the text streamed so far by the in-flight run, so a
	// remounting view (module switch, PiP) can reseed the live bubble.
	PartialReply        string      `json:"partialReply,omitempty"`
	CompactReady        bool        `json:"compactReady"`
	GoalReady           bool        `json:"goalReady"`
	MessageCount        int         `json:"messageCount"`
	TokenUsage          *TokenUsage `json:"tokenUsage,omitempty"`
	LastUsage           *TokenUsage `json:"lastUsage,omitempty"`
	ContextWindow       int         `json:"contextWindow,omitempty"`
	ContextUsedPercent  int         `json:"contextUsedPercent,omitempty"`
	CompactionThreshold int         `json:"compactAtPercent,omitempty"`
	Error               string      `json:"error,omitempty"`
}
