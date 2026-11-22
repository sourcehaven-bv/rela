package queryplan

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ConditionLowering is a query condition rewritten as store predicates that
// REPLACE its Go evaluation (TKT-I39Y7N4, TKT-8HY05WB). The condition is
// the AND of Props, Traversals and Where; a caller must apply all three, or
// it answers a wider query than the condition.
//
// Props and Traversals are the plain leaves of the top-level AND, kept apart
// so a store can use an index on them. Each traversal still has to pass the
// principal's traversal gate before it becomes a [store.DirectedRelation];
// a negated one then sets [store.RelationPredicate.Negate]. Where holds the
// other conjuncts (an `or`, a `not` over anything but a traversal), nil when
// there are none.
type ConditionLowering struct {
	Props      []store.PropPredicate
	Traversals []predicate.TraversalTerm
	Where      *LoweredCond
}

// LoweredCond is one node of [ConditionLowering.Where]: a [store.Cond] whose
// relation leaves are still traversals for the caller to gate. Op is a
// [store.CondOp]; Prop is meaningful for CondProp, Traversal (bound to the
// identity) for CondRelated, Args for the others. Literal true is an AND
// without arguments and literal false an OR without arguments.
type LoweredCond struct {
	Op        store.CondOp
	Prop      store.PropPredicate
	Traversal predicate.TraversalSpec
	Args      []LoweredCond
}

// LowerCondition decides whether a query condition on entityType can be
// answered by the store alone, and if so returns the predicates that answer
// it. A query condition is one that narrows a collection read: a list's
// `condition:`, a nav status rule, a list query scope, an MCP list
// condition. A desktop notification rule has the same shape and can use it
// too (TKT-I3MGCRT).
//
// The Go path it replaces must evaluate with
// [predicatefns.Evaluator.LoweringSemantics]: an ordered comparison on an
// unset or off-type value is then false, as in the store. Under the default
// semantics it is an error, which no store query reproduces.
//
// It is NOT [LowerScope], and must not replace it. LowerScope also decides
// whether a `visible:` grant compiles into a search field scope, so it keeps
// refusing every shape a field scope cannot express. This lowering admits
// more, because its store query answers the whole condition:
//
//   - `and`, `or` and `not` at any depth ([store.Cond]), with literal
//     `true` and `false`
//   - `entity.p ~= v` lowers to [store.PropNotEqualOrEmpty], whose "not v,
//     or unset" reading is the language's `nil ~= v` (true)
//   - an ordered comparison of a string property (string, enum or an
//     enum-like custom type; not a date) with a string literal lowers with
//     [store.OrderString]; of an integer property with an int literal, with
//     [store.OrderInteger]
//   - `==` and `~=` of an integer property with an int literal lower to the
//     range `>= v and <= v` and its negation, which is false and true on an
//     unset value as the language's equality is
//   - a membership (`has_current_user(entity.list)`) lowers to a non-scalar
//     [store.PropEqual], which every backend reads as "some element equals"
//   - `related(...)` on the entity lowers to a traversal leaf
//
// Every leaf must produce a predicate with the same meaning, or ok is false
// and the caller keeps the Go path:
//
//   - an equality the metamodel gate refuses (an undeclared property, a
//     boolean, decimal, date or list, or `id`) declines, as does a
//     membership over anything but a declared list of strings
//   - an ordered comparison on a decimal, date, datetime, boolean or list
//     declines (dates wait for canonical storage, TKT-4Z2BIRG)
//   - a rollup property declines: the stored row holds no rollup value
//   - `current_user.tool` declines, and a `current_user` reference with no
//     identity declines, as in [LowerScope]
//   - a tree over [store.MaxCondDepth], [store.MaxCondLeaves] or
//     [store.MaxCondRelations] declines
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
	out, err := LowerConditionExplained(prog, meta, entityType, identity)
	return out, err == nil
}

// LowerConditionExplained is [LowerCondition] for a caller that reports
// why a condition does not lower: the error is an
// [*UnsupportedConditionError] naming the refused part. A caller with no
// Go path to fall back on (MCP list_entities) returns it to its user.
func LowerConditionExplained(
	prog *predicate.Program, meta *metamodel.Metamodel, entityType, identity string,
) (ConditionLowering, error) {
	if prog == nil || meta == nil || entityType == "" {
		return ConditionLowering{}, &UnsupportedConditionError{Construct: "condition", Reason: "nothing to lower"}
	}
	tree, unsupported := prog.ConditionTreeExplained(predicatefns.CurrentUserPrefilterSpec())
	if unsupported != "" {
		return ConditionLowering{}, &UnsupportedConditionError{
			Construct: unsupported,
			Reason: "the store cannot answer this; it answers a comparison of a property with a " +
				"string or integer literal, related(...), and and/or/not over those",
		}
	}
	l := lowerer{meta: meta, types: []string{entityType}, identity: identity}
	root, err := l.lower(tree)
	if err != nil {
		return ConditionLowering{}, err
	}
	if !withinCondLimits(root) {
		return ConditionLowering{}, &UnsupportedConditionError{
			Construct: "condition",
			Reason: fmt.Sprintf("too large: at most %d levels, %d comparisons and %d related(...) terms",
				store.MaxCondDepth, store.MaxCondLeaves, store.MaxCondRelations),
		}
	}
	return splitConjuncts(root), nil
}

// UnsupportedConditionError reports the part of a query condition the store
// cannot answer. Construct is the source text of that part (an expression,
// or a comparison as the lowering read it); Reason says why. Both name
// schema and filter syntax only, never a stored value.
type UnsupportedConditionError struct {
	Construct string
	Reason    string
}

func (e *UnsupportedConditionError) Error() string {
	return fmt.Sprintf("%s: %s", e.Construct, e.Reason)
}

// splitConjuncts moves the plain leaves of root's top-level AND into Props
// and Traversals and leaves the rest in Where.
func splitConjuncts(root LoweredCond) ConditionLowering {
	conjuncts := []LoweredCond{root}
	if root.Op == store.CondAnd {
		conjuncts = root.Args
	}
	var out ConditionLowering
	var rest []LoweredCond
	for len(conjuncts) > 0 {
		c := conjuncts[0]
		conjuncts = conjuncts[1:]
		switch {
		case c.Op == store.CondAnd && len(c.Args) > 0:
			conjuncts = append(slices.Clone(c.Args), conjuncts...)
		case c.Op == store.CondProp:
			out.Props = append(out.Props, c.Prop)
		case c.Op == store.CondRelated:
			out.Traversals = append(out.Traversals, predicate.TraversalTerm{Spec: c.Traversal})
		case c.Op == store.CondNot && c.Args[0].Op == store.CondRelated:
			out.Traversals = append(out.Traversals, predicate.TraversalTerm{Spec: c.Args[0].Traversal, Negated: true})
		case c.Op == store.CondAnd:
			// Literal true: no constraint.
		default:
			rest = append(rest, c)
		}
	}
	switch len(rest) {
	case 0:
	case 1:
		out.Where = &rest[0]
	default:
		out.Where = &LoweredCond{Op: store.CondAnd, Args: rest}
	}
	return out
}

// withinCondLimits reports whether root fits the store's [store.Cond]
// limits, so a caller never builds a query every backend refuses.
func withinCondLimits(root LoweredCond) bool {
	leaves, rels := 0, 0
	var depth func(LoweredCond) int
	depth = func(c LoweredCond) int {
		switch c.Op {
		case store.CondProp:
			leaves++
		case store.CondRelated:
			leaves++
			rels++
		default:
		}
		d := 0
		for _, a := range c.Args {
			d = max(d, depth(a))
		}
		return d + 1
	}
	return depth(root) <= store.MaxCondDepth && leaves <= store.MaxCondLeaves && rels <= store.MaxCondRelations
}

// lowerer lowers a [predicate.CondTree] against one metamodel and identity.
type lowerer struct {
	meta     *metamodel.Metamodel
	types    []string
	identity string
}

func (l lowerer) lower(t predicate.CondTree) (LoweredCond, error) {
	switch t.Op {
	case predicate.CondTreeConst:
		if t.Const {
			return LoweredCond{Op: store.CondAnd}, nil
		}
		return LoweredCond{Op: store.CondOr}, nil
	case predicate.CondTreeTraversal:
		spec, err := predicatefns.BindTraversal(t.Traversal, l.identity)
		if err != nil {
			return LoweredCond{}, &UnsupportedConditionError{
				Construct: "related(entity, " + strings.Join(t.Traversal.Path, ", ") + ", ...)",
				Reason:    err.Error(),
			}
		}
		return LoweredCond{Op: store.CondRelated, Traversal: spec}, nil
	case predicate.CondTreeCompare:
		return l.lowerCompare(t.Cmp)
	case predicate.CondTreeAnd, predicate.CondTreeOr, predicate.CondTreeNot:
		op := map[predicate.CondTreeOp]store.CondOp{
			predicate.CondTreeAnd: store.CondAnd,
			predicate.CondTreeOr:  store.CondOr,
			predicate.CondTreeNot: store.CondNot,
		}[t.Op]
		out := LoweredCond{Op: op, Args: make([]LoweredCond, 0, len(t.Args))}
		for _, a := range t.Args {
			la, err := l.lower(a)
			if err != nil {
				return LoweredCond{}, err
			}
			out.Args = append(out.Args, la)
		}
		return out, nil
	}
	return LoweredCond{}, &UnsupportedConditionError{Construct: "condition", Reason: "unknown node"}
}

// lowerCompare lowers one comparison leaf.
func (l lowerer) lowerCompare(c predicate.Comparison) (LoweredCond, error) {
	refuse := func(reason string) (LoweredCond, error) {
		return LoweredCond{}, &UnsupportedConditionError{Construct: describeComparison(c), Reason: reason}
	}
	if rollupOnAnyType(l.meta, l.types, c.Attribute) {
		return refuse("a rollup property is not stored, so the store cannot compare it")
	}
	switch {
	case c.Int:
		if !integerOnEveryType(l.meta, l.types, c.Attribute) {
			return refuse("not a declared integer property")
		}
		at := func(op store.PropOp) LoweredCond {
			return LoweredCond{Op: store.CondProp, Prop: store.PropPredicate{
				Property: c.Attribute, Op: op, Value: c.Value, Kind: store.OrderInteger,
			}}
		}
		if c.Order != "" {
			return at(orderedOps[c.Order]), nil
		}
		eq := LoweredCond{Op: store.CondAnd, Args: []LoweredCond{at(store.PropGreaterEqual), at(store.PropLessEqual)}}
		if c.NotEqual {
			return LoweredCond{Op: store.CondNot, Args: []LoweredCond{eq}}, nil
		}
		return eq, nil
	case c.Order != "":
		if !stringOrderableOnEveryType(l.meta, l.types, c.Attribute) {
			return refuse("an ordered comparison needs a string, enum or integer property " +
				"(dates, decimals and booleans are not supported)")
		}
		return LoweredCond{Op: store.CondProp, Prop: store.PropPredicate{
			Property: c.Attribute, Op: orderedOps[c.Order], Value: c.Value, Kind: store.OrderString,
		}}, nil
	}
	p, reason := lowerComparison(l.meta, l.types, c, l.identity)
	if reason != "" {
		return refuse(reason)
	}
	return LoweredCond{Op: store.CondProp, Prop: p}, nil
}

// describeComparison writes c back as the condition text it was read from,
// with the attribute on the left.
func describeComparison(c predicate.Comparison) string {
	attr := predicatefns.CurrentUserPrefilterSpec().RecordVar + "." + c.Attribute
	rhs := strconv.Quote(c.Value)
	switch {
	case c.FromVar != "":
		rhs = predicatefns.CurrentUserPrefilterSpec().ConstVar + "." + c.FromVar
	case c.Int:
		rhs = c.Value
	}
	switch {
	case c.List:
		return "membership of " + rhs + " in " + attr
	case c.Order != "":
		return attr + " " + c.Order + " " + rhs
	case c.NotEqual:
		return attr + " ~= " + rhs
	default:
		return attr + " == " + rhs
	}
}

// orderedOps maps a comparison's ordered operator to its store operator.
var orderedOps = map[string]store.PropOp{
	"<": store.PropLess, "<=": store.PropLessEqual, ">": store.PropGreater, ">=": store.PropGreaterEqual,
}

// integerOnEveryType reports whether prop is a scalar integer on every type.
func integerOnEveryType(meta *metamodel.Metamodel, types []string, prop string) bool {
	return everyTypeProp(meta, types, prop, func(pd metamodel.PropertyDef) bool {
		return !pd.List && pd.Type == metamodel.PropertyTypeInteger
	})
}

// stringOrderableOnEveryType reports whether prop is a scalar string whose
// predicate type is a string on every type: [StringShaped] without dates,
// which the predicate engine compares as dates.
func stringOrderableOnEveryType(meta *metamodel.Metamodel, types []string, prop string) bool {
	return everyTypeProp(meta, types, prop, func(pd metamodel.PropertyDef) bool {
		return StringShaped(meta, pd) &&
			pd.Type != metamodel.PropertyTypeDate && pd.Type != metamodel.PropertyTypeDatetime
	})
}

// rollupOnAnyType reports whether prop is a rollup on any of types.
func rollupOnAnyType(meta *metamodel.Metamodel, types []string, prop string) bool {
	for _, typ := range types {
		if def, ok := meta.GetEntityDef(typ); ok {
			if pd, ok := def.Properties[prop]; ok && pd.Rollup != nil {
				return true
			}
		}
	}
	return false
}

func everyTypeProp(meta *metamodel.Metamodel, types []string, prop string, ok func(metamodel.PropertyDef) bool) bool {
	if len(types) == 0 {
		return false
	}
	for _, typ := range types {
		def, found := meta.GetEntityDef(typ)
		if !found {
			return false
		}
		pd, found := def.Properties[prop]
		if !found || !ok(pd) {
			return false
		}
	}
	return true
}

// lowerComparison lowers one comparison leaf. reason is empty when it
// lowers, and otherwise says why not.
func lowerComparison(
	meta *metamodel.Metamodel, types []string, c predicate.Comparison, identity string,
) (pred store.PropPredicate, reason string) {
	value := c.Value
	switch c.FromVar {
	case "":
	case predicatefns.FieldCurrentUserID:
		if identity == "" {
			return store.PropPredicate{}, "no caller identity to compare with"
		}
		value = identity
	default:
		return store.PropPredicate{}, "only current_user.id can be compared in the store"
	}
	switch {
	case c.List:
		if c.NotEqual || !stringListOnEveryType(meta, types, c.Attribute) {
			return store.PropPredicate{}, "membership needs a declared list of strings"
		}
		return store.PropPredicate{Property: c.Attribute, Op: store.PropEqual, Value: value}, ""
	case !stringComparableOnEveryType(meta, types, c.Attribute):
		return store.PropPredicate{}, "equality needs a declared string, enum or integer property " +
			"(not a boolean, decimal, date, list or id)"
	case c.NotEqual:
		// Scalar: Go binds a non-string value under a string declaration
		// to nil, and `nil ~= v` is true, so only a stored string equal to
		// v is excluded.
		return store.PropPredicate{
			Property: c.Attribute, Op: store.PropNotEqualOrEmpty, Value: value, Scalar: true,
		}, ""
	default:
		return store.PropPredicate{Property: c.Attribute, Op: store.PropEqual, Value: value, Scalar: true}, ""
	}
}
