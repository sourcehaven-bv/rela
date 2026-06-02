//go:build postgres

package cli

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// openMCPBackend — `postgres` build. Delegates pool construction, migration,
// and store+search wiring to pgstore.Open (the single owner of that logic,
// shared with appbuild's postgres recipe). MCP has no flag surface, so the DSN
// comes from RELA_DATABASE_URL only (a --database-url flag exists on
// rela / rela-server, not on `rela mcp`).
//
// The metamodel and templates still come from the filesystem — PostgreSQL backs
// entities/relations/attachments/search only.
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
	st, searcher, closer, err := pgstore.Open(ctx, dsn)
	if err != nil {
		return nil, nil, nil, err
	}
	return st, searcher, closer, nil
}
