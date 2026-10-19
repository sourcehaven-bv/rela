//go:build sqlite && !postgres

package appbuild

import (
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
	"github.com/Sourcehaven-BV/rela/internal/channels/sqlitechannels"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// txChannelsFor returns how a promote joins the channel store to a
// transaction of st: a sqlitechannels store on the transaction's
// connection, which writes under a savepoint.
//
// Nil: returned when st is not a sqlitestore or cs is the file
// channel store; the promote is then not atomic.
func txChannelsFor(st store.Store, cs channels.Store) joinChannels {
	_, file := cs.(*filechannels.Store)
	if _, db := store.Innermost(st).(*sqlitestore.Store); file || !db {
		return nil
	}
	return func(view store.Store) (channels.Store, error) {
		conn, ok := sqlitestore.TxConn(store.Innermost(view))
		if !ok {
			return nil, errNotTxView
		}
		return sqlitechannels.NewInTx(conn)
	}
}

// errNotTxView is a join of a store that is not a transaction view of the
// database store. It rolls the promote back.
var errNotTxView = errors.New("appbuild: promote: the transaction view is not a database store")
