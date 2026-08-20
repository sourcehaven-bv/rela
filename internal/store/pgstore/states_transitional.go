package pgstore

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Content-state support (TKT-DOFYR1) lands in pgstore in PR-B: migration
// 0011 adds the compound `(id, pointer)` entities PK and the
// `relations.from_pointer` column. Until then this backend FAILS CLOSED
// on anything state-shaped: silently dropping a Pointer/FromPointer on
// write would be data loss, and pretending to read states would be a
// lie. Rejection is safe in practice — no pg deployment can carry a
// pointered metamodel before Step 2 exists — and this file is deleted
// by PR-B together with the transitional storetest Capabilities.States
// flag.
//
// TODO(TKT-DOFYR1-PR-B): delete this file; implement states natively.

// errStatesNotSupported is the transitional refusal. It names the fix
// so an operator hitting it knows the state of the world.
func errStatesNotSupported(op string) error {
	return fmt.Errorf("pgstore: %s: content states are not supported by this backend yet (TKT-DOFYR1 PR-B)", op)
}

// GetEntityState implements store.EntityReader. The zero pointer is the
// default state and delegates to GetEntity; any other pointer cannot
// exist in this backend yet.
func (s *Store) GetEntityState(ctx context.Context, id string, p entity.Pointer) (*entity.Entity, error) {
	if p.IsDefault() {
		return s.GetEntity(ctx, id)
	}
	return nil, errStatesNotSupported("GetEntityState")
}

// rejectPointeredEntity is the fail-closed write guard for the
// transitional window.
func rejectPointeredEntity(op string, e *entity.Entity) error {
	if e != nil && !e.Pointer.IsDefault() {
		return errStatesNotSupported(op)
	}
	return nil
}

// rejectStateQuery fails closed on AllStates reads: answering "here is
// the raw storage truth" while silently filtering to default rows would
// be exactly the lie this file exists to prevent — and the first
// AllStates consumers (undeclared-pointer detection, observer backfill)
// are integrity checks, where a quiet wrong answer is worse than an
// error.
func rejectStateQuery(op string, q store.EntityQuery) error {
	if q.AllStates {
		return errStatesNotSupported(op + " (AllStates)")
	}
	return nil
}
