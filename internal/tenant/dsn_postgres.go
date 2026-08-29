//go:build postgres

package tenant

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
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
// multi-tenancy — it never learns it is multi-tenant.
//
// Built by re-serializing the parsed config rather than appending
// "?search_path=" to the caller's string: pgx accepts both URL and key/value
// ("host=x dbname=y") DSNs, and query-string surgery silently produces garbage
// for the latter — a DSN that still parses, still connects, and lands the
// tenant on the wrong `search_path`. That failure is a cross-tenant leak that
// looks like a working deployment, which is why this mirrors
// `backendtest.dsnWithSearchPath` rather than reinventing it.
//
// `public` stays on the path because the search backend's `pg_trgm` operators
// live there. It holds no rela tables, so it is not a tenant-visible surface.
//
// Only the postgres build has a DSN to derive; see the stub in
// dsn_nopostgres.go.
func dsnForSchema(base, schema string) (string, error) {
	cfg, err := pgx.ParseConfig(base)
	if err != nil {
		// pgx redacts the password in parse errors, so this is safe to wrap.
		return "", fmt.Errorf("parse base_dsn: %w", err)
	}
	kv := []string{
		"host=" + cfg.Host,
		fmt.Sprintf("port=%d", cfg.Port),
		"user=" + cfg.User,
		"dbname=" + cfg.Database,
		"search_path=" + schema + ",public",
	}
	if cfg.Password != "" {
		kv = append(kv, "password="+cfg.Password)
	}
	// TLSConfig is nil exactly when the DSN disabled it; preserve that rather
	// than defaulting to on, which would make a plaintext cluster unreachable.
	if cfg.TLSConfig == nil {
		kv = append(kv, "sslmode=disable")
	}
	return strings.Join(kv, " "), nil
}
