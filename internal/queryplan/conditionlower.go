package queryplan

import (
	"slices"

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

// Walk calls visit on c and on every node below it, depth first.
func (c LoweredCond) Walk(visit func(LoweredCond)) {
	visit(c)
	for _, a := range c.Args {
		a.Walk(visit)
	}
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
	if prog == nil || meta == nil || entityType == "" {
		return ConditionLowering{}, false
	}
	tree, ok := prog.ConditionTree(predicatefns.CurrentUserPrefilterSpec())
	if !ok {
		return ConditionLowering{}, false
	}
	l := lowerer{meta: meta, types: []string{entityType}, identity: identity}
	root, ok := l.lower(tree)
	if !ok || !withinCondLimits(root) {
		return ConditionLowering{}, false
	}
	return splitConjuncts(root), true
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

func (l lowerer) lower(t predicate.CondTree) (LoweredCond, bool) {
	switch t.Op {
	case predicate.CondTreeConst:
		if t.Const {
			return LoweredCond{Op: store.CondAnd}, true
		}
		return LoweredCond{Op: store.CondOr}, true
	case predicate.CondTreeTraversal:
		spec, err := predicatefns.BindTraversal(t.Traversal, l.identity)
		if err != nil {
			return LoweredCond{}, false
		}
		return LoweredCond{Op: store.CondRelated, Traversal: spec}, true
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
			la, ok := l.lower(a)
			if !ok {
				return LoweredCond{}, false
			}
			out.Args = append(out.Args, la)
		}
		return out, true
	}
	return LoweredCond{}, false
}

// lowerCompare lowers one comparison leaf.
func (l lowerer) lowerCompare(c predicate.Comparison) (LoweredCond, bool) {
	if rollupOnAnyType(l.meta, l.types, c.Attribute) {
		return LoweredCond{}, false
	}
	switch {
	case c.Int:
		if !integerOnEveryType(l.meta, l.types, c.Attribute) {
			return LoweredCond{}, false
		}
		at := func(op store.PropOp) LoweredCond {
			return LoweredCond{Op: store.CondProp, Prop: store.PropPredicate{
				Property: c.Attribute, Op: op, Value: c.Value, Kind: store.OrderInteger,
			}}
		}
		if c.Order != "" {
			return at(orderedOps[c.Order]), true
		}
		eq := LoweredCond{Op: store.CondAnd, Args: []LoweredCond{at(store.PropGreaterEqual), at(store.PropLessEqual)}}
		if c.NotEqual {
			return LoweredCond{Op: store.CondNot, Args: []LoweredCond{eq}}, true
		}
		return eq, true
	case c.Order != "":
		if !stringOrderableOnEveryType(l.meta, l.types, c.Attribute) {
			return LoweredCond{}, false
		}
		return LoweredCond{Op: store.CondProp, Prop: store.PropPredicate{
			Property: c.Attribute, Op: orderedOps[c.Order], Value: c.Value, Kind: store.OrderString,
		}}, true
	}
	p, ok := lowerComparison(l.meta, l.types, c, l.identity)
	if !ok {
		return LoweredCond{}, false
	}
	return LoweredCond{Op: store.CondProp, Prop: p}, true
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
