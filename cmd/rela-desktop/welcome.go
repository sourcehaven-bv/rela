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
<script>
(function(){
  var t=localStorage.getItem('theme');
  if(t){document.documentElement.setAttribute('data-theme',t)}
  else if(matchMedia('(prefers-color-scheme:dark)').matches){
    document.documentElement.setAttribute('data-theme','dark')
  }
})();
</script>
<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
:root {
  --bg: #f8fafc; --bg-card: #fff; --text: #1e293b; --text-muted: #64748b;
  --border: #e2e8f0; --primary: #3b82f6; --primary-hover: #2563eb;
  --primary-light: #eff6ff; --danger: #ef4444; --danger-light: #fef2f2;
  --danger-border: #fecaca; --radius: 8px;
  --font: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  --font-mono: "SF Mono", "Fira Code", monospace;
  --shadow: 0 1px 3px rgba(0,0,0,0.08);
}
[data-theme="dark"] {
  --bg: #0f172a; --bg-card: #1e293b; --text: #e2e8f0; --text-muted: #94a3b8;
  --border: #334155; --primary: #60a5fa; --primary-hover: #3b82f6;
  --primary-light: rgba(59,130,246,0.15); --danger: #f87171;
  --danger-light: rgba(239,68,68,0.15); --danger-border: rgba(239,68,68,0.3);
  --shadow: 0 1px 3px rgba(0,0,0,0.3);
}
@media (prefers-color-scheme: dark) {
  :root:not([data-theme="light"]) {
    --bg: #0f172a; --bg-card: #1e293b; --text: #e2e8f0; --text-muted: #94a3b8;
    --border: #334155; --primary: #60a5fa; --primary-hover: #3b82f6;
    --primary-light: rgba(59,130,246,0.15); --danger: #f87171;
    --danger-light: rgba(239,68,68,0.15); --danger-border: rgba(239,68,68,0.3);
    --shadow: 0 1px 3px rgba(0,0,0,0.3);
  }
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
.btn-secondary { background: var(--bg-card); color: var(--text); border: 1px solid var(--border); }
.btn-secondary:hover { background: var(--primary-light); border-color: var(--primary); }
.btn-small { padding: 8px 16px; font-size: 13px; }
.btn:disabled { opacity: 0.6; cursor: not-allowed; }
.button-row { display: flex; gap: 12px; justify-content: center; }
.clone-dialog { margin-top: 24px; padding: 20px; background: var(--bg-card); border: 1px solid var(--border);
  border-radius: var(--radius); text-align: left; }
.clone-dialog h3 { font-size: 16px; margin-bottom: 12px; }
.clone-dialog input { width: 100%%; padding: 10px 12px; border: 1px solid var(--border); border-radius: var(--radius);
  font-size: 14px; font-family: var(--font-mono); background: var(--bg); color: var(--text); margin-bottom: 12px; }
.clone-dialog input:focus { outline: none; border-color: var(--primary); }
.clone-actions { display: flex; gap: 8px; justify-content: flex-end; }
.clone-status { margin-top: 12px; font-size: 13px; color: var(--text-muted); }
.clone-status.error { color: var(--danger); }
.auth-hint { font-size: 13px; color: var(--text-muted); margin: 12px 0 8px; }
.auth-code { font-family: var(--font-mono); font-size: 24px; font-weight: 700; letter-spacing: 4px;
  padding: 12px; background: var(--primary-light); border-radius: var(--radius); text-align: center; margin: 12px 0; }
#auth-section, #auth-pending, #project-picker, #no-project-section {
  margin-top: 16px; padding-top: 16px; border-top: 1px solid var(--border);
}
.project-list {
  background: var(--bg); border: 1px solid var(--border); border-radius: var(--radius);
  overflow: hidden; max-height: 200px; overflow-y: auto;
}
.clone-dest { display: flex; gap: 8px; margin-bottom: 12px; }
.clone-dest input { flex: 1; cursor: pointer; }
.setup-info { font-size: 13px; margin: 12px 0; line-height: 1.6; }
.setup-info span { font-family: var(--font-mono); color: var(--text-muted); }
.error { margin-bottom: 20px; padding: 12px 16px; background: var(--danger-light);
  border: 1px solid var(--danger-border); border-radius: var(--radius); color: var(--danger);
  font-size: 13px; font-family: var(--font-mono); text-align: left; word-break: break-word; }
.hint { margin-top: 24px; color: var(--text-muted); font-size: 13px; line-height: 1.6; }
.hint code { background: var(--bg-card); border: 1px solid var(--border); padding: 2px 6px; border-radius: 4px;
  font-family: var(--font-mono); font-size: 12px; }
.loading { display: none; margin-top: 16px; color: var(--text-muted); font-size: 14px; }
.runtime-error { display: none; margin-top: 20px; padding: 12px 16px; background: var(--danger-light);
  border: 1px solid var(--danger-border); border-radius: var(--radius); color: var(--danger);
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
.theme-toggle { position: fixed; top: 16px; right: 16px; width: 36px; height: 36px;
  border-radius: 50%%; border: 1px solid var(--border); background: var(--bg-card);
  color: var(--text); cursor: pointer; display: flex; align-items: center;
  justify-content: center; font-size: 16px; box-shadow: var(--shadow); transition: all 0.2s; }
.theme-toggle:hover { background: var(--primary-light); border-color: var(--primary); }
.theme-toggle .icon-sun, .theme-toggle .icon-moon { display: none; }
:root:not([data-theme]) .theme-toggle .icon-sun, [data-theme="light"] .theme-toggle .icon-sun { display: block; }
[data-theme="dark"] .theme-toggle .icon-moon { display: block; }
@media (prefers-color-scheme: dark) {
  :root:not([data-theme]) .theme-toggle .icon-moon { display: block; }
  :root:not([data-theme]) .theme-toggle .icon-sun { display: none; }
}
</style>
</head>
<body>
<button class="theme-toggle" onclick="toggleTheme()" title="Toggle dark mode">
  <span class="icon-sun">&#9788;</span>
  <span class="icon-moon">&#9790;</span>
</button>
<div class="welcome">
  <h1>Rela Desktop</h1>
  <p class="subtitle">Open a rela project directory to get started.<br>The folder should contain a <code>metamodel.yaml</code> file.</p>

  %s

  <div class="button-row">
    <button class="btn btn-primary" id="open-btn" onclick="openProject()">Open Project...</button>
    <button class="btn btn-secondary" id="clone-btn" onclick="showCloneDialog()">Clone from GitHub</button>
  </div>

  <div class="loading" id="loading">Opening...</div>
  <div class="runtime-error" id="runtime-error"></div>

  <div id="clone-dialog" class="clone-dialog" style="display:none;">
    <h3 id="clone-title">Clone Repository</h3>
    <div id="clone-url-section">
      <input type="text" id="clone-url" placeholder="https://github.com/user/repo" />
      <div class="clone-dest">
        <input type="text" id="clone-dest" placeholder="Clone to..." readonly />
        <button class="btn btn-secondary btn-small" onclick="pickCloneDir()">Browse...</button>
      </div>
      <div class="clone-actions">
        <button class="btn btn-secondary btn-small" onclick="hideCloneDialog()">Cancel</button>
        <button class="btn btn-primary btn-small" id="do-clone-btn" onclick="doClone()">Clone</button>
      </div>
    </div>
    <div id="clone-status" class="clone-status"></div>
    <div id="auth-section" style="display:none;">
      <p class="auth-hint">For private repos, authenticate with GitHub:</p>
      <button class="btn btn-secondary btn-small" onclick="startAuth()">Sign in with GitHub</button>
    </div>
    <div id="auth-pending" style="display:none;">
      <p>Enter this code at <a href="#" id="auth-link" target="_blank">github.com/login/device</a>:</p>
      <div class="auth-code" id="auth-code"></div>
      <p class="auth-hint">Waiting for authorization...</p>
    </div>
    <div id="project-picker" style="display:none;">
      <p class="auth-hint">Multiple rela projects found. Select one:</p>
      <div id="project-list" class="project-list"></div>
    </div>
    <div id="no-project-section" style="display:none;">
      <p class="auth-hint">No rela project found in this repository.</p>
      <input type="text" id="init-subfolder" placeholder="Subfolder (leave empty for root)" />
      <div class="clone-actions">
        <button class="btn btn-secondary btn-small" onclick="hideCloneDialog()">Cancel</button>
        <button class="btn btn-primary btn-small" onclick="initProject()">Initialize Project</button>
      </div>
    </div>
  </div>

  <div id="setup-dialog" class="clone-dialog" style="display:none;">
    <h3>Setup Data Entry</h3>
    <p class="auth-hint">
      This project has a metamodel but no data-entry.yaml configuration.
    </p>
    <p class="setup-info">
      <strong>Path:</strong> <span id="setup-path"></span><br>
      <strong>Entity types:</strong> <span id="setup-types"></span>
    </p>
    <input type="text" id="setup-app-name" placeholder="Application name (e.g. My Project)" />
    <div class="clone-actions">
      <button class="btn btn-secondary btn-small" onclick="hideSetupDialog()">Cancel</button>
      <button class="btn btn-primary btn-small" onclick="generateConfig()">Generate Config</button>
    </div>
    <div id="setup-status" class="clone-status"></div>
  </div>

  <div class="hint">
    You can also launch from the command line:<br>
    <code>rela-desktop -project /path/to/project</code>
  </div>

  %s
</div>
<script>
function toggleTheme() {
  var current = document.documentElement.getAttribute('data-theme');
  var next = current === 'dark' ? 'light' : 'dark';
  document.documentElement.setAttribute('data-theme', next);
  localStorage.setItem('theme', next);
}
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
    } else if (result === "needs_setup") {
      showSetupDialog();
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
    } else if (result === "needs_setup") {
      showSetupDialog();
    } else {
      errorEl.textContent = result;
      errorEl.style.display = "block";
    }
  } catch (e) {
    errorEl.textContent = String(e);
    errorEl.style.display = "block";
  }
}

async function showSetupDialog() {
  try {
    var info = await window.go.main.Desktop.GetSetupInfo();
    if (info.error) {
      document.getElementById("runtime-error").textContent = info.error;
      document.getElementById("runtime-error").style.display = "block";
      return;
    }
    document.getElementById("setup-path").textContent = info.path;
    var typeList = info.entity_types.join(", ") || "none defined";
    document.getElementById("setup-types").textContent = typeList;
    document.getElementById("setup-dialog").style.display = "block";
  } catch (e) {
    document.getElementById("runtime-error").textContent = String(e);
    document.getElementById("runtime-error").style.display = "block";
  }
}

function hideSetupDialog() {
  document.getElementById("setup-dialog").style.display = "none";
}

async function generateConfig() {
  var appName = document.getElementById("setup-app-name").value.trim() || "My Project";
  var statusEl = document.getElementById("setup-status");
  statusEl.textContent = "Generating...";

  try {
    var result = await window.go.main.Desktop.GenerateDataEntryConfig(appName);
    if (result === "") {
      window.location.reload();
    } else if (result === "needs_setup") {
      statusEl.textContent = "Generated but still needs configuration.";
    } else {
      statusEl.textContent = result;
    }
  } catch (e) {
    statusEl.textContent = String(e);
  }
}

async function showCloneDialog() {
  document.getElementById("clone-dialog").style.display = "block";
  document.getElementById("clone-url").focus();
  // Load default clone directory
  var defaultDir = await window.go.main.Desktop.GetDefaultCloneDir();
  document.getElementById("clone-dest").value = defaultDir;
  checkAuthStatus();
}

async function pickCloneDir() {
  try {
    var dir = await window.go.main.Desktop.PickCloneDirectory();
    if (dir) {
      document.getElementById("clone-dest").value = dir;
    }
  } catch (e) {
    console.error("Failed to pick directory:", e);
  }
}

function hideCloneDialog() {
  document.getElementById("clone-dialog").style.display = "none";
  document.getElementById("clone-status").textContent = "";
  document.getElementById("clone-url-section").style.display = "block";
  document.getElementById("auth-pending").style.display = "none";
  document.getElementById("auth-section").style.display = "none";
  document.getElementById("project-picker").style.display = "none";
  document.getElementById("no-project-section").style.display = "none";
  document.getElementById("clone-title").textContent = "Clone Repository";
  document.getElementById("do-clone-btn").disabled = false;
}

async function checkAuthStatus() {
  try {
    var hasToken = await window.go.main.Desktop.HasGitHubToken();
    document.getElementById("auth-section").style.display = hasToken ? "none" : "block";
  } catch (e) {
    console.error("Failed to check auth status:", e);
  }
}

async function doClone() {
  var url = document.getElementById("clone-url").value.trim();
  var targetDir = document.getElementById("clone-dest").value.trim();
  var statusEl = document.getElementById("clone-status");
  var btn = document.getElementById("do-clone-btn");

  if (!url) {
    statusEl.textContent = "Please enter a repository URL";
    statusEl.className = "clone-status error";
    return;
  }

  if (!targetDir) {
    statusEl.textContent = "Please select a destination folder";
    statusEl.className = "clone-status error";
    return;
  }

  btn.disabled = true;
  statusEl.textContent = "Cloning...";
  statusEl.className = "clone-status";

  try {
    var result = await window.go.main.Desktop.CloneProject(url, targetDir);
    if (result.error) {
      statusEl.textContent = result.error;
      statusEl.className = "clone-status error";
      btn.disabled = false;
      return;
    }

    if (result.status === "opened") {
      window.location.reload();
      return;
    }

    // Hide URL input section
    document.getElementById("clone-url-section").style.display = "none";
    statusEl.textContent = "";

    if (result.status === "multiple") {
      // Show project picker
      document.getElementById("clone-title").textContent = "Select Project";
      var listEl = document.getElementById("project-list");
      listEl.innerHTML = "";
      result.projects.forEach(function(proj) {
        var item = document.createElement("a");
        item.className = "recent-item";
        item.href = "#";
        item.innerHTML = '<div class="name">' + proj + '</div>';
        item.onclick = function() { selectProject(proj); return false; };
        listEl.appendChild(item);
      });
      document.getElementById("project-picker").style.display = "block";
    } else if (result.status === "no_projects") {
      // Show init option
      document.getElementById("clone-title").textContent = "Initialize Project";
      document.getElementById("no-project-section").style.display = "block";
    }
  } catch (e) {
    statusEl.textContent = String(e);
    statusEl.className = "clone-status error";
    btn.disabled = false;
  }
}

async function selectProject(subfolder) {
  var statusEl = document.getElementById("clone-status");
  statusEl.textContent = "Opening...";
  statusEl.className = "clone-status";

  try {
    var result = await window.go.main.Desktop.OpenClonedProject(subfolder);
    if (result === "") {
      window.location.reload();
    } else {
      statusEl.textContent = result;
      statusEl.className = "clone-status error";
    }
  } catch (e) {
    statusEl.textContent = String(e);
    statusEl.className = "clone-status error";
  }
}

async function initProject() {
  var subfolder = document.getElementById("init-subfolder").value.trim();
  var statusEl = document.getElementById("clone-status");
  statusEl.textContent = "Initializing...";
  statusEl.className = "clone-status";

  try {
    var result = await window.go.main.Desktop.InitRelaProject(subfolder);
    if (result === "") {
      window.location.reload();
    } else {
      statusEl.textContent = result;
      statusEl.className = "clone-status error";
    }
  } catch (e) {
    statusEl.textContent = String(e);
    statusEl.className = "clone-status error";
  }
}

async function startAuth() {
  var statusEl = document.getElementById("clone-status");
  statusEl.textContent = "Starting authentication...";

  try {
    var result = await window.go.main.Desktop.StartGitHubAuth();
    if (result.error) {
      statusEl.textContent = result.error;
      statusEl.className = "clone-status error";
      return;
    }

    document.getElementById("auth-section").style.display = "none";
    document.getElementById("auth-pending").style.display = "block";
    document.getElementById("auth-code").textContent = result.user_code;
    var link = document.getElementById("auth-link");
    link.href = result.verification_url;
    link.textContent = result.verification_url;

    statusEl.textContent = "";

    // Open browser for user
    window.open(result.verification_url, "_blank");

    // Wait for authorization
    var authResult = await window.go.main.Desktop.CompleteGitHubAuth();
    if (authResult === "") {
      document.getElementById("auth-pending").style.display = "none";
      statusEl.textContent = "Authenticated! You can now clone private repositories.";
      statusEl.className = "clone-status";
    } else {
      statusEl.textContent = authResult;
      statusEl.className = "clone-status error";
      document.getElementById("auth-pending").style.display = "none";
      document.getElementById("auth-section").style.display = "block";
    }
  } catch (e) {
    statusEl.textContent = String(e);
    statusEl.className = "clone-status error";
  }
}

// Listen for menu event to show clone dialog
if (window.runtime) {
  window.runtime.EventsOn("show-clone-dialog", function() {
    showCloneDialog();
  });
}
</script>
</body>
</html>`
