package domain

type PromptDebugImageInfo struct {
	MimeType   string `json:"mimeType"`
	DataLength int    `json:"dataLength"`
}

type PromptDebugSnapshot struct {
	ID           string                 `json:"id"`
	ModuleID     string                 `json:"moduleId,omitempty"`
	Timestamp    string                 `json:"timestamp"`
	Provider     string                 `json:"provider,omitempty"`
	Model        string                 `json:"model,omitempty"`
	Turn         int                    `json:"turn,omitempty"`
	PlanMode     bool                   `json:"planMode"`
	SystemPrompt string                 `json:"systemPrompt"`
	UserPrompt   string                 `json:"userPrompt"`
	Images       []PromptDebugImageInfo `json:"images,omitempty"`
	ActiveTools  []string               `json:"activeTools,omitempty"`
	Raw          map[string]any         `json:"raw,omitempty"`
}
