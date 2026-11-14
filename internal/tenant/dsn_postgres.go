//go:build postgres

package tenant

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// dsnForSchema returns base with `search_path` pinned to schema, which is what
// scopes a tenant to its own data.
//
// This is the entire isolation mechanism, so it is worth being precise about
// why it is enough. rela's PostgreSQL SQL is unqualified — `FROM entities`, not
// `FROM tenant_a.entities` — so every table reference resolves through the
// connection's `search_path`. Pinning it in the DSN means a store opened for
// tenant A cannot name tenant B's tables even in principle: the attempt is a
// "relation does not exist" error from PostgreSQL, not a row-filtering decision
// rela could get wrong. It is also why `pgstore` needs no changes for
// multi-tenancy — it never learns it is multi-tenant. [pinSchema] checks at
// open that the schema exists, since PostgreSQL silently skips a missing one.
//
// [pgstore.PinSearchPath] keeps the base string and changes only
// `search_path`, so TLS, pool and connection settings survive (BUG-BR9CXF).
//
// Only the postgres build has a DSN to derive; see the stub in
// dsn_nopostgres.go.
func dsnForSchema(base, schema string) (string, error) {
	dsn, err := pgstore.PinSearchPath(base, schema)
	if err != nil {
		return "", fmt.Errorf("base_dsn: %w", err)
	}
	return dsn, nil
}

// schemaCheckTimeout bounds [pinSchema], which runs on a tenant's cold
// open.
const schemaCheckTimeout = 30 * time.Second

// DescribeTarget names the database dsn connects to, as host, port,
// database and user, for an operator confirming an erasure. It never
// includes the password. An unparseable DSN is described as such.
func DescribeTarget(dsn string) string {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return "an unparseable DSN"
	}
	return fmt.Sprintf("database %q on %s:%d as user %q", cfg.Database, cfg.Host, cfg.Port, cfg.User)
}

// Supported reports whether this build can serve tenant mode. Only the
// postgres store is addressed by a DSN, so only it can open one store per
// tenant.
const Supported = true
