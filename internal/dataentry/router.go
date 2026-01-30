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
	return mux
}
