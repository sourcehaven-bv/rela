package pgstore

import (
	"context"
	"fmt"
	"io"

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
	return st, search.New(st, backend), &poolCloser{pool}, nil
}

// poolCloser closes the pgx pool the wiring layer received from Open. The
// store's own Close only tears down the watcher, so the pool is closed here.
type poolCloser struct{ pool *pgxpool.Pool }

func (c *poolCloser) Close() error {
	c.pool.Close()
	return nil
}
