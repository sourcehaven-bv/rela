package main

import (
	"encoding/json"
	"log/slog"
	"net/url"
	goruntime "runtime"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Native window chrome.
//
// On macOS the title bar is transparent and the page runs underneath it, the
// way Mail and Notes look: the traffic lights sit over the sidebar header and
// there is no separate grey strip with the project name. That moves two jobs
// from the OS to the page, both done by chromeStyle: leaving room for the
// traffic lights, and marking where the window can be dragged.
//
// The SPA is not told about any of this. The style targets the component
// library's class names, which TestChromeStyle_TargetsShippedClasses pins
// against the embedded bundle, so a renamed class fails a test rather than
// leaving a window that cannot be moved.

// titleBarHeight is the height of the macOS title bar the page runs under.
const titleBarHeight = "28px"

// macWindow is the macOS chrome every project window gets. Other platforms
// keep their native title bar, so the options are empty there.
func macWindow() application.MacWindow {
	return application.MacWindow{
		TitleBar: application.MacTitleBar{
			AppearsTransparent: true,
			HideTitle:          true,
			FullSizeContent:    true,
		},
	}
}

// chromeStyle is injected into every SPA page on macOS. It is empty elsewhere,
// where the native title bar still sits above the page.
var chromeStyle = func() string {
	if goruntime.GOOS != "darwin" {
		return ""
	}
	return `
<style id="rela-desktop-chrome">
/* Room for the traffic lights, which sit over the sidebar header. Full
   screen hides them, so the room goes too. */
:root { --rl-sidebar-safe-top: ` + titleBarHeight + `; }
:root.rela-fullscreen { --rl-sidebar-safe-top: 0px; }

/* Drag regions: the sidebar header and page headers move the window, the
   controls inside them stay clickable. */
.rl-sidebar__header, .rl-page-header { --wails-draggable: drag; }
.rl-sidebar__header :is(button, a, input, select, textarea, [role="button"]),
.rl-page-header :is(button, a, input, select, textarea, [role="button"]) {
  --wails-draggable: no-drag;
}

/* Chrome is not text: pressing a button or dragging over the sidebar should
   not select words. Content stays selectable. */
.rl-sidebar, .rl-page-header__actions, button, [role="button"], [role="menuitem"], [role="tab"] {
  -webkit-user-select: none;
  user-select: none;
}

/* A Mac app shows the arrow over its controls; the hand is for links. */
button, [role="button"], [role="tab"], [role="menuitem"]:not(a) { cursor: default; }
</style>
`
}()

// Context menus are named, and the page picks one by setting a CSS custom
// property on what was right-clicked (see contextMenuScript).
const (
	linkMenuName   = "rela-link"
	entityMenuName = "rela-entity-link"
)

// contextTarget is what the page reports about a right-clicked link.
type contextTarget struct {
	Path  string `json:"path"`  // origin-relative path of the link
	Base  string `json:"base"`  // the page's project base, "/" or "/p/<id>/"
	Title string `json:"title"` // the link's text, trimmed
}

// parseContextTarget decodes the data a context menu click carries. The page
// URI-encodes a JSON object so it survives as a single CSS token.
func parseContextTarget(data string) (contextTarget, bool) {
	raw, err := url.PathUnescape(strings.TrimSpace(data))
	if err != nil {
		return contextTarget{}, false
	}
	var t contextTarget
	if err := json.Unmarshal([]byte(raw), &t); err != nil || t.Path == "" {
		return contextTarget{}, false
	}
	return t, true
}

// entityIDFromPath returns the entity id in an entity page path, which may
// carry a project or space prefix: /p/<proj>/s/<space>/entity/<type>/<id>.
func entityIDFromPath(path string) (string, bool) {
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "entity" && i+3 == len(parts) {
			id, err := url.PathUnescape(parts[i+2])
			if err != nil || id == "" {
				return "", false
			}
			return id, true
		}
	}
	return "", false
}

// registerContextMenus creates the link context menus. Called once, after
// the application exists.
// windowOpener opens route in a new window; it returns an error message, or
// "" on success. Desktop.OpenWindow is one.
type windowOpener func(route, title string, base ...string) string

// coverage-ignore-func: requires Wails runtime
func registerContextMenus(app *application.App, open windowOpener) {
	openInWindow := func(ctx *application.Context) {
		t, ok := parseContextTarget(ctx.ContextMenuData())
		if !ok {
			return
		}
		if errMsg := open(t.Path, t.Title, t.Base); errMsg != "" {
			slog.Warn("could not open window", "error", errMsg)
		}
	}
	copyTitle := func(ctx *application.Context) {
		if t, ok := parseContextTarget(ctx.ContextMenuData()); ok && t.Title != "" {
			app.Clipboard.SetText(t.Title)
		}
	}
	copyID := func(ctx *application.Context) {
		t, ok := parseContextTarget(ctx.ContextMenuData())
		if !ok {
			return
		}
		if id, ok := entityIDFromPath(t.Path); ok {
			app.Clipboard.SetText(id)
		}
	}

	link := application.NewContextMenu(linkMenuName)
	link.Add("Open in New Window").OnClick(openInWindow)
	link.AddSeparator()
	link.Add("Copy Title").OnClick(copyTitle)
	link.Update()

	entity := application.NewContextMenu(entityMenuName)
	entity.Add("Open in New Window").OnClick(openInWindow)
	entity.AddSeparator()
	entity.Add("Copy ID").OnClick(copyID)
	entity.Add("Copy Title").OnClick(copyTitle)
	entity.Update()
}

// shellCommand is the detail of the event the SPA's useShellCommands
// listens for.
type shellCommand struct {
	Command string `json:"command"`
	Arg     string `json:"arg,omitempty"`
}

// shellCommandJS is the script that hands cmd to the page. JSON-encoding the
// detail is what keeps an argument from breaking out of the script.
func shellCommandJS(cmd shellCommand) string {
	detail, _ := json.Marshal(cmd) //nolint:errchkjson // two strings cannot fail to encode
	return `window.dispatchEvent(new CustomEvent("rela:shell-command",{detail:` + string(detail) + `}))`
}

// sendShellCommand runs a menu command in the focused window.
// coverage-ignore-func: requires Wails runtime
func sendShellCommand(app *application.App, command, arg string) {
	if app == nil {
		return
	}
	win := app.Window.Current()
	if win == nil {
		return
	}
	win.ExecJS(shellCommandJS(shellCommand{Command: command, Arg: arg}))
}

// trackFullscreen keeps the traffic-light room in step with full screen,
// where macOS hides the traffic lights.
// coverage-ignore-func: requires Wails runtime
func trackFullscreen(win *application.WebviewWindow) {
	if goruntime.GOOS != "darwin" {
		return
	}
	set := func(on string) func(*application.WindowEvent) {
		js := `document.documentElement.classList.toggle("rela-fullscreen",` + on + `)`
		return func(*application.WindowEvent) { win.ExecJS(js) }
	}
	win.OnWindowEvent(events.Common.WindowFullscreen, set("true"))
	win.OnWindowEvent(events.Common.WindowUnFullscreen, set("false"))
}
