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
	goruntime "runtime"
	"sync"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// Desktop is the backend bound to the Wails frontend.
// It manages project lifecycle: opening a directory picker and loading a project.
type Desktop struct {
	ctx     context.Context
	mu      sync.RWMutex
	app     *dataentry.App
	handler http.Handler
	loadErr string
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
		runtime.WindowSetTitle(d.ctx, app.Cfg.App.Name)
	}
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, welcomePage, loadErr)
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

// newAppMenu builds the application menu bar.
func newAppMenu(d *Desktop) *menu.Menu {
	appMenu := menu.NewMenu()

	if goruntime.GOOS == "darwin" {
		appMenu.Append(menu.AppMenu())
	}

	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("Open Project...", keys.CmdOrCtrl("o"), d.openProjectFromMenu)
	if goruntime.GOOS != "darwin" {
		fileMenu.AddSeparator()
		fileMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
			runtime.Quit(d.ctx)
		})
	}

	if goruntime.GOOS == "darwin" {
		appMenu.Append(menu.EditMenu())
	}

	return appMenu
}

// coverage-ignore: main function - entry point
func main() {
	projectDir := flag.String("project", ".", "Path to the rela project directory")
	flag.Parse()

	d := &Desktop{}

	// Try loading the project early so the app opens directly if valid.
	// Errors are deferred to the welcome screen instead of crashing.
	if *projectDir != "." || isRelaProject(*projectDir) {
		if errMsg := d.LoadProject(*projectDir); errMsg != "" {
			log.Printf("Could not load project from %q: %s", *projectDir, errMsg)
		}
	}

	title := "Rela Desktop"
	if d.app != nil {
		title = d.app.Cfg.App.Name
	}

	err := wails.Run(&options.App{
		Title:  title,
		Width:  1280,
		Height: 800,
		Menu:   newAppMenu(d),
		AssetServer: &assetserver.Options{
			Handler: d,
		},
		OnStartup: d.startup,
		Bind:      []interface{}{d},
	})
	if err != nil {
		log.Fatalf("Wails error: %v", err)
	}
}

// isRelaProject checks if the directory looks like a rela project.
func isRelaProject(dir string) bool {
	_, err := os.Stat(dir + "/metamodel.yaml")
	return err == nil
}

const welcomePage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Rela Desktop</title>
<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
:root {
  --bg: #f8fafc; --bg-card: #fff; --text: #1e293b; --text-muted: #64748b;
  --border: #e2e8f0; --primary: #3b82f6; --primary-hover: #2563eb;
  --primary-light: #eff6ff; --danger: #ef4444; --radius: 8px;
  --font: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  --font-mono: "SF Mono", "Fira Code", monospace;
  --shadow: 0 1px 3px rgba(0,0,0,0.08);
}
body { font-family: var(--font); background: var(--bg); color: var(--text);
  display: flex; align-items: center; justify-content: center; min-height: 100vh; }
.welcome { text-align: center; max-width: 480px; padding: 48px 32px; }
.welcome h1 { font-size: 28px; font-weight: 700; margin-bottom: 8px; }
.welcome .subtitle { color: var(--text-muted); font-size: 15px; margin-bottom: 32px; line-height: 1.6; }
.btn { padding: 12px 32px; border: none; border-radius: var(--radius);
  font-size: 15px; font-weight: 600; cursor: pointer; font-family: var(--font);
  transition: all 0.15s; display: inline-flex; align-items: center; gap: 8px; }
.btn-primary { background: var(--primary); color: #fff; }
.btn-primary:hover { background: var(--primary-hover); }
.btn:disabled { opacity: 0.6; cursor: not-allowed; }
.error { margin-top: 20px; padding: 12px 16px; background: #fef2f2;
  border: 1px solid #fecaca; border-radius: var(--radius); color: var(--danger);
  font-size: 13px; font-family: var(--font-mono); text-align: left; word-break: break-word; }
.hint { margin-top: 24px; color: var(--text-muted); font-size: 13px; line-height: 1.6; }
.hint code { background: #f1f5f9; padding: 2px 6px; border-radius: 4px;
  font-family: var(--font-mono); font-size: 12px; }
.loading { display: none; margin-top: 16px; color: var(--text-muted); font-size: 14px; }
</style>
</head>
<body>
<div class="welcome">
  <h1>Rela Desktop</h1>
  <p class="subtitle">Open a rela project directory to get started.<br>The folder should contain a <code>metamodel.yaml</code> file.</p>

  <button class="btn btn-primary" id="open-btn" onclick="openProject()">Open Project...</button>

  <div class="loading" id="loading">Opening...</div>

  <div id="error" style="display:none" class="error"></div>

  <div class="hint">
    You can also launch from the command line:<br>
    <code>rela-desktop -project /path/to/project</code>
  </div>
</div>
<script>
// Pre-populate error from a failed -project flag load.
(function() {
  var initialErr = %q;
  if (initialErr) {
    var el = document.getElementById("error");
    el.textContent = initialErr;
    el.style.display = "block";
  }
})();

async function openProject() {
  var btn = document.getElementById("open-btn");
  var loading = document.getElementById("loading");
  var errorEl = document.getElementById("error");
  btn.disabled = true;
  loading.style.display = "block";
  errorEl.style.display = "none";

  try {
    var result = await window.go.main.Desktop.OpenProject();
    if (result === "") {
      // Success or user cancelled — reload to show the app (or stay on welcome).
      window.location.reload();
    } else {
      errorEl.textContent = result;
      errorEl.style.display = "block";
    }
  } catch (e) {
    errorEl.textContent = String(e);
    errorEl.style.display = "block";
  } finally {
    btn.disabled = false;
    loading.style.display = "none";
  }
}
</script>
</body>
</html>`
