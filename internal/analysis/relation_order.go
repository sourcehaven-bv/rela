package analysis

import (
	"context"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RelationOrderIssue is a soft finding produced by CheckRelationOrder.
// Severity is always "warning" — duplicate/missing managed order values
// don't block writes, they're just inconsistencies the engine tolerates
// per the permissive-storage policy.
type RelationOrderIssue struct {
	EntityID     string
	EntityType   string
	RelationType string
	Side         string // "outgoing" or "incoming"
	Property     string // "_order_out" or "_order_in"
	Kind         string // "duplicate" or "missing"
	Count        int    // number of edges affected on this side under this entity
}

// CheckRelationOrder inspects every orderable relation type and reports
// missing or duplicate values on the managed order property, grouped by
// the parent entity on the relevant side.
//
// Returns warnings only — never errors. Callers map this to whatever
// surface the operator is on (CLI table, MCP JSON, web UI dashboard).
func (s *Service) CheckRelationOrder(ctx context.Context, opts Options) []RelationOrderIssue {
	issues := make([]RelationOrderIssue, 0)
	st := s.deps.Store
	meta := s.deps.Meta

	relNames := make([]string, 0, len(meta.Relations))
	for name := range meta.Relations {
		relNames = append(relNames, name)
	}
	sort.Strings(relNames)

	for _, relName := range relNames {
		relDef := meta.Relations[relName]
		if outProp := relDef.OutgoingOrderProperty(); outProp != "" {
			issues = append(issues, checkOrderSide(ctx, st, meta, relName, outProp, "outgoing", opts.Scope)...)
		}
		if inProp := relDef.IncomingOrderProperty(); inProp != "" {
			issues = append(issues, checkOrderSide(ctx, st, meta, relName, inProp, "incoming", opts.Scope)...)
		}
	}
	return issues
}

func checkOrderSide(
	ctx context.Context, st store.Store, _ *metamodel.Metamodel,
	relType, prop, side string, scope map[string]bool,
) []RelationOrderIssue {
	parents := map[string][]*entity.Relation{}
	for r, err := range st.ListRelations(ctx, store.RelationQuery{Type: relType}) {
		if err != nil {
			continue // iterator errors don't block the rest of the analysis
		}
		var parent string
		if side == "outgoing" {
			parent = r.From
		} else {
			parent = r.To
		}
		if !inScope(parent, scope) {
			continue
		}
		parents[parent] = append(parents[parent], r)
	}

	var issues []RelationOrderIssue
	parentIDs := make([]string, 0, len(parents))
	for id := range parents {
		parentIDs = append(parentIDs, id)
	}
	sort.Strings(parentIDs)

	for _, pid := range parentIDs {
		rels := parents[pid]
		if len(rels) < 2 {
			continue
		}
		seen := map[float64]bool{}
		missing := 0
		duplicate := 0
		for _, r := range rels {
			v, ok := metamodel.FiniteOrder(r.Properties[prop])
			if !ok {
				missing++
				continue
			}
			if seen[v] {
				duplicate++
			}
			seen[v] = true
		}
		etype := ""
		if e, gErr := st.GetEntity(ctx, pid); gErr == nil {
			etype = e.Type
		}
		if missing > 0 {
			issues = append(issues, RelationOrderIssue{
				EntityID: pid, EntityType: etype, RelationType: relType,
				Side: side, Property: prop, Kind: "missing", Count: missing,
			})
		}
		if duplicate > 0 {
			issues = append(issues, RelationOrderIssue{
				EntityID: pid, EntityType: etype, RelationType: relType,
				Side: side, Property: prop, Kind: "duplicate", Count: duplicate,
			})
		}
	}
	return issues
}
