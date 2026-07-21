package domain

type WebObservation struct {
	ID          string `json:"id"`
	ChatID      string `json:"chatId"`
	RunID       string `json:"runId,omitempty"`
	Browser     string `json:"browser"`
	TabID       string `json:"tabId,omitempty"`
	SiteKey     string `json:"siteKey"`
	URL         string `json:"url,omitempty"`
	Title       string `json:"title,omitempty"`
	CapturedAt  string `json:"capturedAt"`
	PayloadJSON string `json:"payloadJson"`
}
