package queryplan

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RollupLeafFacts are the load-time facts about one roll-up property that
// decide whether a list condition's comparison of it lowers to a roll-up
// leaf (design gap 3.5, step 2). The caller that compiled the roll-up
// supplies them; the decision is [RollupLeafFacts.Refusal].
type RollupLeafFacts struct {
	// Exists reports an `exists` roll-up.
	Exists bool
	// Exact reports that every node condition, where and when_parent has
	// an exact store form.
	Exact bool
	// Nested reports a roll-up whose node conditions read roll-ups.
	Nested bool
	// Siblings reports a siblings_before roll-up.
	Siblings bool
	// OrderFieldGated reports that some role has a `visible:` grant on
	// the order field of the siblings relation, so a reader may see the
	// roll-up absent where the store would count.
	OrderFieldGated bool
}

// Refusal says why a comparison of the roll-up does not lower, or is empty
// when it does. min and max, defer-date and other inexact node conditions,
// nested roll-ups, and a siblings roll-up under a relation field grant keep
// the Go path.
func (f RollupLeafFacts) Refusal() string {
	switch {
	case !f.Exists:
		return "only an exists roll-up lowers"
	case f.Nested:
		return "a roll-up over a roll-up is judged in Go"
	case !f.Exact:
		return "a node condition of the roll-up has no exact store form"
	case f.Siblings && f.OrderFieldGated:
		return "a visible: grant on the relation's order field can hide the order"
	}
	return ""
}

// RollupFacts returns the facts of the roll-up property of entityType, or
// false when it knows none.
//
// Nil: accepted by [LowerConditionRollups]; no roll-up lowers.
type RollupFacts func(entityType, property string) (RollupLeafFacts, bool)

// LoweredRollup is the roll-up leaf of a [LoweredCond]: the condition holds
// where the reader's roll-up Property is Holds. The caller binds it to the
// reader's [store.RollupCond], as it gates a traversal.
type LoweredRollup struct {
	Property string
	Holds    bool
}

// LowerConditionRollups is [LowerConditionExplained] that also lowers
// `entity.p == true` and `entity.p == false` on a roll-up p whose facts
// admit it ([RollupLeafFacts.Refusal]) to a [store.CondRollup] node in
// Where. Any other comparison of a roll-up declines as there. The caller
// must still check that p is visible to the reader on every row.
func LowerConditionRollups(
	prog *predicate.Program, meta *metamodel.Metamodel, entityType, identity string, facts RollupFacts,
) (ConditionLowering, error) {
	return lowerCondition(prog, meta, entityType, identity, facts)
}

// lowerRollup lowers a comparison of a roll-up property.
func (l lowerer) lowerRollup(c predicate.Comparison) (LoweredCond, error) {
	refuse := func(reason string) (LoweredCond, error) {
		return LoweredCond{}, &UnsupportedConditionError{Construct: describeComparison(c), Reason: reason}
	}
	if l.rollups == nil || len(l.types) != 1 {
		return refuse("a rollup property is not stored, so the store cannot compare it")
	}
	if !c.Bool {
		return refuse("a roll-up lowers only as `== true` or `== false`")
	}
	facts, ok := l.rollups(l.types[0], c.Attribute)
	if !ok {
		return refuse("the roll-up is not compiled")
	}
	if reason := facts.Refusal(); reason != "" {
		return refuse(reason)
	}
	return LoweredCond{Op: store.CondRollup, Rollup: LoweredRollup{Property: c.Attribute, Holds: c.Value == "true"}}, nil
}
