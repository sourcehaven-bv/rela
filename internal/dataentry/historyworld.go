package dataentry

import (
	"context"
	"net/http"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
)

// historySubject is the face lineage one history request reads, resolved
// once at the top of the request (BUG-4SYAA6).
//
// A lineage is keyed by (id, face): a draft and its published face have
// separate histories. So the subject is an [entityPkg.Ref], never a bare id,
// and every read and write the request makes is scoped to that face.
type historySubject struct {
	// ref is the id and the face whose lineage is read.
	ref entityPkg.Ref
	// live is the face's raw row when it exists and the principal may read
	// it, and nil for a deleted face. It is write-prep for a restore and is
	// never served.
	live *entityPkg.Entity
	// worldAbsent reports that the request's world resolves no face, so
	// there is no timeline in that world. The response is an empty
	// timeline, not a 404: the entity view answers the same question with
	// `_world_absent`.
	worldAbsent bool
}

// resolveHistorySubject decides which face lineage the address names and
// whether this principal may read it. ok=false is the uniform not-found; an
// error is a gate failure.
//
// A live face is resolved by the [visibility.Resolver], exactly as the
// entity GET resolves the same address: a named face literally, a bare id
// through the request's world. The two surfaces therefore agree on what an
// address names and on who may read it, and a hidden face is the same 404 as
// an absent one.
//
// A miss is then split, from a raw header read, into two cases the resolver
// deliberately cannot tell apart:
//
//   - The addressed face is live but the gates refused it: hidden, another
//     type, a denied world, or a bare id of a faced type in the default
//     world. That is the uniform 404.
//   - The face has no live row. A deleted face has no per-entity verdict
//     left to evaluate, so its history needs the global
//     acl.PermHistoryRead plus the face half of the read grant. A
//     non-holder gets the same 404 as for an id that never existed. When
//     sibling faces still live, the caller must also pass the entity's row
//     gate on one of them.
//
// A failed stored-faces read is an error, never "deleted": the deleted-face
// rule grants on a global permission, so a read fault must not reach it.
//
// A bare id in a non-default world that resolves no readable face is
// worldAbsent when the principal may read some face of the entity: the
// entity exists for this caller, it just has no face in this world. A denied
// world answers the same way, so a denial looks like an empty world.
func resolveHistorySubject(
	ctx context.Context, vr visibleReader, typeName string, ref entityPkg.Ref,
) (historySubject, bool, error) {
	e, ok, err := vr.addressRef(ctx, typeName, ref)
	if err != nil {
		return historySubject{}, false, err
	}
	if ok {
		return historySubject{ref: e.Ref(), live: e}, true, nil
	}

	world := worldFromContext(ctx)
	bareInWorld := ref.Face.IsDefault() && !world.isDefault()
	storedType, stored, err := loadStoredFaces(ctx, vr.store, ref.ID)
	if err != nil {
		return historySubject{}, false, err
	}
	if len(stored) > 0 {
		switch {
		case bareInWorld:
			if _, readable, ferr := vr.family(ctx, typeName, ref.ID); ferr != nil || !readable {
				return historySubject{}, false, ferr
			}
			return historySubject{ref: ref, worldAbsent: true}, true, nil
		case ref.Face.IsDefault(), storedType != typeName, slices.Contains(stored, ref.Face):
			return historySubject{}, false, nil
		}
		// A named face that was deleted while its siblings live on takes
		// the deleted-face rule below, and the entity still has a row gate
		// to evaluate: the caller must be able to read one of its live
		// faces, as for any other read of a live entity.
		if _, readable, ferr := vr.family(ctx, typeName, ref.ID); ferr != nil || !readable {
			return historySubject{}, false, ferr
		}
	}

	if !readGateFromContext(ctx).HoldsPermission(ctx, acl.PermHistoryRead) {
		return historySubject{}, false, nil
	}
	if bareInWorld {
		// A world resolves faces of live rows, so a deleted entity has none
		// in it. Its history is read by naming the face, or without a world.
		return historySubject{ref: ref, worldAbsent: true}, true, nil
	}
	if world.blocksAllReads() {
		// A named face in a denied world is refused, as the live face is.
		// Answering a deleted one differently would tell the two apart.
		return historySubject{}, false, nil
	}
	if !faceReadable(ctx, typeName, ref.Face) {
		return historySubject{}, false, nil
	}
	return historySubject{ref: ref}, true, nil
}

// historySubjectOr404 is [resolveHistorySubject] that writes the response
// itself on a miss or a gate error.
func historySubjectOr404(
	w http.ResponseWriter, r *http.Request, vr visibleReader, typeName string, ref entityPkg.Ref,
) (historySubject, bool) {
	subject, ok, err := resolveHistorySubject(r.Context(), vr, typeName, ref)
	if err != nil {
		writeGateError(w, r, err)
		return historySubject{}, false
	}
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return historySubject{}, false
	}
	return subject, true
}
