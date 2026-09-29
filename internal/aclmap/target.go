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
func (e *Engine) target(ctx context.Context, addr string) (entity.Ref, string, error) {
	ref, err := entity.ParseRef(addr)
	if err != nil {
		return entity.Ref{}, "", fmt.Errorf("aclmap: invalid entity address %q: %w", addr, err)
	}
	var (
		typ       string
		faceFound bool
	)
	q := store.EntityQuery{IDs: []string{ref.ID}, AllStates: true}
	for h, err := range store.ListEntityHeaders(ctx, e.src, q) {
		if err != nil {
			return entity.Ref{}, "", fmt.Errorf("aclmap: load entity %q: %w", ref.ID, err)
		}
		if h.ID != ref.ID {
			continue
		}
		typ = h.Type
		if h.Face == ref.Face {
			faceFound = true
		}
	}
	if typ == "" || (!ref.Face.IsDefault() && !faceFound) {
		return entity.Ref{}, "", fmt.Errorf("%w: %s", ErrEntityNotFound, addr)
	}
	return ref, typ, nil
}

// entityTarget is Engine.target for a report that answers per entity: an
// `ID@face` address is refused with [ErrFaceAddress].
func (e *Engine) entityTarget(ctx context.Context, addr string) (id, typ string, err error) {
	ref, typ, err := e.target(ctx, addr)
	if err != nil {
		return "", "", err
	}
	if !ref.Face.IsDefault() {
		return "", "", fmt.Errorf("%w: %s", ErrFaceAddress, ref.ID)
	}
	return ref.ID, typ, nil
}
