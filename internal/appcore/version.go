package appcore

// appVersion is stamped at build time: the buildgate passes
// -ldflags "-X aw/internal/appcore.appVersion=<tag>" when AW_VERSION is set
// (scripts/release.sh does). Local builds stay "dev", which the future
// updater compares as 0.0.0.
var appVersion = "dev"

// AppVersion reports the stamped build version ("dev" for local builds).
func AppVersion() string { return appVersion }

// GetAppVersion exposes the build version to the frontend (Settings › About).
func (a *App) GetAppVersion() string { return AppVersion() }
