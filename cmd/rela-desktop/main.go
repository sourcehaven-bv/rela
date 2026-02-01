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
	"sync"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/desktop"
)

// coverage-ignore: main function - entry point

// SwitchableHandler is an http.Handler that delegates to an underlying handler
// which can be swapped at runtime for project switching.
type SwitchableHandler struct {
	mu      sync.RWMutex
	handler http.Handler
}

func (s *SwitchableHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	h := s.handler
	s.mu.RUnlock()
	h.ServeHTTP(w, r)
}

func (s *SwitchableHandler) SetHandler(h http.Handler) {
	s.mu.Lock()
	s.handler = h
	s.mu.Unlock()
}

// DesktopApp is the central desktop application struct, bound to Wails so
// the frontend can call its exported methods.
type DesktopApp struct {
	ctx     context.Context
	handler *SwitchableHandler
	prefs   *desktop.Preferences
	app     *dataentry.App // nil when no project is loaded
}

// OnStartup is called by Wails when the application starts.
func (d *DesktopApp) OnStartup(ctx context.Context) {
	d.ctx = ctx
}

// OpenProject shows a directory picker dialog and switches to the selected project.
func (d *DesktopApp) OpenProject() error {
	dir, err := wailsruntime.OpenDirectoryDialog(d.ctx, wailsruntime.OpenDialogOptions{
		Title: "Open Rela Project",
	})
	if err != nil {
		return err
	}
	if dir == "" {
		return nil // user cancelled
	}
	return d.switchProject(dir)
}

// OpenRecentProject switches to a project from the recent projects list.
func (d *DesktopApp) OpenRecentProject(path string) error {
	return d.switchProject(path)
}

// switchProject validates and loads a new project, swaps the handler, and reloads the window.
func (d *DesktopApp) switchProject(dir string) error {
	app, err := dataentry.NewApp(dir)
	if err != nil {
		_, _ = wailsruntime.MessageDialog(d.ctx, wailsruntime.MessageDialogOptions{
			Type:    wailsruntime.ErrorDialog,
			Title:   "Cannot Open Project",
			Message: fmt.Sprintf("Failed to open project at %s:\n\n%v", dir, err),
		})
		return err
	}

	d.app = app
	d.handler.SetHandler(app.NewRouter())

	// Update preferences.
	d.prefs.AddRecentProject(app.ProjectRoot(), app.ProjectName())
	if saveErr := d.prefs.Save(); saveErr != nil {
		log.Printf("Warning: could not save preferences: %v", saveErr)
	}

	// Update window title and menu.
	wailsruntime.WindowSetTitle(d.ctx, app.ProjectName())
	d.updateMenu()

	// Reload the window to show the new project.
	wailsruntime.WindowReloadApp(d.ctx)
	return nil
}

// buildMenu constructs the application menu.
func (d *DesktopApp) buildMenu() *menu.Menu {
	appMenu := menu.NewMenu()

	// macOS app menu (About, Services, Quit, etc.)
	appMenu.Append(menu.AppMenu())

	// File menu
	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("Open Project...", keys.CmdOrCtrl("o"), func(_ *menu.CallbackData) {
		if err := d.OpenProject(); err != nil {
			log.Printf("Open project error: %v", err)
		}
	})
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
				if err := d.OpenRecentProject(proj.Path); err != nil {
					log.Printf("Open recent project error: %v", err)
				}
			})
		}
		recentMenu.AddSeparator()
		recentMenu.AddText("Clear Recent Projects", nil, func(_ *menu.CallbackData) {
			d.prefs.ClearRecentProjects()
			if err := d.prefs.Save(); err != nil {
				log.Printf("Warning: could not save preferences: %v", err)
			}
			d.updateMenu()
		})
		fileMenu.AddSeparator()
	}

	fileMenu.AddText("Close Window", keys.CmdOrCtrl("w"), func(_ *menu.CallbackData) {
		wailsruntime.Quit(d.ctx)
	})

	// Standard Edit menu (Undo, Redo, Cut, Copy, Paste, Select All)
	appMenu.Append(menu.EditMenu())

	return appMenu
}

// updateMenu rebuilds and applies the application menu.
func (d *DesktopApp) updateMenu() {
	m := d.buildMenu()
	wailsruntime.MenuSetApplicationMenu(d.ctx, m)
	wailsruntime.MenuUpdateApplicationMenu(d.ctx)
}

func main() {
	projectDir := flag.String("project", "", "Path to the rela project directory")
	flag.Parse()

	// Load desktop preferences.
	prefs, err := desktop.Load()
	if err != nil {
		log.Printf("Warning: could not load preferences: %v", err)
		prefs = &desktop.Preferences{}
	}

	// Determine which project to open.
	var (
		app      *dataentry.App
		errorMsg string
	)

	dir := *projectDir
	if dir == "" {
		// No explicit project flag — try last project from preferences.
		if prefs.LastProject != "" {
			dir = prefs.LastProject
		}
	}

	if dir != "" {
		// Validate and load the project.
		if !isValidProjectDir(dir) {
			errorMsg = fmt.Sprintf("Last project directory no longer exists or is not a valid rela project: %s", dir)
			prefs.RemoveRecentProject(dir)
			_ = prefs.Save()
		} else {
			app, err = dataentry.NewApp(dir)
			if err != nil {
				errorMsg = fmt.Sprintf("Could not open project at %s: %v", dir, err)
			}
		}
	}

	// Build the handler.
	switchable := &SwitchableHandler{}
	if app != nil {
		switchable.SetHandler(app.NewRouter())
		// Update preferences with successfully opened project.
		prefs.AddRecentProject(app.ProjectRoot(), app.ProjectName())
		if saveErr := prefs.Save(); saveErr != nil {
			log.Printf("Warning: could not save preferences: %v", saveErr)
		}
	} else {
		switchable.SetHandler(newWelcomeHandler(prefs, errorMsg))
	}

	dApp := &DesktopApp{
		handler: switchable,
		prefs:   prefs,
		app:     app,
	}

	// Window title.
	title := "Rela Desktop"
	if app != nil {
		title = app.ProjectName()
	}

	wailsErr := wails.Run(&options.App{
		Title:  title,
		Width:  1280,
		Height: 800,
		Menu:   dApp.buildMenu(),
		AssetServer: &assetserver.Options{
			Handler: switchable,
		},
		OnStartup: dApp.OnStartup,
		Bind:      []interface{}{dApp},
	})
	if wailsErr != nil {
		log.Fatalf("Wails error: %v", wailsErr)
	}
}

// isValidProjectDir checks that a directory exists and contains a metamodel.yaml file.
func isValidProjectDir(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, "metamodel.yaml"))
	return err == nil
}
