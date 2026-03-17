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
	inner.HandleFunc("/analyze", a.handleAnalyze)
	inner.HandleFunc("/search", a.handleSearch)
	inner.HandleFunc("/list/", a.handleList)
	inner.HandleFunc("/kanban/", a.handleKanban)
	inner.HandleFunc("/form/", a.handleForm)
	inner.HandleFunc("/entity/", a.handleEntity)
	inner.HandleFunc("/view/", a.handleView)
	inner.HandleFunc("/document/", a.handleDocument)
	inner.HandleFunc("/api/create", a.handleCreate)
	inner.HandleFunc("/api/update", a.handleUpdate)
	inner.HandleFunc("/api/delete", a.handleDelete)
	inner.HandleFunc("/api/kanban/move", a.handleKanbanMove)
	inner.HandleFunc("/api/toggle-checkbox", a.handleToggleCheckbox)
	inner.HandleFunc("/api/inline-create", a.handleInlineCreate)
	inner.HandleFunc("/api/inline-form/", a.handleInlineForm)
	inner.HandleFunc("/api/help/", a.handleEntityHelp)
	inner.HandleFunc("/api/link-candidates", a.handleLinkCandidates)
	inner.HandleFunc("/api/link-existing", a.handleLinkExisting)
	inner.HandleFunc("/graph", a.handleGraph)
	inner.HandleFunc("/api/graph-data", a.handleGraphData)
	inner.HandleFunc("/api/ui/toggle-group", a.handleToggleGroup)
	inner.HandleFunc("/api/command/", a.handleCommandExec)
	inner.HandleFunc("/api/command-cancel/", a.handleCommandCancel)
	inner.HandleFunc("/api/open-file", a.handleOpenFile)
	inner.HandleFunc("/api/open-url", a.handleOpenURL)
	inner.HandleFunc("/settings", a.handleSettings)
	inner.HandleFunc("/api/settings", a.handleSaveSettings)
	inner.HandleFunc("/conflicts", a.handleConflicts)
	inner.HandleFunc("/conflicts/", a.handleConflicts)
	inner.HandleFunc("/api/conflict-resolve", a.handleConflictApply)
	inner.HandleFunc("/api/git/status", a.handleGitStatus)
	inner.HandleFunc("/api/git/sync", a.handleGitSync)

	// JSON API endpoints for mobile/programmatic access (legacy, will be deprecated)
	inner.HandleFunc("/api/entity-types", a.handleAPIEntityTypes)
	inner.HandleFunc("/api/entities", a.handleAPIEntitiesCRUD)
	inner.HandleFunc("/api/entities/", a.handleAPIEntityCRUD)
	inner.HandleFunc("/api/relations", a.handleAPIRelationsCRUD)
	inner.HandleFunc("/api/metamodel", a.handleAPIMetamodel)
	inner.HandleFunc("/api/analyze", a.handleAPIAnalyze)
	inner.HandleFunc("/api/search", a.handleAPISearch)
	inner.HandleFunc("/api/openapi.json", a.handleV1OpenAPI)

	// REST API v1 - RESTful endpoints with entity type in path
	// /api/v1/{entity-type-plural}[/{id}[/relations[/{rel-type}[/{target-id}]]]]
	inner.HandleFunc("/api/v1/", a.handleRESTEntities)

	locked := a.reloadLockMiddleware(inner)
	mux.Handle("/", locked)

	return mux
}
