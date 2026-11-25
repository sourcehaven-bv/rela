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

// resolveHistorySubject decides which face lineage the address names and whether
// this principal may read it; ok=false is the uniform not-found, an error a gate
// failure. A live face resolves through the [visibility.Resolver] as the entity
// GET does, and an unreadable live entity is the same 404 as a never-stored id.
// A face with no live row needs the global acl.PermHistoryRead plus the face
// read grant, and, while siblings live, the row gate on one of them. A failed
// stored-faces read is an error, never "deleted". A bare id with no readable
// face in a non-default world is worldAbsent when some face is readable.
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
	bareInWorld := ref.Face.IsImplicit() && !world.ranksNothing()
	storedType, stored, err := loadStoredFaces(ctx, vr.store, ref.ID)
	if err != nil {
		return historySubject{}, false, err
	}
	if len(stored) > 0 && bareInWorld {
		_, readable, ferr := vr.family(ctx, typeName, ref.ID)
		if ferr != nil {
			return historySubject{}, false, ferr
		}
		if readable {
			return historySubject{ref: ref, worldAbsent: true}, true, nil
		}
		stored = nil
	}
	if len(stored) > 0 {
		switch {
		case ref.Face.IsImplicit(), storedType != typeName, slices.Contains(stored, ref.Face):
			return historySubject{}, false, nil
		}
		if _, readable, ferr := vr.family(ctx, typeName, ref.ID); ferr != nil || !readable {
			return historySubject{}, false, ferr
		}
	}

	if !readGateFromContext(ctx).HoldsPermission(ctx, acl.PermHistoryRead) {
		return historySubject{}, false, nil
	}
	if bareInWorld {
		return historySubject{ref: ref, worldAbsent: true}, true, nil
	}
	if world.blocksAllReads() {
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
