package main

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Quick entry popup size (TKT-24L1H36): one create form, no sidebar.
const (
	quickEntryWidth  = 520
	quickEntryHeight = 640
	quickEntryRoute  = "/quick?popup=1"
)

// shortcutRegistrar is the part of Wails' GlobalShortcutManager the hotkey
// needs. Its callbacks run on a goroutine of their own.
type shortcutRegistrar interface {
	Register(accelerator string, callback func()) error
	Unregister(accelerator string) error
}

// popupWindow is the part of a Wails window the hotkey keeps.
type popupWindow interface {
	Focus()
	Close()
}

// quickEntryHotkey owns the global shortcut of the active project's
// `quick_entry:` block and the one popup it opens. The active project owns
// the shortcut: a project switch re-syncs it and closes an open popup, so a
// form never saves into a project the user has left.
type quickEntryHotkey struct {
	reg  shortcutRegistrar
	open func(route string, onClose func()) popupWindow // opens the popup; nil when there is no window system

	// swapMu serializes syncs, so two project changes apply in order. It is
	// held across Register and Unregister, which wait for the main thread;
	// nothing on the main thread takes it.
	swapMu sync.Mutex

	// mu guards the fields below. It is never held across a window or
	// shortcut call: those wait for the main thread, and a window's close
	// handler runs there and takes mu.
	mu      sync.Mutex
	current string      // the registered accelerator; "" when none
	route   string      // the popup route, under the active project's /p/<id>/ base
	popup   popupWindow // the open popup; nil when none
	popupID uint64      // identifies popup to its close handler
	opening bool        // a popup is being created
	gen     uint64      // bumped by sync, so a popup opened for an old project is closed
}

// sync registers the accelerator resolve returns, replacing the previous
// one, and routes new popups to the route it returns. An empty accelerator
// removes the shortcut. Any open popup is closed, because it belongs to the
// project that was active before. resolve runs under swapMu, so the state it
// reads and the order syncs apply in agree.
func (q *quickEntryHotkey) sync(resolve func() (accel, route string)) {
	q.swapMu.Lock()
	defer q.swapMu.Unlock()
	accel, route := resolve()

	q.mu.Lock()
	q.gen++
	q.route = route
	old := q.popup
	q.popup = nil
	prev := q.current
	q.mu.Unlock()

	if old != nil {
		old.Close()
	}
	if accel == prev {
		return
	}
	registered := q.swap(prev, accel)
	q.mu.Lock()
	q.current = registered
	q.mu.Unlock()
}

// swap replaces prev with accel and returns what is now registered.
func (q *quickEntryHotkey) swap(prev, accel string) string {
	if prev != "" {
		if err := q.reg.Unregister(prev); err != nil {
			slog.Warn("quick entry: unregister shortcut", "accelerator", prev, "error", err)
		}
	}
	if accel == "" {
		return ""
	}
	if err := q.reg.Register(accel, q.trigger); err != nil {
		// Another application may hold the shortcut. The SPA's `q` key and
		// the /quick route still work, so this is a warning, not a failure.
		slog.Warn("quick entry: register shortcut", "accelerator", accel, "error", err)
		return ""
	}
	return accel
}

// trigger focuses the open popup, or opens one.
func (q *quickEntryHotkey) trigger() {
	q.mu.Lock()
	if p := q.popup; p != nil {
		q.mu.Unlock()
		p.Focus()
		return
	}
	if q.opening || q.open == nil {
		q.mu.Unlock()
		return
	}
	q.opening = true
	gen := q.gen
	route := q.route
	q.popupID++
	id := q.popupID
	q.mu.Unlock()

	w := q.open(route, func() { q.closed(id) })

	q.mu.Lock()
	q.opening = false
	stale := gen != q.gen
	if !stale {
		q.popup = w
	}
	q.mu.Unlock()
	if stale && w != nil {
		w.Close()
	}
}

// closed forgets popup id once its window closes, unless a newer popup
// replaced it.
func (q *quickEntryHotkey) closed(id uint64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if id == q.popupID {
		q.popup = nil
	}
}

// syncQuickEntry points the hotkey of d at the active project's configuration.
// Call it whenever the active project changes.
func syncQuickEntry(d *Desktop) {
	if d.quick == nil {
		return
	}
	d.quick.sync(func() (accel, route string) {
		d.mu.RLock()
		app := d.app
		d.mu.RUnlock()
		if app != nil {
			// Bound to the project explicitly, so a save can only reach the
			// project that owned the shortcut when the popup opened.
			route = joinBase("/p/"+projectID(app.ProjectRoot())+"/", quickEntryRoute)
			if qe := app.Cfg().QuickEntry; qe != nil {
				accel = qe.Accelerator()
			}
		}
		return accel, route
	})
}

// newQuickEntryHotkey wires the hotkey to the Wails runtime.
func newQuickEntryHotkey(app *application.App, d *Desktop) *quickEntryHotkey {
	return &quickEntryHotkey{
		reg: app.GlobalShortcut,
		open: func(route string, onClose func()) popupWindow {
			// The shortcut callback runs off the main thread, and windows
			// must be created on it.
			return application.InvokeSyncWithResult(func() popupWindow {
				return openQuickEntryWindow(app, route, d.projectTitle(), onClose)
			})
		},
	}
}

func openQuickEntryWindow(app *application.App, route, title string, onClose func()) popupWindow {
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:            fmt.Sprintf("quick-entry-%d", windowSeq.Add(1)),
		Title:           title,
		URL:             route,
		Width:           quickEntryWidth,
		Height:          quickEntryHeight,
		AlwaysOnTop:     true,
		Mac:             macWindow(),
		InitialPosition: application.WindowCentered,
	})
	win.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { onClose() })
	win.Show()
	win.Focus()
	return win
}
