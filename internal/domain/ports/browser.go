package ports

import (
	"context"

	"aw/internal/domain"
)

// BrowserAutomation manages real browsers (Chrome/Edge) over CDP for the
// Agent Browser modules. Instances run on an aw-owned debugging port; the
// profile is the isolated aw one by default, the user's own on explicit
// opt-in (BrowserStartOptions.PersonalProfile).
type BrowserAutomation interface {
	ConfigureExecutable(id string, path string) error
	DefaultExecutable(id string) (string, error)
	Start(ctx context.Context, id string, opts domain.BrowserStartOptions) (domain.BrowserStatus, error)
	Stop(ctx context.Context, id string) (domain.BrowserStatus, error)
	Status(ctx context.Context, id string) (domain.BrowserStatus, error)
	Tabs(ctx context.Context, id string) ([]domain.BrowserTab, error)
	// Command runs one page-level automation command: navigate, snapshot,
	// click, fill or screenshot. Params mirror the ui.* actions.
	Command(ctx context.Context, id string, command string, params map[string]any) (any, error)
}
