//go:build !darwin

package agent

// ensureMicAccess is a no-op off macOS: there is no TCC layer to satisfy and
// native voice capture is darwin-only anyway (see nativeCaptureInputArgs).
func ensureMicAccess() error { return nil }
