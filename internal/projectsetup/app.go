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
}

// ScaffoldApp creates a starter custom-app folder apps/<id>/ with an index.html
// wired up to the bridge SDK (_rela.js) and the optional theme stylesheet
// (_rela.css). startDir is where project discovery begins (empty = cwd).
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
	indexPath := filepath.Join(appDir, "index.html")

	if _, err := fs.Stat(indexPath); err == nil {
		return nil, fmt.Errorf("app %q already exists (%s)", id, indexPath)
	}

	if err := fs.MkdirAll(appDir, 0o755); err != nil {
		return nil, fmt.Errorf("create app directory: %w", err)
	}
	if err := fs.WriteFile(indexPath, []byte(starterAppHTML(id)), 0o644); err != nil {
		return nil, fmt.Errorf("write index.html: %w", err)
	}

	return &ScaffoldAppResult{ID: id, Dir: appDir, IndexAbs: indexPath}, nil
}

// starterAppHTML returns a minimal, working app: it links the bridge SDK and the
// optional rela theme, reads through the bridge on load, and renders a result.
// Authors edit from here.
func starterAppHTML(id string) string {
	return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <title>` + id + `</title>
    <!-- Sidebar label + description (optional). -->
    <meta name="rela-app:label" content="` + id + `" />
    <meta name="rela-app:description" content="A custom rela app." />
    <!-- The rela bridge SDK — provides window.rela. Required. -->
    <script src="_rela.js"></script>
    <!-- Optional: rela's theme tokens + base controls (.btn/.input/.card).
         Dark mode follows the host automatically. Remove for full control. -->
    <link rel="stylesheet" href="_rela.css" />
    <style>
      body {
        font-family: inherit;
        margin: 0;
        padding: 1.5rem;
        color: var(--text-color);
        background: var(--bg-color);
      }
      h1 { font-size: 1.25rem; margin: 0 0 1rem; }
      .muted { color: var(--muted-text); }
    </style>
  </head>
  <body>
    <h1>` + id + `</h1>
    <div id="out" class="muted">Loading…</div>

    <script>
      // window.rela is ready after the one-time 'rela:ready' event. Calls made
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
    </script>
  </body>
</html>
`
}
