//go:build postgres

package tenant

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

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
// multi-tenancy — it never learns it is multi-tenant. [pinSchema] checks at
// open that the schema exists, since PostgreSQL silently skips a missing one.
//
// The base string is kept and only `search_path` is set in it, in the
// dialect it is written in: a URL query parameter, or a trailing keyword for
// the key/value form ("host=x dbname=y"), where a later keyword overrides an
// earlier one. Rebuilding the DSN from parsed fields instead would drop every
// setting not copied across, such as `sslmode=verify-full` and its root
// certificate, and silently weaken TLS. The result is parsed again and
// refused unless it carries exactly the intended path.
//
// `public` stays on the path because the search backend's `pg_trgm` operators
// live there. It holds no rela tables, so it is not a tenant-visible surface.
//
// Only the postgres build has a DSN to derive; see the stub in
// dsn_nopostgres.go.
func dsnForSchema(base, schema string) (string, error) {
	if _, err := pgx.ParseConfig(base); err != nil {
		// pgx redacts the password in parse errors, so this is safe to wrap.
		return "", fmt.Errorf("parse base_dsn: %w", err)
	}
	searchPath := schema + ",public"
	var out string
	if strings.HasPrefix(base, "postgres://") || strings.HasPrefix(base, "postgresql://") {
		u, err := url.Parse(base)
		if err != nil {
			// Not wrapped: a url.Parse error quotes the whole URL,
			// password included.
			return "", errors.New("parse base_dsn: not a valid URL")
		}
		q := u.Query()
		q.Set("search_path", searchPath)
		u.RawQuery = q.Encode()
		out = u.String()
	} else {
		out = strings.TrimSpace(base) + " search_path='" + searchPath + "'"
	}
	cfg, err := pgx.ParseConfig(out)
	if err != nil {
		return "", fmt.Errorf("pin search_path: %w", err)
	}
	if got := cfg.RuntimeParams["search_path"]; got != searchPath {
		return "", fmt.Errorf("pin search_path: the DSN resolves to search_path %q, want %q", got, searchPath)
	}
	// PostgreSQL reads setting names without regard to case, and pgx sends
	// runtime parameters in no fixed order, so a second spelling of
	// search_path, or one inside options, could win on some connections.
	for k, v := range cfg.RuntimeParams {
		if (k != "search_path" && strings.EqualFold(k, "search_path")) ||
			(strings.EqualFold(k, "options") && strings.Contains(strings.ToLower(v), "search_path")) {

			return "", fmt.Errorf("pin search_path: the base DSN sets search_path through %q; remove it", k)
		}
	}
	return out, nil
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
