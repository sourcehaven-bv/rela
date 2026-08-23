package sqlitestore

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Tx implements [store.Transactor] at the STRONG tier (DEC-8UIL0): an error
// from fn rolls the transaction back and no events are delivered, matching
// pgstore rather than fsstore/memstore. The spike measured that SQLite gives
// both properties essentially for free, so there is no reason to offer the
// reduced contract.
//
// The serialization it does NOT provide is cross-process: this package holds a
// single-writer lock precisely because it cannot coordinate with other
// processes the way pgstore's advisory lock does.
//
// A nested Tx on the view JOINS the open transaction and must never acquire a
// second connection. With a bounded pool that is an instant self-deadlock, and
// the conformance stress test's watchdog would report it as a driver locking
// failure — the shape is copied from pgstore/tx.go.
func (s *Store) Tx(ctx context.Context, fn func(store.Store) error) error {
	if s.txPending != nil {
		return fn(s) // nested: join, do not open a second transaction
	}

	// Serialize writers in-process so a queued writer waits on this mutex
	// rather than spending its busy_timeout budget contending in SQLite.
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("sqlitestore: acquire connection: %w", err)
	}
	defer conn.Close()

	// database/sql exposes no BEGIN IMMEDIATE option, so issue it directly on
	// the pinned connection. Deferred would take the write lock lazily, and a
	// read-then-write transaction would have to UPGRADE mid-flight — an
	// upgrade cannot wait, so it returns SQLITE_BUSY regardless of
	// busy_timeout. Measured in the spike; not a precaution.
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("sqlitestore: begin: %w", err)
	}

	pending := &pendingEvents{}
	view := &Store{
		db:          s.db,
		opts:        s.opts,
		journalMode: s.journalMode,
		conn:        conn,
		txPending:   pending,
		parent:      s,
	}

	if err := fn(view); err != nil {
		// WithoutCancel so a cancelled ctx still rolls back, rather than
		// leaving an open transaction on a connection returning to the pool.
		if _, rbErr := conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK"); rbErr != nil {
			return errors.Join(err, fmt.Errorf("sqlitestore: rollback: %w", rbErr))
		}
		return err // returned unchanged, per the contract
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("sqlitestore: commit: %w", err)
	}

	// Replay only after the commit succeeded: a subscriber must never observe
	// a write that was rolled back.
	for _, ev := range pending.drain() {
		s.publish(ev)
	}
	return nil
}

// drain returns the buffered events and empties the buffer.
func (p *pendingEvents) drain() []store.Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	events := p.events
	p.events = nil
	return events
}
