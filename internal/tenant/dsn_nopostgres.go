//go:build !postgres

package tenant

import "errors"

// dsnForSchema refuses to derive a per-tenant DSN on a build with no PostgreSQL.
//
// Split from the postgres implementation for the reason `pgstore` is:
// deriving a DSN needs `pgx.ParseConfig` to handle both DSN dialects safely,
// and importing pgx here unconditionally would link it into the default and
// memory builds. CI asserts it is absent from those binaries, and the isolation
// is worth keeping even while nothing wires this package yet — the alternative
// is a violation that only appears once a host does.
//
// It errors rather than returning the base DSN unchanged, which is the
// fail-closed choice: an unpinned DSN would put every tenant on the same
// `search_path` and hand them each other's data. Refusing to boot is the only
// safe way to be wrong here.
//
// A deployment on these builds is single-store by construction (fsstore or
// memstore), so a tenant map is meaningless rather than merely unsupported. A
// tenant carrying an explicit DSN never reaches this function.
func dsnForSchema(_, _ string) (string, error) {
	return "", errors.New(
		"tenant: deriving a per-tenant DSN requires the postgres build; " +
			"schema-per-tenant has no meaning for the filesystem or memory stores")
}
