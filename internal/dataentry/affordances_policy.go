package dataentry

import (
	"context"
	"maps"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/search"
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

// PrimeTraversals implements [traversalPrimer].
func (p *policyResolver) PrimeTraversals(ctx context.Context, rows []*entityPkg.Entity) context.Context {
	return p.inner.PrimeTraversals(ctx, rows)
}

// PrimeFlowRuns implements [flowRunPrimer].
func (p *policyResolver) PrimeFlowRuns(ctx context.Context, rows []*entityPkg.Entity) context.Context {
	return affordances.PrimeFlowRuns(ctx, p.inner, rows)
}

// FieldVerdicts maps affordances field verdicts onto the wire-shape
// type, carrying the deny attribution through unchanged.
func (p *policyResolver) FieldVerdicts(ctx context.Context, e *entityPkg.Entity) FieldVerdicts {
	return fieldVerdictsWire(p.inner.FieldVerdicts(ctx, e))
}

// CreateFieldVerdicts maps [affordances.PolicyResolver.CreateFieldVerdicts]
// like FieldVerdicts, making policyResolver satisfy
// [createFieldVerdictResolver].
func (p *policyResolver) CreateFieldVerdicts(ctx context.Context, e *entityPkg.Entity) FieldVerdicts {
	return fieldVerdictsWire(p.inner.CreateFieldVerdicts(ctx, e))
}

// fieldVerdictsWire maps affordances field verdicts onto the wire-shape type,
// carrying the deny attribution through unchanged.
func fieldVerdictsWire(v affordances.FieldVerdicts) FieldVerdicts {
	return FieldVerdicts{
		Writable:    v.Writable,
		Visible:     v.Visible,
		Options:     v.Options,
		Attribution: v.Attribution,
	}
}

// RelationVerdicts maps [affordances.PolicyResolver.RelationVerdicts] onto
// the wire-shape type.
func (p *policyResolver) RelationVerdicts(ctx context.Context, e *entityPkg.Entity) RelationVerdicts {
	return relationVerdictsWire(p.inner.RelationVerdicts(ctx, e))
}

// UnconditionalRelationVerdicts maps
// [affordances.PolicyResolver.UnconditionalRelationVerdicts] like
// RelationVerdicts.
func (p *policyResolver) UnconditionalRelationVerdicts(ctx context.Context, e *entityPkg.Entity) RelationVerdicts {
	return relationVerdictsWire(p.inner.UnconditionalRelationVerdicts(ctx, e))
}

// FieldVisibleOnEveryRow delegates to
// [affordances.PolicyResolver.FieldVisibleOnEveryRow].
func (p *policyResolver) FieldVisibleOnEveryRow(ctx context.Context, entityType, property string) bool {
	return p.inner.FieldVisibleOnEveryRow(ctx, entityType, property)
}

// SearchFieldScope delegates to [affordances.PolicyResolver.SearchFieldScope].
func (p *policyResolver) SearchFieldScope(ctx context.Context, entityType string) (*search.FieldScope, error) {
	return p.inner.SearchFieldScope(ctx, entityType)
}

// relationVerdictsWire wraps the resolver's relation verdicts, which
// dataentry uses as they are; only the map is copied.
func relationVerdictsWire(v affordances.RelationVerdicts) RelationVerdicts {
	if len(v.Types) == 0 {
		return RelationVerdicts{}
	}
	return RelationVerdicts{Types: maps.Clone(v.Types)}
}

// RelationMetaRestricted implements [relationMetaRestrictor].
func (p *policyResolver) RelationMetaRestricted(ctx context.Context, from *entityPkg.Entity, relType string) bool {
	return affordances.RelationMetaRestricted(ctx, p.inner, from, relType)
}

// RelationFieldVerdicts forwards to the wrapped policy resolver, making
// policyResolver satisfy [RelationVisibilityResolver] (TKT-B1F5Q1). Only the
// policy-backed resolver implements this — Nop / Demo do not, so relation meta
// is emitted un-redacted under them.
func (p *policyResolver) RelationFieldVerdicts(
	ctx context.Context, from *entityPkg.Entity, relType string, metaKeys []string,
) map[string]bool {
	return p.inner.RelationFieldVerdicts(ctx, from, relType, metaKeys)
}

// TransitionVerdicts forwards to the wrapped policy resolver, making
// policyResolver satisfy [TransitionResolver] (TKT-3G93B8). The affordances
// resolver returns transition verdicts directly (no wire-shape mapping here);
// the serializer maps them to v1.Transition. Only the policy-backed resolver
// implements this — Nop / Demo do not, so `_transitions` is absent under them.
func (p *policyResolver) TransitionVerdicts(
	ctx context.Context, e *entityPkg.Entity,
) map[string][]flowcfg.TransitionVerdict {
	return p.inner.TransitionVerdicts(ctx, e)
}

// EntryValues forwards to the wrapped policy resolver so policyResolver
// satisfies the create-lock half of [TransitionResolver] (TKT-3G93B8).
func (p *policyResolver) EntryValues(entityType string) map[string]string {
	return p.inner.EntryValues(entityType)
}

// storeRelationLookup implements [affordances.RelationLookup] against a
// store snapshot. It is built per resolver and queries the live store;
// the resolver itself snapshots once per call by virtue of being
// invoked once per per-entity GET (the App captures appState.Load() at
// the top of each handler).
// NOTE: internal/appbuild/relationlookup.go holds a deliberate copy of this
// adapter (appbuild cannot import dataentry). The two are behaviourally
// identical and both feed affordance `when:` predicates — fix bugs in BOTH or
// the two surfaces will disagree about who may see what.
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
