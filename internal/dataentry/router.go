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

	// Static files (favicon only - v1 assets removed)
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("embedded static filesystem: " + err.Error())
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Vue SPA - serve at root and /v2/ for backward compatibility
	v2FS, err := fs.Sub(staticFiles, "static/v2")
	if err != nil {
		panic("embedded v2 filesystem: " + err.Error())
	}
	mux.Handle("/v2/", http.StripPrefix("/v2/", spaHandler(v2FS)))

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
	mux.Handle("/", spaHandler(v2FS))

	return mux
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
