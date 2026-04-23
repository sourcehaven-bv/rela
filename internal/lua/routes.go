package lua

// RouteHasFunc verifies whether a literal path matches a known
// frontend-route pattern. It's the minimum surface rela.url needs and
// the only thing the Lua runtime knows about the route catalog — defined
// here per CLAUDE.md so internal/lua does not import the concrete
// catalog package. The actual route catalog (internal/frontendroutes)
// exports a package-level Has function that satisfies this type.
type RouteHasFunc func(path string) bool

// WithRouteCatalog wires a route-has function into the runtime so
// rela.url is registered on the rela table. If unset, rela.url is absent
// (same pattern as rela.cache), and any access from Lua raises
// "attempt to index a nil value".
func WithRouteCatalog(has RouteHasFunc) Option {
	return func(r *Runtime) {
		r.routes = has
	}
}
