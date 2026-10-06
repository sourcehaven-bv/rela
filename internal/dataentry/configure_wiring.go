package dataentry

import (
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// ConfigurePath is where the Configure API is mounted.
const ConfigurePath = "/api/v1/_configure"

// interactivePrincipalType is the principal_type claim of a person signed
// in, as opposed to an app, token or service acting for one.
const interactivePrincipalType = "user"

// sseConfigChanged tells browsers the server now runs a new configuration,
// so they reload rather than keep a schema that no longer applies.
const sseConfigChanged = "config-changed"

// SetConfigure mounts the Configure API (TKT-F5NGMG) at [ConfigurePath].
// Call it before NewRouter. h is not authorized here: the mount admits only
// a known principal holding [acl.PermConfigEdit] under a loaded acl.yaml,
// and h may assume that.
//
// A free function rather than an App method: App's method count is pinned
// (plimsoll), and this is one-shot wiring.
func SetConfigure(a *App, h http.Handler) { a.configure = h }

// PauseConfigReload makes the app ignore changes to data-entry.yaml until
// resume is called. The Configure save writes schema.yaml and
// data-entry.yaml one after the other, and must not have the running app
// reload a half-written pair.
func PauseConfigReload(a *App) (resume func()) {
	a.reloadPaused.Add(1)
	return func() { a.reloadPaused.Add(-1) }
}

// Retire takes an app out of service after a new one replaced it: it stops
// watching files and the store, tells every live-update stream the
// configuration changed and closes it. Requests already running finish
// normally.
func Retire(a *App) {
	a.StopWatching()
	a.broker.broadcast(sseConfigChanged)
	a.broker.close()
}

// mayConfigure reports whether the request's principal may use the Configure
// API. It needs a real acl.yaml: without one every principal holds every
// permission, and editing the configuration must never be that open.
func mayConfigure(a *App, r *http.Request) bool {
	if _, ok := a.acl.(*acl.Declarative); !ok {
		return false
	}
	ctx := r.Context()
	// From never returns a zero principal: an unstamped ctx reads as
	// unknown/unknown, so the user is what tells an identity apart.
	p := principal.From(ctx)
	if p.User == "" || p.User == principal.Unknown {
		return false
	}
	// A client acting for a user never configures. The ceiling refuses
	// config:edit only where a baseline matches the client's principal
	// type, and an unmatched type is unrestricted, so the gate does not
	// rely on that: only an interactive caller (no type, or "user") passes.
	if t := p.PrincipalType(); t != "" && t != interactivePrincipalType {
		return false
	}
	return readGateFromContext(ctx).HoldsPermission(ctx, acl.PermConfigEdit)
}

// serveConfigure returns the handler that gates the Configure API. Without it mounted the routes do
// not exist (404); a caller without the permission gets 403, which names a
// configuration capability rather than hiding one (see CLAUDE.md, "The
// configuration is not a secret").
//
// The gate helpers are free functions for the same reason as [SetConfigure].
func serveConfigure(a *App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.configure == nil {
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
			return
		}
		if !mayConfigure(a, r) {
			writeV1Error(w, r, http.StatusForbidden, "forbidden", "Forbidden",
				"Editing the configuration needs the "+acl.PermConfigEdit+" permission.")
			return
		}
		a.configure.ServeHTTP(w, r)
	}
}
