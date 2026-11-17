package visibility

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// PageQuery asks for one page of the rows of a type that a caller may read
// (TKT-D9UO9Y4). Rows come in ascending id order, then face order, the
// default order of [store.GraphQuery].
type PageQuery struct {
	// Type is the entity type. Required: the read scope is composed per
	// type.
	Type string
	// Faces is the caller's face selection, as on [store.EntityQuery].
	Faces store.FaceSelection
	// Props narrows the rows by the type's own properties. Each property
	// must be visible to the caller on every row it reads (see
	// [ErrPropsNotPushable]); the store compares raw values, so a property
	// hidden on some row would answer for a value the caller cannot see.
	Props []store.PropPredicate
	// Traversals narrow the rows by their edges (TKT-Z099FGY). The reader
	// gates each one itself with the caller's traversal gate, so the row
	// set never depends on an edge the caller cannot see. See
	// [PageTraversal].
	Traversals []PageTraversal
	// Where narrows the rows by a boolean condition ANDed with Props and
	// Traversals (TKT-8HY05WB). Its properties follow the rule of Props and
	// its relation leaves the rule of Traversals. See [PageCond].
	Where *PageCond
	// Limit is the page size. It must be positive: a page read never
	// returns the whole type.
	Limit int
	// Offset counts rows in the ordered result.
	Offset int
}

// PageTraversal is one `related(...)` term of a page filter: the hop it
// walks and whether the filter negates it. It carries no store predicate:
// the reader builds the edge test from the gate's answer for the caller,
// so a caller cannot pass one the gate did not produce. A traversal onto a
// type the caller may not read makes a positive term match nothing and a
// negated term match everything, as the Go evaluation does.
type PageTraversal struct {
	Hop    acl.TraversalHop
	Negate bool
}

// PageCond is a boolean page filter: a [store.Cond] whose relation leaves
// are hops rather than store predicates. Op is a [store.CondOp]; Prop is
// meaningful for CondProp, Hop for CondRelated, Args for the others. As with
// [PageTraversal], the reader builds each edge test from the gate's answer,
// so a caller cannot pass an ungated one; a hop onto a type the caller may
// not read is false.
type PageCond struct {
	Op   store.CondOp
	Prop store.PropPredicate
	Hop  acl.TraversalHop
	Args []PageCond
}

// Props returns the property predicates of c's leaves.
func (c PageCond) Props() []store.PropPredicate {
	var out []store.PropPredicate
	c.walk(func(n PageCond) {
		if n.Op == store.CondProp {
			out = append(out, n.Prop)
		}
	})
	return out
}

func (c PageCond) walk(visit func(PageCond)) {
	visit(c)
	for _, a := range c.Args {
		a.walk(visit)
	}
}

// GateCond turns c into a [store.Cond] through gate, the caller's traversal
// gate. A denied hop becomes false (an OR without arguments), as
// `related(...)` is false in Go for a principal who may not read the far
// type. Every hop is gated before the result is built, so a refusal wins
// over a denial: a missing or mismatched identity is returned as is, and
// any other gate error, a nil predicate or one the gate already negated is
// [ErrPropsNotPushable].
func GateCond(
	ctx context.Context, gate func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error),
	entityType string, c PageCond,
) (store.Cond, error) {
	switch c.Op {
	case store.CondRelated:
		rels, denied, err := gateTraversals(ctx, gate, entityType, []PageTraversal{{Hop: c.Hop}})
		switch {
		case err != nil:
			return store.Cond{}, err
		case denied:
			return store.OrCond(), nil
		}
		return store.RelatedCond(rels[0]), nil
	case store.CondProp:
		return store.PropCond(c.Prop), nil
	default:
	}
	out := store.Cond{Op: c.Op, Args: make([]store.Cond, 0, len(c.Args))}
	for _, a := range c.Args {
		ga, err := GateCond(ctx, gate, entityType, a)
		if err != nil {
			return store.Cond{}, err
		}
		out.Args = append(out.Args, ga)
	}
	return out, nil
}

// traversalGate answers one hop for the caller: the store edge test, or
// [acl.ErrTraversalDenied]. [DeclarativeGate.GateTraversal] and
// [acl.UngatedTraversal] (wrapped) are the two in use.
type traversalGate func(ctx context.Context, candidateType string, hop acl.TraversalHop) (
	*store.RelationPredicate, error)

// traversalGater is the binder capability a page read with traversals
// needs. [DeclarativeGate] implements it.
type traversalGater interface {
	GateTraversal(ctx context.Context, candidateType string, hop acl.TraversalHop) (*store.RelationPredicate, error)
}

// gateTraversals turns ts into store edge tests through gate. empty
// reports that a positive term is denied and none is refused, so no row
// matches. A denied negated term drops out. A missing or mismatched
// identity fails the read. Any other gate error, a nil predicate, or a
// predicate the gate already negated is
// [ErrPropsNotPushable], so the caller answers the filter on its own path,
// which reports the same refusal.
func gateTraversals(
	ctx context.Context, gate traversalGate, entityType string, ts []PageTraversal,
) (rels []store.DirectedRelation, empty bool, err error) {
	for _, t := range ts {
		pred, gerr := gate(ctx, entityType, t.Hop)
		switch {
		case errors.Is(gerr, acl.ErrTraversalDenied):
			// Keep gating the rest: a refusal must still win over a denial.
			if !t.Negate {
				empty = true
			}
			continue
		case errors.Is(gerr, acl.ErrNoIdentity), errors.Is(gerr, acl.ErrIdentityMismatch):
			// No caller to answer for: fail the read, never fall back.
			return nil, false, gerr
		case gerr != nil:
			return nil, false, fmt.Errorf("%w: %w", ErrPropsNotPushable, gerr)
		case pred == nil, pred.Negate:
			return nil, false, ErrPropsNotPushable
		}
		rel := store.DirectedRelation{Incoming: t.Hop.Incoming, Pred: *pred}
		rel.Pred.Negate = t.Negate
		rels = append(rels, rel)
	}
	if empty {
		return nil, true, nil
	}
	return rels, false, nil
}

// EntityPage is one page of a [PageQuery]. Rows carry no body: they are
// read as headers, then redacted for the caller.
type EntityPage struct {
	Rows []*entity.Entity
	// Total is how many rows the query matches for the caller: counted in
	// the store over the same read scope as Rows, never the type's raw row
	// count, which would tell the caller how many rows are hidden from it.
	Total int
}

// ErrPropsNotPushable reports a [PageQuery] narrowing the reader cannot
// push: Props naming a property the caller does not see on every row, a
// traversal the gate refuses, or, on a reader without pushdown, any Props
// or Traversals. The caller answers such a filter over redacted rows
// instead.
var ErrPropsNotPushable = errors.New("visibility: the page filter cannot be pushed to the store")

// errPageLimit reports a [PageQuery] without a positive Limit.
var errPageLimit = errors.New("visibility: a page query needs a positive limit")

// fieldVisibility is the redactor capability a filtered page read needs:
// whether the caller sees property on every row of entityType it reads.
// [PolicyRedactor] and [NopRedactor] implement it. A redactor without it
// gets no filter pushed, which is a refusal, never a weaker gate.
type fieldVisibility interface {
	FieldVisibleOnEveryRow(ctx context.Context, entityType, property string) bool
}

// FieldVisibleOnEveryRow implements [fieldVisibility]. See
// [affordances.PolicyResolver.FieldVisibleOnEveryRow].
func (p PolicyRedactor) FieldVisibleOnEveryRow(ctx context.Context, entityType, property string) bool {
	return p.r.FieldVisibleOnEveryRow(ctx, entityType, property)
}

// FieldVisibleOnEveryRow implements [fieldVisibility]: nothing is hidden.
func (NopRedactor) FieldVisibleOnEveryRow(context.Context, string, string) bool { return true }

// propsFilterable reports whether every property of props is visible to the
// caller on every row of entityType, so the store may filter on it.
func (r *PolicyReader) propsFilterable(ctx context.Context, entityType string, props []store.PropPredicate) bool {
	if len(props) == 0 {
		return true
	}
	fv, ok := r.redact.(fieldVisibility)
	if !ok {
		return false
	}
	for _, p := range props {
		if !fv.FieldVisibleOnEveryRow(ctx, entityType, p.Property) {
			return false
		}
	}
	return true
}

// propsFilterer is the [PolicyReader] half of a filtered page read, taken by
// type assertion like [pushdownRedactor].
type propsFilterer interface {
	propsFilterable(ctx context.Context, entityType string, props []store.PropPredicate) bool
}

// checkPageQuery rejects the query shapes every implementation refuses.
func checkPageQuery(q PageQuery) error {
	if q.Type == "" {
		return errors.New("visibility: a page query needs a type")
	}
	if q.Limit <= 0 {
		return errPageLimit
	}
	return nil
}

// ListEntityPage reads one page of the rows of q.Type the caller may read,
// redacted, and the number of rows the query matches for the caller.
//
// The row gate runs in the store: the page and the total are one
// [store.GraphQuery] and one [store.CountMatched] over the caller's composed
// read scope, so the cost follows the page, not the type. The field gate
// runs on the page, primed once for it (PolicyReader.redactPage).
//
// Props are pushed only when the caller sees each property on every row it
// reads; otherwise the read returns [ErrPropsNotPushable]. Traversals are
// gated with the binder's traversal gate before anything is read (see
// [PageTraversal]); a binder without one refuses them. Without pushdown (a
// reader that cannot redact without gating, or no read-scope provider) a
// query without Props or Traversals is answered by load-then-Filter, and
// one with either is refused with ErrPropsNotPushable. Both are a cost or a
// refusal, never a weaker gate.
func (s *ScriptReader) ListEntityPage(ctx context.Context, q PageQuery) (EntityPage, error) {
	if err := checkPageQuery(q); err != nil {
		return EntityPage{}, err
	}
	bound := s.bind(ctx)
	if props := pageProps(q); len(props) > 0 {
		if pf, ok := s.reader.(propsFilterer); !ok || !pf.propsFilterable(bound, q.Type, props) {
			return EntityPage{}, ErrPropsNotPushable
		}
	}
	var (
		related []store.DirectedRelation
		where   *store.Cond
	)
	if len(q.Traversals) > 0 || q.Where != nil {
		g, ok := s.binder.(traversalGater)
		if !ok {
			return EntityPage{}, ErrPropsNotPushable
		}
		var empty bool
		var err error
		related, where, empty, err = gatePageFilter(bound, g.GateTraversal, q)
		switch {
		case err != nil:
			return EntityPage{}, err
		case empty:
			return EntityPage{}, nil
		}
	}
	red, ok := s.reader.(pushdownRedactor)
	if !ok {
		return s.pageViaFilter(bound, q)
	}
	scope, ok, err := composeReadScope(bound, s.provider, s.raw, store.EntityQuery{Type: q.Type, Faces: q.Faces})
	switch {
	case !ok:
		return s.pageViaFilter(bound, q)
	case err != nil:
		return EntityPage{}, err
	case scope.deny:
		return EntityPage{}, nil
	}
	var gq store.GraphQuery
	switch {
	case scope.allow != nil:
		gq = store.GraphQuery{EntityType: q.Type, Faces: q.Faces, FaceIn: scope.allow.FaceIn}
	case scope.graph != nil:
		gq = *scope.graph // composeReadScope already copied the ACL's query
	default:
		return EntityPage{}, errEmptyScope
	}
	raw, ok := s.raw.(store.GraphQueryer)
	if !ok { // coverage-ignore: composeReadScope returned ok, so raw is a GraphQueryer
		return EntityPage{}, errEmptyScope
	}
	return readPage(bound, raw, gq, q, related, where, red.redactPage)
}

// pageProps returns every property predicate of q, Where's included.
func pageProps(q PageQuery) []store.PropPredicate {
	if q.Where == nil {
		return q.Props
	}
	return slices.Concat(q.Props, q.Where.Props())
}

// gatePageFilter gates q's Traversals and Where through gate. Where is
// gated even when a traversal is denied, so a refusal in it still wins. A
// Where its denied hops make constant is folded: false is empty, true is
// no Where.
func gatePageFilter(ctx context.Context, gate traversalGate, q PageQuery) (
	related []store.DirectedRelation, where *store.Cond, empty bool, err error) {
	related, empty, err = gateTraversals(ctx, gate, q.Type, q.Traversals)
	if err != nil {
		return nil, nil, false, err
	}
	if q.Where != nil {
		w, werr := GateCond(ctx, gate, q.Type, *q.Where)
		if werr != nil {
			return nil, nil, false, werr
		}
		switch v, known := w.Constant(); {
		case !known:
			where = &w
		case !v:
			empty = true
		}
	}
	return related, where, empty, nil
}

// readPage runs the page and the count of q on gq, a copy of the caller's
// read scope, narrowed by related, the gated edge tests of q.Traversals,
// and where, the gated q.Where, and redacts the page with redact.
func readPage(
	ctx context.Context, raw store.GraphQueryer, gq store.GraphQuery, q PageQuery,
	related []store.DirectedRelation, where *store.Cond, redact pageRedactor,
) (EntityPage, error) {
	gq.Props = slices.Concat(gq.Props, q.Props)
	gq.Related = slices.Concat(gq.Related, related)
	gq.Where = store.AndWhere(gq.Where, where)
	total, err := store.CountMatched(ctx, raw, gq)
	if err != nil {
		return EntityPage{}, err
	}
	if total <= q.Offset {
		return EntityPage{Total: total}, nil
	}
	gq.Limit = q.Limit
	gq.Offset = max(q.Offset, 0)
	rows := make([]*entity.Entity, 0, min(q.Limit, total-q.Offset))
	for h, err := range store.GraphQueryHeaders(ctx, raw, gq) {
		if err != nil {
			return EntityPage{}, err
		}
		rows = append(rows, headerRow(h))
	}
	return EntityPage{Rows: redact(ctx, rows), Total: total}, nil
}

// headerRow is the content-free entity a page serves for header h.
func headerRow(h store.EntityHeader) *entity.Entity {
	return &entity.Entity{
		ID: h.ID, Type: h.Type, Face: h.Face, Properties: h.Properties,
		UpdatedAt: h.UpdatedAt, Inaccessible: h.Inaccessible,
	}
}

// pageViaFilter is [ScriptReader.ListEntityPage] without pushdown: the rows
// of the type, gated by Filter, then cut to the page. It reads the whole
// type, so it only serves a reader that cannot push the gate down; no
// production wiring builds one.
func (s *ScriptReader) pageViaFilter(ctx context.Context, q PageQuery) (EntityPage, error) {
	if len(q.Props) > 0 || len(q.Traversals) > 0 || q.Where != nil {
		return EntityPage{}, ErrPropsNotPushable
	}
	rows, err := s.listGatedThenRanked(ctx, store.EntityQuery{Type: q.Type, Faces: q.Faces})
	if err != nil {
		return EntityPage{}, err
	}
	kept := s.reader.Filter(ctx, rows)
	slices.SortFunc(kept, compareRowKey)
	total := len(kept)
	start := min(max(q.Offset, 0), total)
	page := kept[start:min(start+q.Limit, total)]
	for i, e := range page {
		c := *e
		c.Content = ""
		page[i] = &c
	}
	return EntityPage{Rows: page, Total: total}, nil
}

// compareRowKey orders rows as [store.GraphQuery] does by default: id, then
// face.
func compareRowKey(a, b *entity.Entity) int {
	return cmp.Or(cmp.Compare(a.ID, b.ID), cmp.Compare(a.Face, b.Face))
}

// ListEntityPage implements the page read with no gate: one
// [store.GraphQuery] page and one [store.CountMatched] on the raw store.
func (r *UnrestrictedReader) ListEntityPage(ctx context.Context, q PageQuery) (EntityPage, error) {
	if err := checkPageQuery(q); err != nil {
		return EntityPage{}, err
	}
	gq, ok := r.st.(store.GraphQueryer)
	if !ok { // coverage-ignore: store.Store embeds GraphQueryer
		return EntityPage{}, fmt.Errorf("visibility: store %T cannot run a graph query", r.st)
	}
	related, where, empty, err := gatePageFilter(ctx, acl.UngatedGate, q)
	switch {
	case err != nil:
		return EntityPage{}, err
	case empty: // coverage-ignore: the ungated lowering never denies
		return EntityPage{}, nil
	}
	keep := func(_ context.Context, rows []*entity.Entity) []*entity.Entity { return rows }
	return readPage(ctx, gq, store.GraphQuery{EntityType: q.Type, Faces: q.Faces}, q, related, where, keep)
}

// ListEntityPage implements the page read: always refuses.
func (DenyReader) ListEntityPage(context.Context, PageQuery) (EntityPage, error) {
	return EntityPage{}, ErrReaderUnavailable
}
