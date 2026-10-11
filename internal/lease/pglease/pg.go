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
// Every write runs in one transaction whose first statement locks the single
// lease_counters row (SELECT ... FOR UPDATE). That row is the one
// serialization point for the schema: the prune, the held check, both caps,
// the token and the version all see a stable set of rows. Because every
// write takes the same lock first, and takes no other lock before it, two
// writes cannot wait on each other in opposite orders. Reads take no lock.
//
// The implementation plan for TKT-6QC5EL also put a per-identity advisory
// lock in front of claims, for the per-identity cap. It was dropped: the
// counter row already serializes every write, caps included, and a second
// lock only added a lock order to get wrong.
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
	if err := lease.CheckClaim(id, h, leaseHash, now, until, status); err != nil {
		return lease.Lease{}, err
	}
	now, until = lease.Instant(now), lease.Instant(until)
	var out lease.Lease
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		last, err := lockCounters(ctx, tx)
		if err != nil {
			return err
		}
		// Rolled back with the rest when the claim is refused; the next
		// write prunes again.
		if _, err = tx.Exec(ctx, `DELETE FROM leases WHERE expires <= $1`, now); err != nil {
			return fmt.Errorf("pglease: prune: %w", err)
		}
		cur, held, err := liveOn(ctx, tx, id, now)
		if err != nil {
			return err
		}
		if held {
			cur.LeaseHash = ""
			return &lease.HeldError{Current: cur}
		}
		if capErr := checkCaps(ctx, tx, h.Identity, now); capErr != nil {
			return capErr
		}
		token := nextToken(last, now)
		rows, err := tx.Query(ctx, `
			INSERT INTO leases (`+leaseColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING `+leaseColumns,
			id, leaseHash, h.Identity, h.User, h.Tool, h.Label, token, now, until, status)
		if err != nil {
			return fmt.Errorf("pglease: claim: %w", err)
		}
		got, err := pgx.CollectExactlyOneRow(rows, scanLease)
		if isUniqueViolation(err, hashIndex) {
			return fmt.Errorf("%w: lease hash already in use", lease.ErrInvalid)
		}
		if err != nil {
			return fmt.Errorf("pglease: claim: %w", err)
		}
		out = got
		return bumpVersion(ctx, tx, &token)
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
	if err := lease.CheckRenew(leaseHash, now, until, status); err != nil {
		return lease.Lease{}, err
	}
	now, until = lease.Instant(now), lease.Instant(until)
	var out lease.Lease
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := lockCounters(ctx, tx); err != nil {
			return err
		}
		// The counter lock is held, so the row read here is the row updated
		// below.
		var oldExpires time.Time
		var oldStatus string
		err := tx.QueryRow(ctx, `
			SELECT expires, status FROM leases WHERE lease_hash = $1 AND identity = $2 AND expires > $3`,
			leaseHash, identity, now).Scan(&oldExpires, &oldStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			return lease.ErrLost
		}
		if err != nil {
			return fmt.Errorf("pglease: renew: %w", err)
		}
		rows, err := tx.Query(ctx, `
			UPDATE leases SET expires = $4, status = COALESCE($5::text, status)
			WHERE lease_hash = $1 AND identity = $2 AND expires > $3
			RETURNING `+leaseColumns, leaseHash, identity, now, until, status)
		if err != nil {
			return fmt.Errorf("pglease: renew: %w", err)
		}
		got, err := pgx.CollectExactlyOneRow(rows, scanLease)
		if err != nil {
			return fmt.Errorf("pglease: renew: %w", err)
		}
		out = got
		if !until.Before(oldExpires) && (status == nil || *status == oldStatus) {
			return nil // a plain extension; see the Version section of lease.Store
		}
		return bumpVersion(ctx, tx, nil)
	})
	return out, err
}

// Lookup implements [lease.Store.Lookup].
func (s *Store) Lookup(ctx context.Context, leaseHash, identity string, now time.Time) (lease.Lease, error) {
	if err := lease.CheckHash(leaseHash); err != nil {
		return lease.Lease{}, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT `+leaseColumns+` FROM leases WHERE lease_hash = $1 AND identity = $2 AND expires > $3`,
		leaseHash, identity, lease.Instant(now))
	if err != nil {
		return lease.Lease{}, fmt.Errorf("pglease: lookup: %w", err)
	}
	got, err := pgx.CollectRows(rows, scanLease)
	if err != nil {
		return lease.Lease{}, fmt.Errorf("pglease: lookup: %w", err)
	}
	if len(got) == 0 {
		return lease.Lease{}, lease.ErrLost
	}
	return got[0], nil
}

// Release implements [lease.Store.Release].
func (s *Store) Release(ctx context.Context, leaseHash, identity string, now time.Time) error {
	if err := lease.CheckHash(leaseHash); err != nil {
		return err
	}
	return s.removeWhere(ctx, "release",
		`DELETE FROM leases WHERE lease_hash = $1 AND identity = $2 RETURNING expires > $3`,
		leaseHash, identity, lease.Instant(now))
}

// Drop implements [lease.Store.Drop].
func (s *Store) Drop(ctx context.Context, id string, now time.Time) error {
	return s.removeWhere(ctx, "drop",
		`DELETE FROM leases WHERE entity_id = $1 RETURNING expires > $2`, id, lease.Instant(now))
}

// removeWhere runs del, a DELETE that returns whether each removed lease was
// live, and moves the version when one was.
func (s *Store) removeWhere(ctx context.Context, op, del string, args ...any) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := lockCounters(ctx, tx); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, del, args...)
		if err != nil {
			return fmt.Errorf("pglease: %s: %w", op, err)
		}
		live, err := pgx.CollectRows(rows, pgx.RowTo[bool])
		if err != nil {
			return fmt.Errorf("pglease: %s: %w", op, err)
		}
		for _, l := range live {
			if l {
				return bumpVersion(ctx, tx, nil)
			}
		}
		return nil
	})
}

// Get implements [lease.Store.Get].
func (s *Store) Get(ctx context.Context, ids []string, now time.Time) (map[string]lease.Lease, error) {
	out := make(map[string]lease.Lease, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT `+leaseColumns+` FROM leases WHERE entity_id = ANY($1) AND expires > $2`, ids, lease.Instant(now))
	if err != nil {
		return nil, fmt.Errorf("pglease: get: %w", err)
	}
	got, err := pgx.CollectRows(rows, scanLease)
	if err != nil {
		return nil, fmt.Errorf("pglease: get: %w", err)
	}
	for _, l := range got {
		l.LeaseHash = ""
		out[l.ID] = l
	}
	return out, nil
}

// ListByHolder implements [lease.Store.ListByHolder].
func (s *Store) ListByHolder(ctx context.Context, identity string, now time.Time) ([]lease.Lease, error) {
	rows, err := s.db.Query(ctx, `
		SELECT `+leaseColumns+` FROM leases WHERE identity = $1 AND expires > $2
		ORDER BY since, entity_id`, identity, lease.Instant(now))
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

// NextExpiry implements [lease.Store.NextExpiry], served by the expires
// index.
func (s *Store) NextExpiry(ctx context.Context, now time.Time) (time.Time, bool, error) {
	var next *time.Time
	if err := s.db.QueryRow(ctx,
		`SELECT min(expires) FROM leases WHERE expires > $1`, lease.Instant(now)).Scan(&next); err != nil {
		return time.Time{}, false, fmt.Errorf("pglease: next expiry: %w", err)
	}
	if next == nil {
		return time.Time{}, false, nil
	}
	return next.UTC(), true, nil
}

// Rekey implements [lease.Store.Rekey].
//
// The lease on from is deleted first, so its hash is free for the row on to.
// The row on to is then an upsert that overwrites only an expired lease, so a
// live lease on to stays even if the counter lock were ever bypassed, and no
// unique violation on the entity id can surface.
func (s *Store) Rekey(ctx context.Context, from, to string, now time.Time) error {
	if from == "" || to == "" {
		return fmt.Errorf("%w: entity id must be non-empty", lease.ErrInvalid)
	}
	if from == to {
		return nil
	}
	now = lease.Instant(now)
	return s.inTx(ctx, func(tx pgx.Tx) error {
		last, err := lockCounters(ctx, tx)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `DELETE FROM leases WHERE entity_id = $1 RETURNING `+leaseColumns, from)
		if err != nil {
			return fmt.Errorf("pglease: rekey: %w", err)
		}
		moved, err := pgx.CollectRows(rows, scanLease)
		if err != nil {
			return fmt.Errorf("pglease: rekey: %w", err)
		}
		if len(moved) == 0 || !moved[0].Live(now) {
			return nil
		}
		l := moved[0]
		token := nextToken(last, now)
		tag, err := tx.Exec(ctx, `
			INSERT INTO leases (`+leaseColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (entity_id) DO UPDATE SET
				lease_hash = EXCLUDED.lease_hash, identity = EXCLUDED.identity,
				holder_user = EXCLUDED.holder_user, holder_tool = EXCLUDED.holder_tool,
				label = EXCLUDED.label, token = EXCLUDED.token, since = EXCLUDED.since,
				expires = EXCLUDED.expires, status = EXCLUDED.status
			WHERE leases.expires <= $11`,
			to, l.LeaseHash, l.Holder.Identity, l.Holder.User, l.Holder.Tool, l.Holder.Label,
			token, l.Since, l.Expires, l.Status, now)
		if err != nil {
			return fmt.Errorf("pglease: rekey: %w", err)
		}
		if tag.RowsAffected() == 0 {
			// to has a live lease: it stays, and the one on from is gone.
			return bumpVersion(ctx, tx, nil)
		}
		return bumpVersion(ctx, tx, &token)
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

// lockCounters locks the counter row, the serialization point every write
// takes first, and returns the last token issued.
func lockCounters(ctx context.Context, tx pgx.Tx) (int64, error) {
	var token int64
	if err := tx.QueryRow(ctx, `SELECT token FROM lease_counters FOR UPDATE`).Scan(&token); err != nil {
		return 0, fmt.Errorf("pglease: lock counters: %w", err)
	}
	return token, nil
}

// bumpVersion moves the version inside tx, so a watcher sees it only once the
// change is committed, and stores token as the last one issued when it is
// non-nil.
func bumpVersion(ctx context.Context, tx pgx.Tx, token *int64) error {
	if _, err := tx.Exec(ctx,
		`UPDATE lease_counters SET version = version + 1, token = COALESCE($1::bigint, token)`, token); err != nil {
		return fmt.Errorf("pglease: bump version: %w", err)
	}
	return nil
}

// nextToken is [lease.NextToken] over the signed column: max(last+1, now in
// microseconds), with a now before the Unix epoch counting as 0.
func nextToken(last int64, now time.Time) int64 { return max(last+1, now.UnixMicro(), 0) }

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
