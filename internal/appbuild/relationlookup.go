package appbuild

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// storeRelationLookup adapts a [store.Store] to
// [affordances.RelationLookup], the read surface the affordance
// predicate host functions (has_relation, count_relations) call.
//
// It holds the store rather than a snapshot: the resolver is built once at
// wiring time but answers per-request, so a captured snapshot would go stale.
// Each call scans the store directly, which is why the resolver is only ever
// consulted per-entity rather than per-row of a list.
//
// This mirrors the adapter internal/dataentry has used since affordances
// landed; it lives here too so the appbuild-wired paths can build a resolver
// without importing dataentry (TKT-0XL8MF).
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

// HasEdge reports whether the (from, type, to) triple exists.
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
