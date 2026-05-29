package dataentry

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// policyResolver adapts an [affordances.Resolver] to the dataentry
// [FieldVerdictResolver] interface. The affordances package returns
// its own verdict types (it does not import dataentry, keeping the
// dependency one-way); this adapter maps them onto the wire-shape
// verdict types the serializer consumes. Deny attribution rides on
// the verdict itself — the write path reads it from the freshly
// computed verdict and stamps it into the audit Summary (DR-C5),
// so there is no cross-request side table.
type policyResolver struct {
	inner *affordances.PolicyResolver
}

// FieldVerdicts maps affordances field verdicts onto the wire-shape
// type, carrying the deny attribution through unchanged.
func (p *policyResolver) FieldVerdicts(ctx context.Context, e *entityPkg.Entity) FieldVerdicts {
	v := p.inner.FieldVerdicts(ctx, e)
	return FieldVerdicts{
		Writable:    v.Writable,
		Visible:     v.Visible,
		Options:     v.Options,
		Attribution: v.Attribution,
	}
}

// RelationVerdicts maps affordances relation verdicts onto the
// wire-shape type. Each relation type's per-dimension attribution
// ("create" / "remove" / "fields.<name>") is preserved.
func (p *policyResolver) RelationVerdicts(ctx context.Context, e *entityPkg.Entity) RelationVerdicts {
	v := p.inner.RelationVerdicts(ctx, e)
	if len(v.Types) == 0 {
		return RelationVerdicts{}
	}
	out := RelationVerdicts{Types: make(map[string]RelationVerdict, len(v.Types))}
	for rt, rv := range v.Types {
		out.Types[rt] = RelationVerdict{
			Creatable:   rv.Creatable,
			Removable:   rv.Removable,
			Fields:      rv.Fields,
			Attribution: rv.Attribution,
		}
	}
	return out
}

// storeRelationLookup implements [affordances.RelationLookup] against a
// store snapshot. It is built per resolver and queries the live store;
// the resolver itself snapshots once per call by virtue of being
// invoked once per per-entity GET (the App captures appState.Load() at
// the top of each handler).
type storeRelationLookup struct {
	st store.Store
}

// OutgoingCounts tallies outgoing edges by type in a single scan.
func (l storeRelationLookup) OutgoingCounts(ctx context.Context, fromID string) map[string]int {
	counts := map[string]int{}
	for rel, err := range l.st.ListRelations(ctx, store.RelationQuery{
		From: fromID, Direction: store.DirectionOutgoing,
	}) {
		if err != nil || rel == nil {
			continue
		}
		counts[rel.Type]++
	}
	return counts
}

func (l storeRelationLookup) HasEdge(ctx context.Context, fromID, relType, toID string) bool {
	for rel, err := range l.st.ListRelations(ctx, store.RelationQuery{
		From: fromID, Type: relType, To: toID, Direction: store.DirectionOutgoing,
	}) {
		if err != nil || rel == nil {
			continue
		}
		return true
	}
	return false
}
