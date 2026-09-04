package dataentry

import (
	"context"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// historyFace resolves WHICH FACE of an entity the request's world is asking
// about, and reports whether a face-scoped read is possible at all (BUG-2).
//
// # Why history needs this
//
// Content versioning is PER-FACE: `entity_versions` is keyed by the content
// state and TKT-C1XUA8 added per-face capture, so a draft and its published
// face have genuinely different histories. A world-bound page that showed the
// DEFAULT face's history was not merely dropping a query param — it presented
// the wrong record as the right one, with nothing on screen naming the face it
// belonged to.
//
// # The face is read back, never re-derived
//
// The face is taken from the entity the world RESOLVED, exactly as
// [worldProvenance] takes it: the store did the resolution, this reads the
// answer. Re-walking the chain here would be a second implementation of the
// semantics that decide which face a reader sees, free to drift from the
// store's — the mistake this arc has now refused in four places.
//
// # Absence and denial are the caller's problem, deliberately
//
// This does NOT gate. `authorizeHistoryRead` runs before it and applies the
// same world-independent row gate a live GET applies (guard rule 1), so a
// caller reaching here is already cleared to read the entity. What this
// reports is only which coordinate to read, plus `ok=false` when the world
// resolves NO face — in which case there is no history to show, because in
// that world the entity has no content state at all.
//
// Returns the zero face under the default world, which IS the default
// face's coordinate — so the default-world call is byte-identical to the
// pre-BUG-2 [store.HistoryReader] read.
//
// # Deleted entities
//
// The default-world arm returns before touching the store, which is what keeps
// DELETED-entity history working: such an entity has surviving versions but no
// live row, and `authorizeHistoryRead` admits it on the global
// acl.PermHistoryRead. Probing the store for it would report `ok=false` and
// silently empty a timeline the caller is entitled to.
//
// Under a NON-default world the same probe reports absence, and that is the
// right answer rather than the same bug: a world resolves faces of live rows,
// so a deleted entity has no face in one. Its history is reachable by asking
// for it without a world — which is also the only spelling under which "the
// history of a thing that no longer exists" is well-defined.
func historyFace(
	ctx context.Context, st store.Store, entityID string,
) (p entityPkg.Face, ok bool, err error) {
	h := worldFromContext(ctx)
	if h.denied {
		// Same answer as a permitted world holding no face of this entity —
		// an empty timeline — so the denial is not distinguishable here
		// either. The handle's scope is the zero scope, so without this a
		// denied world served the DEFAULT face's history.
		return "", false, nil
	}
	scope := h.scope
	if scope.IsDefaultWorld() {
		return "", true, nil
	}
	for e, ierr := range st.ListEntities(ctx, store.EntityQuery{
		IDs:   []string{entityID},
		World: scope,
	}) {
		if ierr != nil {
			// An infrastructure failure is NOT "this entity has no face in
			// this world" (RR-4TFZNL) — reporting it as absence would render a
			// backend outage as an empty timeline.
			return "", false, ierr
		}
		return e.Face, true, nil
	}
	return "", false, nil
}

// faceHistoryReader narrows a history reader to ONE FACE, or returns the
// unscoped reader when the request is in the default world.
//
// The face-scoped capability ([store.StateHistoryReader]) is OPTIONAL and
// asserted separately from [store.HistoryReader], mirroring how every other
// optional store capability is reached. A backend that captures per-face
// versions but does not implement the face-scoped reader would otherwise
// silently serve the default face's history under a world — the exact
// wrong-record failure BUG-2 is about — so the assertion FAILS the request
// rather than falling back.
//
// Nil: never returned with a nil error.
func faceHistoryReader(
	reader store.HistoryReader, p entityPkg.Face,
) (store.HistoryReader, bool) {
	if p == "" {
		// The default face. The zero-face call IS HistoryReader (see the
		// StateHistoryReader doc), so there is nothing to narrow.
		return reader, true
	}
	sh, ok := reader.(store.StateHistoryReader)
	if !ok {
		return nil, false
	}
	return stateHistoryAdapter{sh: sh, p: p}, true
}

// stateHistoryAdapter presents one FACE of an entity's history through the
// unscoped [store.HistoryReader] shape, so the timeline and snapshot handlers
// need no world-aware branch of their own.
//
// Binding the face at construction rather than threading it through every
// call site is what keeps the handlers from being able to forget it: once a
// handler holds one of these, every read it makes is face-scoped by
// construction.
type stateHistoryAdapter struct {
	sh store.StateHistoryReader
	p  entityPkg.Face
}

func (a stateHistoryAdapter) ListVersions(
	ctx context.Context, id string,
) ([]store.VersionMeta, error) {
	return a.sh.ListStateVersions(ctx, id, a.p)
}

func (a stateHistoryAdapter) GetVersion(
	ctx context.Context, id string, version int,
) (*store.VersionSnapshot, error) {
	return a.sh.GetStateVersion(ctx, id, a.p, version)
}
