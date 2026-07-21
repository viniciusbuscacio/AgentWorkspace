package ports

import "time"

type AppZoomStore interface {
	LoadAppZoomPercent() (int, error)
	SaveAppZoomPercent(percent int) error
}

type AppConfigStore interface {
	SaveVaultDir(dir string) error
	SaveAutoLockMinutes(minutes int) error
}

type SubagentModeStore interface {
	LoadSubagentMode() string
	SaveSubagentMode(mode string) error
}

type BrowserExecutableStore interface {
	LoadBrowserExecutable(moduleID string) (string, error)
	SaveBrowserExecutable(moduleID string, path string) error
}

type AutoLockPolicy interface {
	Touch(now time.Time)
	SetTimeoutMinutes(minutes int)
	TimeoutMinutes() int
	Expired(now time.Time) bool
}
