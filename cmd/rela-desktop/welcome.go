package main

import (
	"html/template"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/desktop"
)

// welcomeData holds data passed to the welcome page template.
type welcomeData struct {
	ErrorMessage   string
	RecentProjects []desktop.RecentProject
}

var welcomeTmpl = template.Must(template.New("welcome").Parse(welcomeHTML))

// newWelcomeHandler returns an http.Handler that serves the welcome/project picker page.
func newWelcomeHandler(prefs *desktop.Preferences, errorMsg string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		data := welcomeData{
			ErrorMessage:   errorMsg,
			RecentProjects: prefs.RecentProjects,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := welcomeTmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	return mux
}

const welcomeHTML = `<!DOCTYPE html>
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
  --primary-light: #eff6ff; --danger: #ef4444; --danger-light: #fef2f2;
  --radius: 8px; --font: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  --font-mono: "SF Mono", "Fira Code", monospace;
  --shadow: 0 1px 3px rgba(0,0,0,0.08);
}
body {
  font-family: var(--font); background: var(--bg); color: var(--text);
  display: flex; align-items: center; justify-content: center; min-height: 100vh;
}
.welcome {
  text-align: center; max-width: 480px; width: 100%; padding: 40px;
}
.welcome h1 {
  font-size: 28px; font-weight: 700; margin-bottom: 8px;
}
.welcome .subtitle {
  font-size: 14px; color: var(--text-muted); margin-bottom: 32px;
}
.error-banner {
  background: var(--danger-light); border: 1px solid #fecaca; border-radius: var(--radius);
  padding: 12px 16px; margin-bottom: 24px; font-size: 13px; color: #991b1b; text-align: left;
}
.btn {
  padding: 10px 24px; border: 1px solid var(--border); border-radius: 6px;
  font-size: 14px; font-weight: 500; cursor: pointer; font-family: var(--font);
  transition: all 0.15s; text-decoration: none; display: inline-flex;
  align-items: center; gap: 6px;
}
.btn-primary {
  background: var(--primary); color: #fff; border-color: var(--primary);
}
.btn-primary:hover { background: var(--primary-hover); }
.recent-section {
  margin-top: 32px; text-align: left;
}
.recent-section h2 {
  font-size: 13px; font-weight: 600; text-transform: uppercase;
  letter-spacing: 0.04em; color: var(--text-muted); margin-bottom: 12px;
}
.recent-list {
  list-style: none; background: var(--bg-card); border: 1px solid var(--border);
  border-radius: var(--radius); box-shadow: var(--shadow); overflow: hidden;
}
.recent-item {
  padding: 12px 16px; border-bottom: 1px solid var(--border); cursor: pointer;
  transition: background 0.15s; display: block; text-decoration: none; color: var(--text);
}
.recent-item:last-child { border-bottom: none; }
.recent-item:hover { background: var(--primary-light); }
.recent-item .name { font-weight: 500; font-size: 14px; }
.recent-item .path {
  font-size: 12px; color: var(--text-muted); font-family: var(--font-mono);
  margin-top: 2px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
</style>
</head>
<body>
<div class="welcome">
  <h1>Rela Desktop</h1>
  <p class="subtitle">Open a rela project to get started</p>

  {{if .ErrorMessage}}
  <div class="error-banner">{{.ErrorMessage}}</div>
  {{end}}

  <button class="btn btn-primary" onclick="openProject()">Open Project...</button>

  {{if .RecentProjects}}
  <div class="recent-section">
    <h2>Recent Projects</h2>
    <div class="recent-list">
      {{range .RecentProjects}}
      <a class="recent-item" href="#" onclick="openRecent('{{.Path}}'); return false;">
        <div class="name">{{.Name}}</div>
        <div class="path">{{.Path}}</div>
      </a>
      {{end}}
    </div>
  </div>
  {{end}}
</div>

<script>
// coverage-ignore: Wails frontend bindings
async function openProject() {
  try {
    await window.go.main.DesktopApp.OpenProject();
  } catch(e) {
    console.error('OpenProject failed:', e);
  }
}

async function openRecent(path) {
  try {
    await window.go.main.DesktopApp.OpenRecentProject(path);
  } catch(e) {
    console.error('OpenRecentProject failed:', e);
  }
}
</script>
</body>
</html>
`
