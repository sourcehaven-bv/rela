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
	SourceGlobal:                   0,
	SourceGroup:                    1,
	SourceLocal:                    2,
	SourceLocalViaGroup:            3,
	SourceLocalViaAncestor:         4,
	SourceLocalViaGroupAndAncestor: 5,
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
	}
	return "unknown"
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
// Source is comparable; safe to use as a map key when paired with the
// role name (see RoleAttribution).
type Source struct {
	Kind     SourceKind
	Group    string
	Ancestor string
	Relation string
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
	}
	return s.Kind.String()
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
	return a.Relation < b.Relation
}
