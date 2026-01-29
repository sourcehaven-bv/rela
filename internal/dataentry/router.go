package dataentry

import "net/http"

// NewRouter returns an http.Handler with all data entry routes registered.
func (a *App) NewRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleIndex)
	mux.HandleFunc("/list/", a.handleList)
	mux.HandleFunc("/form/", a.handleForm)
	mux.HandleFunc("/entity/", a.handleEntity)
	mux.HandleFunc("/view/", a.handleView)
	mux.HandleFunc("/api/create", a.handleCreate)
	mux.HandleFunc("/api/update", a.handleUpdate)
	mux.HandleFunc("/api/delete", a.handleDelete)
	mux.HandleFunc("/api/inline-create", a.handleInlineCreate)
	mux.HandleFunc("/api/inline-form/", a.handleInlineForm)
	// Conflict resolution routes
	mux.HandleFunc("/conflicts", a.handleConflicts)
	mux.HandleFunc("/conflicts/resolve/", a.handleConflictResolve)
	mux.HandleFunc("/api/conflicts/resolve", a.handleConflictSubmit)
	mux.HandleFunc("/api/conflicts/status", a.handleConflictStatus)
	mux.HandleFunc("/api/conflicts/resolve-all", a.handleConflictResolveAll)
	mux.HandleFunc("/api/conflicts/load-test", a.handleConflictLoadTest)
	// Sync routes
	mux.HandleFunc("/api/sync/status", a.handleSyncStatus)
	mux.HandleFunc("/api/sync/indicator", a.handleSyncStatusIndicator)
	mux.HandleFunc("/api/sync/sse", a.handleSyncSSE)
	mux.HandleFunc("/api/sync/branches", a.handleSyncBranches)
	mux.HandleFunc("/api/sync/branches-list", a.handleSyncBranchesList)
	mux.HandleFunc("/api/sync/branch", a.handleSyncBranch)
	mux.HandleFunc("/api/sync/push", a.handleSyncPush)
	mux.HandleFunc("/api/sync/pull", a.handleSyncPull)
	mux.HandleFunc("/api/sync/move-to-branch", a.handleSyncMoveToBranch)
	return mux
}
