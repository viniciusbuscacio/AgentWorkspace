package pip

import (
	"encoding/json"
	"net/url"
)

// Config carries what the PiP window's frontend needs to talk back to the
// host app: the token-scoped localhost API plus the chat to show.
type Config struct {
	API    string `json:"api"`
	Token  string `json:"token"`
	ChatID string `json:"chatId"`
}

// ParseConfig extracts the Config from a BuildURL-produced launch URL.
func ParseConfig(pipURL string) Config {
	parsed, err := url.Parse(pipURL)
	if err != nil {
		return Config{}
	}
	query := parsed.Query()
	return Config{
		API:    query.Get("api"),
		Token:  query.Get("token"),
		ChatID: query.Get("chatId"),
	}
}

// ConfigScript renders the inline script tag injected into the PiP window's
// index.html so the React frontend boots in PiP mode.
func ConfigScript(pipURL string) string {
	encoded, err := json.Marshal(ParseConfig(pipURL))
	if err != nil {
		return ""
	}
	return "<script>window.aw_PIP = " + string(encoded) + ";</script>"
}
