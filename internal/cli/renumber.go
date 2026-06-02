package cli

import (
	"context"
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RenumberCmd is `rela renumber`. Walks every orderable relation and
// rewrites the managed order property to dense integer ordinals
// 1.0..N. Missing-ness is preserved.
type RenumberCmd struct {
	DryRun bool `name:"dry-run" help:"Preview the renumber without writing."`
}

// Run dispatches `rela renumber`.
func (c *RenumberCmd) Run(ctx context.Context, svc *cliServices) error {
	st := svc.Store()
	schema := svc.Meta()

	var plan []renumberEntry

	relNames := make([]string, 0, len(schema.Relations))
	for name := range schema.Relations {
		relNames = append(relNames, name)
	}
	sort.Strings(relNames)

	for _, relName := range relNames {
		relDef := schema.Relations[relName]
		if p := relDef.OutgoingOrderProperty(); p != "" {
			plan = append(plan, buildRenumberPlan(ctx, st, relName, p, "outgoing")...)
		}
		if p := relDef.IncomingOrderProperty(); p != "" {
			plan = append(plan, buildRenumberPlan(ctx, st, relName, p, "incoming")...)
		}
	}

	if len(plan) == 0 {
		out.WriteSuccess("All orderable relations already have dense ordinals")
		return nil
	}

	if c.DryRun {
		out.WriteInfo("DRY RUN — %d relation(s) would be rewritten", len(plan))
		for _, p := range plan {
			cur, _ := metamodel.FiniteOrder(p.rel.Properties[p.prop])
			out.WriteInfo("  %s --%s--> %s: %s %v -> %v",
				p.rel.From, p.rel.Type, p.rel.To, p.prop, cur, p.newVal)
		}
		return nil
	}

	for _, p := range plan {
		props := make(map[string]interface{}, len(p.rel.Properties)+1)
		for k, v := range p.rel.Properties {
			props[k] = v
		}
		props[p.prop] = p.newVal
		data := store.RelationData{Properties: props, Content: p.rel.Content}
		if _, err := st.UpdateRelation(ctx, p.rel.From, p.rel.Type, p.rel.To, data); err != nil {
			return fmt.Errorf("renumber write failed for %s--%s--%s: %w", p.rel.From, p.rel.Type, p.rel.To, err)
		}
	}
	out.WriteSuccess("Renumbered %d relation(s)", len(plan))
	return nil
}

type renumberEntry struct {
	rel    *entity.Relation
	prop   string
	newVal float64
}

// buildRenumberPlan walks relations of relType and returns the (relation,
// new-value) tuples needed to redistribute existing finite values to dense
// ordinals 1.0..N on the named side. Siblings with missing/non-finite
// values are skipped, preserving missing-ness.
func buildRenumberPlan(
	ctx context.Context, st store.Store,
	relType, prop, side string,
) []renumberEntry {
	parents := map[string][]*entity.Relation{}
	for r, err := range st.ListRelations(ctx, store.RelationQuery{Type: relType}) {
		if err != nil {
			continue
		}
		var parent string
		if side == "outgoing" {
			parent = r.From
		} else {
			parent = r.To
		}
		c := *r
		if r.Properties != nil {
			c.Properties = make(map[string]interface{}, len(r.Properties))
			for k, v := range r.Properties {
				c.Properties[k] = v
			}
		}
		parents[parent] = append(parents[parent], &c)
	}

	parentIDs := make([]string, 0, len(parents))
	for id := range parents {
		parentIDs = append(parentIDs, id)
	}
	sort.Strings(parentIDs)

	var plan []renumberEntry
	for _, pid := range parentIDs {
		rels := parents[pid]
		withValue := make([]*entity.Relation, 0, len(rels))
		for _, r := range rels {
			if _, ok := metamodel.FiniteOrder(r.Properties[prop]); ok {
				withValue = append(withValue, r)
			}
		}
		if len(withValue) < 2 {
			continue
		}
		asValues := make([]entity.Relation, len(withValue))
		for i, r := range withValue {
			asValues[i] = *r
		}
		sorted := entitymanager.SortRelations(asValues, prop)
		byKey := make(map[string]*entity.Relation, len(withValue))
		for _, r := range withValue {
			byKey[r.From+"--"+r.Type+"--"+r.To] = r
		}
		for i, s := range sorted {
			newVal := float64(i + 1)
			key := s.From + "--" + s.Type + "--" + s.To
			r := byKey[key]
			if cur, ok := metamodel.FiniteOrder(r.Properties[prop]); ok && cur == newVal {
				continue
			}
			plan = append(plan, renumberEntry{rel: r, prop: prop, newVal: newVal})
		}
	}
	return plan
}
