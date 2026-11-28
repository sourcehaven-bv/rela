package visibility

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// An earlier-siblings roll-up (design gap 2) is judged in the reader's
// world, gate before fold:
//
//   - every parent and every sibling is gated by the reader's read query
//     for its type ([Rollups.gate]), so one the reader cannot read is no
//     parent and no sibling;
//   - a where or when_parent equality holds only where the reader sees the
//     property it reads, so a hidden value never counts;
//   - the order of a parent's children is read from [store.SiblingOrderProperty]
//     on their edges, so when a relation field grant hides it on the edge
//     of a sibling the reader can read, the roll-up is absent for every
//     child of that parent. The edge of a sibling the reader cannot read is
//     never judged: its order is already left out.

// relationFieldVerdicter is the part of a [FieldScoper] that judges the
// meta of a relation for the reader: false for each key it hides on the
// edge from from. A scoper without it hides no relation meta.
type relationFieldVerdicter interface {
	RelationFieldVerdicts(ctx context.Context, from *entity.Entity, relType string, metaKeys []string) map[string]bool
}

// siblingsColumn runs one earlier-siblings roll-up for ids. withheld are
// the ids whose value is absent because the order of one of their parents
// is hidden from the reader.
func (r *Rollups) siblingsColumn(
	ctx context.Context, col rollup.Column, ids []string, world store.WorldScope, rowCap int,
) (results map[string]store.RollupResult, withheld map[string]bool, err error) {
	q, err := r.siblingsQuery(ctx, col, ids, world, rowCap)
	if err != nil {
		return nil, nil, err
	}
	results, err = store.Rollup(ctx, r.st, q)
	if err != nil {
		return nil, nil, err
	}
	dark, err := r.darkParents(ctx, col.Siblings.Relation, results, q.Siblings.Siblings)
	if err != nil {
		return nil, nil, err
	}
	withheld = map[string]bool{}
	for src, res := range results {
		if slices.ContainsFunc(res.Parents, func(p string) bool { return dark[p] }) {
			withheld[src] = true
		}
	}
	return results, withheld, nil
}

// siblingsQuery is the store query of col for ids: every parent and
// sibling gated for the reader in world.
func (r *Rollups) siblingsQuery(
	ctx context.Context, col rollup.Column, ids []string, world store.WorldScope, rowCap int,
) (store.RollupQuery, error) {
	sw := col.Siblings
	parents, err := r.siblingNodes(ctx, sw.Parents, sw.WhenParent, world)
	if err != nil {
		return store.RollupQuery{}, err
	}
	members, err := r.siblingNodes(ctx, sw.Members, sw.Where, world)
	if err != nil {
		return store.RollupQuery{}, err
	}
	return store.RollupQuery{
		Sources: ids, Func: col.StoreFunc(), Siblings: &store.SiblingWalk{
			Relation: sw.Relation, Parents: parents, Siblings: members, Climb: sw.Climb,
			CountSiblings: sw.CountSiblings, RowCap: rowCap,
		},
	}, nil
}

// siblingNodes is the reader's gate per type of types, each judged by its
// equalities in conds with every property they read required visible. A
// type the reader reads none of is left out.
func (r *Rollups) siblingNodes(
	ctx context.Context, types []string, conds map[string][]store.PropPredicate, world store.WorldScope,
) (map[string]store.AlongNode, error) {
	out := map[string]store.AlongNode{}
	for _, typ := range types {
		g, denied, err := r.gate(ctx, typ, world)
		switch {
		case err != nil:
			return nil, err
		case denied:
			continue
		}
		node := store.AlongNode{Gate: g}
		if preds := conds[typ]; len(preds) > 0 {
			fs, err := r.fields.SearchFieldScope(ctx, typ)
			if err != nil {
				return nil, fmt.Errorf("field scope: %w", err)
			}
			faces := returnableFaces(g.FaceIn, g.Faces, typ)
			args := make([]store.Cond, 0, 2*len(preds))
			for _, p := range preds {
				v, err := visibleCond(fs, typ, p.Property, faces)
				if err != nil {
					return nil, err
				}
				args = append(args, v, store.PropLeaf(p))
			}
			c := store.AndCond(args...)
			if err := store.ValidateCond(c); err != nil {
				return nil, fmt.Errorf("%w: %w", errFieldCondUnsupported, err)
			}
			node.Counts = &c
		}
		out[typ] = node
	}
	return out, nil
}

// darkParents is the parents in results on the edge of whose readable
// children the reader may not read [store.SiblingOrderProperty]: one
// relation read for their incoming edges, one gated header read per member
// type, and one traversal priming for the policy's verdicts.
func (r *Rollups) darkParents(
	ctx context.Context, rel string, results map[string]store.RollupResult, members map[string]store.AlongNode,
) (dark map[string]bool, _ error) {
	fv, judges := r.fields.(relationFieldVerdicter)
	gated, ok := r.fields.(interface{ RelationMetaGated(relType string) bool })
	if !judges || ok && !gated.RelationMetaGated(rel) {
		return dark, nil
	}
	var parents []string
	for _, res := range results {
		parents = append(parents, res.Parents...)
	}
	slices.Sort(parents)
	parents = slices.Compact(parents)
	if len(parents) == 0 {
		return dark, nil
	}
	identity := entity.Face("")
	parentsOf := map[string][]string{}
	for e, err := range r.st.ListRelations(ctx, store.RelationQuery{
		Type: rel, EntityIDs: parents, FromFace: &identity, Direction: store.DirectionIncoming,
	}) {
		if err != nil {
			return nil, err
		}
		parentsOf[e.From] = append(parentsOf[e.From], e.To)
	}
	var froms []*entity.Entity
	for _, typ := range slices.Sorted(maps.Keys(members)) {
		gate := members[typ].Gate
		gate.Related = append(slices.Clone(gate.Related), store.DirectedRelation{
			Pred: store.RelationPredicate{OfTypes: []string{rel}, Endpoints: parents},
		})
		for h, err := range store.GraphQueryHeaders(ctx, r.st, gate) {
			if err != nil {
				return nil, err
			}
			froms = append(froms, &entity.Entity{ID: h.ID, Type: typ, Face: h.Face, Properties: h.Properties})
		}
	}
	if primer, ok := r.fields.(TraversalPrimer); ok {
		ctx = primer.PrimeTraversals(ctx, froms)
	}
	dark = map[string]bool{}
	keys := []string{store.SiblingOrderProperty}
	for _, from := range froms {
		if shown, ok := fv.RelationFieldVerdicts(ctx, from, rel, keys)[store.SiblingOrderProperty]; ok && !shown {
			for _, p := range parentsOf[from.ID] {
				dark[p] = true
			}
		}
	}
	return dark, nil
}
