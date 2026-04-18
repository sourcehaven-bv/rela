package store

import "github.com/Sourcehaven-BV/rela/internal/metamodel"

// Factory constructs Store instances on demand.
//
// Callers (workspace, dataentry, ...) depend on a Factory rather than on
// a concrete backend so the choice of backend (filesystem, in-memory,
// remote, caching wrapper, ...) stays pluggable at the process
// boundary.
//
// OpenStore returns a ready-to-use Store configured for the given
// metamodel. The caller owns the returned store and must Close it when
// done. Implementations may choose to cache or pool stores; in that
// case Close semantics are documented on the implementation.
type Factory interface {
	OpenStore(meta *metamodel.Metamodel) (Store, error)
}
