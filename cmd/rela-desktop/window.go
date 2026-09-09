package main

import (
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Multi-window support.
//
// Every window is a webview onto the SAME in-process http.Handler, so windows
// need no cross-process coordination: internal/dataentry already broadcasts
// store changes over SSE to every connected client, and each window is just
// another client. Two windows on one project therefore stay in sync with no
// extra machinery.
//
// This does NOT extend to multiple processes. The sqlite backend takes an
// exclusive lock at Open (DEC-LFSYNY), which is why the single-instance lock
// exists — many windows, one process.

// windowSeq names windows uniquely. Wails looks windows up by name, and a
// duplicate name would silently return the wrong window.
var windowSeq atomic.Uint64

// defaultSecondaryWidth/Height size a secondary window. Deliberately smaller
// than the main window: a secondary is usually a detail view placed beside it,
// not a replacement for it.
const (
	defaultSecondaryWidth  = 1000
	defaultSecondaryHeight = 720
)

// OpenWindow opens a native window on the given SPA route (for example
// "/entity/ticket/TKT-1" or "/list/tickets"). It is bound to the frontend, so
// the SPA can request a window without knowing anything about Wails.
//
// The path is validated rather than trusted: it is interpolated into the
// window URL, and the frontend is the one surface that takes user content.
// Returns an error string (empty on success), matching the other bound methods.
func (d *Desktop) OpenWindow(path, title string, base ...string) string {
	if d.wails == nil {
		return "application not ready"
	}
	route, err := safeRoute(path)
	if err != nil {
		return err.Error()
	}

	// Links inside a document are origin-relative, so a link clicked in a
	// window served under /p/<id>/ must reopen under that same project. The
	// caller passes its own base; absent or "/", the route is used as-is.
	if len(base) > 0 {
		b, berr := safeBase(base[0])
		if berr != nil {
			return berr.Error()
		}
		route = joinBase(b, route)
	}

	if title == "" {
		title = d.projectTitle()
	}

	name := fmt.Sprintf("secondary-%d", windowSeq.Add(1))

	// Windows must be created on the main thread — the same constraint that
	// makes Menu.Set panic when called too early. InvokeAsync rather than
	// InvokeSync: the latter blocks until the main thread runs the closure,
	// so calling it FROM the main thread (a menu callback, say) would
	// deadlock. Nothing here needs the window back, so dispatch and return.
	application.InvokeAsync(func() {
		win := d.wails.Window.NewWithOptions(application.WebviewWindowOptions{
			Name:   name,
			Title:  title,
			URL:    route,
			Width:  defaultSecondaryWidth,
			Height: defaultSecondaryHeight,
		})
		// Secondary windows are deliberately NOT persisted: only the main
		// window's geometry is restored, so a stack of detail windows does
		// not reopen on next launch.
		win.Show()
	})
	slog.Debug("opened window", "name", name, "route", route)
	return ""
}

// safeRoute validates a frontend-supplied SPA path.
//
// Only a same-origin absolute path is allowed. Rejecting "//host" matters as
// well as "http://host": a protocol-relative URL is absolute, so a bare
// prefix check on "/" would let a window navigate off-origin.
func safeRoute(path string) (string, error) {
	switch {
	case path == "":
		return "", errors.New("invalid route: empty path")
	case !strings.HasPrefix(path, "/"):
		return "", errors.New("invalid route: path must start with /")
	case strings.HasPrefix(path, "//"):
		return "", errors.New("invalid route: protocol-relative paths are not allowed")
	// A control character would let a crafted path smuggle a second header or
	// break out of the URL when interpolated.
	case strings.ContainsAny(path, "\r\n\x00"):
		return "", errors.New("invalid route: path contains control characters")
	}
	return path, nil
}

// projectTitle is the loaded project's name, or the app name when no project
// is open. Safe to call with no project loaded.
func (d *Desktop) projectTitle() string {
	d.mu.RLock()
	app := d.app
	d.mu.RUnlock()
	if app != nil {
		return app.ProjectName()
	}
	return "Rela Desktop"
}

// multiWindowScript teaches the SPA to open native windows. It is injected by
// the desktop shell rather than shipped in the Vue bundle, so frontend/ keeps
// zero Wails coupling and rela-server is completely unaffected.
//
// It hooks the intent the SPA already documents but cannot honor inside a
// webview: useDocumentClicks passes modifier-clicks and target="_blank"
// through "to default browser behavior", which in a browser opens a tab and
// in a webview does nothing at all.
const multiWindowScript = `
<!-- v3 serves its runtime at /wails/runtime.js but does not inject it, so a
     page that never requests it has no window.wails. Must load before the
     script below, which depends on it. -->
<script src="/wails/runtime.js"></script>
<script>
(function () {
  if (!window.wails || !window.wails.Call) return; // browser: leave as-is

  // The base this page is served under ("/p/<id>/", or "/" at the root).
  // A link in the document is origin-relative ("/form/x"), so the window we
  // open has to be told which project that path belongs to — otherwise a
  // second project's link opens against the active one.
  function relaBase() {
    var m = document.querySelector('meta[name="rela-base"]');
    return (m && m.content) || "/";
  }

  function openWindow(path, title) {
    window.wails.Call.ByName("main.Desktop.OpenWindow", path, title || "", relaBase())
      .catch(function (e) { console.error("OpenWindow failed:", e); });
  }
  window.relaOpenWindow = openWindow;

  // Same-origin internal link? Return its path, else "".
  function internalPath(a) {
    if (!a || !a.getAttribute) return "";
    var raw = a.getAttribute("href") || "";
    if (!raw || raw.charAt(0) === "#") return "";
    var url;
    try { url = new URL(a.href, window.location.href); } catch (e) { return ""; }
    if (url.origin !== window.location.origin) return "";
    return url.pathname + url.search;
  }

  // Capture phase, so this runs before vue-router's own handler.
  document.addEventListener("click", function (e) {
    // Middle-click (button 1) and cmd/ctrl-click are the two conventional
    // "open elsewhere" gestures. Shift/alt are left alone: the SPA uses them
    // for range-select.
    var wants = (e.button === 1) || ((e.button === 0) && (e.metaKey || e.ctrlKey));
    if (!wants) return;

    var a = e.target.closest && e.target.closest("a");
    var path = internalPath(a);
    if (!path) return;

    e.preventDefault();
    e.stopPropagation();
    openWindow(path, a.textContent ? a.textContent.trim().slice(0, 80) : "");
  }, true);

  // Middle-click fires auxclick, not click, in some engines.
  document.addEventListener("auxclick", function (e) {
    if (e.button !== 1) return;
    var a = e.target.closest && e.target.closest("a");
    var path = internalPath(a);
    if (!path) return;
    e.preventDefault();
    e.stopPropagation();
    openWindow(path, a.textContent ? a.textContent.trim().slice(0, 80) : "");
  }, true);
})();
</script>
`

// injectMultiWindow appends the multi-window script to an HTML document, and
// declares the project base the SPA should prefix its requests with.
// Non-HTML responses pass through untouched.
//
// The base is a <meta> tag rather than an inline <script> deliberately: the
// SPA shell is served without a CSP today, but router.go records that as a
// property that could lapse, and a meta tag needs no 'unsafe-inline'.
func injectMultiWindow(body []byte, base string) []byte {
	const closing = "</body>"
	idx := strings.LastIndex(string(body), closing)
	if idx < 0 {
		return body
	}
	meta := ""
	if base != "" {
		meta = `<meta name="rela-base" content="` + html.EscapeString(base) + `/">`
	}
	out := make([]byte, 0, len(body)+len(multiWindowScript)+len(meta))
	out = append(out, body[:idx]...)
	out = append(out, meta...)
	out = append(out, multiWindowScript...)
	out = append(out, body[idx:]...)
	return out
}

// htmlInjector buffers a response so the multi-window script can be appended
// to HTML. Everything else — JSON, assets, and notably SSE — streams straight
// through, because buffering a long-lived event stream would hang the client.
type htmlInjector struct {
	http.ResponseWriter
	// base is the project's URL prefix ("/p/<id>"), empty when serving the
	// active project at the root. Emitted for the SPA to read at boot.
	base        string
	buf         []byte
	isHTML      bool
	wroteHeader bool
	status      int
}

func (h *htmlInjector) WriteHeader(status int) {
	if h.wroteHeader {
		return
	}
	h.wroteHeader = true
	h.status = status
	ct := h.Header().Get("Content-Type")
	h.isHTML = strings.HasPrefix(ct, "text/html")
	if !h.isHTML {
		h.ResponseWriter.WriteHeader(status)
		return
	}
	// Length changes once the script is appended.
	h.Header().Del("Content-Length")
}

func (h *htmlInjector) Write(b []byte) (int, error) {
	if !h.wroteHeader {
		h.WriteHeader(http.StatusOK)
	}
	if !h.isHTML {
		return h.ResponseWriter.Write(b)
	}
	h.buf = append(h.buf, b...)
	return len(b), nil
}

// Flush forwards to the underlying writer for streaming responses. An HTML
// response is never flushed early — it is held until finish().
func (h *htmlInjector) Flush() {
	if h.isHTML {
		return
	}
	if f, ok := h.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// finish writes the buffered HTML with the script appended.
func (h *htmlInjector) finish() {
	if !h.isHTML {
		return
	}
	body := injectMultiWindow(h.buf, h.base)
	h.ResponseWriter.WriteHeader(h.status)
	if _, err := h.ResponseWriter.Write(body); err != nil {
		slog.Debug("could not write injected body", "error", err)
	}
}

// safeBase validates a base supplied by the frontend. It is interpolated into
// a window URL, so it gets the same treatment as a route: same-origin absolute
// path only, no protocol-relative escape, no control characters.
func safeBase(base string) (string, error) {
	if base == "" || base == "/" {
		return "/", nil
	}
	if _, err := safeRoute(base); err != nil {
		return "", err
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base, nil
}

// joinBase mounts an origin-relative route under a base, without doubling the
// separator and without prefixing a route that already carries it.
func joinBase(base, route string) string {
	if base == "/" {
		return route
	}
	if strings.HasPrefix(route, base) {
		return route // already prefixed; do not double it
	}
	return strings.TrimSuffix(base, "/") + route
}
