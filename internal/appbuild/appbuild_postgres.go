//go:build postgres

package appbuild

import (
	"context"
	"errors"
	"io"

	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// New builds the services bundle for the `postgres` build: a pgstore and its
// in-database search backend, both sharing one pgx pool built from the resolved
// DSN. This recipe owns only the backend choice; [prepare] and [assemble] do
// the build-agnostic work shared with the FS/memory builds.
//
// The metamodel and templates still come from the filesystem (see
// Config.Paths) — PostgreSQL backs entities/relations/attachments/search only.
// A DSN is required: Config.DatabaseURL, which Discover populates from the
// RELA_DATABASE_URL environment variable (env-only, never a flag).
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

// openBackend delegates pool construction, migration, and store+search wiring
// to pgstore.Open — the single owner of that logic (shared with the MCP
// wiring's postgres recipe).
func openBackend(ctx context.Context, base *buildBase) (store.Store, search.Searcher, io.Closer, error) {
	if base.cfg.DatabaseURL == "" {
		return nil, nil, nil, errors.New(
			"appbuild: postgres build requires a database URL (set RELA_DATABASE_URL)")
	}
	st, searcher, closer, err := pgstore.Open(ctx, base.cfg.DatabaseURL)
	if err != nil {
		return nil, nil, nil, err
	}
	return st, searcher, closer, nil
}
