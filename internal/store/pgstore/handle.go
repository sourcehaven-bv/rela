package pgstore

import (
	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// HandleFor returns st's database handle, or nil if st is not a pgstore. A
// pgstore behind [store.Decorator] layers counts as one.
//
// It is for services that keep their own tables in the tenant's schema but are
// not store concerns (internal/piles/pgpiles), so the composition root can hand
// them the same pool the store runs on from the store alone. Deriving it from
// the store rather than from the recipe's pool makes every assembly path agree,
// including [store.Store] values assembled through appbuild.SharedBase.
//
// A package function rather than a method on [Store], for the reason
// [StateStoreFor] gives.
func HandleFor(st store.Store) DBTX {
	s, ok := store.Innermost(st).(*Store)
	if !ok {
		return nil
	}
	return unwrapDB(s.db)
}

// TxHandleFor returns the transaction of view, a view [Store.Tx] handed its
// callback, as a handle. A service that joins the transaction writes on it,
// and its writes commit or roll back with the store's.
//
// Nil: returned when view is not a pgstore transaction view, such as the
// store itself, whose handle is the pool.
func TxHandleFor(view store.Store) DBTX {
	h := HandleFor(view)
	if _, ok := h.(pgx.Tx); !ok {
		return nil
	}
	return h
}
