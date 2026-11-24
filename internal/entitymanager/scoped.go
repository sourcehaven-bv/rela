package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Scoped relations (TKT-5GK80Q9). A relation with `within: <scope>` admits an
// edge from source S to target T only when the entity S's scope edge points
// to owns T: it is the from of an `owning:` edge into T. The owner is read
// one hop, so cyclic ownership data cannot make the check loop.
//
// The check runs in the write's transaction, on its view, before anything is
// written, like the relation rules ([checkRelationRules]). It derives the
// edges after the write from the stored ones and the write's adds and
// removes. Which edges a write can affect:
//
//   - a scoped edge it adds or removes, at the edge's source;
//   - a scope edge it adds or removes, at every scoped edge of that source;
//   - an owning edge it adds or removes, at every scoped edge into its
//     target (re-parenting).
//
// An out-of-scope edge the write adds is refused. A stored edge the write
// leaves out of scope is cleared in the same write (`on_change: clear`) or
// refuses the write (`on_change: reject`). The loader keeps scoping one
// level deep, so a clear never leaves another scoped edge out of scope, and
// one pass suffices.
//
// Edges are read raw, as on the rest of the write path, so the verdict is
// the same for every principal. The refusal names the relation and its
// `within`, never an entity. Like a relation rule, it tells the caller one
// bit about an end the caller can read, which can concern an edge from an
// entity the caller cannot see (re-parenting a target that a hidden entity
// points at). That is the accepted channel in docs/acl-security.md.
//
// Entity deletes are not checked, for the reason relation rules give.

// scopeIndex is the scoped relations of a metamodel, compiled once by [New].
type scopeIndex struct {
	scoped map[string]scopedRel
	// byScope lists, per scope relation, the relations scoped within it.
	byScope map[string][]string
	// byOwning lists, per owning relation, the scoped relations whose
	// targets it owns.
	byOwning map[string][]string
}

// scopedRel is one relation declared with `within:`.
type scopedRel struct {
	name     string
	within   string
	onChange metamodel.ScopeChange
	// owners are the owning relations through which a target of this
	// relation has its owner, one per target type, sorted and distinct.
	owners []string
}

// newScopeIndex compiles the scoped relations of meta. Nil when there are
// none, which every method accepts.
func newScopeIndex(meta *metamodel.Metamodel) *scopeIndex {
	if meta == nil {
		return nil
	}
	x := &scopeIndex{scoped: map[string]scopedRel{}, byScope: map[string][]string{}, byOwning: map[string][]string{}}
	for _, name := range slices.Sorted(maps.Keys(meta.Relations)) {
		def := meta.Relations[name]
		if def.Within == "" {
			continue
		}
		s := scopedRel{name: name, within: def.Within, onChange: def.OnChange}
		for _, t := range def.To {
			if o, ok := metamodel.ScopeOwner(meta, name, t); ok && !slices.Contains(s.owners, o) {
				s.owners = append(s.owners, o)
			}
		}
		slices.Sort(s.owners)
		x.scoped[name] = s
		x.byScope[def.Within] = append(x.byScope[def.Within], name)
		for _, o := range s.owners {
			x.byOwning[o] = append(x.byOwning[o], name)
		}
	}
	if len(x.scoped) == 0 {
		return nil
	}
	return x
}

// touches reports whether a write of any of keys can affect a scoped edge,
// so it must run in a transaction and call [checkScopes]. It is decided from
// the relation types alone, never from stored edges, so the choice of path
// cannot race a concurrent write.
func (x *scopeIndex) touches(keys ...entity.RelationKey) bool {
	if x == nil {
		return false
	}
	for _, k := range keys {
		if _, ok := x.scoped[k.Type]; ok {
			return true
		}
		if len(x.byScope[k.Type]) > 0 || len(x.byOwning[k.Type]) > 0 {
			return true
		}
	}
	return false
}

// scopeRefusal is the refusal for an out-of-scope edge of s. It names only
// schema, so it is the same for every caller and every entity.
func scopeRefusal(s scopedRel) *ValidationError {
	return newValidationError([]*metamodel.ValidationError{{
		Type:     metamodel.ValidationErrorScope,
		Property: s.name,
		Message: fmt.Sprintf("relation %q must point to an entity owned by the target of %q",
			s.name, s.within),
	}})
}

// edgeDelta is the adds and removes of one write, by key.
type edgeDelta struct {
	adds    []entity.RelationKey
	added   map[entity.RelationKey]bool
	removed map[entity.RelationKey]bool
}

func newEdgeDelta(adds, removes []entity.RelationKey) edgeDelta {
	d := edgeDelta{adds: adds, added: map[entity.RelationKey]bool{}, removed: map[entity.RelationKey]bool{}}
	for _, k := range adds {
		d.added[k] = true
	}
	for _, k := range removes {
		d.removed[k] = true
	}
	return d
}

// checkScopes judges the scoped edges a write of adds and removes affects, on
// view, and returns the stored edges the write must clear. A refused write
// returns a [*ValidationError]. view must be the view the write runs in, and
// nothing may be written yet. Adds whose end is blank (an entity the same
// write creates) are judged with that end blank: it has no edges.
func checkScopes(
	ctx context.Context, x *scopeIndex, view store.Store, adds, removes []entity.RelationKey,
) ([]*entity.Relation, error) {
	if !x.touches(slices.Concat(adds, removes)...) {
		return nil, nil
	}
	// A scoped edge from an entity the same write creates: that entity has no
	// scope edge yet, so no target is in its scope.
	for _, k := range adds {
		if s, ok := x.scoped[k.Type]; ok && k.From == "" {
			return nil, scopeRefusal(s)
		}
	}
	d := newEdgeDelta(adds, removes)
	pairs, err := x.affected(ctx, view, slices.Concat(adds, removes))
	if err != nil {
		return nil, err
	}
	var clears []*entity.Relation
	var refused []*ValidationError
	refusedRel := map[string]bool{}
	for _, p := range pairs {
		s := x.scoped[p.rel]
		scope, err := afterFar(ctx, view, d, p.source, s.within)
		if err != nil {
			return nil, err
		}
		edges, err := afterScopedEdges(ctx, view, d, p.source, s.name)
		if err != nil {
			return nil, err
		}
		for _, e := range edges {
			ok, err := ownedByAny(ctx, view, d, e.To, s.owners, scope)
			if err != nil {
				return nil, err
			}
			if ok {
				continue
			}
			if d.added[e.Identity()] || s.onChange != metamodel.ScopeChangeClear {
				if !refusedRel[s.name] {
					refusedRel[s.name] = true
					refused = append(refused, scopeRefusal(s))
				}
				continue
			}
			clears = append(clears, e)
		}
	}
	if err := joinValidationErrors(refused); err != nil {
		return nil, err
	}
	return clears, nil
}

// checkCascadeScope is the scope check of an edge an automation creates. An
// automation does not clear: a create that would leave a stored edge out of
// scope is refused, whatever `on_change:` says, so a trigger never deletes
// an edge as a side effect the operator did not write down.
func checkCascadeScope(ctx context.Context, x *scopeIndex, view store.Store, key entity.RelationKey) error {
	return checkScopesNoClear(ctx, x, view, []entity.RelationKey{key}, nil)
}

// checkScopesNoClear is [checkScopes] for a writer that does not clear: an
// automation, or an entity copy. A write that would leave a stored edge out
// of scope is refused, whatever `on_change:` says.
func checkScopesNoClear(
	ctx context.Context, x *scopeIndex, view store.Store, adds, removes []entity.RelationKey,
) error {
	clears, err := checkScopes(ctx, x, view, adds, removes)
	if err != nil {
		return err
	}
	if len(clears) > 0 {
		return scopeRefusal(x.scoped[clears[0].Type])
	}
	return nil
}

// scopePair is a source and one of its scoped relations to judge.
type scopePair struct{ source, rel string }

// affected returns, sorted and distinct, the (source, scoped relation) pairs
// a write of keys can affect.
func (x *scopeIndex) affected(ctx context.Context, view store.Store, keys []entity.RelationKey) ([]scopePair, error) {
	seen := map[scopePair]bool{}
	add := func(p scopePair) {
		if p.source != "" {
			seen[p] = true
		}
	}
	for _, k := range keys {
		if _, ok := x.scoped[k.Type]; ok {
			add(scopePair{k.From, k.Type})
		}
		for _, s := range x.byScope[k.Type] {
			add(scopePair{k.From, s})
		}
		if k.To == "" {
			continue
		}
		for _, s := range x.byOwning[k.Type] {
			q := store.RelationQuery{To: k.To, Type: s, Direction: store.DirectionIncoming}
			for r, err := range view.ListRelations(ctx, q) {
				if err != nil {
					return nil, fmt.Errorf("scope check: list %s edges into %s: %w", s, k.To, err)
				}
				add(scopePair{r.From, s})
			}
		}
	}
	out := slices.Collect(maps.Keys(seen))
	slices.SortFunc(out, func(a, b scopePair) int {
		if a.source != b.source {
			if a.source < b.source {
				return -1
			}
			return 1
		}
		if a.rel < b.rel {
			return -1
		}
		if a.rel > b.rel {
			return 1
		}
		return 0
	})
	return out, nil
}

// afterFar returns the targets of source's relType edges after the write.
func afterFar(ctx context.Context, view store.Store, d edgeDelta, source, relType string) (map[string]bool, error) {
	edges, err := afterScopedEdges(ctx, view, d, source, relType)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(edges))
	for _, e := range edges {
		if e.To != "" {
			out[e.To] = true
		}
	}
	return out, nil
}

// afterScopedEdges returns source's outgoing relType edges after the write:
// the stored ones not removed, then the added ones not stored.
func afterScopedEdges(
	ctx context.Context, view store.Store, d edgeDelta, source, relType string,
) ([]*entity.Relation, error) {
	var out []*entity.Relation
	stored := map[entity.RelationKey]bool{}
	q := store.RelationQuery{From: source, Type: relType, Direction: store.DirectionOutgoing}
	for r, err := range view.ListRelations(ctx, q) {
		if err != nil {
			return nil, fmt.Errorf("scope check: list %s edges of %s: %w", relType, source, err)
		}
		stored[r.Identity()] = true
		if !d.removed[r.Identity()] {
			out = append(out, r)
		}
	}
	for _, k := range d.adds {
		if k.From == source && k.Type == relType && !stored[k] {
			out = append(out, &entity.Relation{From: k.From, FromFace: k.FromFace, Type: k.Type, To: k.To})
		}
	}
	return out, nil
}

// ownedByAny reports whether, after the write, one of scope is the from of an
// owning edge of one of owners into target. One hop: an owner's own owner is
// never read.
func ownedByAny(
	ctx context.Context, view store.Store, d edgeDelta, target string, owners []string, scope map[string]bool,
) (bool, error) {
	if len(scope) == 0 || target == "" {
		return false, nil
	}
	for _, o := range owners {
		for _, k := range d.adds {
			if k.Type == o && k.To == target && scope[k.From] {
				return true, nil
			}
		}
		q := store.RelationQuery{To: target, Type: o, Direction: store.DirectionIncoming}
		for r, err := range view.ListRelations(ctx, q) {
			if err != nil {
				return false, fmt.Errorf("scope check: list %s edges into %s: %w", o, target, err)
			}
			if scope[r.From] && !d.removed[r.Identity()] {
				return true, nil
			}
		}
	}
	return false, nil
}

// scopeClears is one write's clears: the pre-pass made before its
// transaction, and what the transaction decided.
type scopeClears struct {
	// pre is what the pre-pass on the outer store found to clear, with the
	// lineage id of each, for the delete versions written after the commit.
	pre map[entity.RelationKey]int64
	// cleared is what the last attempt's transaction decided to clear, and
	// applied what it deleted.
	cleared []*entity.Relation
	applied []*entity.Relation
}

// prepass runs [checkScopes] on the outer store before the write's
// transaction, and reads the lineage id of every edge it would clear. The
// pre-pass is advisory: the transaction decides. A refusal here is not
// returned; the transaction refuses again on its own view.
func (c *scopeClears) prepass(ctx context.Context, m *Manager, adds, removes []entity.RelationKey) error {
	c.pre = map[entity.RelationKey]int64{}
	if !m.deps.scopes.touches(slices.Concat(adds, removes)...) {
		return nil
	}
	// Inside a transaction the caller joined, the outer store is that
	// transaction's view: the file and memory stores would deadlock on the
	// outer handle.
	st := m.deps.Store
	if v := store.TxView(ctx); v != nil {
		st = v
	}
	clears, err := checkScopes(ctx, m.deps.scopes, st, adds, removes)
	if err != nil {
		if _, refused := errors.AsType[*ValidationError](err); refused {
			return nil
		}
		return err
	}
	keys := make([]entity.RelationKey, len(clears))
	for i, r := range clears {
		keys[i] = r.Identity()
	}
	ids, err := lineageIDs(ctx, m.deps.RelationVersionRecorder, keys)
	if err != nil {
		return err
	}
	for _, k := range keys {
		c.pre[k] = ids[k]
	}
	return nil
}

// decide runs [checkScopes] in the write's transaction and authorizes each
// clear as the caller. A clear the pre-pass did not see fails the write with
// [store.ErrConflict]: its lineage id is unknown. That needs a concurrent
// write between the two reads; the caller may retry.
func (c *scopeClears) decide(
	ctx context.Context, m *Manager, view store.Store, adds, removes []entity.RelationKey,
) ([]entity.RelationKey, error) {
	c.cleared, c.applied = nil, nil
	clears, err := checkScopes(ctx, m.deps.scopes, view, adds, removes)
	if err != nil {
		return nil, err
	}
	if len(clears) == 0 {
		return nil, nil
	}
	keys := make([]entity.RelationKey, len(clears))
	for i, r := range clears {
		keys[i] = r.Identity()
		if _, ok := c.pre[keys[i]]; !ok {
			return nil, fmt.Errorf("scope check: an edge to clear appeared during the write: %w", store.ErrConflict)
		}
	}
	if err := authorizeClears(ctx, m, view, clears); err != nil {
		return nil, err
	}
	c.cleared = clears
	return keys, nil
}

// authorizeClears asks, for each edge a write clears, whether the caller may
// delete it. The edges are the source's own when the write changes its scope,
// and another entity's when the write re-parents their target. Either way the
// caller's write deletes them, so each source is row-gated first. A source
// the caller cannot read, or one that is missing, refuses the write with the
// scope refusal: a clear never deletes, or answers 403 about, a row the
// caller cannot see. Each edge is then authorized on its own source, because
// a grant can depend on the row (role_relations, when:).
func authorizeClears(ctx context.Context, m *Manager, view store.Store, clears []*entity.Relation) error {
	for _, r := range clears {
		family, err := familyRows(ctx, view, r.From)
		if err != nil {
			return err
		}
		if len(family) == 0 {
			return scopeRefusal(m.deps.scopes.scoped[r.Type])
		}
		if rErr := requireReadableRow(ctx, faceGateOf(m), r.From, family); rErr != nil {
			if errors.Is(rErr, ErrEntityNotFound) {
				return scopeRefusal(m.deps.scopes.scoped[r.Type])
			}
			return rErr
		}
		req := RelationDeleteRequest(m.deps.Meta, r.Type, family[0].Type, r.From, r.FromFace)
		if aErr := m.authorizeAndAudit(ctx, req); aErr != nil {
			return aErr
		}
	}
	return nil
}

// apply deletes the cleared edges on view and audits each.
func (c *scopeClears) apply(ctx context.Context, m *Manager, view store.Store) error {
	for _, r := range c.cleared {
		if err := view.DeleteRelation(ctx, r.Identity()); err != nil {
			return fmt.Errorf("clear out-of-scope relation: %w", err)
		}
		if err := m.recordRelationAudit(ctx, view, audit.OpDeleteRelation, r,
			"cleared: out of scope of "+m.deps.scopes.scoped[r.Type].within); err != nil {
			return err
		}
	}
	c.applied = c.cleared
	return nil
}

// version records the delete version of every edge the committed
// transaction cleared.
func (c *scopeClears) version(ctx context.Context, m *Manager) {
	for _, r := range c.applied {
		m.recordRelationVersion(ctx, store.VersionOpDelete, r, c.pre[r.Identity()], "", "", "")
	}
}
