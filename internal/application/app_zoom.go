package application

import "aw/internal/domain/ports"

const (
	AppZoomMinPercent     = 50
	AppZoomMaxPercent     = 200
	AppZoomDefaultPercent = 100
)

type AppZoomPreferences struct {
	store ports.AppZoomStore
}

func NewAppZoomPreferences(store ports.AppZoomStore) AppZoomPreferences {
	return AppZoomPreferences{store: store}
}

func ClampAppZoomPercent(percent int) int {
	if percent < AppZoomMinPercent {
		return AppZoomMinPercent
	}
	if percent > AppZoomMaxPercent {
		return AppZoomMaxPercent
	}
	return percent
}

func (prefs AppZoomPreferences) Get() int {
	if prefs.store == nil {
		return AppZoomDefaultPercent
	}
	percent, err := prefs.store.LoadAppZoomPercent()
	if err != nil || percent == 0 {
		return AppZoomDefaultPercent
	}
	return ClampAppZoomPercent(percent)
}

func (prefs AppZoomPreferences) Set(percent int) (int, error) {
	percent = ClampAppZoomPercent(percent)
	if prefs.store == nil {
		return percent, nil
	}
	return percent, prefs.store.SaveAppZoomPercent(percent)
}
