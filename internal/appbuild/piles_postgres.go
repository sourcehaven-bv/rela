//go:build postgres

package appbuild

import (
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/piles/pgpiles"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// storePilesFor returns the database piles backend over st's pool, so every
// rela-server process on the schema sees the same piles. The KV backend
// rewrites one document per change and would lose writes across processes.
//
// A store that is not a pgstore gets an in-memory backend and an error log,
// never the cache-directory KV: that directory belongs to the project, not
// to the schema, so on a multi-tenant host every tenant would share one
// piles document. Assembly does not fail, because the postgres-tagged tests
// assemble memstores; a real deployment always hands in a pgstore.
//
// Nil: never returned on this build.
func storePilesFor(st store.Store) piles.Store {
	db := pgstore.HandleFor(st)
	if db == nil {
		slog.Error("piles: store is not a pgstore; piles are kept in memory and lost on restart")
		return memoryPiles()
	}
	s, err := pgpiles.New(db)
	if err != nil { // coverage-ignore: unreachable: db is non-nil here
		panic(err)
	}
	return s
}
