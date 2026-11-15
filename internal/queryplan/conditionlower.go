package queryplan

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ConditionLowering is a query condition rewritten as store predicates that
// REPLACE its Go evaluation (TKT-I39Y7N4). Props are pushed as they are.
// Each traversal still has to pass the principal's traversal gate before it
// becomes a [store.DirectedRelation]; a negated one then sets
// [store.RelationPredicate.Negate].
type ConditionLowering struct {
	Props      []store.PropPredicate
	Traversals []predicate.TraversalTerm
}

// LowerCondition decides whether a query condition on entityType can be
// answered by the store alone, and if so returns the predicates that answer
// it. A query condition is one that narrows a collection read: a list's
// `condition:`, a nav status rule, a list query scope. A desktop
// notification rule has the same shape and can use it too (TKT-I3MGCRT).
//
// It is NOT [LowerScope], and must not replace it. LowerScope also decides
// whether a `visible:` grant compiles into a search field scope, so it keeps
// refusing every shape a field scope cannot express. This lowering admits
// more, because its store query answers the whole condition:
//
//   - `entity.p ~= v` lowers to [store.PropNotEqualOrEmpty], whose "not v,
//     or unset" reading is the language's `nil ~= v` (true)
//   - `not related(...)` lowers to a negated traversal; the caller sets
//     Negate on the gated predicate
//   - a membership (`has_current_user(entity.list)`) lowers to a non-scalar
//     [store.PropEqual], which every backend reads as "some element equals"
//   - an attribute constrained twice lowers to both predicates, since the
//     store ANDs them as the program does
//
// Every leaf of the AND spine must still produce exactly one predicate with
// the same meaning, or ok is false and the caller keeps the Go path:
//
//   - `or`, ordered comparisons and any `not` other than over a traversal
//     decline. An ordered comparison on an unset value is an evaluation
//     error in Go, which no store predicate reproduces
//   - an equality or inequality the metamodel gate refuses (an undeclared
//     property, an integer, boolean or list, or `id`) declines, as does a
//     membership over anything but a declared list of strings
//   - `current_user.tool` declines, and a `current_user` reference with no
//     identity declines, as in [LowerScope]
//
// The returned predicates match on the STORED row. A caller whose Go path
// judges a redacted row must push only when every attribute the condition
// reads is visible to the principal on every row.
//
// identity is the request's query identity, empty when there is none. It is
// an argument, never state: the resolvers that call this are shared across
// principals.
func LowerCondition(
	prog *predicate.Program, meta *metamodel.Metamodel, entityType, identity string,
) (ConditionLowering, bool) {
	if prog == nil || meta == nil || entityType == "" {
		return ConditionLowering{}, false
	}
	cmps, traversals, ok := prog.ConditionConjunction(predicatefns.CurrentUserPrefilterSpec())
	if !ok {
		return ConditionLowering{}, false
	}
	types := []string{entityType}
	props := make([]store.PropPredicate, 0, len(cmps))
	for _, c := range cmps {
		p, ok := lowerComparison(meta, types, c, identity)
		if !ok {
			return ConditionLowering{}, false
		}
		props = append(props, p)
	}
	bound := make([]predicate.TraversalTerm, 0, len(traversals))
	for _, term := range traversals {
		spec, err := predicatefns.BindTraversal(term.Spec, identity)
		if err != nil {
			return ConditionLowering{}, false
		}
		bound = append(bound, predicate.TraversalTerm{Spec: spec, Negated: term.Negated})
	}
	return ConditionLowering{Props: props, Traversals: bound}, true
}

// lowerComparison lowers one comparison leaf, or reports ok=false.
func lowerComparison(
	meta *metamodel.Metamodel, types []string, c predicate.Comparison, identity string,
) (store.PropPredicate, bool) {
	value := c.Value
	switch c.FromVar {
	case "":
	case predicatefns.FieldCurrentUserID:
		if identity == "" {
			return store.PropPredicate{}, false
		}
		value = identity
	default:
		return store.PropPredicate{}, false
	}
	switch {
	case c.List:
		if c.NotEqual || !stringListOnEveryType(meta, types, c.Attribute) {
			return store.PropPredicate{}, false
		}
		return store.PropPredicate{Property: c.Attribute, Op: store.PropEqual, Value: value}, true
	case !stringComparableOnEveryType(meta, types, c.Attribute):
		return store.PropPredicate{}, false
	case c.NotEqual:
		// Scalar: Go binds a non-string value under a string declaration
		// to nil, and `nil ~= v` is true, so only a stored string equal to
		// v is excluded.
		return store.PropPredicate{
			Property: c.Attribute, Op: store.PropNotEqualOrEmpty, Value: value, Scalar: true,
		}, true
	default:
		return store.PropPredicate{Property: c.Attribute, Op: store.PropEqual, Value: value, Scalar: true}, true
	}
}
