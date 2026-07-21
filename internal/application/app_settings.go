package application

import "aw/internal/domain/ports"

// LastSession is the persisted view + chat to restore after unlock.
type LastSession struct {
	View   string
	ChatID string
}

// LoadLastSession reads the persisted last-active view and chat.
func LoadLastSession(store ports.SessionRestoreStore) LastSession {
	view, _ := store.LoadLastView()
	chatID, _ := store.LoadLastChatID()
	return LastSession{View: view, ChatID: chatID}
}

// SaveLastSession persists the last-active view and chat.
func SaveLastSession(store ports.SessionRestoreStore, view string, chatID string) error {
	return store.SaveLastSession(view, chatID)
}

// LoadProviderFallbackOrder returns the user-defined provider priority list.
func LoadProviderFallbackOrder(store ports.ProviderFallbackStore) []string {
	order, err := store.LoadProviderFallbackOrder()
	if err != nil {
		return nil
	}
	return order
}

// SaveProviderFallbackOrder persists the provider priority list (index 0 = #1).
func SaveProviderFallbackOrder(store ports.ProviderFallbackStore, ids []string) error {
	return store.SaveProviderFallbackOrder(ids)
}

// GetProviderCooldownMinutes returns the circuit-breaker bench time in minutes.
func GetProviderCooldownMinutes(store ports.ProviderFallbackStore) int {
	return store.LoadProviderCooldownMinutes()
}

// SaveProviderCooldownMinutes persists the circuit-breaker bench time.
func SaveProviderCooldownMinutes(store ports.ProviderFallbackStore, minutes int) error {
	return store.SaveProviderCooldownMinutes(minutes)
}

// DesktopNotificationsEnabled reports the user's desktop-notifications toggle.
func DesktopNotificationsEnabled(store ports.DesktopNotificationPrefStore) bool {
	return store.LoadDesktopNotificationsEnabled()
}
