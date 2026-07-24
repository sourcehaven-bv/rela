package visibility

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// AllowAllReader is the explicit pass-through [Reader] capability for
// system jobs (schedulers, reindexers) that legitimately read the whole
// graph. No gate, no redaction, no copying — outputs are byte-identical
// to raw store reads, and returned entities must be treated read-only
// (same contract raw reads carry today).
//
// This is a CAPABILITY handed out at a wiring site, never inferred from
// the principal: the job keeps its genuine system principal for audit
// while this reader grants the visibility (DEC-ZBI39P; the read-side
// analog of WriteDeps.ElevatedManager, TKT-D8T148). Wire it deliberately
// and visibly.
//
// The one non-pass-through behavior it keeps is the [Reader.Get]
// stored-type contract: a type-mismatched claim is a miss. That check is
// part of Reader semantics (RR-SRZK6X), not of policy, so every
// implementation enforces it identically.
type AllowAllReader struct {
	get EntityGetter
}

// NewAllowAllReader builds an AllowAllReader over get (required).
func NewAllowAllReader(get EntityGetter) (*AllowAllReader, error) {
	if get == nil {
		return nil, errors.New("visibility: NewAllowAllReader: get must be non-nil")
	}
	return &AllowAllReader{get: get}, nil
}

// Get implements [Reader]: plain load plus the stored-type check.
func (r *AllowAllReader) Get(ctx context.Context, entityType, id string) (*entity.Entity, bool, error) {
	e, err := r.get.GetEntity(ctx, id)
	if err != nil {
		return nil, false, nil //nolint:nilerr // store miss == not-found, by design
	}
	if e.Type != entityType {
		return nil, false, nil
	}
	return e, true, nil
}

// Filter implements [Reader]: pass-through, input returned unchanged.
func (r *AllowAllReader) Filter(_ context.Context, candidates []*entity.Entity) []*entity.Entity {
	return candidates
}

// FilterRelations implements [Reader]: pass-through, input returned
// unchanged.
func (r *AllowAllReader) FilterRelations(_ context.Context, rels []*entity.Relation) []*entity.Relation {
	return rels
}
