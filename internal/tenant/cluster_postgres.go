//go:build postgres

package tenant

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// controlPoolSize caps the resolver's pool. Each request does one short
// primary-key lookup, so a few connections serve many tenants' requests.
const controlPoolSize = 4

// controlTable is the qualified name of the control table. Every statement
// names it qualified: no connection's `search_path` includes [ControlSchema].
const controlTable = ControlSchema + ".tenants"

// controlDDL creates the control table. Only [Provisioner] runs it; a server
// that finds the table missing refuses to start rather than create it.
const controlDDL = `CREATE SCHEMA IF NOT EXISTS ` + ControlSchema + `;
CREATE TABLE IF NOT EXISTS ` + controlTable + ` (
	schema_name  text PRIMARY KEY,
	org_id       text NOT NULL UNIQUE,
	status       text NOT NULL CONSTRAINT ` + statusCheck + ` CHECK (status IN ` + statusValues + `),
	suspended_by text CHECK (suspended_by IN ('operator', 'idp')),
	erased_at    timestamptz,
	created_at   timestamptz NOT NULL DEFAULT now(),
	updated_at   timestamptz NOT NULL DEFAULT now()
)`

// statusCheck names the CHECK constraint on `status`. PostgreSQL gives an
// unnamed column constraint the same name, so a table an older release made
// has it too.
const statusCheck = "tenants_status_check"

// statusValues lists every [Status] the control table accepts.
const statusValues = `('provisioning', 'active', 'suspended', 'erasing', 'erased')`

// controlUpgradeDDL brings a control table made by an older release up to
// this one: it adds the missing columns and widens the status check to
// allow erased. Each statement is idempotent, and they run in one
// transaction, so a table with `erased_at` always has the wide check.
// [ensureControl] runs it only when a column is missing, because ALTER
// TABLE locks the table even when it changes nothing.
//
// It drops every CHECK constraint on `status`, whatever its name, before
// adding the wide one, so an old narrow check cannot survive beside it.
const controlUpgradeDDL = `ALTER TABLE ` + controlTable + `
	ADD COLUMN IF NOT EXISTS suspended_by text CHECK (suspended_by IN ('operator', 'idp')),
	ADD COLUMN IF NOT EXISTS erased_at timestamptz;
DO $$
DECLARE c name;
BEGIN
	FOR c IN SELECT con.conname FROM pg_constraint con
		JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = ANY (con.conkey)
		WHERE con.conrelid = '` + controlTable + `'::regclass AND con.contype = 'c' AND a.attname = 'status'
	LOOP
		EXECUTE format('ALTER TABLE ` + controlTable + ` DROP CONSTRAINT %I', c);
	END LOOP;
END $$;
ALTER TABLE ` + controlTable + ` ADD CONSTRAINT ` + statusCheck + ` CHECK (status IN ` + statusValues + `)`

// controlCurrentQuery reports whether the control table has every column
// this release writes. `erased_at` is the newest, and [controlUpgradeDDL]
// adds it in the same transaction as the others and the status check.
const controlCurrentQuery = `SELECT count(*) = 2 FROM pg_attribute
	WHERE attrelid = to_regclass($1) AND attname IN ('suspended_by', 'erased_at') AND NOT attisdropped`

// recordQuery reads one control row and whether its schema exists. It reads
// `suspended_by` and `erased_at` through to_jsonb, which yields NULL for a
// column that does not exist, so a server reads a control table no
// provisioner of this release has upgraded yet.
const recordQuery = `SELECT t.org_id, t.status, coalesce(to_jsonb(t)->>'suspended_by', ''),
	(to_jsonb(t)->>'erased_at')::timestamptz,
	EXISTS (SELECT 1 FROM pg_namespace n WHERE n.nspname = t.schema_name)
	FROM ` + controlTable + ` t WHERE t.schema_name = $1`

// rowQuerier is the one method [queryRecord] needs; a pool and a connection
// both have it.
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// queryRecord reads schema's control row through q.
func queryRecord(ctx context.Context, q rowQuerier, schema string) (Record, bool, error) {
	rec := Record{Schema: schema}
	var status, suspendedBy string
	var erasedAt *time.Time
	err := q.QueryRow(ctx, recordQuery, schema).Scan(&rec.OrgID, &status, &suspendedBy, &erasedAt, &rec.SchemaExists)
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, err
	}
	rec.Status = Status(status)
	rec.SuspendedBy = SuspendCause(suspendedBy)
	if erasedAt != nil {
		rec.ErasedAt = *erasedAt
	}
	return rec, true, nil
}

// pgControl reads the control table over a pool on the base DSN.
type pgControl struct{ pool *pgxpool.Pool }

// lookupTimeout bounds one control-table lookup, so a hung connection
// cannot stall a request or a revalidation pass indefinitely.
const lookupTimeout = 5 * time.Second

func (c pgControl) lookup(ctx context.Context, schema string) (Record, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	return queryRecord(ctx, c.pool, schema)
}

func (c pgControl) ready(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, schemaCheckTimeout)
	defer cancel()
	var exists bool
	if err := c.pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", controlTable).Scan(&exists); err != nil {
		// pgx redacts the password in connect errors.
		return fmt.Errorf("tenant: read the control table: %w", err)
	}
	if !exists {
		return errors.New("tenant: this database has no tenants yet (" + controlTable + " does not exist); " +
			"create one with `rela-server tenant create ORG_ID`")
	}
	return nil
}

// poolCloser closes a pool through io.Closer.
type poolCloser struct{ pool *pgxpool.Pool }

func (p poolCloser) Close() error {
	p.pool.Close()
	return nil
}

// NewClusterResolver builds the production resolver over baseDSN, which is
// $RELA_DATABASE_URL. It does not connect until the first lookup; call
// [ClusterResolver.Check] to find a broken database at startup. Close the
// resolver when done.
func NewClusterResolver(ctx context.Context, baseDSN string) (*ClusterResolver, error) {
	if baseDSN == "" {
		return nil, errors.New("tenant: tenant mode needs the base DSN in $RELA_DATABASE_URL")
	}
	if _, err := dsnForSchema(baseDSN, schemaPrefix+"check"); err != nil {
		return nil, fmt.Errorf("tenant: $RELA_DATABASE_URL: %w", err)
	}
	cfg, err := pgxpool.ParseConfig(baseDSN)
	if err != nil {
		// pgx redacts the password in parse errors.
		return nil, fmt.Errorf("tenant: parse $RELA_DATABASE_URL: %w", err)
	}
	cfg.MaxConns = controlPoolSize
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("tenant: control pool: %w", err)
	}
	return newClusterResolver(baseDSN, pgControl{pool: pool}, poolCloser{pool: pool}), nil
}
