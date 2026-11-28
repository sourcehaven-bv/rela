package rollup

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
)

// LeafFacts returns the facts [queryplan.LowerConditionRollups] decides a
// list condition's roll-up leaf by. orderGated reports whether some role
// may see the order field of a relation's edges hidden. Only an along or
// siblings roll-up can be exact: the other shapes have no per-candidate
// store walk.
//
// Nil: s accepted, it knows no roll-up; orderGated accepted, read as no
// grant.
func (s *Set) LeafFacts(orderGated func(relType string) bool) queryplan.RollupFacts {
	return func(entityType, property string) (queryplan.RollupLeafFacts, bool) {
		for _, c := range s.For(entityType) {
			if c.Property != property {
				continue
			}
			f := queryplan.RollupLeafFacts{Exists: c.Exists()}
			switch {
			case c.Along != nil:
				f.Exact, f.Nested = c.Along.Lowers(), len(c.Inner()) > 0
			case c.Siblings != nil:
				f.Exact, f.Siblings = true, true
				f.OrderFieldGated = orderGated != nil && orderGated(c.Siblings.Relation)
			}
			return f, true
		}
		return queryplan.RollupLeafFacts{}, false
	}
}

// Exists reports whether c is an exists roll-up: a count read as "more
// than zero".
func (c Column) Exists() bool {
	return c.Func == metamodel.RollupExists
}
