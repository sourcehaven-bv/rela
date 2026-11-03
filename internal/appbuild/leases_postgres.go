//go:build postgres

package appbuild

import (
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/lease/pglease"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// storeLeasesFor returns the database lease backend over st's pool, so every
// rela-server process on the schema sees one set of leases. A store that is
// not a pgstore gets an in-memory backend, for the reason storePilesFor
// gives.
//
// Nil: never returned on this build.
func storeLeasesFor(st store.Store) lease.Store {
	db := pgstore.HandleFor(st)
	if db == nil {
		slog.Error("leases: store is not a pgstore; leases are kept in memory")
		return memoryLeases()
	}
	s, err := pglease.New(db)
	if err != nil { // coverage-ignore: unreachable: db is non-nil here
		panic(err)
	}
	return s
}
