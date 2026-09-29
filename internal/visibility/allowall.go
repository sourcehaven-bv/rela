package visibility

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
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
// Its single-entity read is [NewAllowAllResolver] over the same loader,
// which keeps the one non-pass-through behavior: a type-mismatched claim is
// a miss. That check is part of the read contract (RR-SRZK6X), not of
// policy, so every implementation enforces it identically.
type AllowAllReader struct {
	res *Resolver
}

// NewAllowAllReader builds an AllowAllReader over load (required).
func NewAllowAllReader(load Loader) (*AllowAllReader, error) {
	res, err := NewAllowAllResolver(load)
	if err != nil {
		return nil, fmt.Errorf("visibility: NewAllowAllReader: %w", err)
	}
	return &AllowAllReader{res: res}, nil
}

// Resolver returns the ungated single-entity read over this reader's loader.
func (r *AllowAllReader) Resolver() *Resolver { return r.res }

// Filter implements [Reader]: pass-through, input returned unchanged.
func (r *AllowAllReader) Filter(_ context.Context, candidates []*entity.Entity) []*entity.Entity {
	return candidates
}

// FilterHeaders implements [HeaderFilterer]: pass-through, input returned
// unchanged — the same allow-all capability [AllowAllReader.Filter] grants, applied to
// content-free headers.
func (r *AllowAllReader) FilterHeaders(
	_ context.Context, candidates []store.EntityHeader,
) []store.EntityHeader {
	return candidates
}

// FilterRelations implements [Reader]: pass-through, input returned
// unchanged.
func (r *AllowAllReader) FilterRelations(_ context.Context, rels []*entity.Relation) []*entity.Relation {
	return rels
}

// FilterRelationsStrict implements [Reader]: pass-through, never an error.
func (r *AllowAllReader) FilterRelationsStrict(
	_ context.Context, rels []*entity.Relation,
) ([]*entity.Relation, error) {
	return rels, nil
}
