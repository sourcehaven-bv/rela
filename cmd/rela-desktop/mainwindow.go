package main

import (
	"log/slog"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/Sourcehaven-BV/rela/internal/desktop"
)

// The main window.
//
// A Mac app keeps running when its last window closes. The menu bar and the
// Dock icon stay, and everything they offer must still work: Open Recent, a
// .rela document double-clicked in Finder, a click on the Dock icon. Each of
// those shows the main window, so the main window is found by name and
// opened again when the user has closed it, rather than held in a field that
// would point at a destroyed window.

// mainWindowName names the main window. Wails removes a closed window from
// its manager, so a lookup by this name fails once the user closes it.
const mainWindowName = "main"

// mainWindowMu serializes creating the main window, so two events arriving
// together (a Dock click and a Finder open) cannot open two of them.
var mainWindowMu sync.Mutex

// mainWindowOptions is the main window's configuration: the saved geometry
// when there is usable state, else the default size, centered.
func mainWindowOptions(title string, ws desktop.WindowState) application.WebviewWindowOptions {
	opts := application.WebviewWindowOptions{
		Name:   mainWindowName,
		Title:  title,
		Width:  defaultWindowWidth,
		Height: defaultWindowHeight,
		Mac:    macWindow(),
	}
	if ws.Valid() {
		// X/Y are only honored with InitialPosition set; the default centers
		// the window.
		opts.Width, opts.Height = ws.Width, ws.Height
		opts.X, opts.Y = ws.X, ws.Y
		opts.InitialPosition = application.WindowXY
		if ws.Maximized {
			opts.StartState = application.WindowStateMaximised
		}
	}
	return opts
}

// shouldOpenOnReopen reports whether a click on the Dock icon has to open a
// window. Wails already shows hidden windows; this covers the case it does
// not, where the user closed every window.
func shouldOpenOnReopen(hasVisibleWindows bool, windowCount int) bool {
	return !hasVisibleWindows && windowCount == 0
}

// liveMainWindow returns the main window, or nil when it is closed.
// coverage-ignore-func: requires Wails runtime
func (d *Desktop) liveMainWindow() *application.WebviewWindow {
	if d.wails == nil {
		return nil
	}
	w, ok := d.wails.Window.GetByName(mainWindowName)
	if !ok {
		return nil
	}
	win, _ := w.(*application.WebviewWindow)
	return win
}

// newMainWindow creates the main window. Called once from main, before Run,
// and again whenever the user needs it after closing it.
// coverage-ignore-func: requires Wails runtime
func (d *Desktop) newMainWindow(title string) *application.WebviewWindow {
	win := d.wails.Window.NewWithOptions(mainWindowOptions(title, d.prefs.Window))
	d.menu.trackWindow(win, "")
	// Reading the geometry after the window is gone returns zeroes, so it is
	// saved while the window closes, not after.
	win.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		d.saveWindowState(win)
	})
	return win
}

// showMainWindow brings the main window to the front, opening it again when
// the user closed it.
// coverage-ignore-func: requires Wails runtime
func (d *Desktop) showMainWindow() {
	if d.wails == nil {
		return
	}
	mainWindowMu.Lock()
	win := d.liveMainWindow()
	if win == nil {
		win = d.newMainWindow(d.projectTitle())
	}
	mainWindowMu.Unlock()
	win.Show()
	win.UnMinimise()
	win.Focus()
}

// onDockReopen opens the main window when the Dock icon is clicked and no
// window is left.
// coverage-ignore-func: requires the Dock
func (d *Desktop) onDockReopen(e *application.ApplicationEvent) {
	if !shouldOpenOnReopen(e.Context().HasVisibleWindows(), len(d.wails.Window.GetAll())) {
		return
	}
	d.showMainWindow()
}

// saveWindowState records the main window's geometry for the next launch.
// Failures are logged and swallowed: losing window position must never block
// a quit.
// coverage-ignore-func: requires Wails runtime
func (d *Desktop) saveWindowState(win *application.WebviewWindow) {
	w, h := win.Size()
	x, y := win.Position()
	st := desktop.WindowState{Width: w, Height: h, X: x, Y: y, Maximized: win.IsMaximised()}
	if !st.Valid() {
		return // don't overwrite good state with a minimized/degenerate size
	}
	d.prefs.Window = st
	if err := d.prefs.Save(); err != nil {
		slog.Warn("could not save window state", "error", err)
	}
}
