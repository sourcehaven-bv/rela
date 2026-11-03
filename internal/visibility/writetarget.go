package visibility

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// AmbiguousAddressError is returned by [Resolver.WriteTarget] when a bare id
// does not pick exactly one face. Faces lists the faces of the entity the
// principal may read, in declaration order, so the caller can name one.
// It never names a face the principal cannot read.
type AmbiguousAddressError struct {
	ID    string
	Faces []entity.Face
}

func (e *AmbiguousAddressError) Error() string {
	addrs := make([]string, len(e.Faces))
	for i, f := range e.Faces {
		addrs[i] = entity.FormatStateRef(e.ID, f)
	}
	return fmt.Sprintf("%s has faces; address one: %s", e.ID, strings.Join(addrs, ", "))
}

// WriteTarget resolves addr to the one face a face-level write edits
// (TKT-7IZHP0 §6). It reads headers only; the write is authorized on the
// returned face by entitymanager, as for any write.
//
// A named face is the target when it exists and the principal may read it
// (A14). Otherwise it is a miss, so a face the principal may write but not
// read answers the same not-found as an absent one, never a 403.
//
// A bare id names the implicit face, the one face of a faceless type. A type
// that declares faces has no implicit face (DEC-NPZICR), so there it is an
// [*AmbiguousAddressError] listing the readable faces, even when only one
// exists: a content write must name the face it changes. Neither a world's
// ranking nor the number of faces that happen to exist picks it, so a write
// that works today does not start failing when a second face is published.
//
// ok=false is the uniform miss: no such entity, the wrong type, no readable
// face, or a denied world.
//
//archplan:guard=authz reason="row-gates the one face a bare-id write may target"
func (r *Resolver) WriteTarget(
	ctx context.Context, w World, entityType string, addr entity.Address,
) (ref entity.Ref, ok bool, err error) {
	if !w.denied && !w.scope.IsSet() {
		return entity.Ref{}, false, fmt.Errorf("%w: visibility: WriteTarget with an unset world (use WorldOf)",
			store.ErrInvalidQuery)
	}
	faces, ok, err := r.admit(ctx, w, entityType, addr.ID())
	if err != nil || !ok {
		return entity.Ref{}, false, err
	}
	headers, ok := r.headersOf(ctx, entityType, addr.ID())
	if !ok {
		return entity.Ref{}, false, nil
	}
	fam, ok, err := r.familyOf(entityType, addr.ID(), faces, headers)
	if err != nil || !ok {
		return entity.Ref{}, false, err
	}
	return pickWriteFace(addr, fam)
}

// writeAddress is the face a write to addr edits: the named face, or the
// implicit face for a bare id. named reports which.
func writeAddress(addr entity.Address) (target entity.Ref, named bool) {
	if ref, ok := addr.Named(); ok {
		return ref, true
	}
	return entity.Ref{ID: addr.ID(), Face: entity.ImplicitFace}, false
}

// pickWriteFace applies the write-address rule ([Resolver.WriteTarget]) to
// the faces of addr the principal may read. A named face outside fam is a
// miss. A bare id whose implicit face is outside fam is an
// [*AmbiguousAddressError] listing fam's faces.
func pickWriteFace(addr entity.Address, fam Family) (entity.Ref, bool, error) {
	target, named := writeAddress(addr)
	switch {
	case slices.Contains(fam.Faces, target.Face):
		return target, true, nil
	case named:
		return entity.Ref{}, false, nil
	default:
		return entity.Ref{}, false, &AmbiguousAddressError{ID: addr.ID(), Faces: fam.Faces}
	}
}

// WriteRow resolves addr to the one face a face-level write edits, as
// [Resolver.WriteTarget] does, and reads that face, as [Resolver.Ref] does.
// It answers what the pair answers, in one gate pass and, on a hit, one load
// and no header read.
//
// The face it reads is the target: the named face, or the implicit face for
// a bare id. Only when a bare id's implicit face is not there to read does
// it read the family headers, to tell a miss from an
// [*AmbiguousAddressError] listing the readable faces.
//
// The row is gated and redacted like every resolver read. It is the
// caller's view of the target, never a write-prep base: the write itself
// goes through entitymanager, which reads the raw stored row.
//
//archplan:guard=authz reason="row-gates and loads the face a write edits"
func (r *Resolver) WriteRow(
	ctx context.Context, w World, entityType string, addr entity.Address,
) (Resolved, bool, error) {
	if !w.denied && !w.scope.IsSet() {
		return Resolved{}, false, fmt.Errorf("%w: visibility: WriteRow with an unset world (use WorldOf)",
			store.ErrInvalidQuery)
	}
	faces, ok, err := r.admit(ctx, w, entityType, addr.ID())
	if err != nil || !ok {
		return Resolved{}, false, err
	}
	target, named := writeAddress(addr)
	if faces.Contains(target.Face) {
		// serve checks the loaded row's type. That stands for the whole
		// family because every store keeps one type per id across its faces
		// (storetest "TypeMismatchRejected"), so the per-header type check
		// in familyOf has nothing more to find here.
		if e, loaded := r.loadRef(ctx, entityType, target); loaded {
			return r.serve(ctx, w, entityType, faces, e)
		}
	}
	if named {
		return Resolved{}, false, nil
	}
	headers, ok := r.headersOf(ctx, entityType, addr.ID())
	if !ok {
		return Resolved{}, false, nil
	}
	fam, ok, err := r.familyOf(entityType, addr.ID(), faces, headers)
	if err != nil || !ok {
		return Resolved{}, false, err
	}
	// A target the rule picks but the load did not return failed to load;
	// that is a miss, like every other failed load. Otherwise this is the
	// rule's ambiguity error.
	_, _, err = pickWriteFace(addr, fam)
	return Resolved{}, false, err
}
