package lua

// RouteCatalog is the minimum surface rela.url needs: verify whether a literal
// path matches a known frontend-route pattern. Defined at the call site per
// CLAUDE.md so internal/lua does not import the concrete catalog package.
//
// The catalog is stateless config, not a graph capability — it is wired in
// via WithRouteCatalog rather than on ReadDeps/WriteDeps.
type RouteCatalog interface {
	Has(path string) bool
}

// RouteCatalogFunc adapts a plain function into a RouteCatalog, so callers
// can pass frontendroutes.Has directly without defining a wrapper type.
type RouteCatalogFunc func(path string) bool

// Has satisfies RouteCatalog.
func (f RouteCatalogFunc) Has(path string) bool { return f(path) }

// WithRouteCatalog wires a route catalog into the runtime so rela.url is
// registered on the rela table. If unset, rela.url is absent (same pattern
// as rela.cache), and any call from Lua raises "attempt to call a nil value".
func WithRouteCatalog(c RouteCatalog) Option {
	return func(r *Runtime) {
		r.routes = c
	}
}
