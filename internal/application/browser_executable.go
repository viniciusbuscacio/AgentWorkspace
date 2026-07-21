package application

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

type BrowserExecutableSettings struct {
	Path        string
	DefaultPath string
	Custom      bool
}

func BrowserExecutable(ctx context.Context, browser ports.BrowserAutomation, store ports.BrowserExecutableStore, id string) (BrowserExecutableSettings, error) {
	id = strings.TrimSpace(id)
	if store == nil {
		return BrowserExecutableSettings{}, fmt.Errorf("browser executable store is required")
	}
	defaultPath := ""
	if browser != nil {
		var err error
		defaultPath, err = browser.DefaultExecutable(id)
		if err != nil {
			return BrowserExecutableSettings{}, err
		}
	}
	defaultStatus, err := BrowserStatus(ctx, browser, id)
	if err != nil {
		return BrowserExecutableSettings{}, err
	}
	custom, err := store.LoadBrowserExecutable(id)
	if err != nil {
		return BrowserExecutableSettings{}, err
	}
	custom = strings.TrimSpace(custom)
	path := defaultStatus.Binary
	if custom != "" {
		path = custom
	}
	if defaultPath == "" {
		defaultPath = defaultStatus.Binary
	}
	return BrowserExecutableSettings{
		Path:        path,
		DefaultPath: defaultPath,
		Custom:      custom != "",
	}, nil
}

func SaveBrowserExecutable(ctx context.Context, browser ports.BrowserAutomation, store ports.BrowserExecutableStore, id string, path string) (BrowserExecutableSettings, error) {
	id = strings.TrimSpace(id)
	path = strings.TrimSpace(path)
	if store == nil {
		return BrowserExecutableSettings{}, fmt.Errorf("browser executable store is required")
	}
	if path != "" {
		if runtime.GOOS == "windows" && !strings.EqualFold(strings.TrimSpace(ext(path)), ".exe") {
			return BrowserExecutableSettings{}, fmt.Errorf("browser executable must be a .exe file")
		}
		if info, err := os.Stat(path); err != nil {
			return BrowserExecutableSettings{}, fmt.Errorf("browser executable not found at %q", path)
		} else if info.IsDir() {
			return BrowserExecutableSettings{}, fmt.Errorf("browser executable must be a file")
		}
	}
	if err := store.SaveBrowserExecutable(id, path); err != nil {
		return BrowserExecutableSettings{}, err
	}
	if browser != nil {
		if err := browser.ConfigureExecutable(id, path); err != nil {
			return BrowserExecutableSettings{}, err
		}
	}
	return BrowserExecutable(ctx, browser, store, id)
}

func ext(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx < 0 {
		return ""
	}
	return path[idx:]
}

func BrowserExecutableDialogTitle(id string) string {
	switch strings.TrimSpace(id) {
	case domain.BrowserModuleEdge:
		return "Choose Microsoft Edge executable"
	case domain.BrowserModuleChrome:
		return "Choose Google Chrome executable"
	default:
		return "Choose browser executable"
	}
}
