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
	if named, isNamed := addr.Named(); isNamed {
		if !slices.Contains(fam.Faces, named.Face) {
			return entity.Ref{}, false, nil
		}
		return named, true, nil
	}
	if !slices.ContainsFunc(fam.Faces, entity.Face.IsImplicit) {
		return entity.Ref{}, false, &AmbiguousAddressError{ID: addr.ID(), Faces: fam.Faces}
	}
	return entity.Ref{ID: addr.ID(), Face: entity.ImplicitFace}, true, nil
}
