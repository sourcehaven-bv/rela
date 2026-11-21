//go:build postgres

package tenant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Apply makes the tenant API's change to orgID in one transaction and
// returns the row as it is afterwards (TKT-59HWQ76).
//
// A state change follows these rules:
//   - active and read_only serve an active tenant with writes allowed or
//     refused. They resume a tenant the tenant API suspended; one an
//     operator or the IdP suspended is refused with [ErrHeldSuspended].
//   - suspended stops serving an active tenant and records the tenant API
//     as the cause. A tenant that is already suspended stays so and keeps
//     its cause.
//   - A provisioning tenant is refused with an error that wraps
//     [ErrTenantProvisioning]; the caller retries once the create is done.
//
// Plan and entitlements may change on a provisioning, active or suspended
// tenant. Any change to an archived tenant is refused with [ErrArchived],
// and one to an erasing or erased tenant with [ErrWrongStatus].
func (p *Provisioner) Apply(ctx context.Context, orgID string, c Change) (Record, error) {
	if err := c.Validate(); err != nil {
		return Record{}, err
	}
	var out Record
	err := p.withRecord(ctx, orgID, func(ctx context.Context, conn *pgx.Conn, rec Record) error {
		return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			cur, err := lockRecord(ctx, tx, rec)
			if err != nil {
				return err
			}
			if aerr := applyChange(ctx, tx, cur, c); aerr != nil {
				return aerr
			}
			out, err = lockRecord(ctx, tx, rec)
			return err
		})
	})
	return out, err
}

// lockRecord reads rec's row again inside tx and locks it until tx ends.
func lockRecord(ctx context.Context, tx pgx.Tx, rec Record) (Record, error) {
	var locked string
	err := tx.QueryRow(ctx, "SELECT org_id FROM "+controlTable+" WHERE schema_name = $1 FOR UPDATE",
		rec.Schema).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && locked != rec.OrgID) {
		return Record{}, fmt.Errorf("%w: %q", ErrUnknownTenant, rec.OrgID)
	}
	if err != nil {
		return Record{}, fmt.Errorf("tenant: lock org_id %q: %w", rec.OrgID, err)
	}
	cur, _, err := queryRecord(ctx, tx, rec.Schema)
	if err != nil {
		return Record{}, fmt.Errorf("tenant: look up org_id %q: %w", rec.OrgID, err)
	}
	return cur, nil
}

// applyChange writes c to cur's row, which the caller has locked.
func applyChange(ctx context.Context, tx pgx.Tx, cur Record, c Change) error {
	if cur.Archived() {
		return fmt.Errorf("%w: org_id %q", ErrArchived, cur.OrgID)
	}
	switch cur.Status {
	case StatusProvisioning, StatusActive, StatusSuspended:
	default:
		return fmt.Errorf("%w: org_id %q is %s", ErrWrongStatus, cur.OrgID, cur.Status)
	}
	if c.State != nil {
		if err := applyState(ctx, tx, cur, *c.State); err != nil {
			return err
		}
	}
	if c.Plan != nil {
		var plan *string
		if *c.Plan != "" {
			plan = c.Plan
		}
		if _, err := tx.Exec(ctx, "UPDATE "+controlTable+" SET plan = $1, updated_at = now() WHERE schema_name = $2",
			plan, cur.Schema); err != nil {
			return fmt.Errorf("tenant: org_id %q: set plan: %w", cur.OrgID, err)
		}
	}
	if c.Entitlements != nil {
		var raw []byte
		if len(*c.Entitlements) > 0 {
			var err error
			if raw, err = json.Marshal(*c.Entitlements); err != nil {
				return fmt.Errorf("tenant: org_id %q: entitlements: %w", cur.OrgID, err)
			}
		}
		if _, err := tx.Exec(ctx, "UPDATE "+controlTable+" SET entitlements = $1::jsonb, updated_at = now() "+
			"WHERE schema_name = $2", raw, cur.Schema); err != nil {
			return fmt.Errorf("tenant: org_id %q: set entitlements: %w", cur.OrgID, err)
		}
	}
	return nil
}

// applyState moves cur's locked row to state, by the rules on
// [Provisioner.Apply].
func applyState(ctx context.Context, tx pgx.Tx, cur Record, state AccessState) error {
	if cur.Status == StatusProvisioning {
		return fmt.Errorf("%w: %w: org_id %q is still being created", ErrWrongStatus, ErrTenantProvisioning, cur.OrgID)
	}
	var query string
	var args []any
	switch {
	case state == StateSuspended && cur.Status == StatusSuspended:
		return nil
	case state == StateSuspended:
		query = "status = $2, suspended_by = $3"
		args = []any{cur.Schema, string(StatusSuspended), string(SuspendedByAPI)}
	case cur.Status == StatusSuspended && cur.SuspendedBy != SuspendedByAPI:
		return fmt.Errorf("%w: org_id %q (suspended by %s)", ErrHeldSuspended, cur.OrgID, causeName(cur.SuspendedBy))
	default:
		query = "status = $2, suspended_by = NULL, read_only = $3"
		args = []any{cur.Schema, string(StatusActive), state == StateReadOnly}
	}
	_, err := tx.Exec(ctx, "UPDATE "+controlTable+" SET "+query+", updated_at = now() WHERE schema_name = $1", args...)
	if err != nil {
		return fmt.Errorf("tenant: org_id %q: set state %s: %w", cur.OrgID, state, err)
	}
	return nil
}

// causeName names a suspension cause for an error; a suspension recorded
// before causes were is an operator's.
func causeName(c SuspendCause) string {
	if c == "" {
		return "an operator, before causes were recorded"
	}
	return string(c)
}

// Archive stops serving orgID and keeps its data, on the tenant API's
// DELETE. It reports whether the row changed: archiving an archived tenant
// is a no-op. An active or suspended tenant is archived, whoever suspended
// it; any other status is refused with [ErrWrongStatus].
//
// An archived tenant is suspended with the cause archive and an
// archived_at time. The tenant API cannot undo it; an operator resumes it
// with `rela-server tenant resume`, or erases it.
func (p *Provisioner) Archive(ctx context.Context, orgID string) (Record, bool, error) {
	var out Record
	var changed bool
	err := p.withRecord(ctx, orgID, func(ctx context.Context, conn *pgx.Conn, rec Record) error {
		return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			cur, err := lockRecord(ctx, tx, rec)
			if err != nil {
				return err
			}
			if cur.Archived() {
				out = cur
				return nil
			}
			if cur.Status != StatusActive && cur.Status != StatusSuspended {
				return fmt.Errorf("%w: org_id %q is %s", ErrWrongStatus, orgID, cur.Status)
			}
			if _, err = tx.Exec(ctx, "UPDATE "+controlTable+" SET status = $1, suspended_by = $2, "+
				"archived_at = now(), updated_at = now() WHERE schema_name = $3",
				string(StatusSuspended), string(SuspendedByArchive), cur.Schema); err != nil {
				return fmt.Errorf("tenant: org_id %q: archive: %w", orgID, err)
			}
			changed = true
			out, err = lockRecord(ctx, tx, rec)
			return err
		})
	})
	return out, changed, err
}

// usageQuery reads a tenant schema's users and storage. %[1]s is the quoted
// schema. An empty user type counts no users. Entities are counted by id,
// so the faces of one entity count once.
const usageQuery = `SELECT
	(SELECT count(DISTINCT id) FROM %[1]s.entities WHERE $1 <> '' AND type = $1),
	coalesce((SELECT sum(pg_total_relation_size(c.oid)) FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $2 AND c.relkind IN ('r', 'm', 'p')), 0)::bigint`

// activeUsersQuery counts the people who changed something since $1. The
// audit log stores `at` in microseconds since the epoch.
const activeUsersQuery = `SELECT count(DISTINCT principal_user) FROM %[1]s.audit_log
	WHERE at >= $1 AND principal_user <> '' AND principal_user NOT LIKE 'system:%%'`

// usage reads schema's usage over the control pool. A schema whose
// migrations have not made the audit log yet counts no active users: the
// audit query is only sent when the table exists, because PostgreSQL
// resolves every table a query names before it runs.
func (c pgControl) usage(ctx context.Context, schema, userType string, since time.Time) (Usage, error) {
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	quoted := pgx.Identifier{schema}.Sanitize()
	var u Usage
	if err := c.pool.QueryRow(ctx, fmt.Sprintf(usageQuery, quoted), userType, schema).
		Scan(&u.Users, &u.StorageBytes); err != nil {
		return Usage{}, err
	}
	var hasAudit bool
	if err := c.pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", quoted+".audit_log").
		Scan(&hasAudit); err != nil {
		return Usage{}, err
	}
	if !hasAudit {
		return u, nil
	}
	if err := c.pool.QueryRow(ctx, fmt.Sprintf(activeUsersQuery, quoted), since.UnixMicro()).
		Scan(&u.ActiveUsers); err != nil {
		return Usage{}, err
	}
	return u, nil
}

// decodeEntitlements reads the control table's `entitlements` column. NULL
// is no entitlements.
func decodeEntitlements(raw []byte) (Entitlements, error) {
	var e Entitlements
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return e, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&e); err != nil {
		return nil, err
	}
	return e, nil
}
