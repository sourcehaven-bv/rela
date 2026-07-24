package acl

import (
	"slices"
	"strings"
	"testing"
)

// Pure-value tests for Source, SourceKind, and PrimarySource. No
// resolver or graph fixture — these are unit-level invariants on the
// types that the feature tests rely on as building blocks.

func TestSourceKindString_AllKindsRender(t *testing.T) {
	t.Parallel()
	// Pin the wire/log strings for every declared kind. A reorder of
	// the const block (or a typo in a new kind's String case) shows up
	// as a failing test, not as a wire-format regression in audit logs.
	cases := []struct {
		kind SourceKind
		want string
	}{
		{SourceGlobal, "global"},
		{SourceGroup, "group"},
		{SourceLocal, "local"},
		{SourceLocalViaGroup, "local-via-group"},
		{SourceLocalViaAncestor, "local-via-ancestor"},
		{SourceLocalViaGroupAndAncestor, "local-via-group-and-ancestor"},
		{SourceAsserted, "asserted"},
	}
	for _, c := range cases {
		got := c.kind.String()
		if got != c.want {
			t.Errorf("SourceKind(%d).String() = %q, want %q", c.kind, got, c.want)
		}
	}
	if len(cases) != len(allSourceKinds) {
		t.Errorf("table covers %d kinds but %d are declared — add the new kind here",
			len(cases), len(allSourceKinds))
	}
}

// allSourceKinds is the single list every enum test iterates. Adding a kind to
// the const block without adding it here fails TestSourceKinds_Exhaustive
// below, which is the guard that makes the omission LOUD: without it, a new
// kind silently renders "unknown" at priority 999 and every hand-maintained
// table in this file keeps passing.
var allSourceKinds = []SourceKind{
	SourceGlobal,
	SourceAsserted,
	SourceGroup,
	SourceLocal,
	SourceLocalViaGroup,
	SourceLocalViaAncestor,
	SourceLocalViaGroupAndAncestor,
}

func TestSourceKinds_Exhaustive(t *testing.T) {
	t.Parallel()
	// numSourceKinds comes from the const block itself, so this walks every
	// declared kind whether or not anyone remembered to list it in
	// allSourceKinds. That is the whole point: the compiler checks none of
	// this (every switch over SourceKind has a default), so a kind added to
	// the const block and forgotten everywhere else would otherwise render
	// "unknown" at priority 999 with every hand-maintained table still green.
	if len(allSourceKinds) != int(numSourceKinds) {
		t.Fatalf("allSourceKinds has %d entries but %d kinds are declared — "+
			"add the new kind to allSourceKinds", len(allSourceKinds), numSourceKinds)
	}
	for i := range int(numSourceKinds) {
		k := SourceKind(i)
		if k.String() == "unknown" {
			t.Errorf("SourceKind(%d) has no SourceKind.String() case", k)
		}
		if k.priority() == sourceKindUnknownPriority {
			t.Errorf("SourceKind(%d) (%s) is missing from sourceKindPriority", k, k)
		}
		if !slices.Contains(allSourceKinds, k) {
			t.Errorf("SourceKind(%d) (%s) is missing from allSourceKinds", k, k)
		}
		// Source.String() must special-case the kind too; falling through to
		// SourceKind.String() means the kind's payload fields never render.
		if got := (Source{Kind: k}).String(); got == "" {
			t.Errorf("SourceKind(%d) (%s) renders empty via Source.String()", k, k)
		}
	}
}

func TestSourceKindString_UnknownKind(t *testing.T) {
	t.Parallel()
	// A SourceKind outside the declared range must not panic and must
	// not produce a recognized wire string.
	var k SourceKind = 99
	got := k.String()
	if got != "unknown" {
		t.Errorf("unknown SourceKind = %q, want %q", got, "unknown")
	}
}

func TestSourceKindPriority_DeclaredKinds(t *testing.T) {
	t.Parallel()
	// Each declared kind has a priority distinct from every other, matching the
	// documented sort order (Global < Asserted < Group < Local < LocalViaGroup
	// < LocalViaAncestor < LocalViaGroupAndAncestor). allSourceKinds is written
	// in that order, so this also pins the order itself.
	declared := allSourceKinds
	seen := map[int]SourceKind{}
	for _, k := range declared {
		p := k.priority()
		if other, dup := seen[p]; dup {
			t.Errorf("SourceKind %v and %v both have priority %d", other, k, p)
		}
		seen[p] = k
	}
	// Pin the order: priorities increase with the const declaration.
	for i := 1; i < len(declared); i++ {
		if declared[i-1].priority() >= declared[i].priority() {
			t.Errorf("priority order violated: %v(%d) >= %v(%d)",
				declared[i-1], declared[i-1].priority(),
				declared[i], declared[i].priority())
		}
	}
}

func TestSourceKindPriority_UnknownKind(t *testing.T) {
	t.Parallel()
	// Unknown kinds sort after every declared kind, so adding a new
	// kind without registering it in the priority map doesn't
	// accidentally promote it to "primary".
	var k SourceKind = 99
	if k.priority() <= SourceLocalViaGroupAndAncestor.priority() {
		t.Errorf("unknown kind priority %d should be greater than last declared %d",
			k.priority(), SourceLocalViaGroupAndAncestor.priority())
	}
}

func TestSourceString_PerKind(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  Source
		want string
	}{
		{"global", Source{Kind: SourceGlobal}, "global"},
		{"group", Source{Kind: SourceGroup, Group: "engineering"}, "group:engineering"},
		{"local", Source{Kind: SourceLocal, Relation: "editor-of"}, "local:editor-of"},
		{
			"local-via-group",
			Source{Kind: SourceLocalViaGroup, Group: "engineering", Relation: "editor-of"},
			"local-via-group:engineering:editor-of",
		},
		{
			"local-via-ancestor",
			Source{Kind: SourceLocalViaAncestor, Ancestor: "F-eng", Relation: "editor-of"},
			"local-via-ancestor:F-eng:editor-of",
		},
		{
			"local-via-group-and-ancestor",
			Source{
				Kind: SourceLocalViaGroupAndAncestor, Group: "engineering",
				Ancestor: "F-eng", Relation: "editor-of",
			},
			"local-via-group-and-ancestor:engineering:F-eng:editor-of",
		},
		{"asserted", Source{Kind: SourceAsserted, Claim: "admin"}, "asserted:admin"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := c.src.String()
			if got != c.want {
				t.Errorf("Source.String() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestPrimarySource_AssertedClaimTiebreak(t *testing.T) {
	t.Parallel()
	// Two asserted attributions differing ONLY by claim must order
	// deterministically. Without Claim in the lessSource chain they compare
	// equal, and PrimarySource — plus every aclmap artifact downstream —
	// becomes input-order dependent, which surfaces as a flaky golden test
	// rather than an obvious bug.
	a := Source{Kind: SourceAsserted, Claim: "admin"}
	b := Source{Kind: SourceAsserted, Claim: "billing"}

	if got := PrimarySource([]Source{a, b}); got != a {
		t.Errorf("PrimarySource([a b]) = %+v, want %+v", got, a)
	}
	if got := PrimarySource([]Source{b, a}); got != a {
		t.Errorf("PrimarySource([b a]) = %+v, want %+v — order-dependent result "+
			"means Claim is missing from the lessSource tiebreak", got, a)
	}
}

func TestPrimarySource_PolicyAssignmentBeatsAssertedClaim(t *testing.T) {
	t.Parallel()
	// A global policy assignment is the more specific statement about THIS
	// deployment than a claim minted by the IdP, so it wins as primary when
	// both grant the same role.
	global := Source{Kind: SourceGlobal}
	asserted := Source{Kind: SourceAsserted, Claim: "admin"}

	if got := PrimarySource([]Source{asserted, global}); got != global {
		t.Errorf("PrimarySource = %+v, want %+v (global outranks asserted)", got, global)
	}
}

func TestPrimarySource_Empty(t *testing.T) {
	t.Parallel()
	got := PrimarySource(nil)
	if got != (Source{}) {
		t.Errorf("PrimarySource(nil) = %+v, want zero Source", got)
	}
	got = PrimarySource([]Source{})
	if got != (Source{}) {
		t.Errorf("PrimarySource([]) = %+v, want zero Source", got)
	}
}

func TestPrimarySource_Single(t *testing.T) {
	t.Parallel()
	only := Source{Kind: SourceLocalViaGroup, Group: "eng", Relation: "editor-of"}
	got := PrimarySource([]Source{only})
	if got != only {
		t.Errorf("PrimarySource([single]) = %+v, want %+v", got, only)
	}
}

func TestPrimarySource_MultiKind_LowestPriorityWins(t *testing.T) {
	t.Parallel()
	// Group < Local < LocalViaGroup — the Group source should win
	// regardless of input order. Multiple paths to the same role is
	// the realistic case (UC5).
	srcs := []Source{
		{Kind: SourceLocalViaGroup, Group: "eng-leads", Relation: "editor-of"},
		{Kind: SourceLocal, Relation: "editor-of"},
		{Kind: SourceGroup, Group: "eng-leads"},
	}
	got := PrimarySource(srcs)
	want := Source{Kind: SourceGroup, Group: "eng-leads"}
	if got != want {
		t.Errorf("PrimarySource(...) = %+v, want %+v", got, want)
	}
}

func TestPrimarySource_TiedKind_AlphaWins(t *testing.T) {
	t.Parallel()
	// Both Group; alpha sort on Group wins. "a" < "b".
	srcs := []Source{
		{Kind: SourceGroup, Group: "b-team"},
		{Kind: SourceGroup, Group: "a-team"},
	}
	got := PrimarySource(srcs)
	if got.Group != "a-team" {
		t.Errorf("PrimarySource: tied Kind → alpha Group, got %q, want %q",
			got.Group, "a-team")
	}
}

func TestPrimarySource_TiedGroup_AlphaRelationWins(t *testing.T) {
	t.Parallel()
	// Same Kind and Group; tied on Ancestor (empty); Relation alpha wins.
	srcs := []Source{
		{Kind: SourceLocalViaGroup, Group: "eng", Relation: "viewer-of"},
		{Kind: SourceLocalViaGroup, Group: "eng", Relation: "editor-of"},
	}
	got := PrimarySource(srcs)
	if got.Relation != "editor-of" {
		t.Errorf("PrimarySource: tied Group → alpha Relation, got %q, want %q",
			got.Relation, "editor-of")
	}
}

func TestPrimarySource_NonMutating(t *testing.T) {
	t.Parallel()
	// PrimarySource must not reorder the caller's slice; callers may
	// keep the full attribution chain for audit purposes.
	srcs := []Source{
		{Kind: SourceLocalViaGroup, Group: "eng-leads", Relation: "editor-of"},
		{Kind: SourceGroup, Group: "eng-leads"},
		{Kind: SourceLocal, Relation: "editor-of"},
	}
	before := append([]Source(nil), srcs...)
	_ = PrimarySource(srcs)
	for i, s := range srcs {
		if s != before[i] {
			t.Errorf("PrimarySource mutated input at index %d: got %+v, want %+v",
				i, s, before[i])
		}
	}
}

func TestRoleAttributionAsMapKey(t *testing.T) {
	t.Parallel()
	// attrKey is a struct, comparable by value. Confirm that two
	// attributions with the same (Role, Source) collide and two with
	// any difference do not — the dedup logic in computeGlobals /
	// computeForEntity depends on this.
	a := attrKey{Role: "editor", Source: Source{Kind: SourceGroup, Group: "eng"}}
	b := attrKey{Role: "editor", Source: Source{Kind: SourceGroup, Group: "eng"}}
	if a != b {
		t.Errorf("equal attrKeys compare as unequal: %+v vs %+v", a, b)
	}
	c := attrKey{Role: "viewer", Source: Source{Kind: SourceGroup, Group: "eng"}}
	if a == c {
		t.Errorf("different-Role attrKeys compare as equal")
	}
	d := attrKey{Role: "editor", Source: Source{Kind: SourceLocal, Relation: "editor-of"}}
	if a == d {
		t.Errorf("different-Source attrKeys compare as equal")
	}
}

// Compile-time check that Source's wire format doesn't accidentally
// emit one of the substring tokens used by another kind. If two kinds'
// string forms share a prefix that prevents structured parsing, a
// future parser would silently misroute.
func TestSourceKindString_UniquePrefixes(t *testing.T) {
	t.Parallel()
	kinds := []SourceKind{
		SourceGlobal, SourceGroup, SourceLocal,
		SourceLocalViaGroup, SourceLocalViaAncestor, SourceLocalViaGroupAndAncestor,
	}
	for i, a := range kinds {
		for j, b := range kinds {
			if i == j {
				continue
			}
			sa, sb := a.String(), b.String()
			// `local` is a prefix of `local-via-group`; the format is
			// disambiguated by what follows. Verify that a longer kind
			// ALWAYS includes a separator after the shorter prefix.
			if strings.HasPrefix(sb, sa) && len(sb) > len(sa) {
				if sb[len(sa)] != '-' && sb[len(sa)] != ':' {
					t.Errorf("kind %q is an ambiguous prefix of %q (no separator)", sa, sb)
				}
			}
		}
	}
}
