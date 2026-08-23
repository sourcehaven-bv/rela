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

	// The deferred ROLLBACK is not belt-and-braces; without it a panic in fn
	// PERMANENTLY BRICKS THE STORE. conn.Close() returns the driver connection
	// to the pool with BEGIN IMMEDIATE still open, and database/sql then hands
	// that connection out again: every later transaction fails with "cannot
	// start a transaction within a transaction", and the uncommitted write
	// becomes durable and visible. Measured: 10/10 subsequent connections
	// poisoned, leaked row readable.
	//
	// So a panic in an automation, validator or observer inside fn would both
	// commit data reported as rolled back AND take the store down until the
	// process restarts. It runs on WithoutCancel for the same reason the
	// explicit rollback below does.
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		}
		_ = conn.Close()
	}()

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

	// COMMIT also runs on WithoutCancel. A request cancelled between the last
	// write and the commit — a closed browser tab, mid-save — would otherwise
	// fail here and return WITHOUT rolling back, poisoning the pool exactly as
	// a panic does. A transaction that has reached this point is complete;
	// abandoning it because the caller went away helps no one.
	if _, err := conn.ExecContext(context.WithoutCancel(ctx), "COMMIT"); err != nil {
		return fmt.Errorf("sqlitestore: commit: %w", err)
	}
	committed = true

	// Replay only after the commit succeeded: neither a subscriber nor an
	// observer may witness a write that was rolled back.
	for _, note := range pending.drain() {
		note(s)
	}
	return nil
}

// add buffers a callback to run after the transaction commits.
func (p *pendingEvents) add(note func(*Store)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.notes = append(p.notes, note)
}

// drain returns the buffered callbacks and empties the buffer.
func (p *pendingEvents) drain() []func(*Store) {
	p.mu.Lock()
	defer p.mu.Unlock()
	notes := p.notes
	p.notes = nil
	return notes
}
