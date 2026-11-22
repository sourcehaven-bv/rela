package store

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"strconv"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// GraphQuery describes a graph-shape question: "entities of EntityType
// whose own properties match, and which have (or lack) a matching
// inbound or outbound relation." The DSL is intentionally generic — no
// ACL or other consumer vocabulary — so consumers (ACL read filtering,
// analyze tools, search, next-action sources) can compose against one
// stable shape.
//
// All predicates are ANDed: an entity matches when every PropPredicate
// holds AND every relation predicate (HasInbound, HasOutbound and each
// Related entry) holds. A zero-value GraphQuery
// beyond EntityType matches every entity of that type.
//
// graphquerynaive (iterate-and-filter in Go) is the reference
// implementation; pgstore renders the same query as SQL.
//
// # Under a world: which predicates run before the rank
//
// FaceIn and each Any branch's face set trim the CANDIDATE faces; the world
// then picks one prime per entity from what is left. Props and Narrowing
// test that prime (BUG-2SKLD3): filtering the faces on them first would let
// a lower-ranked face answer for an entity whose prime fails the filter,
// serving a list row a GET of the same entity would not. The relation
// predicates depend on the entity id alone, so the order does not matter
// for them.
type GraphQuery struct {
	EntityType  string
	Props       []PropPredicate    // entity's own properties match (AND)
	HasInbound  *RelationPredicate // entity has matching relation FROM (expanded) endpoints
	HasOutbound *RelationPredicate // entity has matching relation TO (expanded) endpoints

	// Faces is the RESULT selection, exactly as [EntityQuery.Faces]: required,
	// and the zero value is [ErrInvalidQuery] at execution. ACL-built
	// GraphQuery values are templates that internal/visibility/pushdown.go
	// completes from the request's EntityQuery, which is why the zero value
	// is checked when the query runs rather than when it is built.
	//
	// It must live here as well as on EntityQuery, not only there: the
	// ACL read path swaps an EntityQuery for a GraphQuery the moment a
	// policy query exists (internal/visibility/pushdown.go), and the
	// AllowAll principal takes the EntityQuery branch. A selection carried
	// on only one of the two would make the list path and the single-entity
	// path disagree — and would do so precisely for the privileged principal.
	//
	// Under InWorld one row per id is returned; under AllFaces and AtFaces
	// one row per selected face, and GraphCount counts those rows.
	//
	// Scoping applies to the entities the query RETURNS. Relation
	// predicates walk the graph's identity structure and are NOT
	// face-resolved: who an entity is related to must not depend on the
	// reader's world. The one exception is an endpoint's own properties
	// under [RelationPredicate.EndpointMatch]; see [EndpointPredicate.Faces].
	Faces FaceSelection

	// FaceIn is [EntityQuery.FaceIn], carried here for the same reason Faces
	// is: the ACL read path swaps an EntityQuery for a GraphQuery the moment
	// a policy query exists, and the AllowAll principal takes the EntityQuery
	// branch. A face set on only one of the two would make the list path and
	// the single-entity path disagree — and would do so precisely for the
	// privileged principal.
	FaceIn []entity.Face

	// Any is a disjunction of per-branch predicates, ANDed with everything
	// above: an entity matches when at least one branch holds. Nil means no
	// branch constraint.
	//
	// It exists for relation-conferred roles whose face grants differ. The
	// ACL compiles `owns → author {read: [policy@draft]}` and `reviews →
	// reviewer {read: [policy@published]}` into two branches, so a principal
	// who merely REVIEWS an entity is held to the reviewer's faces rather
	// than to the union of every conferring role's — the union was a laundering
	// of one relation's faces through another. Like FaceIn, a branch's face
	// set is applied to the CANDIDATE rows before world ranking, so the
	// answer is exact at the store and paging, counts and search stay honest
	// with no post-filter.
	// Any is AUTHORIZATION-derived and belongs to the ACL layer alone. It
	// is a CEILING: it says which rows the principal may see at all. See
	// [Narrowing] for the caller-supplied counterpart, and do not append a
	// caller's branches here — the two are different types precisely so
	// that mistake cannot compile.
	Any []GraphBranch

	// SkipUnreadable leaves out a candidate row whose stored values cannot
	// be read ([ErrConflictedRow]) instead of failing the query. Like Any it
	// belongs to the ACL layer alone: the ACL sets it on its read-grant
	// query, where the answer is an allowlist, so a left-out row is hidden
	// (BUG-HTSW37Q). Nobody else may set it. Where absence grants something,
	// such as a negated traversal or a uniqueness scan, a left-out row would
	// widen the answer, so those queries keep failing closed. It never
	// applies to a relation endpoint, only to the rows the query returns.
	// Only the file store has such rows; the other backends ignore it.
	SkipUnreadable bool

	// Narrowing is a disjunction supplied by a CALLER (a view's
	// `condition:`, a report's filter) rather than by the ACL. It is ANDed
	// with everything above, including [GraphQuery.Any].
	//
	// # Why this is a separate field and a separate type
	//
	// Both this and Any are "a list of OR-ed branches", so the obvious
	// implementation is to reuse Any. That is a privilege-escalation bug.
	// Any is OR-ed internally, so appending caller branches to it yields
	//
	//	acl_a OR acl_b OR caller_x
	//
	// when the required meaning is
	//
	//	(acl_a OR acl_b) AND caller_x
	//
	// In the first form a caller's branch is an ALTERNATIVE ROUTE TO
	// AUTHORIZATION: a principal who matches no ACL branch is admitted by
	// matching a display filter. Every test that only checks "the right
	// rows are shown" still passes, because the escalation is visible only
	// to a principal the test does not use.
	//
	// The ceiling may only ever NARROW — the same rule the ACL policy layer
	// states for grants, so a bug fails toward less access. Keeping the two
	// as distinct types ([GraphBranch] vs [NarrowBranch]) makes the wrong
	// append a compile error rather than a review catch.
	Narrowing []NarrowBranch

	// Related is a CONJUNCTION of caller-supplied relation predicates, each
	// ANDed with everything else. A data-entry query scope lowers each
	// `related(...)` term into one entry (TKT-XKCNCL).
	//
	// It exists because HasInbound and HasOutbound are single slots, and the
	// ACL read path owns HasInbound whenever a principal reads the type
	// through a role relation. A caller's traversal written into that slot
	// would either be refused or overwrite the row gate; here it can only
	// narrow. Like [GraphQuery.Narrowing], the ACL never writes this field.
	Related []DirectedRelation

	// Where is a caller-supplied boolean condition over the entity's own
	// properties and its relations, ANDed with everything else (TKT-8HY05WB).
	// A lowered query condition with `or`, `not` or an ordered comparison
	// rides here; a plain conjunction may still use Props and Related. Nil
	// constrains nothing.
	//
	// Like [GraphQuery.Narrowing] it only narrows, and the ACL never writes
	// it. It is tested on the row a world picks (the prime), as Props is;
	// its relation leaves depend on the id alone, so that placement is exact
	// for them too. See [Cond] for its semantics.
	Where *Cond

	// OrderBy, Limit and Offset page a ROW query (GraphQuery,
	// GraphQueryHeaders) inside the backend (TKT-1U8XYN), so a list page
	// costs one bounded read instead of a whole-type scan sorted and sliced
	// in Go. GraphCount and MatchingFaces IGNORE all three — a count answers
	// for the matched set, and a page must never change it.
	//
	// OrderBy sorts by the STRING form of each property, byte-wise (the
	// data-entry list comparator's semantics), then by id ascending as the
	// tiebreak. A row WITHOUT the property sorts as if it held the largest
	// value: last ascending, first descending — SQL's default null
	// placement, kept on purpose so one index serves both directions. It is
	// meant for scalar string-shaped properties (string, enum, date); a
	// caller sorting anything else stays on its own comparator. Limit 0
	// means unbounded; Offset counts rows in the ordered result.
	OrderBy []OrderSpec
	Limit   int
	Offset  int
}

// DirectedRelation is one entry of [GraphQuery.Related]: a relation
// predicate and the side of the edge the entity stands on.
//
// Incoming is a bool rather than a [Direction] on purpose: Direction's zero
// value is DirectionBoth, which backends read differently, and an entry here
// must mean exactly what HasInbound or HasOutbound means.
type DirectedRelation struct {
	// Incoming reads the entity's incoming edges, as HasInbound does. False
	// reads its outgoing edges, as HasOutbound does.
	Incoming bool
	Pred     RelationPredicate
}

// OrderSpec is one GraphQuery sort key.
type OrderSpec struct {
	Property   string
	Descending bool

	// Values is the declared order of an enum-shaped property, most
	// significant first. Empty — the usual case — means compare the text
	// form byte-wise, which is [GraphQuery.OrderBy]'s default contract.
	//
	// When set, a backend ranks each value by its POSITION in this slice,
	// and any value not listed sorts after every listed one, byte-wise
	// among its peers. That is what lets a workflow enum read in workflow
	// order (`backlog, ready, done`) instead of alphabetically (`backlog,
	// done, ready`) without loading the type into memory to sort it.
	//
	// A backend that ranks in SQL must emit the values as LITERALS, not
	// bind parameters: an expression index is matched by expression
	// equivalence, and a parameterised rank stops matching a
	// literal-valued index as soon as the plan cache goes generic —
	// measured at 4 versus 1,915 buffers on 200k rows (TKT-9OFGH4). The
	// values come from operator-authored schema.yaml, the same trust
	// level the derived-index DDL already interpolates, and must still be
	// escaped.
	//
	// Callers must treat a non-nil Values as read-only; backends may
	// retain it for the life of the query.
	Values []string
}

// GraphHeaderQueryer is the content-free projection of [GraphQueryer]:
// the same predicate evaluation, yielding [EntityHeader] rows without the
// markdown body. OPTIONAL — type-asserted like [HeaderReader], with
// [GraphQueryHeaders] as the generic fallback — because it exists purely to
// keep bodies from crossing the wire on backends where that is a real cost
// (TKT-1U8XYN): a list page needs a type's ids and properties to filter,
// sort and paginate, never its bodies.
type GraphHeaderQueryer interface {
	GraphQueryHeaders(ctx context.Context, q GraphQuery) iter.Seq2[EntityHeader, error]
}

// MatchedCounter is the half of [GraphQueryer.GraphCount] a paged list needs:
// the number of rows q matches, and nothing about the type's total.
// OPTIONAL, type-asserted with [CountMatched] as the generic fallback. It
// exists because GraphCount answers two questions with two statements, and
// the second — the unscoped total — is both the one a gated handler must not
// expose (RR-SSPCCI) and, under a world, a count(DISTINCT) over every row of
// the table (TKT-1U8XYN: 35 ms of a list page's 80).
type MatchedCounter interface {
	CountMatched(ctx context.Context, q GraphQuery) (int, error)
}

// CountMatched returns how many rows q matches on any GraphQueryer, using
// the backend's [MatchedCounter] when it has one and GraphCount's matched
// half otherwise. OrderBy/Limit/Offset on q are ignored, as GraphCount does.
func CountMatched(ctx context.Context, gq GraphQueryer, q GraphQuery) (int, error) {
	if mc, ok := gq.(MatchedCounter); ok {
		return mc.CountMatched(ctx, q)
	}
	matched, _, err := gq.GraphCount(ctx, q)
	return matched, err
}

// GraphQueryHeaders runs q as a content-free query on any GraphQueryer.
//
// Uses the backend's native [GraphHeaderQueryer] when it has one, so the
// body never leaves the backend; otherwise falls back to
// [GraphQueryer.GraphQuery] and projects each row as it is yielded. As with
// [ListEntityHeaders], the fallback bounds retention, not transfer.
func GraphQueryHeaders(ctx context.Context, gq GraphQueryer, q GraphQuery) iter.Seq2[EntityHeader, error] {
	if hq, ok := gq.(GraphHeaderQueryer); ok {
		return hq.GraphQueryHeaders(ctx, q)
	}
	return func(yield func(EntityHeader, error) bool) {
		for e, err := range gq.GraphQuery(ctx, q) {
			if err != nil {
				yield(EntityHeader{}, err)
				return
			}
			if !yield(HeaderOf(e), nil) {
				return
			}
		}
	}
}

// GraphBranch is one arm of [GraphQuery.Any]: relation predicates and the
// faces they grant. A nil FaceIn grants every face; a nil HasInbound or
// HasOutbound holds for every entity of the type. A branch with neither is
// only a face set. Both set means both must hold.
//
// HasOutbound reads the entity's outgoing edges. The ACL uses it to gate a
// flow run by the subject it carries (TKT-BC5G8RP): one branch per subject
// type, each with that type's row gate in its endpoint.
type GraphBranch struct {
	HasInbound  *RelationPredicate
	HasOutbound *RelationPredicate
	FaceIn      []entity.Face
}

// NarrowBranch is one arm of [GraphQuery.Narrowing]: a conjunction of
// property predicates, OR-ed with the other arms. An EMPTY branch holds for
// every row, which makes the whole disjunction vacuous — a caller lowering
// an unsatisfiable arm must drop the Narrowing entirely rather than emit an
// empty branch.
//
// Deliberately NOT [GraphBranch], and deliberately carrying no relation or
// face predicate. Faces and conferred roles are authorization concepts; a
// caller narrowing a result set has no business expressing them, and the
// type is what enforces that rather than a comment nobody reads. See
// [GraphQuery.Narrowing] for why sharing one type would be a privilege
// escalation.
type NarrowBranch struct {
	Props []PropPredicate
}

// CondOp is the node kind of a [Cond].
type CondOp int

const (
	// CondProp is a leaf: [Cond.Prop] holds.
	CondProp CondOp = iota
	// CondRelated is a leaf: [Cond.Rel] holds, with the meaning one entry
	// of [GraphQuery.Related] has, its Negate included.
	CondRelated
	// CondAnd holds when every argument holds. With no arguments it holds.
	CondAnd
	// CondOr holds when at least one argument holds. With no arguments it
	// does not hold.
	CondOr
	// CondNot holds when its one argument does not.
	CondNot
)

// MaxCondDepth bounds the nesting of a [Cond]. A deeper tree is
// [ErrInvalidQuery] on every backend, so a recursive renderer or matcher
// cannot be driven arbitrarily deep.
const MaxCondDepth = 32

// MaxCondLeaves bounds the leaves of a [Cond], and MaxCondRelations its
// relation leaves, each of which a SQL backend renders as its own
// subquery. Depth alone does not bound a wide OR built from a caller's
// filter.
const (
	MaxCondLeaves    = 256
	MaxCondRelations = 16
)

// Cond is a boolean expression over an entity's own properties and its
// relations: the predicate of [GraphQuery.Where].
//
// Every leaf is TWO-valued. A property predicate on an absent key does not
// hold, and its negation does; there is no "unknown". SQL backends must
// therefore keep a NULL from leaking through a NOT (`NOT NULL` is NULL,
// which drops the row a two-valued reading keeps).
//
// Exactly the field of the node's Op is meaningful: Prop for CondProp, Rel
// for CondRelated, Args for the others. Build nodes with [PropLeaf],
// [RelatedCond], [AndCond], [OrCond] and [NotCond].
type Cond struct {
	Op   CondOp
	Prop PropPredicate
	Rel  DirectedRelation
	Args []Cond
}

// PropLeaf is a [CondProp] leaf.
func PropLeaf(p PropPredicate) Cond { return Cond{Op: CondProp, Prop: p} }

// RelatedCond is a [CondRelated] leaf.
func RelatedCond(r DirectedRelation) Cond { return Cond{Op: CondRelated, Rel: r} }

// AndCond is a [CondAnd] node.
func AndCond(args ...Cond) Cond { return Cond{Op: CondAnd, Args: args} }

// OrCond is a [CondOr] node.
func OrCond(args ...Cond) Cond { return Cond{Op: CondOr, Args: args} }

// NotCond is a [CondNot] node.
func NotCond(arg Cond) Cond { return Cond{Op: CondNot, Args: []Cond{arg}} }

// AndWhere returns a [GraphQuery.Where] that holds when both a and b do. A
// nil argument is no constraint. The result is a new root node, so a caller
// can AND a narrowing onto a query it copied from another owner; the copy is
// shallow, and the subtrees below the root stay shared with a and b.
func AndWhere(a, b *Cond) *Cond {
	switch {
	case a == nil && b == nil:
		return nil
	case a == nil:
		c := *b
		return &c
	case b == nil:
		c := *a
		return &c
	}
	c := AndCond(*a, *b)
	return &c
}

// Constant reports whether c holds on every row or on none, judged from its
// shape alone: an [OrCond] without arguments is false, an [AndCond] without
// arguments is true, and CondAnd, CondOr and CondNot fold them. known=false
// means c reads a row. A denied traversal gated to false makes a whole tree
// constant this way, so a caller can answer "no row" without a query.
func (c Cond) Constant() (value, known bool) {
	switch c.Op {
	case CondNot:
		if len(c.Args) != 1 {
			return false, false
		}
		v, k := c.Args[0].Constant()
		return !v, k
	case CondAnd, CondOr:
		// An AND is decided by a false argument, an OR by a true one.
		decider := c.Op == CondOr
		all := true
		for _, a := range c.Args {
			v, k := a.Constant()
			switch {
			case k && v == decider:
				return decider, true
			case !k:
				all = false
			}
		}
		if all {
			return !decider, true
		}
		return false, false
	default:
		return false, false
	}
}

// Walk calls visit on c and on every node below it, depth first. It stops
// and returns false as soon as visit does.
func (c Cond) Walk(visit func(Cond) bool) bool {
	if !visit(c) {
		return false
	}
	for _, a := range c.Args {
		if !a.Walk(visit) {
			return false
		}
	}
	return true
}

// ValidateCond refuses a malformed [Cond]: an unknown Op, a CondNot without
// exactly one argument, a leaf with arguments, nesting beyond
// [MaxCondDepth], more leaves than [MaxCondLeaves] or relation leaves than
// [MaxCondRelations], a relation leaf that sets Pred.Negate (a tree negates
// with CondNot, so there is one way to say it), or a leaf predicate
// [ValidatePropPredicates] refuses. A relation leaf's own shape is checked
// with the other relation predicates (graphquerynaive.CheckEndpointShape).
// Every backend runs it, so they refuse the same trees.
func ValidateCond(c Cond) error {
	if err := validateCond(c, 1); err != nil {
		return err
	}
	leaves, rels := 0, 0
	c.Walk(func(n Cond) bool {
		switch n.Op {
		case CondProp:
			leaves++
		case CondRelated:
			leaves++
			rels++
		default:
		}
		return true
	})
	if leaves > MaxCondLeaves || rels > MaxCondRelations {
		return fmt.Errorf("%w: condition has %d leaves (%d relation leaves); the limits are %d and %d",
			ErrInvalidQuery, leaves, rels, MaxCondLeaves, MaxCondRelations)
	}
	return nil
}

func validateCond(c Cond, depth int) error {
	if depth > MaxCondDepth {
		return fmt.Errorf("%w: condition nested deeper than %d", ErrInvalidQuery, MaxCondDepth)
	}
	switch c.Op {
	case CondProp:
		if len(c.Args) > 0 {
			return fmt.Errorf("%w: a property leaf takes no arguments", ErrInvalidQuery)
		}
		return ValidatePropPredicates([]PropPredicate{c.Prop})
	case CondRelated:
		if len(c.Args) > 0 {
			return fmt.Errorf("%w: a relation leaf takes no arguments", ErrInvalidQuery)
		}
		if c.Rel.Pred.Negate {
			return fmt.Errorf("%w: a relation leaf negates with a not node, not Pred.Negate", ErrInvalidQuery)
		}
		return nil
	case CondNot:
		if len(c.Args) != 1 {
			return fmt.Errorf("%w: not takes exactly one argument, got %d", ErrInvalidQuery, len(c.Args))
		}
	case CondAnd, CondOr:
	default:
		return fmt.Errorf("%w: unknown condition op %d", ErrInvalidQuery, c.Op)
	}
	for _, a := range c.Args {
		if err := validateCond(a, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// PropOp is the comparison a [PropPredicate] applies. The ordered operators
// carry an [OrderKind] saying what the stored value must be to compare; the
// store does not consult the metamodel, so the caller picks the kind from
// the property's declared type.
type PropOp int

const (
	// PropEqual matches when the property equals Value. With an empty
	// Value it means "is empty" — see [PropPredicate]. Against a LIST value
	// (multi-select) a non-scalar PropEqual is MEMBERSHIP: it matches when
	// any element equals Value, which is how `has_current_user(entity.watchers)`
	// lowers without a dedicated operator. [PropPredicate.Scalar] opts out
	// of that reading.
	PropEqual PropOp = iota
	// PropNotEqual is the negation. With an empty Value it means "is not
	// empty".
	PropNotEqual
	// PropNotEqualOrEmpty is PropNotEqual WIDENED to also match an empty
	// property: "not this value, or not set at all".
	//
	// It exists because [internal/predicate] and [internal/filter] answer
	// `p ~= v` differently on an unset property, and both are right for
	// their own contract:
	//
	//   - filter (and therefore PropNotEqual) names a POPULATION: an entity
	//     with no status is not in the "status is something other than
	//     doing" population. See the note on [PropPredicate].
	//   - predicate is a Lua expression subset, and its documented equality
	//     table has `nil == anything -> false`, so `nil ~= 'doing'` is
	//     necessarily TRUE.
	//
	// Lowering a predicate `~=` to PropNotEqual would therefore DROP every
	// row whose property is unset — rows the Go pass keeps — which is a
	// pre-filter removing rows the authoritative pass would return, the one
	// thing a pushdown may never do. This operator is the sound lowering
	// target; PropNotEqual keeps its filter-DSL meaning untouched.
	//
	// With an EMPTY Value it is degenerate ("not empty, or empty") and
	// matches everything; callers lowering `p ~= ''` should emit nothing
	// instead of relying on that.
	PropNotEqualOrEmpty
	// PropGreaterEqual and PropLessEqual compare against Value: `>=` and
	// `<=` respectively. What follows is their [OrderText] reading, the
	// zero [PropPredicate.Kind]: the property's string form, byte-wise.
	// [OrderString] and [OrderInteger] are the typed readings a lowered
	// condition uses.
	//
	// # Byte order, and why that is enough
	//
	// These carry the SAME contract [GraphQuery.OrderBy] already does —
	// "sorts by the STRING form of each property, byte-wise" — and are
	// sound for exactly the types that sorting is: those whose byte order
	// IS their order. An ISO-8601 date is the motivating case: `2026-09-11`
	// sorts and compares identically as text and as a date, which is why
	// [internal/queryplan.StringShaped] already lists date and datetime.
	//
	// The store still does not consult the metamodel, so it cannot check
	// this itself. **The CALLER must gate on the declared type** — that is
	// where the metamodel is — and must not emit these for an `integer`
	// property, where byte order is not numeric order ("10" < "9").
	//
	// # Empty and list values never match
	//
	// An empty property is outside any ordered range, matching the
	// [PropNotEqual] reading (an unset value is not in the population) and
	// SQL's NULL comparison, which is likewise not true.
	//
	// A LIST value never matches either, and that is load-bearing rather
	// than incidental: Go renders []string{"a","b"} as `[a b]` while
	// postgres `->>` renders it as `["a", "b"]`, so a byte-wise comparison
	// against a list would give a DIFFERENT answer per backend. Refusing
	// the shape outright is the only reading both can share. Equality does
	// not have this problem because it branches on jsonb_typeof and treats
	// an array as membership.
	//
	// This runtime refusal does NOT make the caller's declared-type gate
	// redundant; the two catch different things. A DECLARED list is caught
	// at config load by the metamodel gate, which is where the useful
	// error lives ("this property is a list" names the mistake). A list
	// VALUE stored under a scalar DECLARATION — a legacy row, an import, a
	// schema changed after the data was written — reaches the store
	// anyway, and only the check here keeps the backends answering it the
	// same way.
	PropGreaterEqual
	PropLessEqual
	// PropKeyEqual matches when the property holds an OBJECT whose entry
	// [PropPredicate.Key] is a string equal to Value (TKT-SM20FG). It is
	// the lookup of an external ref by id: `{Property: "basecamp", Op:
	// PropKeyEqual, Key: "id", Value: "42"}`.
	//
	// A distinct operator rather than a Key on PropEqual. Every backend
	// switch over PropOp handles each operator in its own arm and answers
	// an unknown one with [ErrInvalidQuery], so a reader that does not know
	// this operator refuses it instead of reading it as a plain equality.
	// Planners that lower other dialects to GraphQuery (queryplan, the list
	// pushdown, derived-index inference) never emit it and refuse it by
	// name. Key and Value must both be non-empty, and Key is only valid
	// with this operator; anything else is [ErrInvalidQuery] (see
	// [ValidatePropPredicates]). A non-object value, a missing entry and a
	// non-string entry never match.
	PropKeyEqual
	// PropLess and PropGreater are the strict ordered operators: `<` and
	// `>`. They follow the [PropPredicate.Kind] rules exactly as
	// [PropLessEqual] and [PropGreaterEqual] do (TKT-8HY05WB).
	PropLess
	PropGreater
	// PropIsDate matches when the property is a string of the form
	// YYYY-MM-DD, ASCII digits only (TKT-TKSCSBT). Value must be empty.
	//
	// It is the shape guard an ordered comparison on a `date` needs to
	// fail closed: byte order equals time order only for that shape, so
	// `valid_until >= 2026-11-16` alone admits a stored `3` or `TBD`. The
	// ACL pairs the two for a conditional grant edge, where admitting a
	// malformed value would be a grant nobody wrote.
	//
	// The shape is checked, not the calendar: `2026-02-30` matches and
	// orders between the 28th and March 1st. Relation writes do not check
	// property values against the schema: the data-entry API stores a
	// mistyped value with a warning, and the other write paths store it
	// as is. Entity writes refuse a malformed date only under
	// `validation: strict`. So a value of any shape can come through the
	// API.
	//
	// The graphquerynaive reading also accepts a time.Time at UTC midnight,
	// which is how yaml.v3 decodes an unquoted date in a file backend. The
	// SQL backends hold JSON, where that type cannot occur.
	PropIsDate
)

// OrderKind says how an ordered [PropPredicate] reads the stored value. It
// is meaningful only on PropLess, PropLessEqual, PropGreater and
// PropGreaterEqual; on any other operator a non-zero kind is
// [ErrInvalidQuery].
//
// Under every kind a value the kind cannot read matches NO ordered
// operator. That is the reading the condition engine gives the same
// comparison on an unset or off-type value (internal/predicate: a nil
// operand makes an ordered comparison false), so a store answer can
// replace the Go evaluation.
type OrderKind int

const (
	// OrderText compares the property's text form byte-wise, and an empty
	// or list value never matches. It is the original contract of the
	// ordered operators; see [PropGreaterEqual].
	OrderText OrderKind = iota
	// OrderString compares a stored STRING byte-wise, the empty string
	// included; any other stored value (absent, null, a number, a list)
	// never matches. It is the condition engine's reading of a
	// string-typed property, which binds a non-string value as nil.
	OrderString
	// OrderInteger compares the stored value numerically as an int64, when
	// internal/propmatch.Integer reads one from it: a whole number in int64 range,
	// or a string strconv.ParseInt accepts. Any other value never matches.
	// Value must parse as an int64 in base 10.
	OrderInteger
)

// isOrdered reports whether op is one of the four ordered operators.
func isOrdered(op PropOp) bool {
	switch op {
	case PropLess, PropLessEqual, PropGreater, PropGreaterEqual:
		return true
	default:
		return false
	}
}

// UnknownPropOpError is the [ErrInvalidQuery] for an operator a backend
// does not know.
func UnknownPropOpError(p PropPredicate) error {
	return fmt.Errorf("%w: property %q: unknown operator %d", ErrInvalidQuery, p.Property, p.Op)
}

// ValidatePropPredicates refuses an unknown operator, a malformed
// [PropKeyEqual] use (a Key on another operator, or a PropKeyEqual with an
// empty Key or Value, or a Key outside [a-z0-9_]), and a malformed
// [OrderKind] use (a kind on a non-ordered operator, an unknown kind, or an
// [OrderInteger] Value that is not an int64). Every backend runs it
// (through the naive shape check) so they refuse the same shapes.
func ValidatePropPredicates(props []PropPredicate) error {
	for _, p := range props {
		switch p.Op {
		case PropEqual, PropNotEqual, PropNotEqualOrEmpty, PropGreaterEqual, PropLessEqual, PropKeyEqual,
			PropLess, PropGreater:
		case PropIsDate:
			if p.Value != "" || p.Key != "" || p.Scalar {
				return fmt.Errorf("%w: property %q: PropIsDate takes no Value, Key or Scalar", ErrInvalidQuery, p.Property)
			}
			continue
		default:
			return UnknownPropOpError(p)
		}
		if err := validateOrderKind(p); err != nil {
			return err
		}
		if p.Op != PropKeyEqual {
			if p.Key != "" {
				return fmt.Errorf("%w: property %q: Key is only valid with PropKeyEqual", ErrInvalidQuery, p.Property)
			}
			continue
		}
		if p.Key == "" || p.Value == "" {
			return fmt.Errorf("%w: property %q: PropKeyEqual needs a Key and a Value", ErrInvalidQuery, p.Property)
		}
		for i := range len(p.Key) {
			c := p.Key[i]
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
				return fmt.Errorf("%w: property %q: Key %q must match [a-z0-9_]+", ErrInvalidQuery, p.Property, p.Key)
			}
		}
	}
	return nil
}

// validateOrderKind checks p's [OrderKind] against its operator and value.
func validateOrderKind(p PropPredicate) error {
	switch p.Kind {
	case OrderText, OrderString:
	case OrderInteger:
		if _, err := strconv.ParseInt(p.Value, 10, 64); err != nil {
			return fmt.Errorf("%w: property %q: OrderInteger needs an int64 Value, got %q",
				ErrInvalidQuery, p.Property, p.Value)
		}
	default:
		return fmt.Errorf("%w: property %q: unknown order kind %d", ErrInvalidQuery, p.Property, p.Kind)
	}
	if p.Kind != OrderText && !isOrdered(p.Op) {
		return fmt.Errorf("%w: property %q: an order kind is only valid on an ordered operator",
			ErrInvalidQuery, p.Property)
	}
	return nil
}

// PropPredicate restricts a GraphQuery to entities whose own property
// matches. Multiple predicates on one query are ANDed.
//
// Emptiness follows [internal/propmatch], the single authoritative
// definition shared with [internal/filter] — a missing key and a
// present-but-empty value are the SAME state, because YAML frontmatter
// parses a valueless key to nil and an operator asking "is this field
// filled in?" does not distinguish the two:
//
//	{Property: "status", Op: PropEqual, Value: "doing"}  // status=doing
//	{Property: "billing_email", Op: PropEqual}           // is empty
//	{Property: "billing_email", Op: PropNotEqual}        // is not empty
//	{Property: "status", Op: PropNotEqualOrEmpty, Value: "doing"} // status~=doing (Lua)
//
// Note that an EMPTY property does not satisfy a PropNotEqual against a
// non-empty Value: an entity with no status is not in the "status is
// something other than doing" population. Treating it as a match would
// silently widen every exclusion filter to include unset rows.
//
// [PropNotEqualOrEmpty] is the deliberate opposite reading, for lowering a
// [internal/predicate] `~=` whose Lua semantics DO match an unset property.
// The two ops differ only on empty values; picking the wrong one is a
// silent wrong answer in one direction or the other, so choose by which
// dialect authored the comparison.
type PropPredicate struct {
	Property string
	Op       PropOp
	Value    string
	// Scalar restricts a non-empty equality predicate to a scalar string and
	// lets SQL backends emit an indexable ->> comparison. On a non-empty
	// [PropNotEqualOrEmpty] it is the negation of that: every row except a
	// stored string equal to Value, so a list or a number matches. It is
	// ignored for empty values and other operators.
	Scalar bool
	// Key names the object entry a [PropKeyEqual] compares. Empty for
	// every other operator.
	Key string
	// Kind says how an ordered operator reads the stored value; see
	// [OrderKind]. Zero ([OrderText]) for every other operator.
	Kind OrderKind
}

// RelationPredicate restricts which relations the surrounding
// GraphQuery is willing to match through.
//
// Two transitive expansions, independent and composable:
//
//   - InheritThrough (endpoint-side) transitively expands Endpoints via
//     these relation types up to Depth. Example: ACL group expansion
//     (InheritThrough = ["member-of"]).
//   - EntityInheritThrough (entity-side) transitively expands the
//     candidate entity via these relation types up to EntityDepth; the
//     match succeeds if any ancestor of the candidate (including itself)
//     has the inbound/outbound edge. Example: ACL containment
//     inheritance (EntityInheritThrough = ["belongs-to"]).
type RelationPredicate struct {
	// Endpoints restricts which entities on the far side of the relation
	// count as a match.
	//
	// An EMPTY (or nil) Endpoints means "ANY endpoint": the predicate is
	// then purely about the edge existing, which is what an absence
	// query needs ("has no implements edge at all", with Negate). Note
	// this is a WIDENING, not a narrowing — a caller deriving endpoints
	// from a principal or a lookup MUST guard against accidentally
	// passing an empty set, or the predicate silently stops constraining
	// (see internal/acl.readQuery, which fails closed for exactly this).
	//
	// InheritThrough is inert when Endpoints is empty: there is nothing
	// to expand from, so the endpoint closure is skipped entirely.
	Endpoints []string
	OfTypes   []string

	// Negate inverts the predicate: the entity matches when NO relation
	// satisfies it ("has no billing-contact edge"). This is a separate
	// flag rather than an overload of a nil *RelationPredicate, because
	// nil already means "do not constrain this direction" — the two are
	// different questions, and conflating them would silently turn every
	// unconstrained query into an absence query.
	//
	// Negation composes with both expansions: with EntityInheritThrough
	// set, a negated predicate matches only when NO ancestor of the
	// candidate (including itself) has the edge.
	Negate bool

	InheritThrough []string
	Depth          int

	EntityInheritThrough []string
	EntityDepth          int

	// EndpointMatch further restricts which entities on the far side of the
	// relation count as a match, by their TYPE and their own PROPERTIES —
	// "has a caused-by edge to a ticket whose status is done". Nil means no
	// endpoint-entity constraint, which is the pre-existing behavior.
	//
	// It composes with [RelationPredicate.Endpoints] as a conjunction: an
	// endpoint must be in the (expanded) id set AND satisfy this match. The
	// two answer different questions — Endpoints names ids the CALLER already
	// knows, EndpointMatch describes ids it does not — so a caller supplying
	// only this one leaves Endpoints empty without that reading as the
	// "any endpoint" widening Endpoints documents.
	//
	// Nested one level per hop, so a chain A -x-> B -y-> C is an
	// EndpointMatch on B carrying its own relation predicate for the y hop.
	// Nesting is bounded by EVERY backend at graphquerynaive.DepthCap, which
	// is a REFUSAL rather than a truncation: a too-deep chain errors. It must
	// not degrade to an unsatisfiable arm instead, because under a Negate that
	// inverts to "match every row" — a guard becoming a row-gate bypass.
	//
	// A nested hop may NOT carry InheritThrough / EntityInheritThrough. The
	// SQL backends cannot emit them there, and a backend that silently ignored
	// an ACL-folded expansion would gate the same principal differently from
	// one that honored it; both refuse instead.
	//
	// A hop carrying an EndpointMatch follows identity-scoped edges only (a
	// "" tail), and reads the endpoint at a face chosen by its selection
	// (TKT-KQXVF7, design A6): the endpoint's own [EndpointPredicate.Faces]
	// when set, else the enclosing hop's, else the query's
	// [GraphQuery.Faces]. Under InWorld(w) the endpoint is w's prime for its
	// own type; under AtFaces(fs) only its rows at fs are tested, so an
	// endpoint with none of fs does not match; under AllFaces it matches when
	// any of its rows does. InWorld of the default world is the historical
	// reading: the "" face only.
	//
	// SECURITY: this predicate reads properties of entities the query does
	// not RETURN, so a caller-supplied EndpointMatch is an inference channel
	// — which rows come back reveals the neighbor's property values, and its
	// existence, without any row of the neighbor ever being serialized (so
	// response redaction never fires). A caller composing this from untrusted
	// input MUST gate the traversed type with its own read query and MUST
	// refuse properties whose visibility is conditional. See TKT-RELTRV.
	EndpointMatch *EndpointPredicate

	// EdgeProps restricts the matching edge by its OWN properties, ANDed,
	// with the [PropPredicate] contract. Nil means no edge constraint.
	//
	// It applies to the edge this predicate names (the one of OfTypes
	// between the candidate, or an EntityInheritThrough ancestor, and an
	// endpoint), never to the edges an InheritThrough or
	// EntityInheritThrough closure walks. The ACL uses it for a grant edge
	// that counts only while a condition holds (TKT-TKSCSBT), so the
	// condition must not leak onto containment hops.
	EdgeProps []PropPredicate

	// Gate marks a predicate that is an ACL read gate folded onto a
	// traversal's endpoint ([EndpointPredicate.HasInbound], set by
	// acl.Request.GateTraversal). Under a read instant ([BindReadInstant]) a
	// gate reads today's edges and rows, like a top-level HasInbound or Any
	// branch, while the traversal around it reads the instant (DEC-TDEF3GD).
	// Outside an instant it changes nothing.
	Gate bool
}

// EndpointPredicate constrains the entity on the far side of a relation: its
// type, its own properties, and (for a chained traversal) its own outgoing or
// incoming relation predicate.
//
// Deliberately NOT [GraphQuery]: an endpoint match is a filter on a row the
// query does not return, so it carries no paging, no ordering and no face
// allowlist. Reusing GraphQuery would offer all three and silently ignore them.
type EndpointPredicate struct {
	// Faces, when set, is the face selection this endpoint (and every hop
	// nested under it that sets none) is evaluated at, instead of the
	// enclosing selection. The zero value inherits; see
	// [RelationPredicate.EndpointMatch] for how each selection reads an
	// endpoint.
	//
	// The ACL sets it on every hop it gates (acl.Request.GateTraversal) to
	// InWorld of the REQUEST's world, so a row gate folded into an endpoint
	// is evaluated where the request reads and never under a query's
	// AllFaces or AtFaces selection, where "any face matches" could satisfy
	// a gate the request's world does not (stage-2 design section 12,
	// RR-QUXMAF).
	Faces FaceSelection

	// EntityType restricts the endpoint to one entity type. Empty means any
	// type. This is what a chained traversal uses to resolve a relation whose
	// declared target is a UNION of types: without it, a property reference
	// has no single declared type to be compared against.
	EntityType string

	// Props are the endpoint's own property predicates, ANDed. They carry the
	// same contract as [GraphQuery.Props], including the caller's obligation
	// to gate ordered comparisons on the declared type (see [PropPredicate]).
	Props []PropPredicate

	// HasInbound and HasOutbound chain the traversal one hop further, with
	// the same meaning they have on [GraphQuery]. Nil means no constraint.
	HasInbound  *RelationPredicate
	HasOutbound *RelationPredicate
}

// GraphQueryer is the read-side interface for graph-shape queries.
// Embedded into Store; surfaces independently so backend
// implementations can be written and tested without the full Store. Its
// iterators follow [EntityReader]'s rule on resources held across yield.
type GraphQueryer interface {
	// GraphQuery returns an iterator over entities matching q. The
	// iterator yields (*entity.Entity, nil) for each match; on error
	// the iterator yields (nil, err) and terminates.
	GraphQuery(ctx context.Context, q GraphQuery) iter.Seq2[*entity.Entity, error]

	// GraphCount returns (matched, total): the number of rows of
	// q.EntityType that satisfy q's predicates, and the total number of
	// rows of q.EntityType ignoring those predicates. Callers use
	// (total - matched) for "filtered by" counts. A row is what GraphQuery
	// would yield: one per id under InWorld, one per selected face under
	// AllFaces and AtFaces.
	GraphCount(ctx context.Context, q GraphQuery) (matched, total int, err error)

	// MatchingFaces answers: "of these candidate ids, which stored face
	// rows satisfy q?" It runs q, with its selection, FaceIn and Any
	// branches, restricted to ids, and returns each matching (id, face)
	// row. Under InWorld that is at most one face per id (the prime); under
	// AllFaces or AtFaces it is every selected row that matches.
	//
	// An id with no matching row is absent from the map, and a present id
	// has at least one face. Faces are distinct and sorted by token; a
	// caller that needs another order sorts them itself.
	//
	// q is passed by value: implementations MUST NOT mutate it, and the
	// caller is free to reuse the input on the next call. ids is the
	// candidate set; an empty slice yields an empty map. OrderBy, Limit and
	// Offset are ignored, as for GraphCount.
	//
	// It is the per-face read verdict the ACL row gate uses
	// (acl.Request.ReadableFacesMany), and [MatchingIDs] projects it for
	// callers that need ids only. Backends implement this method alone, so
	// the two answers cannot drift.
	MatchingFaces(ctx context.Context, q GraphQuery, ids []string) (map[string][]entity.Face, error)
}

// FaceMatcher is the [GraphQueryer.MatchingFaces] half of the graph queryer,
// for a consumer that needs nothing else.
type FaceMatcher interface {
	MatchingFaces(ctx context.Context, q GraphQuery, ids []string) (map[string][]entity.Face, error)
}

// MatchingIDs answers "of these candidate ids, which ones satisfy q?" on any
// [FaceMatcher]: an id matches when any of its selected rows does. The map is
// keyed by every input id, true for a match and false otherwise, so a caller
// can tell "no match" from "not asked".
//
// It projects [GraphQueryer.MatchingFaces]; backends do not implement it.
func MatchingIDs(ctx context.Context, fm FaceMatcher, q GraphQuery, ids []string) (map[string]bool, error) {
	faces, err := fm.MatchingFaces(ctx, q, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = len(faces[id]) > 0
	}
	return out, nil
}

// SortedFaces returns faces deduplicated and sorted by token, the order
// [GraphQueryer.MatchingFaces] promises. Backends that collect rows in
// another order finish with it.
func SortedFaces(faces []entity.Face) []entity.Face {
	out := slices.Clone(faces)
	slices.Sort(out)
	return slices.Compact(out)
}

// IDMatcher is [MatchingIDs] bound to fm, for a consumer that takes the
// match as a function value.
func IDMatcher(fm FaceMatcher) func(ctx context.Context, q GraphQuery, ids []string) (map[string]bool, error) {
	return func(ctx context.Context, q GraphQuery, ids []string) (map[string]bool, error) {
		return MatchingIDs(ctx, fm, q, ids)
	}
}

// RowCond returns q's row predicate as a [Cond]: the AND of its Props, its
// Related entries and its Where, so that `Where: &c` on a query of the same
// type matches the rows q matches. A Related entry with Pred.Negate set
// becomes a [CondNot] over the entry without it, the one way a Cond
// negates. It refuses a query with a predicate a Cond cannot express:
// FaceIn (a face test), Any, Narrowing, HasInbound or HasOutbound. Faces,
// OrderBy, Limit, Offset and SkipUnreadable select or page rows rather than
// test one, and are ignored.
//
// A field grant's `when:` (search.FieldGrant) is a query of this shape;
// a reader that pushes a filter on a property hidden on some rows ANDs it
// with the grant's condition (TKT-9BKVQ9I).
func (q GraphQuery) RowCond() (Cond, error) {
	switch {
	case len(q.FaceIn) > 0:
		return Cond{}, fmt.Errorf("%w: a face set has no condition form", ErrInvalidQuery)
	case len(q.Any) > 0, len(q.Narrowing) > 0:
		return Cond{}, fmt.Errorf("%w: branches have no condition form", ErrInvalidQuery)
	case q.HasInbound != nil, q.HasOutbound != nil:
		return Cond{}, fmt.Errorf("%w: HasInbound and HasOutbound have no condition form", ErrInvalidQuery)
	}
	args := make([]Cond, 0, len(q.Props)+len(q.Related)+1)
	for _, p := range q.Props {
		args = append(args, PropLeaf(p))
	}
	for _, r := range q.Related {
		negate := r.Pred.Negate
		r.Pred.Negate = false
		leaf := RelatedCond(r)
		if negate {
			leaf = NotCond(leaf)
		}
		args = append(args, leaf)
	}
	if q.Where != nil {
		args = append(args, *q.Where)
	}
	if len(args) == 1 {
		return args[0], nil
	}
	return AndCond(args...), nil
}
