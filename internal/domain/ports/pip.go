package ports

// PipLauncher spawns the picture-in-picture window as a separate OS process.
// Spawning a process is external I/O, so the interface layer reaches it through
// this port instead of calling the infrastructure helper directly. Building the
// PiP URL is pure and is done directly by the caller.
type PipLauncher interface {
	StartProcess(pipURL string) error
}
