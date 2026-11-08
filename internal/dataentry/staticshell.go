package dataentry

import (
	"io/fs"
	"net/http"
)

// StaticShell serves the embedded SPA shell and its static assets and
// nothing else: the `/static/` mount and the shell catch-all of
// [App.NewRouter], without an App.
//
// A multi-tenant host serves it to a request that has no tenant yet, so a
// browser can load and render a signed-out state before it presents an
// assertion. Every byte it can return is compiled into the binary, so it
// reads no store and no project file and cannot disclose a tenant's data,
// whichever path it is handed. The operator's `custom/` injection is left
// out because it reads project files; a request with a verified tenant gets
// the tenant's full router instead.
//
// Panics only if the binary was built without the embedded SPA filesystem,
// which [CheckEmbeddedSPA] reports at startup.
func StaticShell() http.Handler {
	staticFS, err := fs.Sub(staticFiles, "static")
	// coverage-ignore-start: panic-invariant: fs.Sub on the compile-time embed.FS with a literal path cannot fail
	if err != nil {
		panic("failed to mount embedded static filesystem (static): " + err.Error())
	}
	spaFS, err := fs.Sub(staticFiles, "static/v2")
	if err != nil {
		panic("failed to mount embedded SPA filesystem (static/v2): " + err.Error())
	}
	// coverage-ignore-end
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.Handle("/", spaHandler(spaFS))
	return mux
}
