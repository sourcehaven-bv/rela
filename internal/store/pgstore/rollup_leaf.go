package pgstore

import (
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// A roll-up leaf of a Where tree (design gap 3.5, step 2) renders as one
// correlated scalar subquery: the along or siblings CTEs of the roll-up
// column, seeded with the candidate row's id alone. Every gate inside it is
// seeded with its level's frontier, as in the column statement, so the leaf
// costs the candidate's walk and never its gated types. It runs where the
// other post conjuncts run, inside the keyset scan of a page.
//
// An absent leaf fails closed under every polarity (see [store.RollupCond]):
// the leaf is the integer 1 when it holds, 0 when it does not and NULL when
// its walk is over the cap. A Where subtree with a leaf renders through
// rollupStrict, whose integer AND, OR and NOT carry a NULL to the root,
// where `= 1` drops the row. Each leaf is rendered, and so walked, once.

// leafSeed selects the candidate row's id: e is the row the Where tree is
// tested on.
const leafSeed = "SELECT e.id"

// rollupStrict renders c, a subtree with a roll-up leaf, as an integer:
// 1 when it holds, 0 when it does not, NULL when a leaf in it is absent.
// A subtree without a leaf is its two-valued boolean as 1 or 0.
func (cb *condBuilder) rollupStrict(c store.Cond) string {
	if !store.HasRollupLeaf(&c) {
		return "(CASE WHEN " + cb.render(c) + " THEN 1 ELSE 0 END)"
	}
	switch c.Op {
	case store.CondRollup:
		return cb.rollupLeaf(c.Rollup)
	case store.CondNot:
		if len(c.Args) != 1 {
			return cb.b.fail(store.ValidateCond(c))
		}
		return "(1 - " + cb.rollupStrict(c.Args[0]) + ")"
	case store.CondAnd, store.CondOr:
		parts := make([]string, len(c.Args))
		for i, a := range c.Args {
			parts[i] = cb.rollupStrict(a)
		}
		if c.Op == store.CondAnd {
			return "(" + strings.Join(parts, " * ") + ")"
		}
		return "sign(" + strings.Join(parts, " + ") + ")"
	default:
		return cb.b.fail(store.ValidateCond(c))
	}
}

// rollupLeaf renders c as an integer over the candidate row e: 1, 0, or
// NULL when the candidate's walk is over its cap.
func (cb *condBuilder) rollupLeaf(c *store.RollupCond) string {
	if err := store.ValidateCond(store.Cond{Op: store.CondRollup, Rollup: c}); err != nil {
		return cb.b.fail(err)
	}
	q := c.Query
	var with []string
	var overCap, reached string
	switch {
	case q.Siblings != nil:
		for _, nodes := range []map[string]store.AlongNode{q.Siblings.Parents, q.Siblings.Siblings} {
			if err := checkLeafGates(nodes); err != nil {
				return cb.b.fail(err)
			}
		}
		with, overCap = siblingsWalkSQL(cb.b, q, leafSeed)
		reached = "EXISTS (SELECT 1 FROM sj j WHERE EXISTS (SELECT 1 FROM sf m WHERE m.parent = j.parent AND " +
			siblingBefore + "))"
	default:
		for _, seg := range q.Along.Segments {
			if err := checkLeafGates(seg.Through); err != nil {
				return cb.b.fail(err)
			}
		}
		levels := store.AlongLevels(q.Along)
		if len(levels) == 0 {
			return sqlBit(!c.Holds)
		}
		limit := q.Along.RowCap
		if limit == 0 {
			limit = store.AlongRowCap
		}
		with, overCap = alongCountedSQL(cb.b, q, levels, leafSeed, limit)
		reached = "EXISTS (SELECT 1 FROM counted)"
	}
	if !c.Holds {
		reached = "NOT " + reached
	}
	return "(" + withClause(with) + "SELECT CASE WHEN " + overCap + " THEN NULL WHEN " + reached +
		" THEN 1 ELSE 0 END)"
}

// checkLeafGates refuses a gate shape every backend refuses.
func checkLeafGates(nodes map[string]store.AlongNode) error {
	for _, node := range nodes {
		if err := checkGraphQueryScope(node.Gate); err != nil {
			return err
		}
	}
	return nil
}

func sqlBit(v bool) string {
	if v {
		return "1"
	}
	return "0"
}
