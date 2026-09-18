package pgstore

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// NewPool builds a pgx pool from a DSN, configured the way every rela process
// expects, and returns it as a [DBTX] with a closer for its lifetime.
//
// Pool OWNERSHIP belongs to the composition root, not to this package: the
// store, the in-database search backend and the comment backend are three equal
// consumers of ONE pool, and only the caller knows when the last of them is
// done. So this closes nothing itself — the returned io.Closer is how the
// caller ends the pool's life.
//
// The pool's CONFIGURATION is pgstore's business even so: the query tracer is
// what produces the per-statement timing `rela-server -verbose` reports, and a
// composition root that built a bare pgxpool would silently lose it.
//
// Returning DBTX rather than *pgxpool.Pool keeps pgx out of the composition
// root — arch-lint grants that vendor to the store layer, not to appbuild, and
// widening it for one type would be the wrong trade.
func NewPool(ctx context.Context, dsn string) (DBTX, io.Closer, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		// ParseConfig parses (not connects); pgx redacts the password in errors.
		return nil, nil, fmt.Errorf("parse database DSN: %w", err)
	}
	// The tracer is always attached; it decides per statement whether to
	// account (stats on the context) or log (Debug enabled) and otherwise
	// costs one context lookup. See queryTracer.
	cfg.ConnConfig.Tracer = queryTracer{}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to database: %w", err)
	}
	return pool, &poolCloser{pool}, nil
}

// poolCloser ends the life of a pool built by [NewPool]. Store.Close only tears
// down the watcher, so without this the pool would outlive every consumer.
type poolCloser struct{ pool *pgxpool.Pool }

func (c *poolCloser) Close() error {
	c.pool.Close()
	return nil
}

// Open wires a Store and its in-database search Backend over an ALREADY-BUILT
// pool, and starts the cross-process change-feed listener.
//
// It takes a handle rather than a DSN (TKT-OGTVJW) because the pool is shared
// with consumers this package does not own. It used to build the pool itself
// and return only the store, the searcher and a closer — which made it a
// composition root living inside the store package, and left the real
// composition root with no handle to give anything else. `pgcomments` is the
// third consumer that made that concrete; `NewSearchBackend(pool)` below was
// always the second.
//
// The caller owns db: Open closes nothing, and Store.Close only tears down the
// watcher. Migrations are the caller's too — run [Migrate] before this.
//
// dsn is still required, and is NOT a second route to the database: the change
// feed's listener holds its own dedicated connection, deliberately outside the
// pool so a slow LISTEN cannot starve query traffic. It is used for that alone.
func Open(ctx context.Context, db DBTX, dsn string) (store.Store, search.Searcher, error) {
	backend := NewSearchBackend(db)
	st, err := New(db, WithObserver(backend))
	if err != nil {
		return nil, nil, fmt.Errorf("open store: %w", err)
	}

	// Start the cross-process change-feed listener (TKT-WZYWM9). It holds its
	// own dedicated connection (from the DSN, not the pool) so a slow LISTEN
	// never starves query traffic. A failure to start is non-fatal: the store
	// and its in-process (local) events still work; only cross-process events
	// are unavailable. This mirrors the "search index unavailable" degradation.
	l, err := startListener(ctx, st, dsn)
	if err != nil {
		slog.Warn("pgstore: cross-process change feed unavailable; "+
			"writes from other processes won't be observed live", "error", err)
	} else {
		st.listener = l
	}

	return st, search.New(st, backend), nil
}

// MigrateDSN opens a short-lived pool for the given DSN, applies pending
// migrations, and closes the pool. It is the entry point for the `rela db
// migrate` admin command, keeping pool construction (and the pgx dependency)
// inside this package rather than in the CLI.
func MigrateDSN(ctx context.Context, dsn string) error {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()
	return Migrate(ctx, pool)
}

// StatusDSN opens a short-lived pool and reports the current vs target schema
// version without changing anything. Entry point for `rela db status`.
func StatusDSN(ctx context.Context, dsn string) (current, target int, err error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return 0, 0, fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()
	return Status(ctx, pool)
}

// ReconcileDSN opens a short-lived pool over dsn and reconciles the derived
// schema (partial unique indexes, TKT-3Q0GP1) for the given desired specs,
// returning the per-object outcomes. Entry point for `rela db reconcile` and
// the derived-schema section of `rela db status` (with opts.DryRun). Keeping
// pool construction here confines the pgx dependency to this package. The caller
// derives the specs from the metamodel (which lives on disk, not in the DB).
func ReconcileDSN(
	ctx context.Context, dsn string, desired []store.DerivedObjectSpec, opts store.ReconcileOptions,
) ([]store.DerivedObjectOutcome, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()
	s, err := New(pool)
	if err != nil {
		return nil, err
	}
	return s.Reconcile(ctx, desired, opts)
}
