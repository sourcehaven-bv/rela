package main

import (
	"log/slog"
	goruntime "runtime"
	"sync"
	"sync/atomic"

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
// its manager, but only after two round trips to the main thread, so a
// lookup by name can still find a window that is closing. [mainWindows]
// covers that gap.
const mainWindowName = "main"

// mainWindows tracks the main window across closes and reopens.
type mainWindows struct {
	// mu serializes creating the main window, so two events arriving
	// together (a Dock click and a Finder open) cannot open two of them.
	mu sync.Mutex
	// closing is the main window the user is closing. Wails still lists it
	// until it is removed, but it shows nothing any more, so it counts as
	// gone.
	closing atomic.Pointer[application.WebviewWindow]
}

// live returns the main window, or nil when it is closed or closing.
// coverage-ignore-func: requires Wails runtime
func (m *mainWindows) live(app *application.App) *application.WebviewWindow {
	if app == nil {
		return nil
	}
	w, ok := app.Window.GetByName(mainWindowName)
	if !ok {
		return nil
	}
	win, _ := w.(*application.WebviewWindow)
	if win == nil || win == m.closing.Load() {
		return nil
	}
	return win
}

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
		opts.Width, opts.Height = ws.Width+restoreInset(), ws.Height+restoreInset()
		opts.X, opts.Y = ws.X, ws.Y
		opts.InitialPosition = application.WindowXY
		if ws.Maximized {
			opts.StartState = application.WindowStateMaximised
		}
	}
	return opts
}

// restoreInset is added to a saved size on restore. On macOS Wails creates a
// window 1pt smaller than asked (webview_window_darwin.go builds the content
// rect as width-1, height-1), while Size reads the whole frame, so without it
// every close and reopen shrinks the window by 1pt. Check it when Wails is
// upgraded: if Wails drops the -1, this makes the window grow instead.
func restoreInset() int {
	if goruntime.GOOS == "darwin" {
		return 1
	}
	return 0
}

// shouldOpenOnReopen reports whether a click on the Dock icon has to open a
// window. Wails already shows hidden windows; this covers the case it does
// not, where the user closed every window.
func shouldOpenOnReopen(hasVisibleWindows bool, windowCount int) bool {
	return !hasVisibleWindows && windowCount == 0
}

// openWindowCount counts the windows that are not closing.
func openWindowCount(windows []application.Window, closing application.Window) int {
	n := 0
	for _, w := range windows {
		if w != closing {
			n++
		}
	}
	return n
}

// newMainWindow creates the main window. Called once from main, before Run,
// and again whenever the user needs it after closing it.
// coverage-ignore-func: requires Wails runtime
func (d *Desktop) newMainWindow(title string) *application.WebviewWindow {
	win := d.wails.Window.NewWithOptions(mainWindowOptions(title, d.prefs.Window))
	d.menu.trackWindow(win, "")
	// A hook, not a listener: Wails runs hooks before it starts the listener
	// that destroys the window, and each listener in a goroutine of its own.
	// Read from a listener, the geometry can come back as zeroes. Quitting
	// closes no window this way; main saves the geometry on shutdown.
	win.RegisterHook(events.Common.WindowClosing, func(*application.WindowEvent) {
		saveWindowState(d.prefs, win)
		d.main.closing.Store(win)
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
	d.main.mu.Lock()
	win := d.main.live(d.wails)
	if win == nil {
		win = d.newMainWindow(d.projectTitle())
	}
	d.main.mu.Unlock()
	win.Show()
	win.UnMinimise()
	win.Focus()
}

// onDockReopen opens the main window when the Dock icon is clicked and no
// window is left.
// coverage-ignore-func: requires the Dock
func (d *Desktop) onDockReopen(e *application.ApplicationEvent) {
	var closing application.Window
	if c := d.main.closing.Load(); c != nil {
		closing = c
	}
	if !shouldOpenOnReopen(e.Context().HasVisibleWindows(), openWindowCount(d.wails.Window.GetAll(), closing)) {
		return
	}
	d.showMainWindow()
}

// saveWindowState records the main window's geometry for the next launch.
// Failures are logged and swallowed: losing window position must never block
// a quit.
// coverage-ignore-func: requires Wails runtime
func saveWindowState(prefs *desktop.Preferences, win *application.WebviewWindow) {
	w, h := win.Size()
	x, y := win.Position()
	st := desktop.WindowState{Width: w, Height: h, X: x, Y: y, Maximized: win.IsMaximised()}
	if !st.Valid() {
		return // don't overwrite good state with a minimized/degenerate size
	}
	prefs.Window = st
	if err := prefs.Save(); err != nil {
		slog.Warn("could not save window state", "error", err)
	}
}
