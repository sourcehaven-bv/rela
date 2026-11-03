package appbuild

import "github.com/Sourcehaven-BV/rela/internal/metamodel"

// storeSchemaRefresher applies a metamodel's entity types to an open
// store. Only the markdown store needs one: it lays files out per type
// and refuses writes of a type it was not told about, so a schema reload
// must reach it. The database stores keep any type and need none.
//
// Supplied by the fs recipe from the store it opened, and carried to
// every re-assembly by [SharedBase.ForReassembly].
type storeSchemaRefresher func(*metamodel.Metamodel) error

// RefreshStoreSchema applies svc's metamodel to its store. A host that
// re-assembles services against an open store on a schema change (see
// [SharedBase.ForReassembly]) calls it once the new services are in
// force, so the store accepts the types the new schema declares. On
// error the store keeps its previous schema.
//
// Nil: rejected — svc is dereferenced.
func RefreshStoreSchema(svc *Services) error {
	if svc.storeSchema == nil {
		return nil
	}
	return svc.storeSchema(svc.meta)
}
