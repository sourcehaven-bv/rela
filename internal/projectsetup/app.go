package projectsetup

import (
	"fmt"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// ScaffoldAppResult reports what `rela apps new` created.
type ScaffoldAppResult struct {
	ID       string
	Dir      string // absolute apps/<id> directory
	IndexAbs string // absolute index.html path
	CSSAbs   string // absolute app.css path
	JSAbs    string // absolute app.js path
}

// ScaffoldApp creates a starter custom-app folder apps/<id>/ wired up to the
// bridge SDK (_rela.js) and the optional theme stylesheet (_rela.css).
// startDir is where project discovery begins (empty = cwd).
//
// It writes THREE files — index.html, app.css, app.js — rather than one
// self-contained page. The app CSP has no 'unsafe-inline', so an inline
// <style> or <script> in a scaffolded app would be silently dead on arrival
// (see dataentry.appCSP). Separate files are what actually runs.
func ScaffoldApp(startDir, id string) (*ScaffoldAppResult, error) {
	fs := storage.NewSafeFS(storage.NewOsFS())
	return ScaffoldAppWithFS(startDir, id, fs)
}

// ScaffoldAppWithFS is ScaffoldApp with an injectable filesystem (for tests).
func ScaffoldAppWithFS(startDir, id string, fs storage.FS) (*ScaffoldAppResult, error) {
	if !dataentryconfig.ValidAppID(id) {
		return nil, fmt.Errorf("invalid app id %q: must match ^[a-z0-9_-]{1,64}$ (lowercase letters, digits, '-', '_')", id)
	}

	ctx, err := project.Discover(startDir, fs)
	if err != nil {
		return nil, fmt.Errorf("no rela project found (run `rela init` first): %w", err)
	}

	appDir := filepath.Join(ctx.Root, project.AppsDir, id)
	indexPath := filepath.Join(appDir, appIndexName)
	cssPath := filepath.Join(appDir, appCSSName)
	jsPath := filepath.Join(appDir, appJSName)

	if _, err := fs.Stat(indexPath); err == nil {
		return nil, fmt.Errorf("app %q already exists (%s)", id, indexPath)
	}

	if err := fs.MkdirAll(appDir, 0o755); err != nil {
		return nil, fmt.Errorf("create app directory: %w", err)
	}
	// index.html last: it is what appExists() keys on, so writing it only after
	// its two assets are on disk means a half-written scaffold is not yet a
	// live app.
	for _, f := range []struct {
		path string
		body string
	}{
		{cssPath, starterAppCSS()},
		{jsPath, starterAppJS()},
		{indexPath, starterAppHTML(id)},
	} {
		if err := fs.WriteFile(f.path, []byte(f.body), 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", filepath.Base(f.path), err)
		}
	}

	return &ScaffoldAppResult{
		ID: id, Dir: appDir, IndexAbs: indexPath, CSSAbs: cssPath, JSAbs: jsPath,
	}, nil
}

// appIndexName, appCSSName and appJSName are the scaffold's three files. The
// asset names are ordinary (no underscore prefix): rela reserves the "_" prefix
// for endpoints it serves from the binary (_rela.js, _rela.css), and an app may
// not shadow those.
const (
	appIndexName = "index.html"
	appCSSName   = "app.css"
	appJSName    = "app.js"
)

// starterAppHTML returns a minimal, working app: it links the bridge SDK, the
// optional rela theme, and the app's own stylesheet and script. Authors edit
// from here.
//
// Deliberately free of inline <style>/<script> and style="" attributes — the
// app CSP has no 'unsafe-inline', so inline code would not run. Keep it that
// way when editing this template.
func starterAppHTML(id string) string {
	return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <title>` + id + `</title>
    <!-- Required: the bridge contract this app targets. The server refuses to
         serve an app that omits this or asks for a newer bridge than it has. -->
    <meta name="rela-app:bridge-version" content="1" />
    <!-- Sidebar label + description (optional). -->
    <meta name="rela-app:label" content="` + id + `" />
    <meta name="rela-app:description" content="A custom rela app." />
    <!-- The rela bridge SDK — provides window.rela. Required. -->
    <script src="_rela.js"></script>
    <!-- Optional: rela's theme tokens + base controls (.btn/.input/.card).
         Dark mode follows the host automatically. Remove for full control. -->
    <link rel="stylesheet" href="_rela.css" />
    <!-- This app's own styles and code. Separate files, not inline: the app
         CSP has no 'unsafe-inline', so an inline <style> or <script> here
         would be blocked and silently do nothing. -->
    <link rel="stylesheet" href="` + appCSSName + `" />
  </head>
  <body>
    <h1>` + id + `</h1>
    <div id="out" class="muted">Loading…</div>

    <script src="` + appJSName + `"></script>
  </body>
</html>
`
}

// starterAppCSS is the scaffold's stylesheet. It uses rela's theme tokens so
// the app follows the host's light/dark mode without extra work.
func starterAppCSS() string {
	return `body {
  font-family: inherit;
  margin: 0;
  padding: 1.5rem;
  color: var(--text-color);
  background: var(--bg-color);
}

h1 {
  font-size: 1.25rem;
  margin: 0 0 1rem;
}

.muted {
  color: var(--muted-text);
}
`
}

// starterAppJS is the scaffold's script: it waits for the bridge, reads through
// it, and renders the result.
func starterAppJS() string {
	return `// window.rela is ready after the one-time 'rela:ready' event. Calls made
// earlier are queued, so this is just to avoid a flash of empty content.
window.addEventListener('rela:ready', async () => {
  const out = document.getElementById('out');
  try {
    // Replace 'ticket' with one of your entity types. See the available
    // bridge methods (rela.list/get/search/create/update/...) in the
    // data-entry guide.
    const res = await rela.list({ type: 'ticket', params: { per_page: 50 } });
    out.classList.remove('muted');
    out.textContent = (res.data ? res.data.length : 0) + ' entities loaded.';
  } catch (e) {
    out.textContent = 'Error: ' + (e && e.message ? e.message : e);
  }
});
`
}
