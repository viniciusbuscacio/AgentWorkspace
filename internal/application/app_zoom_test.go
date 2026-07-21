package application

import "testing"

type memoryZoomStore struct {
	percent int
	err     error
}

func (store *memoryZoomStore) LoadAppZoomPercent() (int, error) {
	return store.percent, store.err
}

func (store *memoryZoomStore) SaveAppZoomPercent(percent int) error {
	store.percent = percent
	return store.err
}

func TestAppZoomPreferencesDefaultAndClamp(t *testing.T) {
	store := &memoryZoomStore{}
	prefs := NewAppZoomPreferences(store)

	if got := prefs.Get(); got != AppZoomDefaultPercent {
		t.Fatalf("Get() = %d, want default %d", got, AppZoomDefaultPercent)
	}
	if got, err := prefs.Set(500); err != nil || got != AppZoomMaxPercent {
		t.Fatalf("Set(high) = %d, %v; want %d, nil", got, err, AppZoomMaxPercent)
	}
	if got := prefs.Get(); got != AppZoomMaxPercent {
		t.Fatalf("Get() after high = %d, want %d", got, AppZoomMaxPercent)
	}
	if got, err := prefs.Set(10); err != nil || got != AppZoomMinPercent {
		t.Fatalf("Set(low) = %d, %v; want %d, nil", got, err, AppZoomMinPercent)
	}
	if got := prefs.Get(); got != AppZoomMinPercent {
		t.Fatalf("Get() after low = %d, want %d", got, AppZoomMinPercent)
	}
}
