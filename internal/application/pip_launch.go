package application

import "aw/internal/domain/ports"

// LaunchPipProcess spawns the picture-in-picture window through the launcher
// port. The PiP URL is built by the caller (pure) and passed in.
func LaunchPipProcess(launcher ports.PipLauncher, pipURL string) error {
	return launcher.StartProcess(pipURL)
}
