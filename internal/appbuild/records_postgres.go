//go:build postgres

package appbuild

import (
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/pgrecords"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// storeRecordsFor returns the database record store over st's pool, so every
// rela-server process on the schema sees one set of records and one set of
// ledger group claims.
//
// Nil: never returned. A store that is not a pgstore gets an in-memory
// store with a logged error, as storeChannelsFor does.
func storeRecordsFor(st store.Store) records.Store {
	db := pgstore.HandleFor(st)
	if db == nil {
		slog.Error("records: store is not a pgstore; records are kept in memory and lost on restart")
		return memoryRecords()
	}
	s, err := pgrecords.New(db)
	if err != nil { // coverage-ignore: unreachable: db is non-nil here
		panic(err)
	}
	return s
}
