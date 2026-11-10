//go:build postgres

package tenant

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
)

// tenantLockClass is the first key of the advisory lock that guards a
// tenant's schema; the second is hashtext(schema). The two-key form keeps
// these locks apart from the single-key locks the store takes. Both sides
// try the lock and never wait, so a hash collision between two schemas makes
// one of them refuse while the other holds it; it never lets a drop through.
const tenantLockClass int32 = 0x72656c61 // "rela"

// eraseTimeout bounds one erasure. A drop waits for no transaction of an
// open tenant, since none can be open past the lock, so this only guards
// against a database that stopped answering.
const eraseTimeout = 5 * time.Minute

// eraseLockTimeout bounds how long the drop waits for a table lock someone
// outside rela holds, such as an operator's psql session.
const eraseLockTimeout = "30s"

// pinSchema connects with dsn, takes the tenant's lock shared and checks
// that the connection lands in schema. The returned closer releases the lock
// by closing the connection, so the opener keeps it until the tenant closes.
//
// The lock comes first. Taken after the check, a drop could run between the
// two and the tenant would open over a schema that is gone.
//
// PostgreSQL skips a `search_path` entry that names no existing schema. A
// tenant listed before its schema was created, or after it was erased, would
// then land in `public`, and pgstore's migrations would create its tables
// there. An explicit `dsn:` that sets no `search_path`, or another one, lands
// in the wrong schema the same way. Both look like a working deployment, so
// a tenant whose connection does not land in its own schema is refused
// before migrations run.
func pinSchema(ctx context.Context, dsn, schema string) (io.Closer, error) {
	ctx, cancel := context.WithTimeout(ctx, schemaCheckTimeout)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("check schema: %w", err)
	}
	keep := false
	defer func() {
		if !keep {
			_ = conn.Close(context.WithoutCancel(ctx))
		}
	}()
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock_shared($1, hashtext($2))",
		tenantLockClass, schema).Scan(&locked); err != nil {
		return nil, fmt.Errorf("check schema: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("schema %q is being erased", schema)
	}
	var got *string
	if err := conn.QueryRow(ctx, "SELECT current_schema()").Scan(&got); err != nil {
		return nil, fmt.Errorf("check schema: %w", err)
	}
	if got == nil || *got != schema {
		landed := "no schema"
		if got != nil {
			landed = fmt.Sprintf("schema %q", *got)
		}
		return nil, fmt.Errorf("the connection lands in %s, not %q: create the schema first, "+
			"and an explicit dsn must set search_path to it", landed, schema)
	}
	keep = true
	return pinnedConn{conn}, nil
}

// pinnedConn is the connection that holds a tenant's lock shared.
type pinnedConn struct{ conn *pgx.Conn }

func (p pinnedConn) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), schemaCheckTimeout)
	defer cancel()
	return p.conn.Close(ctx)
}

// dropSchema drops schema and everything in it, holding the tenant's lock
// exclusively. It reports whether the schema existed.
//
// The connection uses the tenant's own DSN, so it reaches the database the
// tenant lives in, whatever cluster that is. The name is quoted as an
// identifier rather than trusted to the pattern alone.
//
// The lock covers every server that opened the tenant through the registry,
// for as long as the connection holding it lives. A connection can die
// without the server noticing, and then a server that was not restarted
// still has pools on `search_path=<schema>,public`. After the drop their
// queries resolve in `public`. So the drop is refused while `public` holds
// any table: then such a server gets errors, never rows from another
// schema.
func dropSchema(ctx context.Context, dsn, schema string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, eraseTimeout)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		// pgx redacts the password in connect errors.
		return false, fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1, hashtext($2))",
		tenantLockClass, schema).Scan(&locked); err != nil {
		return false, fmt.Errorf("lock: %w", err)
	}
	if !locked {
		return false, ErrTenantInUse
	}
	var publicTables bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p', 'v', 'm', 'f'))`).Scan(&publicTables); err != nil {
		return false, fmt.Errorf("check public: %w", err)
	}
	if publicTables {
		return false, ErrPublicHasTables
	}
	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)",
		schema).Scan(&exists); err != nil {
		return false, fmt.Errorf("look up schema: %w", err)
	}
	if !exists {
		return false, nil
	}
	if _, err := conn.Exec(ctx, "SET lock_timeout = '"+eraseLockTimeout+"'"); err != nil {
		return false, fmt.Errorf("set lock_timeout: %w", err)
	}
	if _, err := conn.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
		return false, fmt.Errorf("drop schema: %w", err)
	}
	return true, nil
}

// ErrLooksLikeTenantDatabase reports a database command run without a
// tenant map against a database that already serves tenants.
var ErrLooksLikeTenantDatabase = errors.New("tenant: this database already holds rela data in other " +
	"schemas, as a tenant deployment does, but none in the schema this connection lands in; run the " +
	"command from the project directory and name the tenant with --tenant")

// CheckBareTarget guards a database command that found no tenant map, such
// as `rela db migrate` run outside the project directory. In a tenant
// deployment $RELA_DATABASE_URL is the base DSN, which sets no tenant
// `search_path`, so such a command would create rela's tables in `public`.
//
// It returns [ErrLooksLikeTenantDatabase] when the connection's schema holds
// no rela tables but another schema of the database does. A single-store
// database, already migrated or still empty, passes.
func CheckBareTarget(ctx context.Context, dsn string) error {
	ctx, cancel := context.WithTimeout(ctx, schemaCheckTimeout)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("check database: %w", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	var here, elsewhere bool
	if err := conn.QueryRow(ctx, `SELECT
		    coalesce(bool_or(schemaname = current_schema()), false),
		    coalesce(bool_or(schemaname IS DISTINCT FROM current_schema()), false)
		  FROM pg_tables WHERE tablename = 'schema_version'`).Scan(&here, &elsewhere); err != nil {
		return fmt.Errorf("check database: %w", err)
	}
	if !here && elsewhere {
		return ErrLooksLikeTenantDatabase
	}
	return nil
}
