package dataentry

import (
	"io/fs"
	"net/http"
	"strings"
)

// NewRouter returns an http.Handler with all data entry routes registered.
// The Vue SPA serves as the primary UI at the root path.
func (a *App) NewRouter() http.Handler {
	mux := http.NewServeMux()

	// Legacy /static/ mount. The Vue bundle is also reachable here as
	// /static/v2/*, but the SPA's built index.html references assets as
	// /assets/*, served via the catch-all below.
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("failed to mount embedded static filesystem (static): " + err.Error())
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Vue SPA served at root. The build output dir is kept as `static/v2` to
	// avoid churn in frontend/vite.config.ts; see TKT-MNOO.
	spaFS, err := fs.Sub(staticFiles, "static/v2")
	if err != nil {
		panic("failed to mount embedded SPA filesystem (static/v2): " + err.Error())
	}
	// Fail fast if the SPA bundle is missing — BUG-W144 class regression.
	if _, err := fs.Stat(spaFS, "index.html"); err != nil {
		panic("embedded SPA is missing index.html (run `just build-frontend`): " + err.Error())
	}

	// SSE endpoints — excluded from reload-lock (long-lived connection)
	mux.HandleFunc("/api/events", a.handleSSE)
	mux.HandleFunc("/api/v1/_events", a.handleSSE)

	// All other routes are wrapped with the reload-lock middleware
	inner := http.NewServeMux()

	// APIs used by Vue SPA
	inner.HandleFunc("/api/toggle-checkbox", a.handleToggleCheckbox)
	inner.HandleFunc("/api/help/", a.handleEntityHelp)
	inner.HandleFunc("/api/graph-data", a.handleGraphData)
	inner.HandleFunc("/api/command/", a.handleCommandExec)
	inner.HandleFunc("/api/command-cancel/", a.handleCommandCancel)
	inner.HandleFunc("/api/open-file", a.handleOpenFile)
	inner.HandleFunc("/api/git/status", a.handleGitStatus)
	inner.HandleFunc("/api/git/sync", a.handleGitSync)

	// REST API v1 - main API for Vue SPA
	a.registerAPIV1Routes(inner)

	locked := a.reloadLockMiddleware(inner)
	mux.Handle("/api/", locked)

	// Serve Vue SPA at root (catch-all for client-side routing)
	mux.Handle("/", spaHandler(spaFS))

	// Apply security middlewares as the outermost wrapper so they protect
	// every route, including the SSE handlers and static assets. The
	// requireSameOrigin middleware internally exempts non-sensitive paths
	// (e.g. static assets, SPA shell) so the SPA still loads cross-origin.
	var handler http.Handler = mux
	if a.security != nil {
		handler = a.security.requireSameOrigin(handler)
		handler = a.security.requireLocalHost(handler)
	}
	return handler
}

// spaHandler wraps a filesystem and serves index.html for any path that doesn't
// match an existing file. This enables client-side routing in SPAs.
func spaHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "" || path == "/" {
			path = "index.html"
		}

		// Check if the file exists
		if _, err := fs.Stat(fsys, strings.TrimPrefix(path, "/")); err != nil {
			// File doesn't exist, serve index.html for SPA routing
			r.URL.Path = "/"
		}

		fileServer.ServeHTTP(w, r)
	})
}
