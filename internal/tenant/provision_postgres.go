//go:build postgres

package tenant

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// provisionTimeout bounds one lifecycle command. Creating a tenant runs every
// store migration, which on an empty schema takes seconds.
const provisionTimeout = 5 * time.Minute

// controlLockClass is the first key of the advisory lock that serializes the
// control table's DDL. It differs from tenantLockClass, so the DDL lock can
// never wait behind a server's lock on a tenant schema whose hash happens to
// equal the control schema's.
const controlLockClass int32 = 0x72656c63 // "relc"

// connect opens a connection on the base DSN. Only Create makes the control
// table ([ensureControl]); every other command finds no tenant without it.
func (p *Provisioner) connect(ctx context.Context) (*pgx.Conn, error) {
	conn, err := pgx.Connect(ctx, p.base)
	if err != nil {
		// pgx redacts the password in connect errors.
		return nil, fmt.Errorf("connect: %w", err)
	}
	return conn, nil
}

// controlExists reports whether the control table exists.
func controlExists(ctx context.Context, conn *pgx.Conn) (bool, error) {
	var exists bool
	if err := conn.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", controlTable).Scan(&exists); err != nil {
		return false, fmt.Errorf("control table: %w", err)
	}
	return exists, nil
}

// ensureControl creates the control schema and table if missing, under a
// transaction-scoped lock so two first creates do not race the DDL.
func ensureControl(ctx context.Context, conn *pgx.Conn) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("control table: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1, hashtext($2))",
		controlLockClass, ControlSchema); err != nil {
		return fmt.Errorf("control table: %w", err)
	}
	if _, err := tx.Exec(ctx, controlDDL); err != nil {
		return fmt.Errorf("control table: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("control table: %w", err)
	}
	return nil
}

// record reads orgID's row on conn and refuses a missing or colliding one,
// as [ClusterResolver.Lookup] does.
func (p *Provisioner) record(ctx context.Context, conn *pgx.Conn, orgID string) (Record, error) {
	schema, err := p.derive(orgID)
	if err != nil {
		return Record{}, err
	}
	exists, err := controlExists(ctx, conn)
	if err != nil {
		return Record{}, err
	}
	if !exists {
		return Record{}, fmt.Errorf("%w: %q", ErrUnknownTenant, orgID)
	}
	rec, found, err := queryRecord(ctx, conn, schema)
	if err != nil {
		return Record{}, fmt.Errorf("look up org_id %q: %w", orgID, err)
	}
	if !found {
		return Record{}, fmt.Errorf("%w: %q", ErrUnknownTenant, orgID)
	}
	if rec.OrgID != orgID {
		return Record{}, fmt.Errorf("%w: org_id %q derives schema %q, which is recorded for another org",
			ErrSchemaCollision, orgID, schema)
	}
	return rec, nil
}

// Create provisions orgID: it records the org as provisioning, creates its
// schema, checks that the tenant's DSN lands in it, runs the store
// migrations, and sets the status to active.
//
// It holds the tenant's lock exclusively throughout, so no server can open
// the tenant and no erasure can run until it is done. A create that failed
// part way leaves the row in provisioning, which is not served, and running
// Create again finishes it.
//
// A schema that already exists without a control row is refused with
// [ErrSchemaExists] unless adopt is set. Adopting is for a schema an
// operator renamed to the derived name on purpose, such as a tenant from a
// deployment that still used a tenant map file.
func (p *Provisioner) Create(ctx context.Context, orgID string, adopt bool) (Tenant, error) {
	// derive runs the same check, but reports an unknown organization;
	// checking first makes create say what is wrong with the org id.
	if err := checkOrgID(orgID); err != nil {
		return Tenant{}, err
	}
	schema, err := p.derive(orgID)
	if err != nil {
		return Tenant{}, err
	}
	dsn, err := dsnForSchema(p.base, schema)
	if err != nil {
		return Tenant{}, err
	}
	t := Tenant{OrgID: orgID, Schema: schema, DSN: dsn}
	ctx, cancel := context.WithTimeout(ctx, provisionTimeout)
	defer cancel()
	conn, err := p.connect(ctx)
	if err != nil {
		return Tenant{}, fmt.Errorf("tenant: create org_id %q: %w", orgID, err)
	}
	defer closeReleasing(ctx, conn)
	if err = ensureControl(ctx, conn); err != nil {
		return Tenant{}, fmt.Errorf("tenant: create org_id %q: %w", orgID, err)
	}

	if err := lockExclusive(ctx, conn, schema); err != nil {
		return Tenant{}, fmt.Errorf("tenant: create org_id %q: %w", orgID, err)
	}
	if err := p.reserve(ctx, conn, t, adopt); err != nil {
		return Tenant{}, err
	}
	if _, err := conn.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+pgx.Identifier{schema}.Sanitize()); err != nil {
		return Tenant{}, fmt.Errorf("tenant: create org_id %q: create schema: %w", orgID, err)
	}
	if err := checkLanding(ctx, dsn, schema); err != nil {
		return Tenant{}, fmt.Errorf("tenant: create org_id %q: %w", orgID, err)
	}
	if err := pgstore.MigrateDSN(ctx, dsn); err != nil {
		return Tenant{}, fmt.Errorf("tenant: create org_id %q: migrate: %w", orgID, err)
	}
	if err := setStatus(ctx, conn, t, StatusActive, StatusProvisioning); err != nil {
		return Tenant{}, fmt.Errorf("tenant: create org_id %q: %w", orgID, err)
	}
	return t, nil
}

// reserve records t as provisioning, or accepts an existing provisioning
// row for the same org, which is a create that failed part way.
func (p *Provisioner) reserve(ctx context.Context, conn *pgx.Conn, t Tenant, adopt bool) error {
	rec, found, err := queryRecord(ctx, conn, t.Schema)
	if err != nil {
		return fmt.Errorf("tenant: create org_id %q: look up: %w", t.OrgID, err)
	}
	switch {
	case found && rec.OrgID != t.OrgID:
		return fmt.Errorf("%w: org_id %q derives schema %q, which is recorded for another org",
			ErrSchemaCollision, t.OrgID, t.Schema)
	case found && rec.Status == StatusProvisioning:
		return nil
	case found:
		return fmt.Errorf("%w: org_id %q is %s", ErrTenantExists, t.OrgID, rec.Status)
	}
	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)",
		t.Schema).Scan(&exists); err != nil {
		return fmt.Errorf("tenant: create org_id %q: look up schema: %w", t.OrgID, err)
	}
	if exists && !adopt {
		return fmt.Errorf("%w: schema %q for org_id %q", ErrSchemaExists, t.Schema, t.OrgID)
	}
	// No ON CONFLICT: a row inserted meanwhile is an error, never merged.
	if _, err := conn.Exec(ctx, "INSERT INTO "+controlTable+" (schema_name, org_id, status) VALUES ($1, $2, $3)",
		t.Schema, t.OrgID, string(StatusProvisioning)); err != nil {
		return fmt.Errorf("tenant: create org_id %q: record: %w", t.OrgID, err)
	}
	return nil
}

// Suspend stops serving orgID. Servers refuse its next request, and close it
// within their revalidation interval. Suspending a suspended tenant is a
// no-op.
func (p *Provisioner) Suspend(ctx context.Context, orgID string) error {
	return p.transition(ctx, orgID, StatusSuspended, StatusActive, StatusSuspended)
}

// Resume serves a suspended orgID again. Only a suspended tenant can be
// resumed: an erasing one is on its way out, and a provisioning one is
// finished by running Create again.
func (p *Provisioner) Resume(ctx context.Context, orgID string) error {
	return p.transition(ctx, orgID, StatusActive, StatusSuspended)
}

// transition sets orgID's status to to when it is one of from.
func (p *Provisioner) transition(ctx context.Context, orgID string, to Status, from ...Status) error {
	ctx, cancel := context.WithTimeout(ctx, provisionTimeout)
	defer cancel()
	conn, err := p.connect(ctx)
	if err != nil {
		return fmt.Errorf("tenant: org_id %q: %w", orgID, err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	rec, err := p.record(ctx, conn, orgID)
	if err != nil {
		return err
	}
	t := Tenant{OrgID: orgID, Schema: rec.Schema}
	return setStatus(ctx, conn, t, to, from...)
}

// setStatus sets t's status to to, only when its row belongs to t's org and
// its status is one of from. Zero rows changed is [ErrWrongStatus].
func setStatus(ctx context.Context, conn *pgx.Conn, t Tenant, to Status, from ...Status) error {
	allowed := make([]string, len(from))
	for i, s := range from {
		allowed[i] = string(s)
	}
	tag, err := conn.Exec(ctx, "UPDATE "+controlTable+" SET status = $1, updated_at = now() "+
		"WHERE schema_name = $2 AND org_id = $3 AND status = ANY($4)",
		string(to), t.Schema, t.OrgID, allowed)
	if err != nil {
		return fmt.Errorf("set status %s: %w", to, err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("%w: org_id %q cannot become %s", ErrWrongStatus, t.OrgID, to)
	}
	return nil
}

// Erase removes all of orgID's data (TKT-TNERAS).
//
// It first sets the status to erasing, which stops every server serving the
// org; the status cannot be undone. It then drops the schema and removes the
// audit directory with [Erase], under the tenant's exclusive lock, and last
// deletes the control row. While a server still has the tenant open the
// drop is refused with [ErrTenantInUse]; servers close a tenant that is no
// longer active within their revalidation interval, and running Erase again
// continues where it stopped.
func (p *Provisioner) Erase(ctx context.Context, orgID string) (Tenant, Erasure, error) {
	ctx, cancel := context.WithTimeout(ctx, provisionTimeout)
	defer cancel()
	conn, err := p.connect(ctx)
	if err != nil {
		return Tenant{}, Erasure{}, fmt.Errorf("tenant: erase org_id %q: %w", orgID, err)
	}
	defer closeReleasing(ctx, conn)
	rec, err := p.record(ctx, conn, orgID)
	if err != nil {
		return Tenant{}, Erasure{}, err
	}
	dsn, err := dsnForSchema(p.base, rec.Schema)
	if err != nil {
		return Tenant{}, Erasure{}, err
	}
	t := Tenant{OrgID: orgID, Schema: rec.Schema, DSN: dsn}
	if err = setStatus(ctx, conn, t, StatusErasing,
		StatusProvisioning, StatusActive, StatusSuspended, StatusErasing); err != nil {
		return t, Erasure{}, fmt.Errorf("tenant: erase org_id %q: %w", orgID, err)
	}
	res, err := Erase(ctx, t, p.auditRoot)
	if err != nil {
		return t, res, err
	}
	if _, err := conn.Exec(ctx, "DELETE FROM "+controlTable+" WHERE schema_name = $1 AND org_id = $2 AND status = $3",
		t.Schema, t.OrgID, string(StatusErasing)); err != nil {
		return t, res, fmt.Errorf("tenant: erase org_id %q: the data is gone, but the control row was not "+
			"deleted: %w; run the command again", orgID, err)
	}
	return t, res, nil
}

// closeReleasing releases every advisory lock conn holds and closes it. A
// plain close only asks the server to end the session, which releases the
// session's locks some time after Close returns, so a command run right
// after this one could still find the tenant locked.
func closeReleasing(ctx context.Context, conn *pgx.Conn) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), schemaCheckTimeout)
	defer cancel()
	_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock_all()")
	_ = conn.Close(ctx)
}

// lockExclusive takes the tenant's lock exclusively on conn, until conn
// closes. It never waits: a held lock is [ErrTenantInUse].
func lockExclusive(ctx context.Context, conn *pgx.Conn, schema string) error {
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1, hashtext($2))",
		tenantLockClass, schema).Scan(&locked); err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	if !locked {
		return ErrTenantInUse
	}
	return nil
}

// checkLanding connects with dsn and checks that it lands in schema, so
// migrations never run in `public`. Unlike pinSchema it takes no lock: the
// caller holds the tenant's lock exclusively.
func checkLanding(ctx context.Context, dsn, schema string) error {
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
		return fmt.Errorf("the tenant connection does not land in schema %q", schema)
	}
	return nil
}
