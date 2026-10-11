package pgstore

import "github.com/Sourcehaven-BV/rela/internal/store"

// HandleFor returns st's database handle, or nil if st is not a pgstore.
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
	s, ok := st.(*Store)
	if !ok {
		return nil
	}
	return s.db
}
