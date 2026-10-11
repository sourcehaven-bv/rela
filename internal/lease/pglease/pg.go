// Package pglease is the PostgreSQL-backed [lease.Store].
//
// It exists because the KV backend keeps leases in one node-local document,
// so several rela-server processes against one database would each grant the
// same entity. Here a lease is a row (migration 0025) in the tenant's schema
// like every other table, and the primary key on the entity id makes one
// claim win across processes.
//
// # Relationship to the store
//
// This package does not depend on internal/store; a lease is not graph
// content. It shares the store's connection pool, injected by the composition
// root, as pgpiles does.
//
// # Whose clock
//
// Every expiry decision uses the caller's `now`, never the database's now(),
// exactly as in kvlease. One rule for every backend is what lets the
// conformance suite drive both with one fake clock. The cost is that several
// rela-server nodes compare expiries against their own clocks, so a node
// whose clock runs ahead sees leases expire early by that skew. NTP keeps
// skew far below the 30 second minimum lease, and a lease is advisory.
//
// # Concurrency
//
// A claim runs in one transaction. It first takes an advisory lock keyed on
// the holder identity, scoped by schema because advisory locks are
// database-global, and then updates the single lease_counters row for its
// token. That row lock serializes every claim in the schema, so the prune,
// the held check and both caps see a stable set of rows. The insert is still
// a conditional upsert on the entity id, which refuses a live lease even
// without the serialization.
package pglease

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Sourcehaven-BV/rela/internal/lease"
)

// DBTX is the database handle this store runs on: a *pgxpool.Pool in
// production. Restated here rather than imported from pgstore, because this
// package must not depend on the store layer.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Store is the PostgreSQL lease backend.
type Store struct {
	db DBTX
}

var _ lease.Store = (*Store)(nil)

// New constructs a Store over db.
//
// Nil: rejected, so a wiring mistake fails at construction rather than at the
// first claim.
func New(db DBTX) (*Store, error) {
	if db == nil {
		return nil, errors.New("pglease: New requires a database handle")
	}
	return &Store{db: db}, nil
}

// hashIndex is the unique index that refuses a lease hash already in use.
const hashIndex = "leases_hash_idx"

const leaseColumns = `entity_id, lease_hash, identity, holder_user, holder_tool, label, token, since, expires, status`

// Claim implements [lease.Store.Claim].
func (s *Store) Claim(
	ctx context.Context, id string, h lease.Holder, leaseHash string, now, until time.Time, status string,
) (lease.Lease, error) {
	if err := lease.CheckClaim(id, h, leaseHash, now, until); err != nil {
		return lease.Lease{}, err
	}
	var out lease.Lease
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`SELECT pg_advisory_xact_lock(hashtext('leases:' || current_schema()), hashtext($1))`,
			h.Identity); err != nil {
			return fmt.Errorf("pglease: lock identity: %w", err)
		}
		var token int64
		if err := tx.QueryRow(ctx, `
			UPDATE lease_counters SET token = GREATEST(token + 1, $1), version = version + 1
			RETURNING token`, microFloor(now)).Scan(&token); err != nil {
			return fmt.Errorf("pglease: next token: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM leases WHERE expires <= $1`, now); err != nil {
			return fmt.Errorf("pglease: prune: %w", err)
		}
		if cur, ok, err := liveOn(ctx, tx, id, now); err != nil {
			return err
		} else if ok {
			return held(cur)
		}
		if err := checkCaps(ctx, tx, h.Identity, now); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			INSERT INTO leases (`+leaseColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (entity_id) DO UPDATE SET
				lease_hash = EXCLUDED.lease_hash, identity = EXCLUDED.identity,
				holder_user = EXCLUDED.holder_user, holder_tool = EXCLUDED.holder_tool,
				label = EXCLUDED.label, token = EXCLUDED.token, since = EXCLUDED.since,
				expires = EXCLUDED.expires, status = EXCLUDED.status
			WHERE leases.expires <= $8
			RETURNING `+leaseColumns,
			id, leaseHash, h.Identity, h.User, h.Tool, h.Label, token, now, until, status)
		if err != nil {
			return fmt.Errorf("pglease: claim: %w", err)
		}
		got, err := pgx.CollectRows(rows, scanLease)
		if isUniqueViolation(err, hashIndex) {
			return fmt.Errorf("%w: lease hash already in use", lease.ErrInvalid)
		}
		if err != nil {
			return fmt.Errorf("pglease: claim: %w", err)
		}
		if len(got) == 0 {
			// The upsert found a live lease: only possible if the
			// serialization above is ever removed.
			cur, _, err := liveOn(ctx, tx, id, now)
			if err != nil {
				return err
			}
			return held(cur)
		}
		out = got[0]
		return nil
	})
	return out, err
}

// checkCaps refuses a claim past either cap, counting live leases only.
func checkCaps(ctx context.Context, tx pgx.Tx, identity string, now time.Time) error {
	var mine, total int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE identity = $1), count(*) FROM leases WHERE expires > $2`,
		identity, now).Scan(&mine, &total); err != nil {
		return fmt.Errorf("pglease: count leases: %w", err)
	}
	if total >= lease.MaxPerStore {
		return lease.ErrFull
	}
	if mine >= lease.MaxPerIdentity {
		return lease.ErrLimit
	}
	return nil
}

// held returns the conflict error for cur, without its lease hash.
func held(cur lease.Lease) error {
	cur.LeaseHash = ""
	return &lease.HeldError{Current: cur}
}

// liveOn returns the lease on id that is live at now, if any.
func liveOn(ctx context.Context, tx pgx.Tx, id string, now time.Time) (lease.Lease, bool, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+leaseColumns+` FROM leases WHERE entity_id = $1 AND expires > $2`, id, now)
	if err != nil {
		return lease.Lease{}, false, fmt.Errorf("pglease: read lease: %w", err)
	}
	got, err := pgx.CollectRows(rows, scanLease)
	if err != nil {
		return lease.Lease{}, false, fmt.Errorf("pglease: read lease: %w", err)
	}
	if len(got) == 0 {
		return lease.Lease{}, false, nil
	}
	return got[0], true, nil
}

// Renew implements [lease.Store.Renew].
func (s *Store) Renew(
	ctx context.Context, leaseHash, identity string, now, until time.Time, status *string,
) (lease.Lease, error) {
	if err := lease.CheckUntil(now, until); err != nil {
		return lease.Lease{}, err
	}
	var out lease.Lease
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE leases SET expires = $4, status = COALESCE($5::text, status)
			WHERE lease_hash = $1 AND identity = $2 AND expires > $3
			RETURNING `+leaseColumns, leaseHash, identity, now, until, status)
		if err != nil {
			return fmt.Errorf("pglease: renew: %w", err)
		}
		got, err := pgx.CollectRows(rows, scanLease)
		if err != nil {
			return fmt.Errorf("pglease: renew: %w", err)
		}
		if len(got) == 0 {
			return lease.ErrLost
		}
		out = got[0]
		return bumpVersion(ctx, tx)
	})
	return out, err
}

// Release implements [lease.Store.Release]. now is not needed: claims prune.
func (s *Store) Release(ctx context.Context, leaseHash, identity string, _ time.Time) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM leases WHERE lease_hash = $1 AND identity = $2`, leaseHash, identity)
		if err != nil {
			return fmt.Errorf("pglease: release: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		return bumpVersion(ctx, tx)
	})
}

// Get implements [lease.Store.Get].
func (s *Store) Get(ctx context.Context, ids []string, now time.Time) (map[string]lease.Lease, error) {
	out := make(map[string]lease.Lease, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT `+leaseColumns+` FROM leases WHERE entity_id = ANY($1) AND expires > $2`, ids, now)
	if err != nil {
		return nil, fmt.Errorf("pglease: get: %w", err)
	}
	got, err := pgx.CollectRows(rows, scanLease)
	if err != nil {
		return nil, fmt.Errorf("pglease: get: %w", err)
	}
	for _, l := range got {
		out[l.ID] = l
	}
	return out, nil
}

// ListByHolder implements [lease.Store.ListByHolder].
func (s *Store) ListByHolder(ctx context.Context, identity string, now time.Time) ([]lease.Lease, error) {
	rows, err := s.db.Query(ctx, `
		SELECT `+leaseColumns+` FROM leases WHERE identity = $1 AND expires > $2
		ORDER BY since, entity_id`, identity, now)
	if err != nil {
		return nil, fmt.Errorf("pglease: list: %w", err)
	}
	got, err := pgx.CollectRows(rows, scanLease)
	if err != nil {
		return nil, fmt.Errorf("pglease: list: %w", err)
	}
	if got == nil {
		got = []lease.Lease{}
	}
	return got, nil
}

// Rekey implements [lease.Store.Rekey].
func (s *Store) Rekey(ctx context.Context, from, to string) error {
	if from == to {
		return nil
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT 1 FROM leases WHERE entity_id = $1 FOR UPDATE`, from)
		if err != nil {
			return fmt.Errorf("pglease: rekey: %w", err)
		}
		found := rows.Next()
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("pglease: rekey: %w", err)
		}
		if !found {
			return nil
		}
		if _, err := tx.Exec(ctx, `DELETE FROM leases WHERE entity_id = $1`, to); err != nil {
			return fmt.Errorf("pglease: rekey: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE leases SET entity_id = $2 WHERE entity_id = $1`, from, to); err != nil {
			return fmt.Errorf("pglease: rekey: %w", err)
		}
		return bumpVersion(ctx, tx)
	})
}

// Drop implements [lease.Store.Drop].
func (s *Store) Drop(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM leases WHERE entity_id = $1`, id)
		if err != nil {
			return fmt.Errorf("pglease: drop: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		return bumpVersion(ctx, tx)
	})
}

// Version implements [lease.Store.Version].
func (s *Store) Version(ctx context.Context) (uint64, error) {
	var v int64
	if err := s.db.QueryRow(ctx, `SELECT version FROM lease_counters`).Scan(&v); err != nil {
		return 0, fmt.Errorf("pglease: version: %w", err)
	}
	return uint64(max(v, 0)), nil
}

// Close implements [lease.Store.Close]. The pool belongs to the composition
// root, so nothing is closed here.
func (s *Store) Close() error { return nil }

// bumpVersion moves the version inside tx, so a watcher sees it only once the
// change is committed.
func bumpVersion(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `UPDATE lease_counters SET version = version + 1`); err != nil {
		return fmt.Errorf("pglease: bump version: %w", err)
	}
	return nil
}

// microFloor is the token floor for a claim at now: now in microseconds, or
// 0 before the Unix epoch.
func microFloor(now time.Time) int64 { return max(now.UnixMicro(), 0) }

func scanLease(row pgx.CollectableRow) (lease.Lease, error) {
	var l lease.Lease
	var token int64
	err := row.Scan(&l.ID, &l.LeaseHash, &l.Holder.Identity, &l.Holder.User, &l.Holder.Tool, &l.Holder.Label,
		&token, &l.Since, &l.Expires, &l.Status)
	l.Token = uint64(max(token, 0))
	l.Since, l.Expires = l.Since.UTC(), l.Expires.UTC()
	return l, err
}

// isUniqueViolation reports whether err is a unique violation of constraint.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

func (s *Store) inTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pglease: begin: %w", err)
	}
	// Rolls back unless Commit already succeeded, in which case it is a no-op.
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("pglease: commit: %w", err)
	}
	return nil
}
