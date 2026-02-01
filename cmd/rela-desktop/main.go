// rela-desktop runs the data entry application as a native desktop app using Wails.
//
// Usage:
//
//	rela-desktop [-project .]
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sync"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/desktop"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// Desktop is the backend bound to the Wails frontend.
// It manages project lifecycle: opening a directory picker, loading a project,
// and persisting recent projects in user preferences.
type Desktop struct {
	ctx     context.Context
	mu      sync.RWMutex
	app     *dataentry.App
	handler http.Handler
	loadErr string
	prefs   *desktop.Preferences
}

// coverage-ignore: Wails lifecycle callback
func (d *Desktop) startup(ctx context.Context) {
	d.ctx = ctx
}

// OpenProject opens a native directory picker and loads the selected project.
// It returns an error string (empty on success) so the JS frontend can react.
func (d *Desktop) OpenProject() string {
	dir, err := runtime.OpenDirectoryDialog(d.ctx, runtime.OpenDialogOptions{
		Title: "Open Rela Project",
	})
	if err != nil {
		return fmt.Sprintf("dialog error: %v", err)
	}
	if dir == "" {
		return "" // user cancelled
	}
	return d.LoadProject(dir)
}

// OpenRecentProject loads a project from the recent projects list.
// It returns an error string (empty on success) so the JS frontend can react.
func (d *Desktop) OpenRecentProject(path string) string {
	errMsg := d.LoadProject(path)
	if errMsg != "" {
		// Project no longer valid — remove from recent list.
		d.prefs.RemoveRecentProject(path)
		_ = d.prefs.Save()
		d.refreshMenu()
	}
	return errMsg
}

// LoadProject loads a rela project from the given directory.
func (d *Desktop) LoadProject(dir string) string {
	app, err := dataentry.NewApp(dir, storage.NewSafeFS(storage.NewOsFS()))
	if err != nil {
		d.mu.Lock()
		d.loadErr = err.Error()
		d.mu.Unlock()
		return err.Error()
	}
	d.mu.Lock()
	d.app = app
	d.handler = app.NewRouter()
	d.loadErr = ""
	d.mu.Unlock()

	if d.ctx != nil {
		runtime.WindowSetTitle(d.ctx, app.ProjectName())
	}

	// Update preferences with successfully opened project.
	d.prefs.AddRecentProject(app.ProjectRoot(), app.ProjectName())
	if saveErr := d.prefs.Save(); saveErr != nil {
		log.Printf("Warning: could not save preferences: %v", saveErr)
	}
	d.refreshMenu()

	return ""
}

// ServeHTTP dispatches to the loaded app router or the welcome page.
func (d *Desktop) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.mu.RLock()
	h := d.handler
	loadErr := d.loadErr
	d.mu.RUnlock()

	if h != nil {
		h.ServeHTTP(w, r)
		return
	}

	serveWelcomePage(w, d.prefs, loadErr)
}

// openProjectFromMenu handles File > Open Project from the native menu bar.
// coverage-ignore: menu callback - requires Wails runtime
func (d *Desktop) openProjectFromMenu(_ *menu.CallbackData) {
	dir, err := runtime.OpenDirectoryDialog(d.ctx, runtime.OpenDialogOptions{
		Title: "Open Rela Project",
	})
	if err != nil || dir == "" {
		return
	}
	if errMsg := d.LoadProject(dir); errMsg != "" {
		runtime.MessageDialog(d.ctx, runtime.MessageDialogOptions{ //nolint:errcheck // best-effort
			Type:    runtime.ErrorDialog,
			Title:   "Failed to open project",
			Message: errMsg,
		})
		return
	}
	runtime.WindowReloadApp(d.ctx)
}

// showAbout displays a dialog with version and build information.
// coverage-ignore: menu callback - requires Wails runtime
func (d *Desktop) showAbout(_ *menu.CallbackData) {
	runtime.MessageDialog(d.ctx, runtime.MessageDialogOptions{ //nolint:errcheck // best-effort
		Type:    runtime.InfoDialog,
		Title:   "About Rela Desktop",
		Message: fmt.Sprintf("Rela Desktop\nVersion %s\n\n%s/%s", Version, goruntime.GOOS, goruntime.GOARCH),
	})
}

// buildAppMenu constructs the application menu bar including recent projects.
func (d *Desktop) buildAppMenu() *menu.Menu {
	appMenu := menu.NewMenu()

	if goruntime.GOOS == "darwin" {
		macAppMenu := appMenu.AddSubmenu("Rela Desktop")
		macAppMenu.AddText("About Rela Desktop", nil, d.showAbout)
		macAppMenu.AddSeparator()
		macAppMenu.Append(menu.AppMenu())
	}

	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("Open Project...", keys.CmdOrCtrl("o"), d.openProjectFromMenu)
	fileMenu.AddSeparator()

	// Recent Projects submenu
	if len(d.prefs.RecentProjects) > 0 {
		recentMenu := fileMenu.AddSubmenu("Recent Projects")
		for _, rp := range d.prefs.RecentProjects {
			proj := rp // capture for closure
			label := proj.Name
			if label == "" {
				label = filepath.Base(proj.Path)
			}
			recentMenu.AddText(label, nil, func(_ *menu.CallbackData) {
				if errMsg := d.LoadProject(proj.Path); errMsg != "" {
					runtime.MessageDialog(d.ctx, runtime.MessageDialogOptions{ //nolint:errcheck // best-effort
						Type:    runtime.ErrorDialog,
						Title:   "Failed to open project",
						Message: errMsg,
					})
					return
				}
				runtime.WindowReloadApp(d.ctx)
			})
		}
		recentMenu.AddSeparator()
		recentMenu.AddText("Clear Recent Projects", nil, func(_ *menu.CallbackData) {
			d.prefs.ClearRecentProjects()
			if err := d.prefs.Save(); err != nil {
				log.Printf("Warning: could not save preferences: %v", err)
			}
			d.refreshMenu()
		})
		fileMenu.AddSeparator()
	}

	if goruntime.GOOS != "darwin" {
		fileMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
			runtime.Quit(d.ctx)
		})
	} else {
		fileMenu.AddText("Close Window", keys.CmdOrCtrl("w"), func(_ *menu.CallbackData) {
			runtime.Quit(d.ctx)
		})
	}

	if goruntime.GOOS == "darwin" {
		appMenu.Append(menu.EditMenu())
	}

	if goruntime.GOOS != "darwin" {
		helpMenu := appMenu.AddSubmenu("Help")
		helpMenu.AddText("About Rela Desktop", nil, d.showAbout)
	}

	return appMenu
}

// refreshMenu rebuilds and applies the application menu.
func (d *Desktop) refreshMenu() {
	if d.ctx == nil {
		return
	}
	m := d.buildAppMenu()
	runtime.MenuSetApplicationMenu(d.ctx, m)
	runtime.MenuUpdateApplicationMenu(d.ctx)
}

// coverage-ignore: main function - entry point
func main() {
	projectDir := flag.String("project", ".", "Path to the rela project directory")
	flag.Parse()

	// Load desktop preferences.
	prefs, err := desktop.Load()
	if err != nil {
		log.Printf("Warning: could not load preferences: %v", err)
		prefs = &desktop.Preferences{}
	}

	d := &Desktop{prefs: prefs}

	// Determine which project to open.
	projectToLoad := resolveProjectDir(*projectDir, prefs)
	if projectToLoad != "" {
		if errMsg := d.LoadProject(projectToLoad); errMsg != "" {
			log.Printf("Could not load project from %q: %s", projectToLoad, errMsg)
		}
	}

	title := "Rela Desktop"
	if d.app != nil {
		title = d.app.ProjectName()
	}

	wailsErr := wails.Run(&options.App{
		Title:  title,
		Width:  1280,
		Height: 800,
		Menu:   d.buildAppMenu(),
		AssetServer: &assetserver.Options{
			Handler: d,
		},
		OnStartup: d.startup,
		Bind:      []interface{}{d},
	})
	if wailsErr != nil {
		log.Fatalf("Wails error: %v", wailsErr)
	}
}

// resolveProjectDir determines which project directory to load at startup.
// Priority: explicit -project flag > current dir if rela project > last project from preferences.
func resolveProjectDir(flagValue string, prefs *desktop.Preferences) string {
	// Explicit flag or current directory is a rela project.
	if flagValue != "." || isRelaProject(flagValue) {
		return flagValue
	}
	// Fall back to last project from preferences.
	if prefs.LastProject != "" && isRelaProject(prefs.LastProject) {
		return prefs.LastProject
	}
	return ""
}

// isRelaProject checks if the directory looks like a rela project.
func isRelaProject(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "metamodel.yaml"))
	return err == nil
}
