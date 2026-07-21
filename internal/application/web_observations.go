package application

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

type SaveWebObservationInput struct {
	ChatID  string
	RunID   string
	Browser string
	TabID   string
	SiteKey string
	URL     string
	Title   string
	Payload any
}

type WebObservationView struct {
	ID         string `json:"id"`
	ChatID     string `json:"chatId"`
	RunID      string `json:"runId,omitempty"`
	Browser    string `json:"browser"`
	TabID      string `json:"tabId,omitempty"`
	SiteKey    string `json:"siteKey"`
	URL        string `json:"url,omitempty"`
	Title      string `json:"title,omitempty"`
	CapturedAt string `json:"capturedAt"`
	Payload    any    `json:"payload"`
}

func SaveWebObservation(store ports.WebObservationStore, input SaveWebObservationInput) (WebObservationView, error) {
	if store == nil || !store.IsUnlocked() {
		return WebObservationView{}, errors.New("vault is locked")
	}
	chatID := strings.TrimSpace(input.ChatID)
	if chatID == "" {
		return WebObservationView{}, errors.New("chatId is required")
	}
	browser := strings.ToLower(strings.TrimSpace(input.Browser))
	if browser == "" {
		return WebObservationView{}, errors.New("browser is required")
	}
	siteKey := normalizeWebObservationSiteKey(input.SiteKey, input.URL)
	if siteKey == "" {
		return WebObservationView{}, errors.New("siteKey or url is required")
	}
	if input.Payload == nil {
		return WebObservationView{}, errors.New("payload is required")
	}
	payloadJSON, err := json.Marshal(input.Payload)
	if err != nil {
		return WebObservationView{}, err
	}
	obs := domain.WebObservation{
		ID:          newWebObservationID(),
		ChatID:      chatID,
		RunID:       strings.TrimSpace(input.RunID),
		Browser:     browser,
		TabID:       strings.TrimSpace(input.TabID),
		SiteKey:     siteKey,
		URL:         strings.TrimSpace(input.URL),
		Title:       strings.TrimSpace(input.Title),
		CapturedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		PayloadJSON: string(payloadJSON),
	}
	saved, err := store.SaveWebObservation(obs)
	if err != nil {
		return WebObservationView{}, err
	}
	return webObservationView(saved), nil
}

func LatestWebObservation(store ports.WebObservationStore, chatID, browser, siteKey, rawURL string) (WebObservationView, bool, error) {
	if store == nil || !store.IsUnlocked() {
		return WebObservationView{}, false, errors.New("vault is locked")
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return WebObservationView{}, false, errors.New("chatId is required")
	}
	obs, ok, err := store.LatestWebObservation(chatID, strings.ToLower(strings.TrimSpace(browser)), normalizeWebObservationSiteKey(siteKey, rawURL))
	if err != nil || !ok {
		return WebObservationView{}, ok, err
	}
	return webObservationView(obs), true, nil
}

func ClearWebObservations(store ports.WebObservationStore) error {
	if store == nil || !store.IsUnlocked() {
		return errors.New("vault is locked")
	}
	return store.ClearWebObservations()
}

func webObservationView(obs domain.WebObservation) WebObservationView {
	var payload any
	if strings.TrimSpace(obs.PayloadJSON) != "" {
		_ = json.Unmarshal([]byte(obs.PayloadJSON), &payload)
	}
	return WebObservationView{
		ID:         obs.ID,
		ChatID:     obs.ChatID,
		RunID:      obs.RunID,
		Browser:    obs.Browser,
		TabID:      obs.TabID,
		SiteKey:    obs.SiteKey,
		URL:        obs.URL,
		Title:      obs.Title,
		CapturedAt: obs.CapturedAt,
		Payload:    payload,
	}
}

func normalizeWebObservationSiteKey(siteKey, rawURL string) string {
	siteKey = strings.TrimSpace(siteKey)
	if siteKey != "" {
		return strings.ToLower(strings.TrimRight(siteKey, "/"))
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host)
}

func newWebObservationID() string {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "webobs-" + hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return "webobs-" + hex.EncodeToString(buf[:])
}
