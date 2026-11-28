package visibility

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/rollup"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// A list condition's comparison of an exists roll-up with true or false
// can be answered inside the store's page scan (design gap 3.5, step 2):
// each candidate is the seed of its own walk, gated for the reader exactly
// as the roll-up column is. [Rollups.LeafQuery] builds that walk;
// [GateCondRollups] binds it into a Where tree.

// RollupLeafBinder returns the reader's store walk of the roll-up property
// of entityType for a [store.RollupCond]. ok=false: the walk has no store
// form for this reader, so the caller keeps its Go path.
type RollupLeafBinder func(ctx context.Context, entityType, property string) (store.RollupQuery, bool, error)

// LeafQuery is the store walk of the exists roll-up property of
// entityType as this reader sees it, with no Sources: the query a
// [store.RollupCond] seeds with each candidate. ok=false when the column is
// not an exists along or siblings roll-up, is nested, judges a node in Go,
// or a gate or a hidden property it reads has no store form.
func (r *Rollups) LeafQuery(
	ctx context.Context, entityType, property string, world store.WorldScope,
) (store.RollupQuery, bool, error) {
	col, found := r.columnOf(entityType, property)
	if !found || !col.Exists() {
		return store.RollupQuery{}, false, nil
	}
	var q store.RollupQuery
	var err error
	switch {
	case col.Siblings != nil:
		q, err = r.siblingsLeaf(ctx, col, world)
	case col.Along != nil && col.Along.Lowers() && len(col.Inner()) == 0:
		q, err = r.alongLeaf(ctx, col, world)
	default:
		return store.RollupQuery{}, false, nil
	}
	switch {
	case errors.Is(err, errFieldCondUnsupported), errors.Is(err, errNoReadQuery):
		return store.RollupQuery{}, false, nil
	case err != nil:
		return store.RollupQuery{}, false, err
	}
	return q, true, nil
}

// OrderFieldGated reports whether the reader may see the order of rel's
// edges hidden, so an earlier-siblings roll-up over rel can be withheld
// (see darkParents); a scoper that judges relation meta but cannot
// say which relations a grant touches is read as gating every one.
func (r *Rollups) OrderFieldGated(rel string) bool {
	if _, judges := r.fields.(relationFieldVerdicter); !judges {
		return false
	}
	g, ok := r.fields.(relationMetaGater)
	return !ok || g.RelationMetaGated(rel)
}

// relationMetaGater is the policy fact [Rollups.OrderFieldGated] reads:
// whether any role declares meta `visible:` grants for a relation type.
type relationMetaGater interface {
	RelationMetaGated(relType string) bool
}

// columnOf is the roll-up column property of entityType.
func (r *Rollups) columnOf(entityType, property string) (rollup.Column, bool) {
	for _, c := range r.set.For(entityType) {
		if c.Property == property {
			return c, true
		}
	}
	return rollup.Column{}, false
}

func (r *Rollups) alongLeaf(ctx context.Context, col rollup.Column, world store.WorldScope) (store.RollupQuery, error) {
	gates, err := r.alongGates(ctx, col.Along, world)
	if err != nil {
		return store.RollupQuery{}, err
	}
	counts, err := r.alongCounts(ctx, col, gates)
	if err != nil {
		return store.RollupQuery{}, err
	}
	return alongQuery(col, nil, gates, counts, false, r.rowCap), nil
}

func (r *Rollups) siblingsLeaf(
	ctx context.Context, col rollup.Column, world store.WorldScope,
) (store.RollupQuery, error) {
	q, err := r.siblingsQuery(ctx, col, nil, world, r.rowCap)
	if err != nil {
		return store.RollupQuery{}, err
	}
	q.Sources = nil
	return q, nil
}

// GateCondRollups is [GateCond] that also binds each roll-up leaf of c
// through leaves. A leaf leaves cannot bind is [ErrPropsNotPushable].
//
// Nil: leaves accepted; a roll-up leaf is then [ErrPropsNotPushable].
func GateCondRollups(
	ctx context.Context, gate traversalGate, leaves RollupLeafBinder, entityType string, c PageCond,
) (store.Cond, error) {
	if c.Op != store.CondRollup {
		return gateCond(ctx, gate, leaves, entityType, c)
	}
	if leaves == nil {
		return store.Cond{}, ErrPropsNotPushable
	}
	q, ok, err := leaves(ctx, entityType, c.Rollup)
	switch {
	case err != nil:
		return store.Cond{}, err
	case !ok:
		return store.Cond{}, ErrPropsNotPushable
	}
	return store.RollupLeaf(q, c.Holds), nil
}
