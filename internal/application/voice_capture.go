package application

import (
	"fmt"

	"aw/internal/domain/ports"
)

func StartVoiceCapture(capture ports.VoiceCapture) error {
	if capture == nil {
		return fmt.Errorf("voice capture is not available")
	}
	return capture.Start()
}

func StopVoiceCapture(capture ports.VoiceCapture) ([]byte, error) {
	if capture == nil {
		return nil, fmt.Errorf("voice capture is not available")
	}
	return capture.Stop()
}

func CancelVoiceCapture(capture ports.VoiceCapture) error {
	if capture == nil {
		return nil
	}
	return capture.Cancel()
}
