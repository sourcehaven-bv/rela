// Package frontendroutes is the Go-side catalog of the data-entry SPA's
// Vue Router routes. It is the single source of truth consumed by the Lua
// rela.url helper, the document link rewriter, and the rela-server routes
// subcommand. A parity test in internal/frontendparity fails CI if the
// catalog drifts from frontend/src/router/index.ts.
//
// The package is stdlib-only and stateless: exported functions operate on
// a private routes slice. No constructor required.
package frontendroutes

import (
	"sort"
	"strings"
)

// Route describes one frontend route.
type Route struct {
	Name            string // e.g. "form-edit"
	Path            string // e.g. "/form/:id/:entityId"
	AcceptsReturnTo bool   // form routes use return_to for post-submit navigation
	Notes           string // optional human-readable hint for rela-server routes
}

// MatchedRoute is returned by Match: the matched Route descriptor.
type MatchedRoute struct {
	Route Route
}

var routes = []Route{
	{Name: "dashboard", Path: "/dashboard"},
	{Name: "list", Path: "/list/:id", Notes: "id = list id"},
	{Name: "form-create", Path: "/form/:id", AcceptsReturnTo: true, Notes: "id = form id"},
	{Name: "form-edit", Path: "/form/:id/:entityId", AcceptsReturnTo: true, Notes: "id = form id; entityId = entity being edited"},
	{Name: "entity", Path: "/entity/:type/:id", Notes: "type = entity type; id = entity id"},
	{Name: "view", Path: "/view/:id/:entityId", Notes: "id = view id; entityId = entity being rendered"},
	{Name: "kanban", Path: "/kanban/:id", Notes: "id = kanban id"},
	{Name: "search", Path: "/search"},
	{Name: "analyze", Path: "/analyze"},
	{Name: "settings", Path: "/settings"},
	{Name: "conflicts", Path: "/conflicts"},
	{Name: "document", Path: "/document/:name/:entityId", Notes: "name = document id; entityId = entity being rendered"},
}

// All returns every known route, sorted by name. The returned slice is a
// fresh copy — callers can mutate the slice header safely. Contents
// should be treated as read-only.
func All() []Route {
	out := make([]Route, len(routes))
	copy(out, routes)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Has reports whether a literal path matches any known route pattern.
// Equivalent to `_, ok := Match(path); ok`.
func Has(path string) bool {
	_, ok := Match(path)
	return ok
}

// Match finds the route whose pattern matches the given literal path. For
// example "/form/full_ticket/TKT-001" matches "/form/:id/:entityId".
//
// Match is deterministic: the first route (in catalog order) whose pattern
// matches wins. Routes are written so patterns do not overlap.
//
// Path segments are compared byte-for-byte; percent-encoded characters are
// not decoded. "/form/edit_ticket/TKT%2F001" matches "/form/:id/:entityId"
// and the captured entityId value would be the literal "TKT%2F001".
func Match(path string) (MatchedRoute, bool) {
	for i := range routes {
		if patternMatches(routes[i].Path, path) {
			return MatchedRoute{Route: routes[i]}, true
		}
	}
	return MatchedRoute{}, false
}

// patternMatches reports whether a literal path matches a Vue-router
// pattern. Only segment count + non-param segment equality is checked;
// param segments (":id" etc.) match any non-empty value.
func patternMatches(pattern, path string) bool {
	pSeg := splitPath(pattern)
	vSeg := splitPath(path)
	if len(pSeg) != len(vSeg) {
		return false
	}
	for i := range pSeg {
		if strings.HasPrefix(pSeg[i], ":") {
			if vSeg[i] == "" {
				return false
			}
			continue
		}
		if pSeg[i] != vSeg[i] {
			return false
		}
	}
	return true
}

// splitPath breaks a path into non-empty segments. "/" returns []; "/a/b"
// returns ["a","b"]. The leading slash is always required for the catalogue's
// paths, so empty input or paths without a leading slash yield a non-matching
// segment count.
func splitPath(p string) []string {
	if p == "" || p == "/" {
		return nil
	}
	p = strings.TrimPrefix(p, "/")
	return strings.Split(p, "/")
}
