package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/writerule"
)

// Write-enforced relation rules (TKT-EBARYBI): a validation rule with
// `enforce: write` whose conditions use `related(...)`. See
// internal/writerule for what such a rule may read.
//
// The check runs in the write's store transaction, on its view, and before
// anything is written. Deciding first is what makes a refusal write nothing
// on every backend: the file and memory stores do not roll a Tx back. The
// edges after the write are therefore not read from the store but derived:
// the stored edges with the write's creates added and its deletes removed.
//
// The edges are counted raw, as every check on the write path reads raw rows:
// an edge to an entity the caller cannot see counts. A count of visible edges
// would make the verdict depend on who writes, and disagree with `rela
// analyze validations`. The refusal names the rule and quotes its
// description, never an entity, so it does not name a hidden neighbor. It
// does tell the caller one bit about an end the caller can read: whether
// that end has another edge the rule counts, hidden ones included. On an
// update the end is the entity being written. On a relation write it is
// either end, and the far end may only be readable, so the bit can say that
// an edge to an entity the caller cannot see exists. This is the accepted
// channel recorded in docs/acl-security.md.
//
// Which writes are checked, and why the others are not, is the table in
// docs/metamodel.md ("Rules that read relations"). In short: an update of the
// entity, and a create or delete of an edge it reads, at either end. Creates
// are not checked, because a new entity has no edges until the request links
// it. Entity deletes are not checked, because the entity whose rule would
// refuse the delete may be hidden from the caller, and the refusal would
// reveal it.

// ChecksRelation reports whether a write-enforced relation rule reads
// relType. A caller that writes several edges of such a type writes them in
// one [Manager.ReplaceRelations], so the rule judges them together rather
// than refusing the second of two edge writes after the first was stored.
//
// It is a function, not a method, to keep Manager under its plimsoll line.
func ChecksRelation(m *Manager, relType string) bool {
	return m != nil && m.deps.writeRules.ReadsRelation(relType)
}

type pendingRelationsKey struct{}

// pendingRelations is what [WithPendingRelations] puts on the ctx.
type pendingRelations struct{ adds, removes []entity.RelationKey }

// WithPendingRelations returns ctx with the edges a request writes right
// after an update of the entity at one of their ends. The update's relation
// rule check judges the entity's new state against its edges as the request
// leaves them. So a data-entry PATCH that sets a status and links the plan
// the status needs is accepted, and one that unlinks the last plan is
// refused before its property change is stored.
//
// The caller must write these edges next, and that write checks the rules
// again. If it fails, the stored update may break a rule. This is the same
// gap as any other failure between the two phases of the PATCH.
func WithPendingRelations(ctx context.Context, adds, removes []entity.RelationKey) context.Context {
	return context.WithValue(ctx, pendingRelationsKey{}, pendingRelations{adds: adds, removes: removes})
}

// afterEdges returns the edges entity id has after the request: the stored
// ones, with the pending edges on ctx that end at id applied.
func afterEdges(ctx context.Context, id string, stored *storedEdges) writerule.Edges {
	pr, ok := ctx.Value(pendingRelationsKey{}).(pendingRelations)
	if !ok {
		return stored
	}
	p := &pendingEdges{stored: stored}
	for _, list := range []struct {
		keys []entity.RelationKey
		add  bool
	}{{pr.adds, true}, {pr.removes, false}} {
		for _, k := range list.keys {
			if k.From == id {
				p.note(writerule.Hop{Relation: k.Type}, k.To, list.add)
			}
			if k.To == id {
				p.note(writerule.Hop{Relation: k.Type, Incoming: true}, k.From, list.add)
			}
		}
	}
	return p
}

// relationRuleCheck returns the check an update of oldEntity to e runs in its
// transaction, or nil when no relation rule applies to e's type. An update
// does not change edges, so both states are judged against the same edges,
// unless the request names the edges it writes next ([WithPendingRelations]).
func relationRuleCheck(deps Deps, oldEntity, e *entity.Entity) func(context.Context) error {
	if oldEntity == nil || !deps.writeRules.HasRelationRules(e.Type) {
		return nil
	}
	return func(ctx context.Context) error {
		view := store.TxView(ctx)
		if view == nil {
			// coverage-ignore: invariant: writeChecked puts the view on the ctx of every txCheck
			return errors.New("relation rule check: no transaction view")
		}
		edges := newStoredEdges(view, e.ID)
		errs, err := deps.writeRules.CheckChange(ctx, oldEntity, e, edges, afterEdges(ctx, e.ID, edges))
		if err != nil {
			return fmt.Errorf("relation rule check: %w", err)
		}
		if len(errs) > 0 {
			return newValidationError(errs)
		}
		return nil
	}
}

// chainTxChecks returns a check that runs a, then b. Either may be nil.
func chainTxChecks(a, b func(context.Context) error) func(context.Context) error {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	}
	return func(ctx context.Context) error {
		if err := a(ctx); err != nil {
			return err
		}
		return b(ctx)
	}
}

// relationWritesChecked reports whether any of the edges in keys is read by
// a relation rule, so the write must run in a Tx and call
// [checkRelationRules].
func relationWritesChecked(deps Deps, keys ...entity.RelationKey) bool {
	for _, k := range keys {
		if deps.writeRules.ReadsRelation(k.Type) {
			return true
		}
	}
	return false
}

// checkRelationRules refuses a relation write that turns a satisfied
// relation rule into a broken one, for either end of each edge. adds are the
// edges the write creates, removes the edges it deletes. view must be the Tx
// view the write runs in, and nothing may be written yet.
//
// An end that is not stored at the implicit face is skipped: it is created by
// the same request (a batch), or it is of a faced type, on which no relation
// rule compiles.
func checkRelationRules(
	ctx context.Context, deps Deps, view store.Store, adds, removes []entity.RelationKey,
) error {
	changes := map[string]*pendingEdges{}
	var order []string
	note := func(id string, hop writerule.Hop, far string, add bool) {
		p, ok := changes[id]
		if !ok {
			p = &pendingEdges{stored: newStoredEdges(view, id)}
			changes[id] = p
			order = append(order, id)
		}
		p.note(hop, far, add)
	}
	for _, list := range []struct {
		keys []entity.RelationKey
		add  bool
	}{{adds, true}, {removes, false}} {
		for _, k := range list.keys {
			if !deps.writeRules.ReadsRelation(k.Type) {
				continue
			}
			// An end is read only when a type the relation allows there has
			// a rule on this hop, so an end that cannot have one costs no
			// read.
			def := deps.Meta.Relations[k.Type]
			out := writerule.Hop{Relation: k.Type}
			in := writerule.Hop{Relation: k.Type, Incoming: true}
			if anyReadsHop(deps.writeRules, def.From, out) {
				note(k.From, out, k.To, list.add)
			}
			if anyReadsHop(deps.writeRules, def.To, in) {
				note(k.To, in, k.From, list.add)
			}
		}
	}
	slices.Sort(order)
	var all []*ValidationError
	for _, id := range order {
		p := changes[id]
		e, err := view.GetEntity(ctx, entity.Ref{ID: id, Face: entity.ImplicitFace})
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("relation rule check: read %s: %w", id, err)
		}
		if !p.readBy(deps.writeRules, e.Type) {
			continue
		}
		errs, err := deps.writeRules.CheckChange(ctx, e, e, p.stored, p)
		if err != nil {
			return fmt.Errorf("relation rule check: %w", err)
		}
		if len(errs) > 0 {
			all = append(all, newValidationError(errs))
		}
	}
	return joinValidationErrors(all)
}

// anyReadsHop reports whether a relation rule of one of types reads hop.
func anyReadsHop(rules *writerule.Set, types []string, hop writerule.Hop) bool {
	for _, t := range types {
		if rules.ReadsHop(t, hop) {
			return true
		}
	}
	return false
}

// joinValidationErrors merges the refusals of several entities into one.
// Nil when there are none.
func joinValidationErrors(errs []*ValidationError) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	}
	// Both ends of an edge may break the same rule; it is reported once.
	merged := &ValidationError{}
	seen := map[string]bool{}
	for _, e := range errs {
		for _, v := range e.Errors {
			if !seen[v.Message] {
				seen[v.Message] = true
				merged.Errors = append(merged.Errors, v)
			}
		}
	}
	return merged
}

// storedEdges answers [writerule.Edges] for one entity from the edges stored
// in a Tx view. Each hop is listed once and each far entity's type is asked
// once, so a check costs one ListRelations per hop and at most two
// MatchingIDs per target type, however many rules read them.
type storedEdges struct {
	view store.Store
	id   string
	far  map[writerule.Hop][]string
	// isType caches, per target type, whether a far id is an existing
	// entity of that type.
	isType map[string]map[string]bool
}

func newStoredEdges(view store.Store, id string) *storedEdges {
	return &storedEdges{view: view, id: id, far: map[writerule.Hop][]string{}, isType: map[string]map[string]bool{}}
}

// Related reports whether a stored edge along hop reaches an entity of type
// target.
func (s *storedEdges) Related(ctx context.Context, hop writerule.Hop, target string) (bool, error) {
	far, err := s.farIDs(ctx, hop)
	if err != nil {
		return false, err
	}
	return s.anyOfType(ctx, target, far)
}

// farIDs lists the ids at the far end of the stored edges along hop.
func (s *storedEdges) farIDs(ctx context.Context, hop writerule.Hop) ([]string, error) {
	if ids, ok := s.far[hop]; ok {
		return ids, nil
	}
	q := store.RelationQuery{From: s.id, Type: hop.Relation, Direction: store.DirectionOutgoing}
	if hop.Incoming {
		q = store.RelationQuery{To: s.id, Type: hop.Relation, Direction: store.DirectionIncoming}
	}
	ids := []string{}
	for r, err := range s.view.ListRelations(ctx, q) {
		if err != nil {
			return nil, fmt.Errorf("list %s edges of %s: %w", hop.Relation, s.id, err)
		}
		if hop.Incoming {
			ids = append(ids, r.From)
		} else {
			ids = append(ids, r.To)
		}
	}
	s.far[hop] = ids
	return ids, nil
}

// anyOfType reports whether any of ids is an existing entity of type target.
// An edge to an entity that is gone does not count, as in the store's own
// answer to `related(...)` in `rela analyze validations`.
func (s *storedEdges) anyOfType(ctx context.Context, target string, ids []string) (bool, error) {
	known := s.isType[target]
	if known == nil {
		known = map[string]bool{}
		s.isType[target] = known
	}
	var ask []string
	for _, id := range ids {
		if _, ok := known[id]; !ok {
			ask = append(ask, id)
		}
	}
	if len(ask) > 0 {
		got, err := store.MatchingIDs(ctx, s.view,
			store.GraphQuery{EntityType: target, Faces: store.AllFaces()}, ask)
		if err != nil {
			return false, fmt.Errorf("match %s ends of %s: %w", target, s.id, err)
		}
		for _, id := range ask {
			known[id] = got[id]
		}
	}
	for _, id := range ids {
		if known[id] {
			return true, nil
		}
	}
	return false, nil
}

// pendingEdges answers [writerule.Edges] for the edges an entity will have
// after a relation write: the stored ones, with the write's creates added
// and its deletes removed.
type pendingEdges struct {
	stored  *storedEdges
	added   map[writerule.Hop][]string
	removed map[writerule.Hop]map[string]bool
}

func (p *pendingEdges) note(hop writerule.Hop, far string, add bool) {
	if add {
		if p.added == nil {
			p.added = map[writerule.Hop][]string{}
		}
		p.added[hop] = append(p.added[hop], far)
		return
	}
	if p.removed == nil {
		p.removed = map[writerule.Hop]map[string]bool{}
	}
	if p.removed[hop] == nil {
		p.removed[hop] = map[string]bool{}
	}
	p.removed[hop][far] = true
}

// readBy reports whether a relation rule of typ reads a hop this write
// changes.
func (p *pendingEdges) readBy(rules *writerule.Set, typ string) bool {
	for hop := range p.added {
		if rules.ReadsHop(typ, hop) {
			return true
		}
	}
	for hop := range p.removed {
		if rules.ReadsHop(typ, hop) {
			return true
		}
	}
	return false
}

// Related reports whether an edge along hop reaches an entity of type
// target after the write.
//
// A removed far id is dropped once for every stored edge to it: two edges
// to one entity differ only by their tail, which a relation rule's
// identity-scoped relation does not have.
//
// The added edges are asked first: they are few, and usually decide it.
func (p *pendingEdges) Related(ctx context.Context, hop writerule.Hop, target string) (bool, error) {
	if added := p.added[hop]; len(added) > 0 {
		ok, err := p.stored.anyOfType(ctx, target, added)
		if err != nil || ok {
			return ok, err
		}
	}
	stored, err := p.stored.farIDs(ctx, hop)
	if err != nil {
		return false, err
	}
	ids := make([]string, 0, len(stored))
	for _, id := range stored {
		if !p.removed[hop][id] {
			ids = append(ids, id)
		}
	}
	return p.stored.anyOfType(ctx, target, ids)
}
