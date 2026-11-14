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

// ensureControl creates the control schema and table if missing, and adds
// the columns an older release's table lacks, under a transaction-scoped
// lock so two first creates do not race the DDL.
func ensureControl(ctx context.Context, conn *pgx.Conn) error {
	return controlDDLTx(ctx, conn, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, controlDDL); err != nil {
			return err
		}
		return upgradeIfOld(ctx, tx)
	})
}

// upgradeControl adds the columns an older release's control table lacks.
// The table exists. It checks first and alters only when a column is
// missing, so the common case takes no table lock.
func upgradeControl(ctx context.Context, conn *pgx.Conn) error {
	var current bool
	if err := conn.QueryRow(ctx, controlCurrentQuery, controlTable).Scan(&current); err != nil {
		return fmt.Errorf("control table: %w", err)
	}
	if current {
		return nil
	}
	return controlDDLTx(ctx, conn, func(tx pgx.Tx) error { return upgradeIfOld(ctx, tx) })
}

// upgradeIfOld runs [controlUpgradeDDL] on tx when a column is missing.
func upgradeIfOld(ctx context.Context, tx pgx.Tx) error {
	var current bool
	if err := tx.QueryRow(ctx, controlCurrentQuery, controlTable).Scan(&current); err != nil {
		return err
	}
	if current {
		return nil
	}
	_, err := tx.Exec(ctx, controlUpgradeDDL)
	return err
}

// controlDDLTx runs fn in a transaction that holds the control table's DDL
// lock.
func controlDDLTx(ctx context.Context, conn *pgx.Conn, fn func(pgx.Tx) error) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("control table: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1, hashtext($2))",
		controlLockClass, ControlSchema); err != nil {
		return fmt.Errorf("control table: %w", err)
	}
	if err := fn(tx); err != nil {
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
	if uerr := upgradeControl(ctx, conn); uerr != nil {
		return Record{}, uerr
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
// [ErrSchemaExists] unless opts.Adopt is set. Adopting is for a schema an
// operator renamed to the derived name on purpose, such as a tenant from a
// deployment that still used a tenant map file.
//
// An erased org's tombstone is refused with [ErrTenantErased] unless
// opts.Recreate is set. Recreating makes a fresh, empty tenant from the
// tombstone; it is an operator's act, and the tenant host never asks for it.
func (p *Provisioner) Create(ctx context.Context, orgID string, opts CreateOptions) (Tenant, error) {
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
	if err := p.reserve(ctx, conn, t, opts); err != nil {
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
	if err := setStatus(ctx, conn, t, StatusActive, "", StatusProvisioning); err != nil {
		return Tenant{}, fmt.Errorf("tenant: create org_id %q: %w", orgID, err)
	}
	return t, nil
}

// reserve records t as provisioning, or accepts an existing provisioning
// row for the same org, which is a create that failed part way. With
// opts.Recreate it turns an erased row back into a provisioning one.
func (p *Provisioner) reserve(ctx context.Context, conn *pgx.Conn, t Tenant, opts CreateOptions) error {
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
	case found && rec.Status == StatusErased && !opts.Recreate:
		return fmt.Errorf("%w: org_id %q; an operator recreates it with `rela-server tenant create -recreate`",
			ErrTenantErased, t.OrgID)
	case found && rec.Status != StatusErased:
		return &ExistsError{OrgID: t.OrgID, Status: rec.Status}
	}
	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)",
		t.Schema).Scan(&exists); err != nil {
		return fmt.Errorf("tenant: create org_id %q: look up schema: %w", t.OrgID, err)
	}
	if exists && !opts.Adopt {
		return fmt.Errorf("%w: schema %q for org_id %q", ErrSchemaExists, t.Schema, t.OrgID)
	}
	if found {
		// The erased row, recreated: a compare-and-set, so a concurrent
		// recreate makes this one fail rather than both proceed.
		tag, err := conn.Exec(ctx, "UPDATE "+controlTable+" SET status = $1, suspended_by = NULL, erased_at = NULL, "+
			"created_at = now(), updated_at = now() WHERE schema_name = $2 AND org_id = $3 AND status = $4",
			string(StatusProvisioning), t.Schema, t.OrgID, string(StatusErased))
		if err != nil {
			return fmt.Errorf("tenant: create org_id %q: recreate: %w", t.OrgID, err)
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("%w: org_id %q is no longer erased", ErrWrongStatus, t.OrgID)
		}
		return nil
	}
	// No ON CONFLICT: a row inserted meanwhile is an error, never merged.
	if _, err := conn.Exec(ctx, "INSERT INTO "+controlTable+" (schema_name, org_id, status) VALUES ($1, $2, $3)",
		t.Schema, t.OrgID, string(StatusProvisioning)); err != nil {
		return fmt.Errorf("tenant: create org_id %q: record: %w", t.OrgID, err)
	}
	return nil
}

// Suspend stops serving orgID on an operator's command. Servers refuse its
// next request, and close it within their revalidation interval.
// Suspending a suspended tenant is a no-op, except that it records the
// operator as the cause: the IdP then cannot resume it.
func (p *Provisioner) Suspend(ctx context.Context, orgID string) error {
	return p.transition(ctx, orgID, StatusSuspended, SuspendedByOperator, StatusActive, StatusSuspended)
}

// Resume serves a suspended orgID again, whoever suspended it. Only a
// suspended tenant can be resumed: an erasing one is on its way out, and a
// provisioning one is finished by running Create again.
func (p *Provisioner) Resume(ctx context.Context, orgID string) error {
	return p.transition(ctx, orgID, StatusActive, "", StatusSuspended)
}

// SuspendForIDP suspends orgID because the IdP suspended its org. It reports
// whether the status changed. A tenant that is already suspended stays so
// and keeps its recorded cause, so an operator suspension stays one. An
// erasing or erased tenant is left alone. A provisioning tenant is refused with an
// error that wraps [ErrTenantProvisioning]: its create would make it
// active, so the caller retries once the create is done.
func (p *Provisioner) SuspendForIDP(ctx context.Context, orgID string) (bool, error) {
	var changed bool
	err := p.withRecord(ctx, orgID, func(ctx context.Context, conn *pgx.Conn, rec Record) error {
		tag, err := conn.Exec(ctx, "UPDATE "+controlTable+" SET status = $1, suspended_by = $2, updated_at = now() "+
			"WHERE schema_name = $3 AND org_id = $4 AND status = $5",
			string(StatusSuspended), string(SuspendedByIDP), rec.Schema, orgID, string(StatusActive))
		if err != nil {
			return fmt.Errorf("tenant: org_id %q: suspend: %w", orgID, err)
		}
		if changed = tag.RowsAffected() == 1; changed {
			return nil
		}
		now, err := p.reread(ctx, conn, rec)
		if err != nil || now.Status != StatusProvisioning {
			return err
		}
		return fmt.Errorf("%w: %w: org_id %q is still being created", ErrWrongStatus, ErrTenantProvisioning, orgID)
	})
	return changed, err
}

// ResumeForIDP serves orgID again because the IdP reactivated its org. It
// reports whether the status changed. Only a tenant the IdP suspended is
// resumed: one an operator suspended is refused with
// [ErrOperatorSuspended], and so is one suspended before the cause was
// recorded. Any other status is left alone.
func (p *Provisioner) ResumeForIDP(ctx context.Context, orgID string) (bool, error) {
	var changed bool
	err := p.withRecord(ctx, orgID, func(ctx context.Context, conn *pgx.Conn, rec Record) error {
		tag, err := conn.Exec(ctx, "UPDATE "+controlTable+" SET status = $1, suspended_by = NULL, updated_at = now() "+
			"WHERE schema_name = $2 AND org_id = $3 AND status = $4 AND suspended_by = $5",
			string(StatusActive), rec.Schema, orgID, string(StatusSuspended), string(SuspendedByIDP))
		if err != nil {
			return fmt.Errorf("tenant: org_id %q: resume: %w", orgID, err)
		}
		if changed = tag.RowsAffected() == 1; changed {
			return nil
		}
		now, err := p.reread(ctx, conn, rec)
		if err != nil || now.Status != StatusSuspended {
			return err
		}
		return fmt.Errorf("%w: org_id %q", ErrOperatorSuspended, orgID)
	})
	return changed, err
}

// reread reads rec's row again after a conditional update changed nothing,
// to tell why. A row that is gone is [ErrUnknownTenant].
func (p *Provisioner) reread(ctx context.Context, conn *pgx.Conn, rec Record) (Record, error) {
	now, found, err := queryRecord(ctx, conn, rec.Schema)
	switch {
	case err != nil:
		return Record{}, fmt.Errorf("tenant: look up org_id %q: %w", rec.OrgID, err)
	case !found || now.OrgID != rec.OrgID:
		return Record{}, fmt.Errorf("%w: %q", ErrUnknownTenant, rec.OrgID)
	}
	return now, nil
}

// transition sets orgID's status to to and its cause to by when its status
// is one of from.
func (p *Provisioner) transition(ctx context.Context, orgID string, to Status, by SuspendCause, from ...Status) error {
	return p.withRecord(ctx, orgID, func(ctx context.Context, conn *pgx.Conn, rec Record) error {
		return setStatus(ctx, conn, Tenant{OrgID: orgID, Schema: rec.Schema}, to, by, from...)
	})
}

// withRecord connects, reads orgID's control row and calls fn with it.
func (p *Provisioner) withRecord(ctx context.Context, orgID string,
	fn func(context.Context, *pgx.Conn, Record) error,
) error {
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
	return fn(ctx, conn, rec)
}

// setStatus sets t's status to to and its suspension cause to by, only when
// its row belongs to t's org and its status is one of from. An empty by
// clears the cause. Zero rows changed is [ErrWrongStatus].
func setStatus(ctx context.Context, conn *pgx.Conn, t Tenant, to Status, by SuspendCause, from ...Status) error {
	allowed := make([]string, len(from))
	for i, s := range from {
		allowed[i] = string(s)
	}
	var cause *string
	if by != "" {
		c := string(by)
		cause = &c
	}
	tag, err := conn.Exec(ctx, "UPDATE "+controlTable+" SET status = $1, suspended_by = $2, updated_at = now() "+
		"WHERE schema_name = $3 AND org_id = $4 AND status = ANY($5)",
		string(to), cause, t.Schema, t.OrgID, allowed)
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
// sets the status to erased and records erased_at. While a server still has
// the tenant open the drop is refused with [ErrTenantInUse]; servers close a
// tenant that is no longer active within their revalidation interval, and
// running Erase again continues where it stopped.
//
// The erased row is a tombstone (TKT-TNPRV8). It holds the org id, schema
// name, status and timestamps, and no tenant data. It stays so that a host
// that provisions automatically finds the org erased, not absent, and does
// not recreate it.
//
// Erasing an erased tenant succeeds and changes nothing in the control
// table. It still runs the idempotent removal, so the result reports the
// schema and audit directory as already absent.
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
	if rec.Status == StatusErased {
		// Only the removal runs. It finds nothing on the host that erased
		// the tenant, and removes the audit directory on any other host.
		res, rerr := Erase(ctx, t, p.auditRoot)
		return t, res, rerr
	}
	if err = setStatus(ctx, conn, t, StatusErasing, "",
		StatusProvisioning, StatusActive, StatusSuspended, StatusErasing); err != nil {
		return t, Erasure{}, fmt.Errorf("tenant: erase org_id %q: %w", orgID, err)
	}
	res, err := Erase(ctx, t, p.auditRoot)
	if err != nil {
		return t, res, err
	}
	if _, err := conn.Exec(ctx, "UPDATE "+controlTable+" SET status = $1, suspended_by = NULL, erased_at = now(), "+
		"updated_at = now() WHERE schema_name = $2 AND org_id = $3 AND status = $4",
		string(StatusErased), t.Schema, t.OrgID, string(StatusErasing)); err != nil {
		return t, res, fmt.Errorf("tenant: erase org_id %q: the data is gone, but the status was not set to "+
			"erased: %w; run the command again", orgID, err)
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
