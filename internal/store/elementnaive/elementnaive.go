// Package elementnaive answers the element range read (store.ElementScope,
// TKT-IQU106C) with ordinary store reads and computes each element in Go
// with the reconcile package. Its [Ranges] is the element range reader of
// memstore and fsstore, and it is the reference the SQL backends are
// compared with (storetest.RunElementRangeDifferential).
//
// The number of store calls does not depend on the number of rows in range:
// one ListEntities per entity type in the plan (or one full read of the
// endpoint rows for a relation range), one ListRelations per relation type,
// one MatchingFaces per gated type, and one store.HoldingRows per hidden
// field. Each read covers the whole type, so the work and memory grow with
// the type, not the range. That is acceptable for the in-process backends it
// serves. The calls are separate reads, not one snapshot.
package elementnaive

import (
	"context"
	"errors"
	"iter"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
)

// Reader is what the reference reads through.
type Reader interface {
	store.GraphQueryer
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
	ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
}

// Ranges is the element range reader of a store that has no native one:
// memstore and fsstore.
type Ranges struct {
	st Reader
}

// New returns the element range reader over st.
// Nil: rejected, as every required collaborator is.
func New(st Reader) (*Ranges, error) {
	if st == nil {
		return nil, errors.New("elementnaive: nil store")
	}
	return &Ranges{st: st}, nil
}

// RangeFingerprint returns the count and fingerprint sum of the elements
// scope admits in r. See store.ElementScope.
func (g *Ranges) RangeFingerprint(
	ctx context.Context, scope store.ElementScope, r store.ElementRange,
) (store.RangeSum, error) {
	return RangeFingerprint(ctx, g.st, scope, r)
}

// ListRange lists the elements scope admits in r, in address order, up to
// l.Limit. See store.ElementScope.
func (g *Ranges) ListRange(
	ctx context.Context, scope store.ElementScope, r store.ElementRange, l store.RangeList,
) ([]store.RangeEntry, error) {
	return ListRange(ctx, g.st, scope, r, l)
}

// RangeFingerprint is [Ranges.RangeFingerprint] on st.
func RangeFingerprint(
	ctx context.Context, st Reader, scope store.ElementScope, r store.ElementRange,
) (store.RangeSum, error) {
	var sum store.RangeSum
	err := walk(ctx, st, scope, r, func(e row) bool {
		sum.Count++
		if e.pending {
			sum.Pending++
			return true
		}
		for i, l := range e.cols.Limbs {
			sum.LimbSums[i] += uint64(l) //nolint:gosec // a limb is below 2^32
		}
		return true
	})
	if err != nil {
		return store.RangeSum{}, err
	}
	return sum, nil
}

// ListRange is [Ranges.ListRange] on st.
func ListRange(
	ctx context.Context, st Reader, scope store.ElementScope, r store.ElementRange, l store.RangeList,
) ([]store.RangeEntry, error) {
	if err := store.ValidateRangeList(l); err != nil {
		return nil, err
	}
	var out []store.RangeEntry
	err := walk(ctx, st, scope, r, func(e row) bool {
		if l.PendingOnly && !e.pending {
			return true
		}
		out = append(out, e.entry())
		return true
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(out, func(a, b store.RangeEntry) int { return strings.Compare(a.Address, b.Address) })
	if len(out) > l.Limit {
		out = out[:l.Limit]
	}
	return out, nil
}

// row is one admitted element.
type row struct {
	store.RangeEntry
	pending bool
	cols    reconcile.Columns
}

func (e row) entry() store.RangeEntry {
	out := e.RangeEntry
	out.Pending = e.pending
	if !e.pending {
		out.Hash = e.cols.Hash
	}
	return out
}

// Plan is store.PlanElementRange plus the shape check of every gate and
// hidden-field condition ([store.TypePlan.Probes]) that a graph query gets.
// Every element range reader plans through it, so all of them refuse the
// same requests, before any read: the SQL builders render only checked
// shapes, and an unchecked one could widen the fold.
func Plan(scope store.ElementScope, r store.ElementRange) (store.ElementRangePlan, error) {
	plan, err := store.PlanElementRange(scope, r)
	if err != nil {
		return store.ElementRangePlan{}, err
	}
	for _, tp := range plan.Types {
		for _, q := range tp.Probes() {
			if err := graphquerynaive.CheckEndpointShape(q); err != nil {
				return store.ElementRangePlan{}, err
			}
		}
	}
	return plan, nil
}

// walk calls yield for every row the scope admits in r, in no particular
// order.
func walk(ctx context.Context, st Reader, scope store.ElementScope, r store.ElementRange, yield func(row) bool) error {
	plan, err := Plan(scope, r)
	if err != nil {
		return err
	}
	if plan.Empty() {
		return nil
	}
	if r.Relations {
		return walkRelations(ctx, st, plan, yield)
	}
	return walkEntities(ctx, st, plan, yield)
}

func walkEntities(ctx context.Context, st Reader, plan store.ElementRangePlan, yield func(row) bool) error {
	for _, tp := range plan.Types {
		var rows []*entity.Entity
		for e, err := range st.ListEntities(ctx, store.EntityQuery{Type: tp.Type, Faces: store.AllFaces()}) {
			if err != nil {
				return err
			}
			if addr := reconcile.EntityAddress(e.Type, e.Ref()); addr >= tp.Lo && addr < tp.Hi {
				rows = append(rows, e)
			}
		}
		admitted, err := admit(ctx, st, tp, rows)
		if err != nil {
			return err
		}
		foldable, err := foldable(ctx, st, tp, admitted)
		if err != nil {
			return err
		}
		for _, e := range admitted {
			el := reconcile.EntityElement(*e)
			if !yield(row{
				RangeEntry: store.RangeEntry{Address: el.Address, Type: e.Type, Ref: e.Ref()},
				pending:    !foldable(e),
				cols:       el.Columns(),
			}) {

				return nil
			}
		}
	}
	return nil
}

// admit keeps the rows tp's gate and face allowlist admit.
func admit(ctx context.Context, st Reader, tp store.TypePlan, rows []*entity.Entity) ([]*entity.Entity, error) {
	var matched map[string][]entity.Face
	if tp.Gate != nil && len(rows) > 0 {
		var err error
		if matched, err = st.MatchingFaces(ctx, *tp.Gate, ids(rows)); err != nil {
			return nil, err
		}
	}
	out := rows[:0:0]
	for _, e := range rows {
		if !tp.Admits(e.Face) {
			continue
		}
		if tp.Gate != nil && !slices.Contains(matched[e.ID], e.Face) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// foldable returns whether a row of tp may fold its stored element.
func foldable(
	ctx context.Context, st Reader, tp store.TypePlan, rows []*entity.Entity,
) (func(*entity.Entity) bool, error) {
	switch tp.Fold {
	case store.FoldStored:
		return func(*entity.Entity) bool { return true }, nil
	case store.FoldPerRow:
	default:
		return func(*entity.Entity) bool { return false }, nil
	}
	ids := ids(rows)
	visible := make([]func(store.EntityHeader) bool, len(tp.Hidden))
	for i, h := range tp.Hidden {
		holds, err := store.HoldingRows(ctx, st, tp.Type, []store.RowCondition{h.Visible}, ids)
		if err != nil {
			return nil, err
		}
		visible[i] = holds
	}
	return func(e *entity.Entity) bool {
		for i, h := range tp.Hidden {
			if _, present := e.Properties[h.Property]; !present {
				continue
			}
			if !visible[i](store.EntityHeader{ID: e.ID, Face: e.Face, Type: e.Type}) {
				return false
			}
		}
		return true
	}, nil
}

func walkRelations(ctx context.Context, st Reader, plan store.ElementRangePlan, yield func(row) bool) error {
	fold := plan.FoldTypes()
	var rels []*entity.Relation
	for _, rp := range plan.Relations {
		for rel, err := range st.ListRelations(ctx, store.RelationQuery{Type: rp.Type}) {
			if err != nil {
				return err
			}
			addr := rel.Identity().String()
			if addr >= plan.Range.Lo && (plan.Range.Unbounded || addr < plan.Range.Hi) {
				rels = append(rels, rel)
			}
		}
	}
	readable, err := readableEndpoints(ctx, st, plan, rels)
	if err != nil {
		return err
	}
	for _, rel := range rels {
		head := len(readable[rel.To]) > 0
		tail := len(readable[rel.From]) > 0
		if !rel.FromFace.IsImplicit() {
			tail = slices.Contains(readable[rel.From], rel.FromFace)
		}
		if !head || !tail {
			continue
		}
		el := reconcile.RelationElement(*rel)
		if !yield(row{
			RangeEntry: store.RangeEntry{Address: el.Address, Relation: rel.Identity()},
			pending:    !slices.Contains(fold, rel.Type),
			cols:       el.Columns(),
		}) {

			return nil
		}
	}
	return nil
}

// readableEndpoints returns, per endpoint id of rels, the faces the scope
// admits. An endpoint stored under a type outside the scope has none.
func readableEndpoints(
	ctx context.Context, st Reader, plan store.ElementRangePlan, rels []*entity.Relation,
) (map[string][]entity.Face, error) {
	out := map[string][]entity.Face{}
	if len(rels) == 0 {
		return out, nil
	}
	var endpoints []string
	for _, rel := range rels {
		endpoints = append(endpoints, rel.From, rel.To)
	}
	slices.Sort(endpoints)
	endpoints = slices.Compact(endpoints)
	byType := map[string][]*entity.Entity{}
	for e, err := range st.ListEntities(ctx, store.EntityQuery{IDs: endpoints, Faces: store.AllFaces()}) {
		if err != nil {
			return nil, err
		}
		byType[e.Type] = append(byType[e.Type], e)
	}
	for _, tp := range plan.Types {
		admitted, err := admit(ctx, st, tp, byType[tp.Type])
		if err != nil {
			return nil, err
		}
		for _, e := range admitted {
			out[e.ID] = append(out[e.ID], e.Face)
		}
	}
	return out, nil
}

func ids(rows []*entity.Entity) []string {
	out := make([]string, 0, len(rows))
	for _, e := range rows {
		out = append(out, e.ID)
	}
	slices.Sort(out)
	return slices.Compact(out)
}
