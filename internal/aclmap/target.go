package aclmap

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ErrFaceAddress is returned when an entity-level report is given an
// `ID@face` address. Can and WhoCan decide per entity type and id, so a face
// in the address would not change the answer; accepting it would attest to a
// face-specific question the report does not ask.
var ErrFaceAddress = errors.New("aclmap: this report is per entity; pass the bare id")

// target resolves the entity an access question names. addr is `ID` or
// `ID@face`. The type comes from one content-free read of every stored face,
// so a faced entity, which has no row at the zero face (DEC-NPZICR), is found
// by its bare id. A named face must exist. A missing entity or face is
// [ErrEntityNotFound].
func (e *Engine) target(ctx context.Context, addr string) (resolvedTarget, error) {
	ref, err := entity.ParseRef(addr)
	if err != nil {
		return resolvedTarget{}, fmt.Errorf("aclmap: invalid entity address %q: %w", addr, err)
	}
	t := resolvedTarget{ref: ref}
	faceFound := false
	headers, err := store.FamilyHeaders(ctx, e.src, ref.ID)
	if err != nil {
		return resolvedTarget{}, fmt.Errorf("aclmap: load entity %q: %w", ref.ID, err)
	}
	for _, h := range headers {
		t.typ = h.Type
		t.faces = append(t.faces, h.Face)
		if h.Face == ref.Face {
			faceFound = true
		}
	}
	if t.typ == "" || (!ref.Face.IsImplicit() && !faceFound) {
		return resolvedTarget{}, fmt.Errorf("%w: %s", ErrEntityNotFound, addr)
	}
	return t, nil
}

// resolvedTarget is the entity an address names: the parsed address, the
// family's type, and every face the family stores.
type resolvedTarget struct {
	ref   entity.Ref
	typ   string
	faces []entity.Face
}

// familyFaces lists the stored faces of a faced family, and nil for a
// faceless one, whose only row is the zero face.
func (t resolvedTarget) familyFaces() []entity.Face {
	for _, f := range t.faces {
		if !f.IsImplicit() {
			return t.faces
		}
	}
	return nil
}

// entityTarget is Engine.target for a report that answers per entity: an
// `ID@face` address is refused with [ErrFaceAddress].
func (e *Engine) entityTarget(ctx context.Context, addr string) (id, typ string, err error) {
	t, err := e.target(ctx, addr)
	if err != nil {
		return "", "", err
	}
	if !t.ref.Face.IsImplicit() {
		return "", "", fmt.Errorf("%w: %s", ErrFaceAddress, t.ref.ID)
	}
	return t.ref.ID, t.typ, nil
}
