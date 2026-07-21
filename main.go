package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"aw/internal/appcore"
	"aw/internal/infrastructure/pip"
)

//go:embed all:frontend/dist
var assets embed.FS

// App is the GUI shell's binding wrapper around the extracted appcore.App. It
// exists only to keep the Wails binding namespace at go/main/App (used by 371
// frontend imports): embedding *appcore.App promotes every exported method, so
// `wails generate module` produces an identical App.js. The real app lives in
// internal/appcore so the headless awd binary can import it too.
type App struct {
	*appcore.App
}

// NewApp builds the GUI app: the extracted appcore.App plus the embedded
// frontend injected for web mode.
func NewApp() *App {
	core := appcore.NewApp()
	if dist, err := fs.Sub(assets, "frontend/dist"); err == nil {
		core.SetWebAssets(dist)
	}
	return &App{App: core}
}

func main() {
	// scripts/release.sh verifies the stamped version before publishing.
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(appcore.AppVersion())
		return
	}

	if pipURL, ok := pipLaunchURL(os.Args[1:]); ok {
		runPipWindow(pipURL)
		return
	}

	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "Agent Workspace",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 23, G: 23, B: 23, A: 1},
		// Without a Mac options block Wails leaves zoomable=false and the
		// window's green maximize/full-screen button is born disabled.
		Mac: &mac.Options{},
		// Cut/Copy/Paste on right-click in production builds. Wails scopes the
		// default menu to editable fields and text selections, so the modules'
		// own context menus are unaffected.
		EnableDefaultContextMenu: true,
		OnStartup:                app.Startup,
		OnDomReady:               app.OnDomReady,
		OnShutdown:               app.Shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

func pipLaunchURL(args []string) (string, bool) {
	pipMode := false
	pipURL := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--aw-pip":
			pipMode = true
		case "--pip-url":
			if i+1 < len(args) {
				pipURL = args[i+1]
				i++
			}
		default:
			if strings.HasPrefix(args[i], "--pip-url=") {
				pipURL = strings.TrimPrefix(args[i], "--pip-url=")
			}
		}
	}
	return pipURL, pipMode && pipURL != ""
}

// runPipWindow serves the same embedded React frontend as the main window,
// with an injected window.aw_PIP config so it boots in PiP mode and talks to
// the host app through the token-scoped localhost API instead of Wails
// bindings. This keeps the PiP chat identical to the main chat by construction.
func runPipWindow(pipURL string) {
	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		println("PiP error:", err.Error())
		return
	}
	pipControl := &PipControl{}
	err = wails.Run(&options.App{
		Title:       "Agent Workspace Chat",
		Width:       560,
		Height:      720,
		MinWidth:    380,
		MinHeight:   440,
		Frameless:   false,
		AlwaysOnTop: true,
		Mac:         &mac.Options{},
		AssetServer: &assetserver.Options{Handler: pip.AssetHandler(dist, pipURL)},
		// Same Cut/Copy/Paste affordance as the main window (see above).
		EnableDefaultContextMenu: true,
		OnStartup:                pipControl.startup,
		Bind: []interface{}{
			pipControl,
		},
		BackgroundColour: &options.RGBA{
			R: 23,
			G: 23,
			B: 23,
			A: 1,
		},
	})
	if err != nil {
		println("PiP error:", err.Error())
	}
}
