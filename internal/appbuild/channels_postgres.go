//go:build postgres

package appbuild

import (
	"context"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
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

// storeHintsFor returns NOTIFY as the hint transport when cs is the
// database channel store over st, so every rela-server process on the
// schema hears the channel changes of the others.
//
// Nil: returned when st is not a pgstore, whose channels then live in this
// process only.
func storeHintsFor(cs channels.Store, st store.Store) hintEndpoint {
	// The fallback in-memory store is a file store; the database one is not.
	if _, mem := cs.(*filechannels.Store); mem || pgstore.HandleFor(st) == nil {
		return nil
	}
	return pgHints{st: st}
}

// pgHints carries channel hints on the store's [pgstore.ChannelHints]
// NOTIFY channel.
type pgHints struct{ st store.Store }

func (h pgHints) SendHint(ctx context.Context, payload []byte) error {
	return pgstore.Notify(ctx, h.st, pgstore.ChannelHints, payload)
}

func (h pgHints) FollowHints(ctx context.Context, deliver func([]byte), gap func()) error {
	cancel, err := pgstore.Listen(h.st, pgstore.ChannelHints, deliver, gap)
	if err != nil {
		return err
	}
	defer cancel()
	<-ctx.Done()
	return nil
}

// storeLeaderLockFor returns the worker lock named key on st's database: a
// keyed advisory lock (see [pgstore.TryKeyedLock]) that the leader holds
// for as long as it leads, and that the other rela-server processes on the
// schema try now and then without waiting. Its check is a round trip on
// the lock's session, so the leader learns when the database dropped it.
//
// Nil: returned when st is not a pgstore; the caller falls back to a lock
// file.
func storeLeaderLockFor(st store.Store, key string) leaderLock {
	if pgstore.HandleFor(st) == nil {
		return nil
	}
	return func(ctx context.Context) (heldLock, bool, error) {
		l, ok, err := pgstore.TryKeyedLock(ctx, st, key)
		if err != nil || !ok {
			return heldLock{}, ok, err
		}
		return heldLock{release: l.Release, check: l.Check}, true, nil
	}
}

// storeLockRoom reports an error when st's pool cannot hold n worker locks
// at once (see [pgstore.CheckLockRoom]).
func storeLockRoom(st store.Store, n int) error { return pgstore.CheckLockRoom(st, n) }
