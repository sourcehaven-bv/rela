package entitymanager

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// assignManagedOrder fills in _order_out / _order_in on a new relation
// when the relation type is orderable and the caller has not supplied a
// finite numeric value. Auto-assignment uses AppendOrder over the
// existing siblings on the enabled side.
//
// A non-finite or non-numeric caller-supplied value is overwritten — the
// HTTP wire validators reject those at the boundary, but MCP/Lua/CLI
// write paths reach Manager directly and can submit garbage. We guarantee
// the on-disk relation always has a finite numeric value when the type
// declares the side orderable, regardless of write entry point.
func (m *Manager) assignManagedOrder(ctx context.Context, rel *entity.Relation, relType string) error {
	relDef, ok := m.deps.Meta.Relations[relType]
	if !ok {
		// Caller already validated the relation type via
		// Meta.ValidateRelation. This branch is only reachable
		// through a metamodel reload race; failing loudly surfaces
		// the race rather than silently writing a relation with no
		// managed order.
		return fmt.Errorf("assignManagedOrder: relation type %q not found in metamodel", relType)
	}
	assignSide := func(prop string, query store.RelationQuery) error {
		if prop == "" {
			return nil
		}
		if rel.Properties == nil {
			rel.Properties = make(map[string]interface{})
		}
		if _, ok := FiniteOrder(rel.Properties[prop]); ok {
			return nil
		}
		vals, err := m.collectSiblingOrders(ctx, query, prop)
		if err != nil {
			return fmt.Errorf("collect siblings for %q: %w", prop, err)
		}
		rel.Properties[prop] = AppendOrder(vals)
		return nil
	}
	if err := assignSide(relDef.OutgoingOrderProperty(), store.RelationQuery{
		From: rel.From, Type: relType,
	}); err != nil {
		return err
	}
	return assignSide(relDef.IncomingOrderProperty(), store.RelationQuery{
		To: rel.To, Type: relType,
	})
}

// collectSiblingOrders walks relations matching q and returns the finite
// float values present at prop.
func (m *Manager) collectSiblingOrders(ctx context.Context, q store.RelationQuery, prop string) ([]float64, error) {
	var vals []float64
	for r, err := range m.deps.Store.ListRelations(ctx, q) {
		if err != nil {
			return nil, err
		}
		if v, ok := FiniteOrder(r.Properties[prop]); ok {
			vals = append(vals, v)
		}
	}
	return vals, nil
}

// touchesOrderKey reports whether opts modifies the given property key,
// either by setting a new value or by unsetting it.
func touchesOrderKey(opts entity.RelationOptions, key string) bool {
	if _, ok := opts.Properties[key]; ok {
		return true
	}
	for _, k := range opts.MetaUnset {
		if k == key {
			return true
		}
	}
	return false
}

// validateOrderUpdate rejects non-finite numeric values on managed order
// properties at the Manager layer so MCP/Lua/CLI write paths can't bypass
// the wire validators. Wire validation runs before Manager for HTTP; this
// is the engine-level backstop.
func validateOrderUpdate(opts entity.RelationOptions, relDef metamodel.RelationDef) error {
	check := func(prop string) error {
		v, present := opts.Properties[prop]
		if !present {
			return nil
		}
		if _, ok := FiniteOrder(v); !ok {
			return fmt.Errorf("invalid value for %s: must be a finite number, got %T", prop, v)
		}
		return nil
	}
	if p := relDef.OutgoingOrderProperty(); p != "" {
		if err := check(p); err != nil {
			return err
		}
	}
	if p := relDef.IncomingOrderProperty(); p != "" {
		if err := check(p); err != nil {
			return err
		}
	}
	return nil
}

// maybeRenumberSide walks the siblings matching q, checks whether the
// finite values at prop have a gap below OrderCollapseThreshold, and if
// so rewrites them to dense integer ordinals 1.0..N. Two-phase: build
// the full plan, then apply it. A mid-loop write failure aborts the
// apply and returns the error — earlier writes still landed (the store
// has no transaction).
//
// Renumber preserves missing-ness: siblings whose value at prop is
// missing (or non-finite) stay missing. Only siblings that previously
// had a finite value are redistributed.
func (m *Manager) maybeRenumberSide(ctx context.Context, q store.RelationQuery, prop string) error {
	var sibs []*entity.Relation
	for r, err := range m.deps.Store.ListRelations(ctx, q) {
		if err != nil {
			return err
		}
		c := *r
		if r.Properties != nil {
			c.Properties = make(map[string]interface{}, len(r.Properties))
			for k, v := range r.Properties {
				c.Properties[k] = v
			}
		}
		sibs = append(sibs, &c)
	}
	if len(sibs) < 2 {
		return nil
	}

	values := make([]float64, 0, len(sibs))
	for _, r := range sibs {
		if v, ok := FiniteOrder(r.Properties[prop]); ok {
			values = append(values, v)
		}
	}
	sort.Float64s(values)
	if !NeedsRenumber(values) {
		return nil
	}

	type planEntry struct {
		rel    *entity.Relation
		newVal float64
	}
	withValue := make([]*entity.Relation, 0, len(sibs))
	for _, r := range sibs {
		if _, ok := FiniteOrder(r.Properties[prop]); ok {
			withValue = append(withValue, r)
		}
	}
	asValues := make([]entity.Relation, len(withValue))
	for i, r := range withValue {
		asValues[i] = *r
	}
	sorted := SortRelations(asValues, prop)
	byKey := make(map[string]*entity.Relation, len(withValue))
	for _, r := range withValue {
		byKey[r.From+"--"+r.Type+"--"+r.To] = r
	}
	plan := make([]planEntry, 0, len(sorted))
	for i, s := range sorted {
		newVal := float64(i + 1)
		key := s.From + "--" + s.Type + "--" + s.To
		r := byKey[key]
		if cur, ok := FiniteOrder(r.Properties[prop]); ok && cur == newVal {
			continue
		}
		plan = append(plan, planEntry{rel: r, newVal: newVal})
	}

	for _, p := range plan {
		props := make(map[string]interface{}, len(p.rel.Properties)+1)
		for k, v := range p.rel.Properties {
			props[k] = v
		}
		props[prop] = p.newVal
		data := store.RelationData{Properties: props, Content: p.rel.Content}
		if _, err := m.deps.Store.UpdateRelation(ctx, p.rel.From, p.rel.Type, p.rel.To, data); err != nil {
			return fmt.Errorf("renumber write failed for %s--%s--%s: %w", p.rel.From, p.rel.Type, p.rel.To, err)
		}
	}
	return nil
}

// runRenumberAfterUpdate logs (does not return) renumber failures from
// the post-update cleanup pass. Called from UpdateRelation when the
// caller touched a managed order property.
func (m *Manager) runRenumberAfterUpdate(ctx context.Context, from, to, relType string, touchedOut, touchedIn bool) {
	relDef, ok := m.deps.Meta.Relations[relType]
	if !ok {
		return
	}
	if touchedOut {
		q := store.RelationQuery{From: from, Type: relType}
		if rErr := m.maybeRenumberSide(ctx, q, relDef.OutgoingOrderProperty()); rErr != nil {
			slog.Error("renumber outgoing side failed", "from", from, "relType", relType, "err", rErr)
		}
	}
	if touchedIn {
		q := store.RelationQuery{To: to, Type: relType}
		if rErr := m.maybeRenumberSide(ctx, q, relDef.IncomingOrderProperty()); rErr != nil {
			slog.Error("renumber incoming side failed", "to", to, "relType", relType, "err", rErr)
		}
	}
}
