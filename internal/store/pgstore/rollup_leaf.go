package pgstore

import (
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// A roll-up leaf of a Where tree (design gap 3.5, step 2) renders as one
// correlated scalar subquery: the along or siblings CTEs of the roll-up
// column, seeded with the candidate row's id alone. Every gate inside it is
// seeded with its level's frontier, as in the column statement, so the leaf
// costs the candidate's walk and never its gated types. It runs where the
// other post conjuncts run, inside the keyset scan of a page.

// leafSeed selects the candidate row's id: e is the row the Where tree is
// tested on.
const leafSeed = "SELECT e.id"

// rollupLeaf renders c as a boolean over the candidate row e.
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
			return sqlBool(!c.Holds)
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
	return "COALESCE((" + withClause(with) + "SELECT NOT (" + overCap + ") AND " + reached + "), FALSE)"
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

func sqlBool(v bool) string {
	if v {
		return "TRUE"
	}
	return "FALSE"
}
