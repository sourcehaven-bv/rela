package main

import (
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/desktop"
)

// serveWelcomePage renders the welcome/project picker page with recent projects and optional error.
func serveWelcomePage(w http.ResponseWriter, prefs *desktop.Preferences, loadErr string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var errorHTML string
	if loadErr != "" {
		errorHTML = fmt.Sprintf(`<div class="error">%s</div>`, html.EscapeString(loadErr))
	}

	var recentHTML string
	if len(prefs.RecentProjects) > 0 {
		var sb strings.Builder
		sb.WriteString(`<div class="recent-section"><h2>Recent Projects</h2><div class="recent-list">`)
		for _, rp := range prefs.RecentProjects {
			name := html.EscapeString(rp.Name)
			path := html.EscapeString(rp.Path)
			fmt.Fprintf(&sb,
				`<a class="recent-item" href="#" onclick="openRecent('%s'); return false;"><div class="name">%s</div><div class="path">%s</div></a>`,
				path, name, path)
		}
		sb.WriteString(`</div></div>`)
		recentHTML = sb.String()
	}

	fmt.Fprintf(w, welcomePage, errorHTML, recentHTML)
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
.error { margin-bottom: 20px; padding: 12px 16px; background: #fef2f2;
  border: 1px solid #fecaca; border-radius: var(--radius); color: var(--danger);
  font-size: 13px; font-family: var(--font-mono); text-align: left; word-break: break-word; }
.hint { margin-top: 24px; color: var(--text-muted); font-size: 13px; line-height: 1.6; }
.hint code { background: #f1f5f9; padding: 2px 6px; border-radius: 4px;
  font-family: var(--font-mono); font-size: 12px; }
.loading { display: none; margin-top: 16px; color: var(--text-muted); font-size: 14px; }
.runtime-error { display: none; margin-top: 20px; padding: 12px 16px; background: #fef2f2;
  border: 1px solid #fecaca; border-radius: var(--radius); color: var(--danger);
  font-size: 13px; font-family: var(--font-mono); text-align: left; word-break: break-word; }
.recent-section { margin-top: 32px; text-align: left; }
.recent-section h2 { font-size: 13px; font-weight: 600; text-transform: uppercase;
  letter-spacing: 0.04em; color: var(--text-muted); margin-bottom: 12px; }
.recent-list { background: var(--bg-card); border: 1px solid var(--border);
  border-radius: var(--radius); box-shadow: var(--shadow); overflow: hidden; }
.recent-item { padding: 12px 16px; border-bottom: 1px solid var(--border); cursor: pointer;
  transition: background 0.15s; display: block; text-decoration: none; color: var(--text); }
.recent-item:last-child { border-bottom: none; }
.recent-item:hover { background: var(--primary-light); }
.recent-item .name { font-weight: 500; font-size: 14px; }
.recent-item .path { font-size: 12px; color: var(--text-muted); font-family: var(--font-mono);
  margin-top: 2px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
</head>
<body>
<div class="welcome">
  <h1>Rela Desktop</h1>
  <p class="subtitle">Open a rela project directory to get started.<br>The folder should contain a <code>metamodel.yaml</code> file.</p>

  %s

  <button class="btn btn-primary" id="open-btn" onclick="openProject()">Open Project...</button>

  <div class="loading" id="loading">Opening...</div>
  <div class="runtime-error" id="runtime-error"></div>

  <div class="hint">
    You can also launch from the command line:<br>
    <code>rela-desktop -project /path/to/project</code>
  </div>

  %s
</div>
<script>
async function openProject() {
  var btn = document.getElementById("open-btn");
  var loading = document.getElementById("loading");
  var errorEl = document.getElementById("runtime-error");
  btn.disabled = true;
  loading.style.display = "block";
  errorEl.style.display = "none";

  try {
    var result = await window.go.main.Desktop.OpenProject();
    if (result === "") {
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

async function openRecent(path) {
  var errorEl = document.getElementById("runtime-error");
  errorEl.style.display = "none";
  try {
    var result = await window.go.main.Desktop.OpenRecentProject(path);
    if (result === "") {
      window.location.reload();
    } else {
      errorEl.textContent = result;
      errorEl.style.display = "block";
    }
  } catch (e) {
    errorEl.textContent = String(e);
    errorEl.style.display = "block";
  }
}
</script>
</body>
</html>`
