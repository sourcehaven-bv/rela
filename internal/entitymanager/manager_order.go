package entitymanager

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/audit"
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
//
// st is the store the siblings are read from: the Tx view when the caller
// runs the read and the create in one transaction.
func (m *Manager) assignManagedOrder(ctx context.Context, st store.Store, rel *entity.Relation, relType string) error {
	relDef, ok := m.deps.Meta.Relations[relType]
	// Caller already validated the relation type via Meta.ValidateRelation. This
	// branch is only reachable through a metamodel reload race; failing loudly
	// surfaces the race rather than silently writing a relation with no managed
	// order.
	if !ok { // coverage-ignore-start: defensive: CreateRelation calls ValidateRelation(relType)
		// before assignManagedOrder, so an absent relType is only reachable via a
		// metamodel reload race between the two calls
		return fmt.Errorf("assignManagedOrder: relation type %q not found in metamodel", relType)
	} // coverage-ignore-end
	assignSide := func(prop string, query store.RelationQuery) error {
		if prop == "" {
			return nil
		}
		if rel.Properties == nil {
			rel.Properties = make(map[string]any)
		}
		if _, ok := FiniteOrder(rel.Properties[prop]); ok {
			return nil
		}
		vals, err := collectSiblingOrders(ctx, st, query, prop)
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
func collectSiblingOrders(ctx context.Context, st store.Store, q store.RelationQuery, prop string) ([]float64, error) {
	var vals []float64
	for r, err := range st.ListRelations(ctx, q) {
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
	return slices.Contains(opts.MetaUnset, key)
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
// the full plan, then apply it. It returns the relations it rewrote so the
// caller can audit them once the writes are durable.
//
// st is the store to read and write: Manager.runRenumberAfterUpdate passes
// the Tx view, so the scan and the rewrites are one transaction and two
// concurrent renumbers of one side cannot interleave.
//
// Renumber preserves missing-ness: siblings whose value at prop is
// missing (or non-finite) stay missing. Only siblings that previously
// had a finite value are redistributed.
func maybeRenumberSide(
	ctx context.Context, st store.Store, q store.RelationQuery, prop string,
) ([]*entity.Relation, error) {
	var sibs []*entity.Relation
	for r, err := range st.ListRelations(ctx, q) {
		if err != nil {
			return nil, err
		}
		c := *r
		if r.Properties != nil {
			c.Properties = make(map[string]any, len(r.Properties))
			maps.Copy(c.Properties, r.Properties)
		}
		sibs = append(sibs, &c)
	}
	if len(sibs) < 2 {
		return nil, nil
	}

	values := make([]float64, 0, len(sibs))
	for _, r := range sibs {
		if v, ok := FiniteOrder(r.Properties[prop]); ok {
			values = append(values, v)
		}
	}
	sort.Float64s(values)
	if !NeedsRenumber(values) {
		return nil, nil
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
	// Keyed by the full identity: two tails of one triple are two siblings
	// (TKT-KQXVF7), and a triple key would collapse them into one.
	byKey := make(map[entity.RelationKey]*entity.Relation, len(withValue))
	for _, r := range withValue {
		byKey[r.Identity()] = r
	}
	plan := make([]planEntry, 0, len(sorted))
	for i, s := range sorted {
		newVal := float64(i + 1)
		r := byKey[s.Identity()]
		if cur, ok := FiniteOrder(r.Properties[prop]); ok && cur == newVal {
			continue
		}
		plan = append(plan, planEntry{rel: r, newVal: newVal})
	}

	// Renumber writes go directly to the store rather than through
	// Manager.UpdateRelation: that method calls runRenumberAfterUpdate, so
	// routing renumber through it would recurse (renumber → update → renumber).
	updated := make([]*entity.Relation, 0, len(plan))
	for _, p := range plan {
		props := make(map[string]any, len(p.rel.Properties)+1)
		maps.Copy(props, p.rel.Properties)
		props[prop] = p.newVal
		data := store.RelationData{Properties: props, Content: p.rel.Content}
		u, err := st.UpdateRelation(ctx, p.rel.Identity(), data)
		if err != nil {
			return nil, fmt.Errorf("renumber write failed for %s: %w", p.rel.Key(), err)
		}
		updated = append(updated, u)
	}
	return updated, nil
}

// runRenumberAfterUpdate logs (does not return) renumber failures from
// the post-update cleanup pass. Called from UpdateRelation when the
// caller touched a managed order property.
//
// Each side runs in its own [store.Transactor.Tx], separate from the
// caller's update: a renumber failure must not roll back a write that
// already succeeded. The rewrites are audited after commit, one record per
// write, the same approach cascadeHost uses for cascade deletes. The
// triggered-by marker distinguishes them from the user-initiated
// UpdateRelation that spawned them (issue #886).
func (m *Manager) runRenumberAfterUpdate(ctx context.Context, from, to, relType string, touchedOut, touchedIn bool) {
	relDef, ok := m.deps.Meta.Relations[relType]
	if !ok {
		return
	}
	renumber := func(q store.RelationQuery, prop string) error {
		var updated []*entity.Relation
		err := m.deps.Store.Tx(ctx, func(view store.Store) error {
			var rErr error
			updated, rErr = maybeRenumberSide(ctx, view, q, prop)
			return rErr
		})
		if err != nil {
			return err
		}
		renumberCtx := audit.WithTriggeredBy(ctx, "renumber:"+prop)
		for _, u := range updated {
			m.recordRelationAudit(renumberCtx, audit.OpUpdateRelation, u, "renumbered "+prop)
		}
		return nil
	}
	if touchedOut {
		// The outgoing list spans every tail of the source: renumbering is
		// per source family, not per face.
		q := store.RelationQuery{From: from, Type: relType}
		if rErr := renumber(q, relDef.OutgoingOrderProperty()); rErr != nil {
			slog.Error("renumber outgoing side failed", "from", from, "relType", relType, "err", rErr)
		}
	}
	if touchedIn {
		q := store.RelationQuery{To: to, Type: relType}
		if rErr := renumber(q, relDef.IncomingOrderProperty()); rErr != nil {
			slog.Error("renumber incoming side failed", "to", to, "relType", relType, "err", rErr)
		}
	}
}

// moveRelation is [Manager.UpdateRelation] for a [entity.RelationOptions]
// with a Position, after its authorization. The sibling read, the
// placement and every write share one [store.Store.Tx], so a concurrent
// move or renumber cannot slip between the values read and the values
// written. Audit records follow the commit: one for the moved edge, one
// per sibling the densify rewrote, marked as a renumber.
//
// A package function rather than a Manager method: the Manager is at its
// plimsoll method line.
func moveRelation(
	ctx context.Context, m *Manager, key entity.RelationKey, relDef metamodel.RelationDef, hasDef bool,
	opts entity.RelationOptions,
) (*entity.Relation, error) {
	if len(opts.Properties) > 0 || len(opts.MetaUnset) > 0 || opts.Content != nil {
		return nil, fmt.Errorf("%w: a position cannot be combined with property or content changes",
			ErrInvalidOrderPosition)
	}
	prop := relDef.OutgoingOrderProperty()
	if opts.Position.Incoming {
		prop = relDef.IncomingOrderProperty()
	}
	if !hasDef || prop == "" {
		return nil, fmt.Errorf("%w: %s", ErrRelationNotOrderable, key.Type)
	}
	keep := orderSiblingFilter(key, *opts.Position)
	if opts.Position.Incoming {
		authorized, aErr := authorizeIncomingSiblings(ctx, m, key, keep)
		if aErr != nil {
			return nil, aErr
		}
		keep = func(r *entity.Relation) bool { return authorized[r.Identity()] }
	}
	var moved *entity.Relation
	var densified []*entity.Relation
	err := m.deps.Store.Tx(ctx, func(view store.Store) error {
		if _, gErr := view.GetRelation(ctx, key); gErr != nil {
			return fmt.Errorf("%w: %s", ErrRelationNotFound, key)
		}
		var siblings []entity.Relation
		for r, lErr := range view.ListRelations(ctx, orderSiblingQuery(key, opts.Position.Incoming)) {
			if lErr != nil {
				return lErr
			}
			if keep(r) {
				siblings = append(siblings, *r)
			}
		}
		plan, pErr := PlaceOrder(siblings, key, *opts.Position, prop)
		if pErr != nil {
			return pErr
		}
		var wErr error
		moved, densified, wErr = writeOrderPlan(ctx, view, siblings, plan, key, prop)
		return wErr
	})
	if err != nil {
		return nil, err
	}
	renumberCtx := audit.WithTriggeredBy(ctx, "renumber:"+prop)
	for _, u := range densified {
		m.recordRelationAudit(renumberCtx, audit.OpUpdateRelation, u, "renumbered "+prop)
	}
	if moved == nil {
		// A densify can keep the moved edge's value and renumber the
		// siblings around it; a no-op move rewrites nothing at all.
		if moved, err = m.deps.Store.GetRelation(ctx, key); err != nil || len(densified) == 0 {
			return moved, err
		}
	}
	m.recordRelationAudit(ctx, audit.OpUpdateRelation, moved, "moved "+prop)
	return moved, nil
}

// orderSiblingQuery lists the edges on key's order side: the source's
// edges of the type on the outgoing side, every edge into the target on
// the incoming side.
func orderSiblingQuery(key entity.RelationKey, incoming bool) store.RelationQuery {
	if incoming {
		return store.RelationQuery{To: key.To, Type: key.Type}
	}
	return store.RelationQuery{From: key.From, Type: key.Type}
}

// orderSiblingFilter keeps the siblings a move plans among: the edges in
// pos.Among, and the moved edge itself. An edge the caller cannot see must
// not shape the result. On the outgoing side it keeps only the moved
// edge's own tail, which is the list a reader sees and the one tail the
// update was authorized for. The target side has no face, so an incoming
// list spans every tail.
func orderSiblingFilter(key entity.RelationKey, pos entity.OrderPosition) func(*entity.Relation) bool {
	var among map[entity.RelationKey]bool
	if pos.Among != nil {
		among = make(map[entity.RelationKey]bool, len(pos.Among))
		for _, k := range pos.Among {
			among[k] = true
		}
	}
	return func(r *entity.Relation) bool {
		if !pos.Incoming && r.FromFace != key.FromFace {
			return false
		}
		return among == nil || among[r.Identity()] || r.Identity() == key
	}
}

// authorizeIncomingSiblings authorizes an update of every edge an incoming
// move may write, and returns their identities. An edge belongs to its
// source, so the update [Manager.UpdateRelation] authorized covers the
// moved edge only, while a densify on the incoming side rewrites edges of
// other sources. One denial fails the whole move, before anything is
// written. It runs before the move's transaction, so the ACL reads stay out
// of it; the transaction then writes only these edges, which leaves an edge
// created in between alone.
func authorizeIncomingSiblings(
	ctx context.Context, m *Manager, key entity.RelationKey, keep func(*entity.Relation) bool,
) (map[entity.RelationKey]bool, error) {
	authorized := map[entity.RelationKey]bool{key: true}
	types := map[string]string{}
	for r, err := range m.deps.Store.ListRelations(ctx, orderSiblingQuery(key, true)) {
		if err != nil {
			return nil, err
		}
		if !keep(r) || authorized[r.Identity()] {
			continue
		}
		typ, seen := types[r.From]
		if !seen {
			// A lookup error leaves the type empty, which matches no grant.
			fam, _ := lookupFamily(ctx, m.deps.Store, r.From)
			typ = fam.typ
			types[r.From] = typ
		}
		if aErr := m.authorizeAndAudit(ctx,
			RelationUpdateRequest(m.deps.Meta, key.Type, typ, r.From, r.FromFace)); aErr != nil {
			return nil, aErr
		}
		authorized[r.Identity()] = true
	}
	return authorized, nil
}

// writeOrderPlan writes each planned order value to its sibling through
// view. It returns the moved edge, when the plan rewrote it, apart from the
// siblings a densify rewrote around it.
func writeOrderPlan(
	ctx context.Context, view store.Store, siblings []entity.Relation, plan map[entity.RelationKey]float64,
	key entity.RelationKey, prop string,
) (moved *entity.Relation, densified []*entity.Relation, err error) {
	for _, r := range siblings {
		value, ok := plan[r.Identity()]
		if !ok {
			continue
		}
		props := make(map[string]any, len(r.Properties)+1)
		maps.Copy(props, r.Properties)
		props[prop] = value
		u, wErr := view.UpdateRelation(ctx, r.Identity(), store.RelationData{Properties: props, Content: r.Content})
		if wErr != nil {
			return nil, nil, fmt.Errorf("move write failed: %w", wErr)
		}
		if r.Identity() == key {
			moved = u
		} else {
			densified = append(densified, u)
		}
	}
	return moved, densified, nil
}
