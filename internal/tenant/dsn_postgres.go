//go:build postgres

package tenant

import (
	"context"
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
// multi-tenancy — it never learns it is multi-tenant. [verifySchema] checks at
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
	return out, nil
}

// schemaCheckTimeout bounds [verifySchema], which runs on a tenant's cold
// open.
const schemaCheckTimeout = 30 * time.Second

// verifySchema connects with dsn and checks that the connection's
// current_schema() is schema.
//
// PostgreSQL skips a `search_path` entry that names no existing schema. A
// tenant listed before its schema was created would then land in `public`,
// next to every other such tenant, and pgstore's migrations would create its
// tables there. An explicit `dsn:` that sets no `search_path`, or another
// one, lands in the wrong schema the same way. Both look like a working
// deployment. So the opener refuses any tenant whose connection does not
// land in its own schema, before migrations run.
func verifySchema(ctx context.Context, dsn, schema string) error {
	ctx, cancel := context.WithTimeout(ctx, schemaCheckTimeout)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("check schema: %w", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	var got *string
	if err := conn.QueryRow(ctx, "SELECT current_schema()").Scan(&got); err != nil {
		return fmt.Errorf("check schema: %w", err)
	}
	if got == nil || *got != schema {
		landed := "no schema"
		if got != nil {
			landed = fmt.Sprintf("schema %q", *got)
		}
		return fmt.Errorf("the connection lands in %s, not %q: create the schema first, "+
			"and an explicit dsn must set search_path to it", landed, schema)
	}
	return nil
}
