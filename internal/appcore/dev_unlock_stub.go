//go:build !devunlock

package appcore

// devAutoUnlock is a no-op in the production build. The real implementation in
// dev_unlock.go is compiled only under the `devunlock` build tag, so the
// release binary contains no auto-unlock code at all.
func (a *App) devAutoUnlock() {}
