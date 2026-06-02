//go:build postgres

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// openMCPBackend — `postgres` build. Builds one pgx pool from
// RELA_DATABASE_URL, migrates the schema, and wires pgstore + its in-DB
// search backend over that single pool. MCP has no flag surface, so the
// DSN comes from the environment only (mirrors the appbuild env path; a
// --database-url flag exists on rela / rela-server, not on `rela mcp`).
//
// The metamodel and templates still come from the filesystem — PostgreSQL
// backs entities/relations/attachments/search only. The returned closer
// closes the pool (the wiring layer owns it; pgstore.Close only tears
// down the watcher).
func openMCPBackend(
	ctx context.Context,
	_ storage.FS,
	_ *project.Context,
	_ *metamodel.Metamodel,
) (store.Store, search.Searcher, io.Closer, error) {
	dsn := os.Getenv("RELA_DATABASE_URL")
	if dsn == "" {
		return nil, nil, nil, errors.New(
			"rela mcp (postgres build) requires RELA_DATABASE_URL to be set")
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("connect to database: %w", err)
	}
	if migErr := pgstore.Migrate(ctx, pool); migErr != nil {
		pool.Close()
		return nil, nil, nil, fmt.Errorf("migrate database: %w", migErr)
	}

	backend := pgstore.NewSearchBackend(pool)
	st, err := pgstore.New(pool, pgstore.WithObserver(backend))
	if err != nil {
		pool.Close()
		return nil, nil, nil, fmt.Errorf("open store: %w", err)
	}
	return st, search.New(st, backend), mcpPoolCloser{pool}, nil
}

// mcpPoolCloser closes the pgx pool when mcpServices.Close releases the
// search closer.
type mcpPoolCloser struct{ pool *pgxpool.Pool }

func (c mcpPoolCloser) Close() error {
	c.pool.Close()
	return nil
}
