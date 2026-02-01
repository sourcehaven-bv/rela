package dataentry

import (
	"io/fs"
	"net/http"
)

// NewRouter returns an http.Handler with all data entry routes registered.
// Handlers are wrapped with a reload-lock middleware so that live-reload
// does not swap state mid-request. The SSE endpoint is excluded from
// the middleware since it holds the connection open indefinitely.
func (a *App) NewRouter() http.Handler {
	mux := http.NewServeMux()
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("embedded static filesystem: " + err.Error())
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// SSE endpoint — excluded from reload-lock (long-lived connection)
	mux.HandleFunc("/api/events", a.handleSSE)

	// All other routes are wrapped with the reload-lock middleware
	inner := http.NewServeMux()
	inner.HandleFunc("/", a.handleIndex)
	inner.HandleFunc("/dashboard", a.handleDashboard)
	inner.HandleFunc("/search", a.handleSearch)
	inner.HandleFunc("/list/", a.handleList)
	inner.HandleFunc("/form/", a.handleForm)
	inner.HandleFunc("/entity/", a.handleEntity)
	inner.HandleFunc("/view/", a.handleView)
	inner.HandleFunc("/api/create", a.handleCreate)
	inner.HandleFunc("/api/update", a.handleUpdate)
	inner.HandleFunc("/api/delete", a.handleDelete)
	inner.HandleFunc("/api/inline-create", a.handleInlineCreate)
	inner.HandleFunc("/api/inline-form/", a.handleInlineForm)
	inner.HandleFunc("/graph", a.handleGraph)
	inner.HandleFunc("/api/graph-data", a.handleGraphData)
	inner.HandleFunc("/api/ui/toggle-group", a.handleToggleGroup)
	inner.HandleFunc("/api/command/", a.handleCommandExec)
	inner.HandleFunc("/api/command-cancel/", a.handleCommandCancel)
	inner.HandleFunc("/api/open-file", a.handleOpenFile)
	inner.HandleFunc("/api/open-url", a.handleOpenURL)

	locked := a.reloadLockMiddleware(inner)
	mux.Handle("/", locked)

	return mux
}
