// Package pgstore provides a PostgreSQL implementation of store.Store.
//
// # Connection ownership
//
// For the query path, pgstore does NOT own a connection or a DSN. New takes an
// injected [DBTX] handle — in production and in tests this is a *pgxpool.Pool.
// The wiring layer (internal/appbuild, behind //go:build postgres) builds the
// pool from the resolved DSN, runs [Migrate], and passes the pool here; it also
// owns closing the pool.
//
// A pool (not a single pgx.Tx) is required: the store opens its own
// transactions for multi-statement atomic operations (RenameEntity,
// cascade DeleteEntity) and must serve concurrent operations safely.
//
// # Change feed (cross-process)
//
// In addition to the injected query pool, a store opened via [Open] owns a
// change-feed listener (see feed.go / listener.go) holding its OWN dedicated
// connection, started in Open and stopped in Close. Each committed write emits
// a NOTIFY on a schema-scoped channel; the listener turns OTHER processes'
// notifications into store.Events on the same in-process Subscribe() fan-out as
// local writes, so multiple processes against one database see each other's
// changes. Store.Close stops the listener (closing its connection) before
// closing subscriber channels; it does not close the query pool.
//
// Event delivery is best-effort: a subscriber never sees an uncommitted write
// (emit and NOTIFY happen on/after commit), but it may MISS an event (full
// buffer, or a notification lost while disconnected). A seq-watermark catch-up
// recovers missed cross-process writes; consumers also re-snapshot. A store
// built with [New] (no listener wiring, e.g. the conformance harness) has only
// the in-process watcher.
//
// # Search
//
// The full-text search Backend lives in the same database (see search.go);
// the package doc on store.Store anticipates this ("smart backends ... provide
// native implementations sharing the same connection").
package pgstore

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// DBTX is the subset of the pgx API pgstore needs. It is satisfied by
// *pgxpool.Pool, *pgx.Conn, and pgx.Tx. Production and tests inject a pool
// (see package doc for why a bare Tx is unsuitable).
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Store is a PostgreSQL-backed store.Store.
//
// TODO(TKT-N0IKN9): the exported surface (29) is the mandated store.Store
// interface, which consumers depend on directly by design. Required-interface
// exception — tracks the interface size, not accreted public API.
//
//plimsoll:max-exported-methods=29
type Store struct {
	db        DBTX
	observers []store.EntityObserver // notified synchronously after committed entity writes

	// Cross-process change feed (see feed.go / listener.go). originID identifies
	// this store's own NOTIFY echoes so the listener can skip them; channel is
	// the schema-scoped NOTIFY channel (empty => the producer is a no-op, e.g.
	// for a New() store with no listener wiring). Both are set at construction.
	originID string
	channel  string
	listener *listener // nil unless a listener was started (see startListener)

	mu          sync.Mutex // guards subscribers + nextSubID only
	subscribers map[int]chan store.Event
	nextSubID   int
	closed      bool
}

// Option configures a Store.
type Option func(*Store)

// WithObserver registers an entity observer notified after committed entity
// writes. A nil observer is dropped silently, matching memstore.WithObserver
// and the app.FSFactory.AddObserver contract, so callers can pass the result
// of an optional search-backend factory without a nil guard.
func WithObserver(o store.EntityObserver) Option {
	return func(s *Store) {
		if o == nil {
			return
		}
		s.observers = append(s.observers, o)
	}
}

// compile-time interface check.
var _ store.Store = (*Store)(nil)

// Delegate ID/property validation to the shared helpers so pgstore behaves
// identically to memstore/fsstore. (Relation matching is expressed directly in
// SQL — see relationWhere — rather than via storeutil.MatchRelation.)
var (
	validateID       = storeutil.ValidateID
	validateProperty = storeutil.ValidateProperty
)

// New constructs a Store over the injected handle. It does not run
// migrations — call [Migrate] first (the wiring layer does this once at
// startup; tests do it per schema). Returns an error if db is nil.
func New(db DBTX, opts ...Option) (*Store, error) {
	if db == nil {
		return nil, errors.New("pgstore: nil DBTX")
	}
	s := &Store{
		db:          db,
		originID:    newOriginID(),
		subscribers: make(map[int]chan store.Event),
	}
	// The producer NOTIFY channel is left empty here (producer no-ops) until a
	// listener is started via Open, which resolves the schema-scoped channel and
	// sets s.channel for both producer and listener. A store built with New and
	// no listener (e.g. the conformance harness) simply emits no cross-process
	// notifications — its in-process watcher is unaffected.
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// --- Freshness ---

// LastModified returns the newest updated_at across entities and relations,
// or the zero time if both tables are empty.
func (s *Store) LastModified(ctx context.Context) (time.Time, error) {
	const q = `
		SELECT max(updated_at) FROM (
			SELECT updated_at FROM entities
			UNION ALL
			SELECT updated_at FROM relations
		) t`
	var t *time.Time
	if err := s.db.QueryRow(ctx, q).Scan(&t); err != nil {
		return time.Time{}, err
	}
	if t == nil {
		return time.Time{}, nil
	}
	return *t, nil
}

// --- Watcher ---

// Subscribe registers a buffered event channel. Events are delivered by
// non-blocking send (dropped when the buffer is full), matching the
// store.Watcher contract.
func (s *Store) Subscribe(bufSize int) (events <-chan store.Event, cancel func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan store.Event, bufSize)
	id := s.nextSubID
	s.nextSubID++
	s.subscribers[id] = ch

	cancel = func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.subscribers[id]; ok {
			delete(s.subscribers, id)
			close(ch)
		}
	}
	return ch, cancel
}

// emit delivers an event to every subscriber via a non-blocking send. Delivery
// is intentionally LOSSY and UNORDERED across subscribers — a full subscriber
// buffer drops the event (matching the store.Watcher contract and memstore).
// It is called AFTER a write transaction commits — never while holding a DB
// transaction — so subscribers never observe uncommitted state.
func (s *Store) emit(ev store.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ch := range s.subscribers {
		select {
		case ch <- ev:
		default:
			// drop — subscriber is slow
		}
	}
}

// emitAll delivers a batch of events in order (used by cascade delete and
// rename, which produce several events per operation).
func (s *Store) emitAll(evs []store.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ev := range evs {
		for _, ch := range s.subscribers {
			select {
			case ch <- ev:
			default:
			}
		}
	}
}

// --- Lifecycle ---

// Close tears down the change-feed listener (if any) and the watcher: it stops
// the listener goroutine and closes its dedicated connection, then closes all
// subscriber channels. It does NOT close the injected pool — the wiring layer
// owns the pool's lifecycle.
//
// The listener is stopped FIRST (before subscriber channels close) so it can't
// emit onto a closing channel; it uses its own connection, not the pool, so
// closing it is independent of the pool's lifecycle.
func (s *Store) Close() error {
	// Stop the listener outside the lock — stop() blocks on the goroutine, which
	// may call emit() (which takes s.mu); holding the lock here would deadlock.
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	l := s.listener
	s.listener = nil
	s.mu.Unlock()

	l.stop() // nil-safe; blocks until the listener goroutine exits

	s.mu.Lock()
	defer s.mu.Unlock()
	for id, ch := range s.subscribers {
		close(ch)
		delete(s.subscribers, id)
	}
	return nil
}
