//go:build postgres

package appbuild

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// New builds the services bundle for the `postgres` build: a pgstore and
// its in-database search backend, both sharing one pgx pool built from
// the resolved DSN. This recipe owns the backend choice; [prepare] and
// [assemble] do the build-agnostic work shared with the FS/memory builds.
//
// The metamodel and templates still come from the filesystem (see
// Config.Paths) — PostgreSQL backs entities/relations/attachments/search
// only. A DSN is required (Config.DatabaseURL or RELA_DATABASE_URL, with
// a --database-url flag overriding via WithDatabaseURL).
func New(cfg Config, opts ...Option) (*Services, error) {
	base, err := prepare(cfg, opts)
	if err != nil {
		return nil, err
	}
	st, searcher, closer, err := openBackend(context.Background(), base)
	if err != nil {
		return nil, err
	}
	return assemble(base, st, searcher, closer)
}

// openBackend builds one pgx pool from the DSN, migrates the schema, and
// wires pgstore + its in-DB search backend over that single pool. The
// returned io.Closer closes the pool — appbuild owns the pool's
// lifecycle; pgstore.Store.Close only tears down the watcher.
func openBackend(ctx context.Context, base *buildBase) (store.Store, search.Searcher, io.Closer, error) {
	dsn := base.cfg.DatabaseURL
	if dsn == "" {
		return nil, nil, nil, errors.New(
			"appbuild: postgres build requires a database URL (set RELA_DATABASE_URL or pass --database-url)")
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		// pgxpool.New parses the DSN but does not connect; a parse error
		// here is safe to surface (pgx redacts the password in its error).
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

	return st, search.New(st, backend), poolCloser{pool}, nil
}

// poolCloser closes the pgx pool when Services.Close releases the search
// closer. The store's own Close only tears down the watcher, so the pool
// must be closed here (appbuild owns it).
type poolCloser struct{ pool *pgxpool.Pool }

func (c poolCloser) Close() error {
	c.pool.Close()
	return nil
}
