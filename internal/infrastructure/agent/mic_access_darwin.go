//go:build darwin

package agent

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AVFoundation -framework Foundation
int aw_request_mic_access(void);
*/
import "C"

import "fmt"

// ensureMicAccess makes the app itself hold the macOS microphone grant before
// ffmpeg is spawned. Without this the TCC request is attributed to the bare
// ffmpeg binary, which macOS silently denies — no prompt, "Cannot use
// Microphone" — even when System Settings shows the app as allowed. Blocks
// while the system permission prompt is up (first run only).
func ensureMicAccess() error {
	if C.aw_request_mic_access() == 1 {
		return nil
	}
	return fmt.Errorf("microphone access is denied for Agent Workspace — enable it in System Settings › Privacy & Security › Microphone, then try again")
}
