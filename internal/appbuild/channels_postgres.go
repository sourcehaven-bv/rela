//go:build postgres

package appbuild

import (
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/pgchannels"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// storeChannelsFor returns the database channel store over st's pool, so
// every rela-server process on the schema sees one set of channels and one
// gapless seq per channel.
//
// Nil: returned for a store that is not a pgstore, which then falls back to
// the file backend in the cache directory, with a logged error because that
// backend is node-local.
func storeChannelsFor(st store.Store) channels.Store {
	db := pgstore.HandleFor(st)
	if db == nil {
		slog.Error("channels: store is not a pgstore; channels fall back to the cache directory")
		return nil
	}
	s, err := pgchannels.New(db)
	if err != nil { // coverage-ignore: unreachable: db is non-nil here
		panic(err)
	}
	return s
}
