//go:build postgres

package appbuild

import (
	"context"
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
// Nil: never returned. A store that is not a pgstore gets an in-memory
// store with a logged error, as storePilesFor does. The cache directory is
// not used: it is node-local, so processes sharing the database would each
// see their own channels and their own seq.
func storeChannelsFor(st store.Store) channels.Store {
	db := pgstore.HandleFor(st)
	if db == nil {
		slog.Error("channels: store is not a pgstore; channels are kept in memory and lost on restart")
		return memoryChannels()
	}
	s, err := pgchannels.New(db)
	if err != nil { // coverage-ignore: unreachable: db is non-nil here
		panic(err)
	}
	return s
}

// storeLeaderLockFor returns the realtime worker lock on st's database: a
// keyed advisory lock tried once per run (see [pgstore.TryKeyedLock]), so
// every rela-server process on the schema contends on one lock and none
// waits on it.
//
// Nil: returned when st is not a pgstore; the caller falls back to a lock
// file.
func storeLeaderLockFor(st store.Store) leaderLock {
	if pgstore.HandleFor(st) == nil {
		return nil
	}
	return func(ctx context.Context) (func(), bool, error) {
		return pgstore.TryKeyedLock(ctx, st, realtimeWorkerLockKey)
	}
}
