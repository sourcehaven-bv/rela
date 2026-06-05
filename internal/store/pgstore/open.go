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

// Open is the one-call backend constructor used by the wiring layer (appbuild
// and the MCP wiring, behind //go:build postgres). It builds a pgx pool from
// the DSN, applies migrations, and wires a Store together with its in-database
// search Backend over that single shared pool.
//
// The returned io.Closer closes the pool — pgstore owns the pool it created
// here, so callers close it via this value (Store.Close only tears down the
// watcher). Keeping pool construction in one place means each composition root
// calls Open once instead of duplicating build-pool/migrate/wire/close logic.
func Open(ctx context.Context, dsn string) (store.Store, search.Searcher, io.Closer, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		// pgxpool.New parses (not connects); pgx redacts the password in errors.
		return nil, nil, nil, fmt.Errorf("connect to database: %w", err)
	}
	if err = Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, nil, nil, fmt.Errorf("migrate database: %w", err)
	}

	backend := NewSearchBackend(pool)
	st, err := New(pool, WithObserver(backend))
	if err != nil {
		pool.Close()
		return nil, nil, nil, fmt.Errorf("open store: %w", err)
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

	return st, search.New(st, backend), &poolCloser{pool}, nil
}

// poolCloser closes the pgx pool the wiring layer received from Open. The
// store's own Close only tears down the watcher, so the pool is closed here.
type poolCloser struct{ pool *pgxpool.Pool }

func (c *poolCloser) Close() error {
	c.pool.Close()
	return nil
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
