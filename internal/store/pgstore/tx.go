package pgstore

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Tx (store.Transactor, DEC-8UIL0): a real database transaction plus
// deployment-wide write serialization.
//
// Tx opens one outer transaction, takes writeAdvisoryLockKey as a
// transaction-scoped advisory lock (so every process of the deployment
// serializes its Tx callbacks and, transitively, the write methods'
// row work — one global key means zero deadlock risk and no retry
// machinery; per-entity granularity is a possible later optimization
// invisible to callers), and hands fn a tx-bound view of the store.
//
// The view's db handle IS the outer transaction: the ordinary write
// methods run unchanged, their per-write Begin/Commit becoming
// savepoints, and their pg_notify calls deferring to the outer commit
// natively. In-process observer/subscriber notifications are buffered
// on the view (see Store.txPending) and replayed against the parent
// store only after the outer commit — a subscriber must never observe
// a write that later rolls back. An error from fn rolls the whole
// transaction back: no rows, no NOTIFYs, no events.
//
// A nested Tx on the view joins the open transaction.

// writeAdvisoryLockKey serializes write transactions across the processes
// sharing one SCHEMA ("RELW"). Distinct from migrateAdvisoryLockKey ("RELA",
// xact-scoped in Migrate) and sweepAdvisoryLockKey ("RELV", session-scoped,
// shared by sweep and purge): a write Tx must not block — or be blocked by — a
// sweep tick, matching today's behavior where ordinary writes never take the
// sweep lock.
//
// Like both of those, it is SCHEMA-SCOPED via the two-key form
// pg_advisory_xact_lock(key, hashtext(current_schema())) — see the
// tryAdvisoryLock doc in sweep.go. Advisory locks are database-GLOBAL, so a
// bare key made every write transaction serialize against writes to UNRELATED
// schemas on the same database: a throughput fault rather than the capture loss
// the sweep suffered, but the same root cause.
const writeAdvisoryLockKey int64 = 0x52_45_4c_57

// txPending buffers the in-process notifications of one open Tx, in
// emission order, for replay against the parent store after commit.
// It is used from the Tx callback's goroutine only (the view must not
// be shared across goroutines — see the store.Transactor contract), so
// it needs no locking.
type txPending struct {
	notes []func(*Store)
}

func (p *txPending) add(n func(*Store)) {
	p.notes = append(p.notes, n)
}

// Tx implements store.Transactor. See the notes above for the pgstore
// mechanics; the behavioral contract is on store.Transactor.
func (s *Store) Tx(ctx context.Context, fn func(store.Store) error) error {
	if s.txPending != nil {
		return fn(s) // nested Tx on the view joins the open transaction
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock($1::int, hashtext(current_schema()))`,
		writeAdvisoryLockKey); err != nil {
		return err
	}

	pending := &txPending{}
	view := &Store{
		db:          tx,
		originID:    s.originID,
		channel:     s.channel,
		subscribers: make(map[int]chan store.Event),
		txPending:   pending,
	}
	if err := fn(view); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	for _, n := range pending.notes {
		n(s)
	}
	return nil
}
