//go:build postgres

package appbuild

import (
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
	"github.com/Sourcehaven-BV/rela/internal/channels/pgchannels"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// txChannelsFor returns how a promote joins the channel store to a
// transaction of st: a pgchannels store over the transaction's handle,
// whose own transactions become savepoints.
//
// Nil: returned when st is not a pgstore or cs is the file channel store,
// as the in-memory fallback is; the promote is then not atomic.
func txChannelsFor(st store.Store, cs channels.Store) joinChannels {
	if _, file := cs.(*filechannels.Store); file || pgstore.HandleFor(st) == nil {
		return nil
	}
	return func(view store.Store) (channels.Store, error) {
		db := pgstore.HandleFor(view)
		if db == nil {
			return nil, errNotTxView
		}
		return pgchannels.New(db)
	}
}

// errNotTxView is a join of a store that is not a transaction view of the
// database store. It rolls the promote back.
var errNotTxView = errors.New("appbuild: promote: the transaction view is not a database store")
