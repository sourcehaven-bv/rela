package acl

// SourceKind enumerates the closed set of ways a role can land in a
// principal's effective set. Order doesn't matter for wire stability —
// the int values are not exposed; sort precedence is in priority().
type SourceKind int

const (
	SourceGlobal SourceKind = iota
	SourceGroup
	SourceLocal
	SourceLocalViaGroup
	SourceLocalViaAncestor
	SourceLocalViaGroupAndAncestor
	// SourceAsserted: the role came from a claim in a verified identity
	// assertion, mapped through asserted_role_assignments. Distinct from
	// SourceGlobal because the provenance question after an incident —
	// "was this granted by our policy, or by a claim in a token?" — has
	// materially different answers and remediations.
	SourceAsserted

	// numSourceKinds is the count of declared kinds. It MUST be last in the
	// const block: it is what lets TestSourceKinds_Exhaustive detect a kind
	// added here and not registered in sourceKindPriority, both String()
	// methods, and lessSource — none of which the compiler checks, since
	// every switch over SourceKind has a default.
	numSourceKinds
)

// priority defines the sort precedence used to pick the primary
// Source from a multi-source attribution set. Lower wins. Defined as
// an explicit map so reordering the const block above is a no-op for
// the public sort order — the relationship lives here, in one place.
// sourceKindUnknownPriority is the sort weight assigned to a Kind
// not listed in sourceKindPriority. Chosen large enough to sort
// after any defined kind without colliding with any future addition.
const sourceKindUnknownPriority = 999

func (k SourceKind) priority() int {
	p, ok := sourceKindPriority[k]
	if !ok {
		return sourceKindUnknownPriority
	}
	return p
}

var sourceKindPriority = map[SourceKind]int{
	SourceGlobal: 0,
	// Asserted sorts immediately after global: like global it is a
	// principal-wide fact rather than an entity-local one, but a policy
	// assignment is the more specific statement about THIS deployment,
	// so it keeps precedence when both apply.
	SourceAsserted:                 1,
	SourceGroup:                    2,
	SourceLocal:                    3,
	SourceLocalViaGroup:            4,
	SourceLocalViaAncestor:         5,
	SourceLocalViaGroupAndAncestor: 6,
}

// String returns the wire/log form of a SourceKind.
func (k SourceKind) String() string {
	switch k {
	case SourceGlobal:
		return "global"
	case SourceGroup:
		return "group"
	case SourceLocal:
		return "local"
	case SourceLocalViaGroup:
		return "local-via-group"
	case SourceLocalViaAncestor:
		return "local-via-ancestor"
	case SourceLocalViaGroupAndAncestor:
		return "local-via-group-and-ancestor"
	case SourceAsserted:
		return "asserted"
	default:
		// Includes numSourceKinds (a count, not a kind) and any value from a
		// future const-block addition that was never registered here.
		return "unknown"
	}
}

// Source describes how a role landed in a principal's effective set.
// Flat struct with all four optional fields — populated per Kind:
//
//	Global                          → none
//	Group                           → Group
//	Local                           → Relation
//	LocalViaGroup                   → Group, Relation
//	LocalViaAncestor                → Ancestor, Relation
//	LocalViaGroupAndAncestor        → Group, Ancestor, Relation
//
// Visually, the four Local* variants correspond to the four corners of
// a (member==user?, target==entity?) decision square — the resolver
// picks the variant in buildLocalSource based on which corner the
// matched HasEdge probe landed in:
//
//	                     target == entity              target != entity
//	                 ┌──────────────────────────┬────────────────────────────┐
//	member == user   │  Local                   │  LocalViaAncestor          │
//	                 │  (Relation)              │  (Ancestor, Relation)      │
//	                 ├──────────────────────────┼────────────────────────────┤
//	member != user   │  LocalViaGroup           │  LocalViaGroupAndAncestor  │
//	(group hop)      │  (Group, Relation)       │  (Group, Ancestor,         │
//	                 │                          │   Relation)                │
//	                 └──────────────────────────┴────────────────────────────┘
//
// Source is comparable; safe to use as a map key when paired with the
// role name (see RoleAttribution).
type Source struct {
	Kind     SourceKind
	Group    string
	Ancestor string
	Relation string
	// Claim is the asserted claim value that granted the role, populated
	// only for SourceAsserted. Keeping it a plain string preserves Source's
	// comparability, which attrKey and aclmap.Route both depend on.
	Claim string
}

// String renders the human/log form of a Source. Audit and 403-body
// consumers should marshal the typed fields rather than parsing this
// string — the format is for log messages and test diagnostics, not a
// stable wire contract.
func (s Source) String() string {
	switch s.Kind {
	case SourceGlobal:
		return "global"
	case SourceGroup:
		return "group:" + s.Group
	case SourceLocal:
		return "local:" + s.Relation
	case SourceLocalViaGroup:
		return "local-via-group:" + s.Group + ":" + s.Relation
	case SourceLocalViaAncestor:
		return "local-via-ancestor:" + s.Ancestor + ":" + s.Relation
	case SourceLocalViaGroupAndAncestor:
		return "local-via-group-and-ancestor:" + s.Group + ":" + s.Ancestor + ":" + s.Relation
	case SourceAsserted:
		return "asserted:" + s.Claim
	default:
		return s.Kind.String()
	}
}

// RoleAttribution is a (role, source) pair. The same role can land
// with multiple sources (e.g. via group AND via direct local edge);
// the resolver returns each as a distinct attribution.
type RoleAttribution struct {
	Role   string
	Source Source
}

// attrKey is the composite map key used to dedupe (role, source)
// attribution pairs. Replaces an earlier string-concat approach that
// would have broken on role/source values containing the separator.
type attrKey struct {
	Role   string
	Source Source
}

// PrimarySource picks the canonical attribution to credit on the wire.
// Sort precedence: (Kind.priority, Group, Ancestor, Relation).
// Returns the zero Source if the input is empty.
//
// Linear pass; n is small (typically <= 6 attributions).
func PrimarySource(srcs []Source) Source {
	if len(srcs) == 0 {
		return Source{}
	}
	best := srcs[0]
	for _, s := range srcs[1:] {
		if lessSource(s, best) {
			best = s
		}
	}
	return best
}

func lessSource(a, b Source) bool {
	if a.Kind.priority() != b.Kind.priority() {
		return a.Kind.priority() < b.Kind.priority()
	}
	if a.Group != b.Group {
		return a.Group < b.Group
	}
	if a.Ancestor != b.Ancestor {
		return a.Ancestor < b.Ancestor
	}
	if a.Relation != b.Relation {
		return a.Relation < b.Relation
	}
	// Without this, two asserted attributions differing only by claim
	// compare equal and PrimarySource picks non-deterministically.
	return a.Claim < b.Claim
}
