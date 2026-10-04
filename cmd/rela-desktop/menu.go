package main

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"gopkg.in/yaml.v3"
)

// The menu bar.
//
// macOS has one menu bar for the whole app, but a document app rebuilds the
// part that belongs to a document whenever another document window comes to
// the front. Rela does the same: the Go menu lists the spaces of the project
// in the focused window, so ⌘1 means that project's first space. Everything
// else in the menu is global.
//
// Items that act on the page (Back, the command palette, a space) do not
// navigate the webview directly. They send a shell command the SPA runs (see
// sendShellCommand), so the page stays the one place that knows its routes.

// maxSpaceShortcuts is how many spaces get ⌘1…⌘9.
const maxSpaceShortcuts = 9

// menuSpace is the part of a space the Go menu shows.
type menuSpace struct {
	ID    string `yaml:"id"`
	Label string `yaml:"label"`
}

// menuBar builds the menu bar and tracks which project's window is in
// front. It holds the Desktop because nearly every item acts on it.
type menuBar struct {
	d *Desktop
	// focused is the project of the front window; "" means the active one.
	focused atomic.Pointer[string]
}

// configLoader reads one of a project's config files. appbuild.Services'
// ProjectFiles satisfies it.
type configLoader interface {
	Load(ctx context.Context, name string) ([]byte, error)
}

// projectSpaces reads the spaces declared in the project's data-entry.yaml.
// A project with no file, no spaces or an unreadable file gets no space
// items: the menu is a shortcut, and the SPA reports config errors itself.
func projectSpaces(ctx context.Context, files configLoader) []menuSpace {
	if files == nil {
		return nil
	}
	data, err := files.Load(ctx, "data-entry.yaml")
	if err != nil {
		return nil
	}
	var cfg struct {
		Spaces []menuSpace `yaml:"spaces"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	out := cfg.Spaces[:0]
	for _, s := range cfg.Spaces {
		if s.ID != "" {
			out = append(out, s)
		}
	}
	return out
}

// projectOfBase returns the project id a window route is mounted under, or ""
// for a route served by the active project.
func projectOfBase(route string) string {
	if id, _, ok := splitProjectPath(route); ok {
		return id
	}
	return ""
}

// trackWindow remembers which project win shows, so that bringing it to the
// front can point the menu at that project. projectID "" means the active
// project.
// coverage-ignore-func: requires Wails runtime
func (m *menuBar) trackWindow(win *application.WebviewWindow, projectID string) {
	d := m.d
	win.OnWindowEvent(events.Common.WindowFocus, func(*application.WindowEvent) {
		m.focused.Store(&projectID)
		d.refreshMenu()
	})
	trackFullscreen(win)
}

// focusedFiles is the config loader of the project in the focused window, or
// nil when no project is open.
func (m *menuBar) focusedFiles() configLoader {
	d := m.d
	if id := m.focused.Load(); id != nil && *id != "" {
		if p := d.registry.get(*id); p != nil && p.svc != nil {
			return p.svc.ProjectFiles()
		}
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.svc == nil {
		return nil
	}
	return d.svc.ProjectFiles()
}

// command returns a menu callback that sends a shell command to the focused
// window.
func (m *menuBar) command(name, arg string) func(*application.Context) {
	d := m.d
	return func(*application.Context) { sendShellCommand(d.wails, name, arg) }
}

// build constructs the menu bar. Roles give the platform's own items
// (Edit's Undo/Copy/Paste, the Window list) and expand to nothing where a
// platform has no such menu.
func (m *menuBar) build() *application.Menu {
	menu := application.NewMenu()
	mac := goruntime.GOOS == "darwin"

	if mac {
		m.addAppMenu(menu)
	}
	m.addFileMenu(menu, mac)
	menu.AddRole(application.EditMenu)
	m.addViewMenu(menu)
	m.addGoMenu(menu)
	if mac {
		menu.AddRole(application.WindowMenu)
	}
	m.addHelpMenu(menu, mac)
	return menu
}

// addAppMenu is the macOS application menu. Built by hand rather than from
// the AppMenu role, because the role has no Settings item.
func (m *menuBar) addAppMenu(menu *application.Menu) {
	d := m.d
	app := menu.AddSubmenu("Rela Desktop")
	app.Add("About Rela Desktop").OnClick(d.showAbout)
	app.AddSeparator()
	app.Add("Settings…").SetAccelerator("CmdOrCtrl+,").OnClick(m.command("settings", ""))
	app.AddSeparator()
	app.AddRole(application.ServicesMenu)
	app.AddSeparator()
	app.AddRole(application.Hide)
	app.AddRole(application.HideOthers)
	app.AddRole(application.UnHide)
	app.AddSeparator()
	app.AddRole(application.Quit)
}

func (m *menuBar) addFileMenu(menu *application.Menu, mac bool) {
	d := m.d
	file := menu.AddSubmenu("File")
	file.Add("New Window").SetAccelerator("CmdOrCtrl+n").OnClick(func(*application.Context) {
		if errMsg := d.OpenWindow("/", ""); errMsg != "" {
			slog.Warn("could not open window", "error", errMsg)
		}
	})
	file.AddSeparator()
	m.addDocumentMenu(file)
	file.Add("Open Project…").SetAccelerator("CmdOrCtrl+o").OnClick(d.openProjectFromMenu)
	m.addRecentMenu(file)
	file.Add("Clone from Git…").SetAccelerator("CmdOrCtrl+shift+o").OnClick(d.cloneFromGitMenu)
	file.AddSeparator()
	m.addDatabaseMenu(file)
	file.Add("Project Settings…").OnClick(func(*application.Context) {
		if errMsg := d.OpenWindow(settingsPath, "Project Settings"); errMsg != "" {
			d.errorDialog("Could not open Project Settings", errMsg)
		}
	})
	file.AddSeparator()

	if mac {
		// Close the window rather than quitting, as every Mac app does.
		file.Add("Close Window").SetAccelerator("CmdOrCtrl+w").OnClick(func(*application.Context) {
			if w := d.wails.Window.Current(); w != nil {
				w.Close()
			}
		})
		return
	}
	file.Add("Quit").SetAccelerator("CmdOrCtrl+q").OnClick(func(*application.Context) {
		if d.wails != nil {
			d.wails.Quit()
		}
	})
}

func (m *menuBar) addRecentMenu(file *application.Menu) {
	d := m.d
	recent := file.AddSubmenu("Open Recent")
	if len(d.prefs.RecentProjects) == 0 {
		recent.Add("No Recent Projects").SetEnabled(false)
		return
	}
	for _, rp := range d.prefs.RecentProjects {
		proj := rp
		label := proj.Name
		if label == "" {
			label = filepath.Base(proj.Path)
		}
		recent.Add(label).OnClick(func(*application.Context) {
			if errMsg := d.OpenRecentProject(proj.Path); errMsg != "" {
				d.errorDialog("Failed to open project", errMsg)
				return
			}
			d.reloadWindow()
		})
	}
	recent.AddSeparator()
	recent.Add("Clear Menu").OnClick(func(*application.Context) {
		d.prefs.ClearRecentProjects()
		if err := d.prefs.Save(); err != nil {
			slog.Warn("could not save preferences", "error", err)
		}
		d.refreshMenu()
	})
}

func (m *menuBar) addViewMenu(menu *application.Menu) {
	d := m.d
	view := menu.AddSubmenu("View")
	view.Add("Toggle Sidebar").SetAccelerator("ctrl+CmdOrCtrl+s").OnClick(m.command("toggle-sidebar", ""))
	view.AddSeparator()
	view.Add("Reload").SetAccelerator("CmdOrCtrl+r").OnClick(func(*application.Context) {
		if w := d.wails.Window.Current(); w != nil {
			w.Reload()
		}
	})
	view.AddSeparator()
	view.AddRole(application.ResetZoom)
	view.AddRole(application.ZoomIn)
	view.AddRole(application.ZoomOut)
	view.AddSeparator()
	view.AddRole(application.ToggleFullscreen)
}

// addGoMenu holds navigation, including the focused project's spaces.
func (m *menuBar) addGoMenu(menu *application.Menu) {
	goMenu := menu.AddSubmenu("Go")
	goMenu.Add("Back").SetAccelerator("CmdOrCtrl+[").OnClick(m.command("back", ""))
	goMenu.Add("Forward").SetAccelerator("CmdOrCtrl+]").OnClick(m.command("forward", ""))
	goMenu.AddSeparator()
	goMenu.Add("Command Palette…").SetAccelerator("CmdOrCtrl+k").OnClick(m.command("palette", ""))

	spaces := projectSpaces(context.Background(), m.focusedFiles())
	if len(spaces) < 2 {
		return // one space is the whole app; there is nothing to switch to
	}
	goMenu.AddSeparator()
	for i, s := range spaces {
		item := goMenu.Add(spaceLabel(s)).OnClick(m.command("space", s.ID))
		if i < maxSpaceShortcuts {
			item.SetAccelerator(fmt.Sprintf("CmdOrCtrl+%d", i+1))
		}
	}
}

// spaceLabel is the menu text for a space, falling back to its id.
func spaceLabel(s menuSpace) string {
	if l := strings.TrimSpace(s.Label); l != "" {
		return l
	}
	return s.ID
}

func (m *menuBar) addHelpMenu(menu *application.Menu, mac bool) {
	d := m.d
	help := menu.AddSubmenu("Help")
	help.Add("Keyboard Shortcuts").SetAccelerator("CmdOrCtrl+/").OnClick(m.command("shortcuts", ""))
	if !mac {
		help.AddSeparator()
		help.Add("About Rela Desktop").OnClick(d.showAbout)
	}
}
