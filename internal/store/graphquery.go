package store

import (
	"context"
	"iter"

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
// holds AND both relation predicates hold. A zero-value GraphQuery
// beyond EntityType matches every entity of that type.
//
// All three backends ship a default implementation that delegates to
// [internal/store/graphquerynaive] (iterate-and-filter in Go). A
// future SQL-pushdown implementation in pgstore is tracked as a
// follow-up.
type GraphQuery struct {
	EntityType  string
	Props       []PropPredicate    // entity's own properties match (AND)
	HasInbound  *RelationPredicate // entity has matching relation FROM (expanded) endpoints
	HasOutbound *RelationPredicate // entity has matching relation TO (expanded) endpoints

	// World scopes the RESULT to each entity's prime under the compiled
	// world, exactly as [EntityQuery.World] does. The zero value is the
	// default world.
	//
	// It must live here as well as on EntityQuery, not only there: the
	// ACL read path swaps an EntityQuery for a GraphQuery the moment a
	// policy query exists (internal/visibility/pushdown.go), and the
	// AllowAll principal takes the EntityQuery branch. A world carried
	// on only one of the two would make the list path and the
	// single-entity path disagree — and would do so precisely for the
	// privileged principal.
	//
	// Scoping applies to the entities the query RETURNS. Relation
	// predicates walk the graph's identity structure and are NOT
	// world-resolved: who an entity is related to must not depend on the
	// reader's world.
	World WorldScope

	// FaceIn is [EntityQuery.FaceIn], carried here for the same reason World
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

	// OrderBy, Limit and Offset page a ROW query (GraphQuery,
	// GraphQueryHeaders) inside the backend (TKT-1U8XYN), so a list page
	// costs one bounded read instead of a whole-type scan sorted and sliced
	// in Go. GraphCount and MatchingIDs IGNORE all three — a count answers
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

// OrderSpec is one GraphQuery sort key.
type OrderSpec struct {
	Property   string
	Descending bool
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

// GraphBranch is one arm of [GraphQuery.Any]: a relation predicate and the
// faces it grants. A nil FaceIn grants every face; a nil HasInbound holds for
// every entity of the type (the branch is then only a face set).
type GraphBranch struct {
	HasInbound *RelationPredicate
	FaceIn     []entity.Face
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

// PropOp is the comparison a [PropPredicate] applies. Deliberately only
// equality and its negation: ordered comparison (`due < 2026-01-01`)
// needs the property's declared type from the metamodel to avoid
// comparing dates lexicographically, and the store layer does not
// consult the metamodel. Typed comparison stays above the store in
// [internal/filter].
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
	// PropGreaterEqual and PropLessEqual compare the property's string form
	// byte-wise against Value: `>=` and `<=` respectively.
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
)

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
	// lets SQL backends emit an indexable ->> comparison. It is ignored for
	// empty values and other operators.
	Scalar bool
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
}

// GraphQueryer is the read-side interface for graph-shape queries.
// Embedded into Store; surfaces independently so backend
// implementations can be written and tested without the full Store.
type GraphQueryer interface {
	// GraphQuery returns an iterator over entities matching q. The
	// iterator yields (*entity.Entity, nil) for each match; on error
	// the iterator yields (nil, err) and terminates.
	GraphQuery(ctx context.Context, q GraphQuery) iter.Seq2[*entity.Entity, error]

	// GraphCount returns (matched, total): the number of entities of
	// q.EntityType that satisfy q's predicates, and the total number of
	// entities of q.EntityType ignoring those predicates. Callers use
	// (total - matched) for "filtered by" counts.
	GraphCount(ctx context.Context, q GraphQuery) (matched, total int, err error)

	// MatchingIDs answers: "of these candidate ids, which ones satisfy
	// q's predicates?" Returns a map keyed by every candidate id with
	// the boolean value indicating match (true) or no-match (false).
	// All input ids appear in the result regardless of outcome, so
	// callers can distinguish "absent because no-match" from "absent
	// because no answer."
	//
	// q is passed by value: implementations MUST NOT mutate it, and
	// the caller is free to reuse the input on the next call. ids is
	// the candidate set; an empty slice yields an empty map.
	//
	// Use this rather than threading id filters through GraphQuery —
	// it's the single-entity-visibility and batched-include shape used
	// by the ACL read gate.
	MatchingIDs(ctx context.Context, q GraphQuery, ids []string) (map[string]bool, error)
}
