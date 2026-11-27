package rollup

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
)

// MaxAlongDepth is the most hops an along roll-up's max_depth may name. It
// is the store's cap on one walk ([store.MaxAlongDepth]); it is not the
// depth cap of a graph query.
const MaxAlongDepth = store.MaxAlongDepth

// Along is the compiled walk of an inherited roll-up (design gap 1).
type Along struct {
	Segments []Segment
	MaxDepth int
	// Nodes are the visited types that can count, by type: the keys of the
	// declared condition.
	Nodes map[string]*Node
}

// Lowers reports whether every node condition has a store form, so the
// store can judge the whole walk. A column that does not lower is folded in
// Go over the visits the store returns.
func (a *Along) Lowers() bool {
	for _, n := range a.Nodes {
		if n.Lowered == nil {
			return false
		}
	}
	return true
}

// Segment is one segment of an along walk: the canonical relation, the
// direction walked, whether it repeats, and the entity types it reaches.
type Segment struct {
	Relation   string
	SourceIsTo bool
	Repeat     bool
	Reaches    []string
}

// Node is the compiled condition of one visited type. It reads only the
// visited entity's stored properties, with their declared defaults filled
// in where the row holds none, and now() and today().
type Node struct {
	Type   string
	Source string
	// Reads are the properties the condition reads, sorted.
	Reads []string
	// Lowered is the condition as a store condition over the stored row,
	// before defaults are filled in ([Node.FillDefaults]). Nil when it has
	// no exact store form.
	Lowered *store.Cond

	prog     *predicate.Program
	eval     *predicatefns.Evaluator
	defaults map[string]any
}

// Holds evaluates the condition on one visited row. props is the row as
// the reader sees it; a property in hidden reads as unset, and every other
// property the condition reads that the row leaves empty holds its declared
// default.
func (n *Node) Holds(ctx context.Context, id string, props map[string]any, hidden []string) (bool, error) {
	filled := make(map[string]any, len(props)+len(n.defaults))
	maps.Copy(filled, props)
	for p, d := range n.defaults {
		if !slices.Contains(hidden, p) && emptyValue(filled[p]) {
			filled[p] = d
		}
	}
	return n.eval.Matches(ctx, n.prog, n.Type, id, filled)
}

// FillDefaults rewrites c, a store condition over this node's type, so that
// each comparison on a property with a declared default answers as it does
// on the default where the row holds no value. A comparison whose answer on
// an unset row already equals its answer on the default is kept as it is.
func (n *Node) FillDefaults(c store.Cond) (store.Cond, error) {
	if c.Op != store.CondProp {
		if len(c.Args) == 0 {
			return c, nil
		}
		out := store.Cond{Op: c.Op, Args: make([]store.Cond, len(c.Args))}
		for i, a := range c.Args {
			fa, err := n.FillDefaults(a)
			if err != nil {
				return store.Cond{}, err
			}
			out.Args[i] = fa
		}
		return out, nil
	}
	p := c.Prop.Property
	d, ok := n.defaults[p]
	if !ok {
		return c, nil
	}
	atDefault, err := graphquerynaive.MatchProps(map[string]any{p: d}, []store.PropPredicate{c.Prop})
	if err != nil {
		return store.Cond{}, err
	}
	atUnset, err := graphquerynaive.MatchProps(map[string]any{}, []store.PropPredicate{c.Prop})
	if err != nil {
		return store.Cond{}, err
	}
	empty := store.PropLeaf(store.PropPredicate{Property: p, Op: store.PropEqual, Value: ""})
	switch {
	case atDefault == atUnset:
		return c, nil
	case atDefault:
		return store.OrCond(c, empty), nil
	default:
		return store.AndCond(c, store.NotCond(empty)), nil
	}
}

// emptyValue reports whether v is a value a default fills: unset or "".
func emptyValue(v any) bool {
	s, isString := v.(string)
	return v == nil || (isString && s == "")
}

// compileAlongColumn checks an along roll-up (design section 3.2) and
// compiles its walk and node conditions, returning every problem found.
func compileAlongColumn(
	meta *metamodel.Metamodel, eval *predicatefns.Evaluator, typeName, name string, pd metamodel.PropertyDef,
	fn metamodel.RollupFunc, prop, at string,
) (Column, []string) {
	r := pd.Rollup
	var problems []string
	if len(r.Via) > 0 {
		problems = append(problems, at+": along cannot be combined with via")
	}
	switch fn {
	case metamodel.RollupExists, metamodel.RollupMin, metamodel.RollupMax:
	case metamodel.RollupCount, metamodel.RollupSum, metamodel.RollupLatest, metamodel.RollupAvg,
		metamodel.RollupLatestRow, metamodel.RollupBodyTasks:
		problems = append(problems, fmt.Sprintf("%s: along folds exists, min or max, not %s", at, fn))
	}
	switch {
	case r.MaxDepth == 0:
		problems = append(problems, fmt.Sprintf("%s: along needs max_depth, from 1 to %d", at, MaxAlongDepth))
	case r.MaxDepth < 1 || r.MaxDepth > MaxAlongDepth:
		problems = append(problems, fmt.Sprintf("%s: max_depth %d is outside 1 to %d", at, r.MaxDepth, MaxAlongDepth))
	}
	segs, reach, segProblems := compileSegments(meta, typeName, r.Along, at)
	problems = append(problems, segProblems...)
	along := &Along{Segments: segs, MaxDepth: r.MaxDepth, Nodes: map[string]*Node{}}
	if len(r.Condition) == 0 {
		problems = append(problems, at+": along needs a condition for at least one visited type")
	}
	for _, typ := range slices.Sorted(maps.Keys(r.Condition)) {
		if segProblems == nil && !reach[typ] {
			problems = append(problems, fmt.Sprintf("%s: condition type %q is not a type the walk reaches", at, typ))
			continue
		}
		if _, ok := meta.Entities[typ]; !ok {
			continue
		}
		node, problem := compileNode(meta, eval, typ, strings.TrimSpace(r.Condition[typ]))
		if problem != "" {
			problems = append(problems, fmt.Sprintf("%s: condition for %q: %s", at, typ, problem))
			continue
		}
		along.Nodes[typ] = node
	}
	col := Column{Property: name, Func: fn, Along: along}
	if fn == metamodel.RollupMin || fn == metamodel.RollupMax {
		prop = strings.TrimSpace(prop)
		col.Value, col.Numeric = prop, pd.Type == metamodel.PropertyTypeInteger
		problems = append(problems, alongValueProblems(meta, r, pd, fn, prop, at)...)
	}
	return col, problems
}

// compileSegments resolves the along segments from the source type, and
// returns the types the walk can reach. Problems is nil when every segment
// resolved.
func compileSegments(
	meta *metamodel.Metamodel, typeName string, along []string, at string,
) (segs []Segment, reach map[string]bool, problems []string) {
	reach = map[string]bool{}
	repeats := make([]store.AlongSegment, len(along))
	for i, raw := range along {
		_, repeats[i].Repeat = strings.CutSuffix(strings.TrimSpace(raw), metamodel.AlongRepeat)
	}
	reached := map[int][]string{-1: {typeName}}
	for j, raw := range along {
		name, repeat := strings.CutSuffix(strings.TrimSpace(raw), metamodel.AlongRepeat)
		name = strings.TrimSpace(name)
		var before []string
		for s := -1; s <= j; s++ {
			if slices.Contains(store.AlongNext(repeats, s), j) {
				before = append(before, reached[s]...)
			}
		}
		seg, problem := compileSegment(meta, name, repeat, before)
		if problem != "" {
			problems = append(problems, fmt.Sprintf("%s: along segment %q: %s", at, raw, problem))
			continue
		}
		reached[j] = seg.Reaches
		for _, t := range seg.Reaches {
			reach[t] = true
		}
		segs = append(segs, seg)
	}
	return segs, reach, problems
}

// compileSegment resolves one segment name, standing on any of the types in
// before, or why it cannot be walked.
func compileSegment(meta *metamodel.Metamodel, name string, repeat bool, before []string) (Segment, string) {
	relType, incoming := name, false
	def, ok := meta.GetRelationDef(name)
	if !ok {
		owner, isInverse := meta.InverseOwner(name)
		if !isInverse {
			return Segment{}, fmt.Sprintf("unknown relation %q", name)
		}
		relType, incoming = owner, true
		if def, ok = meta.GetRelationDef(owner); !ok {
			return Segment{}, fmt.Sprintf("inverse %q names unknown relation %q", name, owner)
		}
	}
	near, far := def.From, def.To
	if incoming {
		near, far = def.To, def.From
	}
	switch {
	case def.Symmetric:
		return Segment{}, fmt.Sprintf("relation %q is symmetric; one direction would miss some edges", relType)
	case metamodel.IsContentScoped(meta, relType):
		return Segment{}, fmt.Sprintf("relation %q is content-scoped; a roll-up follows identity-scoped relations only",
			relType)
	case len(near) > 0 && !slices.ContainsFunc(before, func(t string) bool { return slices.Contains(near, t) }):
		return Segment{}, fmt.Sprintf("its near side (%s) accepts none of %s", strings.Join(near, ", "),
			strings.Join(slices.Compact(slices.Sorted(slices.Values(before))), ", "))
	case len(far) == 0:
		return Segment{}, "it declares no type on the side it walks to"
	}
	for _, t := range far {
		if _, ok := meta.Entities[t]; !ok {
			return Segment{}, fmt.Sprintf("it reaches %q, which is not an entity type", t)
		}
	}
	if repeat {
		toOne := def.MaxOutgoing
		if incoming {
			toOne = def.MaxIncoming
		}
		switch {
		case len(near) > 0 && !slices.ContainsFunc(far, func(t string) bool { return slices.Contains(near, t) }):
			return Segment{}, "a repeated segment must lead from a type back to the same type"
		case toOne == nil || *toOne != 1:
			return Segment{}, "a repeated segment must walk a side the schema declares to-one " +
				"(max_outgoing: 1, or max_incoming: 1 for an inverse)"
		}
	}
	return Segment{Relation: relType, SourceIsTo: incoming, Repeat: repeat, Reaches: slices.Sorted(slices.Values(far))}, ""
}

// compileNode compiles one visited type's condition, or says why it is
// refused: it must compile and read only stored properties, now() and
// today().
func compileNode(meta *metamodel.Metamodel, eval *predicatefns.Evaluator, typ, src string) (*Node, string) {
	def := meta.Entities[typ]
	wide, err := eval.CompileWithRollups(typ, src)
	if err != nil {
		return nil, fmt.Sprintf("does not compile: %v", err)
	}
	for _, a := range wide.Attributes("entity") {
		switch pd := def.Properties[a]; {
		case pd.Rollup != nil:
			return nil, fmt.Sprintf("reads the roll-up %q; a node condition reads stored properties only", a)
		case pd.Computed != "":
			return nil, fmt.Sprintf("reads the computed property %q; a node condition reads stored properties only", a)
		}
	}
	switch {
	case wide.References("current_user"):
		return nil, "reads current_user; a node condition is the same for every reader"
	case len(wide.Traversals()) > 0:
		return nil, "calls related(); a node condition reads the visited entity only"
	}
	prog, err := eval.Compile(typ, src)
	if err != nil {
		return nil, fmt.Sprintf("does not compile: %v", err)
	}
	n := &Node{Type: typ, Source: src, Reads: slices.Sorted(slices.Values(prog.Attributes("entity"))),
		prog: prog, eval: eval, defaults: map[string]any{}}
	n.Reads = slices.Compact(n.Reads)
	for _, p := range n.Reads {
		if d, ok := typedDefault(meta, def.Properties[p]); ok {
			n.defaults[p] = d
		}
	}
	n.Lowered = lowerNode(meta, n)
	return n, ""
}

// lowerNode is the node's exact store condition over the stored row, or
// nil when it has none.
func lowerNode(meta *metamodel.Metamodel, n *Node) *store.Cond {
	low, ok := queryplan.LowerCondition(n.prog, meta, n.Type, "")
	if !ok || len(low.Traversals) > 0 {
		return nil
	}
	args := make([]store.Cond, 0, len(low.Props)+1)
	for _, p := range low.Props {
		args = append(args, store.PropLeaf(p))
	}
	if low.Where != nil {
		w, ok := low.Where.PropCond()
		if !ok {
			return nil
		}
		args = append(args, w)
	}
	c := store.AndCond(args...)
	if len(args) == 1 {
		c = args[0]
	}
	if _, err := n.FillDefaults(c); err != nil {
		return nil
	}
	return &c
}

// typedDefault is the declared default of pd as a stored value: an int64
// for an integer, a bool for a boolean, else the text.
func typedDefault(meta *metamodel.Metamodel, pd metamodel.PropertyDef) (any, bool) {
	d := strings.TrimSpace(metamodel.DeclaredDefault(pd, meta))
	if d == "" {
		return nil, false
	}
	switch pd.Type {
	case metamodel.PropertyTypeInteger:
		n, err := strconv.ParseInt(d, 10, 64)
		return n, err == nil
	case metamodel.PropertyTypeBoolean:
		b, err := strconv.ParseBool(d)
		return b, err == nil
	}
	return d, true
}

// alongValueProblems checks the min or max property on every visited type
// with a condition: declared, integer or a default-format date, of the
// roll-up's declared type, and of one type across the visited types.
func alongValueProblems(
	meta *metamodel.Metamodel, r *metamodel.RollupDef, pd metamodel.PropertyDef, fn metamodel.RollupFunc,
	prop, at string,
) []string {
	var problems []string
	seen := map[string]bool{}
	for _, typ := range slices.Sorted(maps.Keys(r.Condition)) {
		def, ok := meta.Entities[typ]
		if !ok {
			continue
		}
		rp, ok := def.Properties[prop]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s: %s %q is not a property of %q", at, fn, prop, typ))
			continue
		case rp.Rollup != nil || rp.Computed != "" || rp.List:
			problems = append(problems, fmt.Sprintf("%s: %s %q on %q is not a stored scalar", at, fn, prop, typ))
			continue
		case rp.Type != metamodel.PropertyTypeInteger && !isoDate(rp):
			problems = append(problems, fmt.Sprintf(
				"%s: %s %q on %q has type %q; min and max need integer or date in the default format",
				at, fn, prop, typ, rp.Type))
		case rp.Type != pd.Type:
			problems = append(problems, fmt.Sprintf("%s: %s %q on %q has type %q, but the roll-up is declared %q",
				at, fn, prop, typ, rp.Type, pd.Type))
		}
		if len(seen) > 0 && !seen[rp.Type] {
			problems = append(problems, fmt.Sprintf("%s: %s %q has type %q on %q, which differs across visited types",
				at, fn, prop, rp.Type, typ))
		}
		seen[rp.Type] = true
	}
	return problems
}
