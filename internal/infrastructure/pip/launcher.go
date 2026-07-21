package pip

// Launcher is the ports.PipLauncher adapter: it spawns the PiP child process.
type Launcher struct{}

// NewLauncher returns the PiP process launcher adapter.
func NewLauncher() Launcher { return Launcher{} }

// StartProcess implements ports.PipLauncher.
func (Launcher) StartProcess(pipURL string) error { return StartProcess(pipURL) }
