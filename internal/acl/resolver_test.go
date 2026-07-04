package acl

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
)

// RR-AROE: acl.DepthCap and graphquerynaive.DepthCap MUST stay in
// lockstep. The resolver passes its cap into a store.GraphQuery
// Depth/EntityDepth field; the naive backend then caps again as a
// backstop. Drift would silently change semantics — fewer/more
// ancestors materialized on the read path than the write path expects.
//
// The constants live in two packages because making graphquerynaive
// import acl would create an arch-lint exception (acl is a domain
// package; graphquerynaive is a generic store helper). The test below
// is the structural pin.
func TestDepthCap_LockstepWithGraphquerynaive(t *testing.T) {
	t.Parallel()
	if DepthCap != graphquerynaive.DepthCap {
		t.Fatalf("acl.DepthCap=%d, graphquerynaive.DepthCap=%d; must be equal", DepthCap, graphquerynaive.DepthCap)
	}
}

// Resolver unit tests. These cover invariants that the feature tests
// in features_test.go deliberately don't pin:
//
//   - Walk termination on cycles, self-loops, and the depth-cap boundary
//   - Globals memoization (call-counter discipline)
//   - Graph error propagation
//   - Unstamped-principal handling
//
// The tests drive the resolver directly via package-internal access
// rather than going through the feature-test World; they use a
// hand-rolled fakeGraph to control exactly which edges exist and to
// count calls.

// ---- Fake graph ---------------------------------------------------------

// fakeGraph is a deterministic in-memory graph for resolver tests.
// Edges keyed by (from, relType) → []to. Tracks call counts so tests
// can assert "this walk ran exactly once."
type fakeGraph struct {
	edges map[edgeKey][]string
	err   error // when non-nil, OutgoingRelations returns this error

	outgoingCalls int
	hasEdgeCalls  int
	outgoingByRel map[string]int // OutgoingRelations calls bucketed by relType
}

type edgeKey struct{ from, relType string }

func newFakeGraph() *fakeGraph {
	return &fakeGraph{
		edges:         map[edgeKey][]string{},
		outgoingByRel: map[string]int{},
	}
}

func (g *fakeGraph) add(from, relType, to string) {
	k := edgeKey{from, relType}
	g.edges[k] = append(g.edges[k], to)
}

func (g *fakeGraph) HasEdge(_ context.Context, from, relType, to string) bool {
	g.hasEdgeCalls++
	for _, t := range g.edges[edgeKey{from, relType}] {
		if t == to {
			return true
		}
	}
	return false
}

func (g *fakeGraph) OutgoingRelations(_ context.Context, from, relType string) ([]string, error) {
	g.outgoingCalls++
	g.outgoingByRel[relType]++
	if g.err != nil {
		return nil, g.err
	}
	return slices.Clone(g.edges[edgeKey{from, relType}]), nil
}

// helpers -----------------------------------------------------------------

func newTestDeclarative(t *testing.T, p *Policy, g Graph) *Declarative {
	t.Helper()
	d, err := NewDeclarative(p, g, NullGraphQueryer{})
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	return d
}

func aliceDataEntry() principal.Principal {
	return principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
}

// ---- Unknown principal --------------------------------------------------

func TestForPrincipal_UnstampedRejected(t *testing.T) {
	t.Parallel()
	// Each of the unstamped shapes the resolver must refuse. The
	// audit-log misattribution-visible default that From(ctx) returns
	// is a non-issue here — the resolver is the boundary where the
	// soft default becomes a hard error.
	cases := []struct {
		name string
		p    principal.Principal
	}{
		{"empty user", principal.Principal{Tool: principal.ToolDataEntry}},
		{"empty tool", principal.Principal{User: "alice"}},
		{"unknown user", principal.Principal{User: "unknown", Tool: principal.ToolDataEntry}},
		{"unknown tool", principal.Principal{User: "alice", Tool: "unknown"}},
		{"both unknown", principal.Principal{User: "unknown", Tool: "unknown"}},
		{"whitespace user", principal.Principal{User: "   ", Tool: principal.ToolDataEntry}},
	}
	d := newTestDeclarative(t, &Policy{}, newFakeGraph())
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			req, err := d.ForPrincipal(c.p)
			if req != nil {
				t.Errorf("ForPrincipal(%+v) returned non-nil Request", c.p)
			}
			if !errors.Is(err, ErrUnstampedPrincipal) {
				t.Errorf("ForPrincipal(%+v) error = %v, want errors.Is(_, ErrUnstampedPrincipal)", c.p, err)
			}
		})
	}
}

func TestNewDeclarative_RejectsNil(t *testing.T) {
	t.Parallel()
	// The constructor must reject nil policy, graph, and graphQueryer
	// at construction time — silently producing a half-built
	// Declarative would defer the failure to a downstream symptom
	// that's harder to diagnose. RR-A62O tightens this by asserting
	// the error MESSAGE so a future change that drops one check (or
	// keeps the check but rewords the error) doesn't silently weaken
	// the contract.
	cases := []struct {
		name       string
		policy     *Policy
		graph      Graph
		gq         store.GraphQueryer
		wantSubstr string
	}{
		{"nil policy", nil, NullGraph{}, NullGraphQueryer{}, "policy must be non-nil"},
		{"nil graph", &Policy{}, nil, NullGraphQueryer{}, "graph must be non-nil"},
		{"nil graphQueryer", &Policy{}, NullGraph{}, nil, "graphQueryer must be non-nil"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d, err := NewDeclarative(tc.policy, tc.graph, tc.gq)
			if d != nil {
				t.Errorf("returned non-nil *Declarative")
			}
			if err == nil {
				t.Fatalf("returned nil error; want %q", tc.wantSubstr)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

// ---- Walk termination ---------------------------------------------------

func TestWalkMembers_SelfLoop(t *testing.T) {
	t.Parallel()
	// alice --member-of--> alice. The walk must terminate after one
	// iteration with just {alice}.
	g := newFakeGraph()
	g.add("alice", "member-of", "alice")

	d := newTestDeclarative(t, &Policy{}, g)
	req, err := d.ForPrincipal(aliceDataEntry())
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}

	members := req.walkMembers(context.Background())
	if got := len(members); got != 1 || members[0] != "alice" {
		t.Errorf("walkMembers with self-loop = %v, want [alice]", members)
	}
}

func TestWalkMembers_Cycle(t *testing.T) {
	t.Parallel()
	// A --member-of--> B --member-of--> C --member-of--> A.
	// Starting from A, the walk must reach {A, B, C} and terminate
	// (not loop back to A).
	g := newFakeGraph()
	g.add("A", "member-of", "B")
	g.add("B", "member-of", "C")
	g.add("C", "member-of", "A")

	d := newTestDeclarative(t, &Policy{}, g)
	req, err := d.ForPrincipal(principal.Principal{User: "A", Tool: principal.ToolDataEntry})
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}

	members := req.walkMembers(context.Background())
	got := append([]string(nil), members...)
	slices.Sort(got)
	want := []string{"A", "B", "C"}
	if !slices.Equal(got, want) {
		t.Errorf("walkMembers with cycle = %v, want %v", got, want)
	}
}

func TestWalkMembers_DepthCap_ReachableAtCap(t *testing.T) {
	t.Parallel()
	// A chain of length equal to depthCap. The role assigned at the
	// cap-th hop is reachable.
	g := newFakeGraph()
	chain := []string{"alice"}
	for i := 1; i <= depthCap; i++ {
		next := chainID(i)
		g.add(chain[len(chain)-1], "member-of", next)
		chain = append(chain, next)
	}

	d := newTestDeclarative(t, &Policy{}, g)
	req, err := d.ForPrincipal(aliceDataEntry())
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}

	members := req.walkMembers(context.Background())
	last := chain[len(chain)-1]
	if !slices.Contains(members, last) {
		t.Errorf("walkMembers at cap=%d: last node %q not in result %v",
			depthCap, last, members)
	}
}

func TestWalkMembers_DepthCap_TruncatedBeyondCap(t *testing.T) {
	t.Parallel()
	// Chain longer than depthCap. Nodes beyond the cap must NOT be
	// reached.
	g := newFakeGraph()
	chain := []string{"alice"}
	overshoot := depthCap + 3
	for i := 1; i <= overshoot; i++ {
		next := chainID(i)
		g.add(chain[len(chain)-1], "member-of", next)
		chain = append(chain, next)
	}

	d := newTestDeclarative(t, &Policy{}, g)
	req, err := d.ForPrincipal(aliceDataEntry())
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}

	members := req.walkMembers(context.Background())
	beyondCap := chain[depthCap+1]
	if slices.Contains(members, beyondCap) {
		t.Errorf("walkMembers at depth %d: node %q (beyond cap=%d) leaked into result %v",
			overshoot, beyondCap, depthCap, members)
	}
}

func TestWalkMembers_GraphError_AbortsWalk(t *testing.T) {
	t.Parallel()
	// When OutgoingRelations errors, the walk must abort rather than
	// silently undercount or proceed with partial data. The result is
	// the order accumulated so far — at minimum the principal itself.
	g := newFakeGraph()
	g.err = errors.New("backend failure")

	d := newTestDeclarative(t, &Policy{}, g)
	req, err := d.ForPrincipal(aliceDataEntry())
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}

	members := req.walkMembers(context.Background())
	// Walk aborts: at least we got the principal. We must not loop
	// indefinitely on the failure.
	if !slices.Contains(members, "alice") {
		t.Errorf("walkMembers under error: principal not in result %v", members)
	}
	// One call attempted, then abort. (More precise: we don't pin the
	// exact count; we pin that it terminated.)
	if g.outgoingCalls == 0 {
		t.Errorf("walkMembers under error: graph was not called at all")
	}
}

// ---- Globals memoization ------------------------------------------------

func TestRequest_GlobalsMemoized(t *testing.T) {
	t.Parallel()
	// Two calls to Globals on the same Request must share the cached
	// result — the second call performs zero graph traffic.
	g := newFakeGraph()
	g.add("alice", "member-of", "engineering")

	d := newTestDeclarative(t, &Policy{
		Roles:       map[string]RoleDef{"editor": {Create: []string{"ticket"}, Update: []string{"ticket"}, Delete: []string{"ticket"}}},
		Assignments: map[string]string{"engineering": "editor"},
	}, g)

	req, err := d.ForPrincipal(aliceDataEntry())
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}

	ctx := context.Background()
	_ = req.Globals(ctx)
	callsAfterFirst := g.outgoingCalls
	_ = req.Globals(ctx)
	callsAfterSecond := g.outgoingCalls

	if callsAfterSecond != callsAfterFirst {
		t.Errorf("Globals memoization broken: second call added %d more graph calls (first=%d, second=%d)",
			callsAfterSecond-callsAfterFirst, callsAfterFirst, callsAfterSecond)
	}
}

func TestRequest_ForEntityReusesGlobals(t *testing.T) {
	t.Parallel()
	// ForEntity must not re-walk member-of when Globals is already
	// cached. The architect's S5a no-rewalk discipline — pinned here
	// by the call counter on the fake graph.
	g := newFakeGraph()
	g.add("alice", "member-of", "engineering")
	g.add("alice", "editor-of", "PRJ-foo")

	d := newTestDeclarative(t, &Policy{
		Roles:         map[string]RoleDef{"editor": {Create: []string{"project"}, Update: []string{"project"}, Delete: []string{"project"}}},
		RoleRelations: map[string]RoleRelationDef{"editor-of": {Confers: "editor"}},
	}, g)

	req, err := d.ForPrincipal(aliceDataEntry())
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}

	ctx := context.Background()
	_ = req.Globals(ctx)
	walksAfterGlobals := g.outgoingByRel["member-of"]
	_ = req.ForEntity(ctx, "project", "PRJ-foo")
	walksAfterForEntity := g.outgoingByRel["member-of"]

	if walksAfterForEntity != walksAfterGlobals {
		t.Errorf("ForEntity re-walked member-of: globals=%d, after ForEntity=%d",
			walksAfterGlobals, walksAfterForEntity)
	}
}

// RR-MBK0: ForEntity's local-role attribution iteration over
// RoleRelations is a map iteration — Go intentionally randomizes it.
// Without the sort fix, the resulting Attributions slice (and the
// formatDeniedSummary string it feeds) would vary across runs. We pin
// the contract by running ForEntity many times against a scenario
// with multiple role-relations all matching, and asserting that the
// (role, source) sequence is byte-identical every iteration.
func TestRequest_ForEntity_AttributionsDeterministic(t *testing.T) {
	t.Parallel()
	g := newFakeGraph()
	// alice is a direct member of two groups so member-of is multi-edge.
	g.add("alice", "member-of", "engineering")
	g.add("alice", "member-of", "reviewers")
	// Two distinct role-relations, both granting from a group:
	g.add("engineering", "editor-of", "PRJ-foo")
	g.add("reviewers", "reviewer-of", "PRJ-foo")

	d := newTestDeclarative(t, &Policy{
		Roles: map[string]RoleDef{
			"editor":   {Create: []string{"project"}, Update: []string{"project"}, Delete: []string{"project"}},
			"reviewer": {Read: []string{"project"}},
		},
		RoleRelations: map[string]RoleRelationDef{
			"editor-of":   {Confers: "editor"},
			"reviewer-of": {Confers: "reviewer"},
		},
	}, g)

	ctx := context.Background()
	var first []RoleAttribution
	for i := range 50 {
		req, err := d.ForPrincipal(aliceDataEntry())
		if err != nil {
			t.Fatalf("ForPrincipal: %v", err)
		}
		attrs := req.ForEntity(ctx, "project", "PRJ-foo")
		if i == 0 {
			first = attrs
			continue
		}
		if len(attrs) != len(first) {
			t.Fatalf("iteration %d: len(Attributions)=%d, want %d", i, len(attrs), len(first))
		}
		for j := range attrs {
			if attrs[j] != first[j] {
				t.Fatalf("iteration %d, index %d: got %+v, want %+v (first run)",
					i, j, attrs[j], first[j])
			}
		}
	}
}

// ---- Configurable membership relation (TKT-Z8A62F) ----------------------
//
// The resolver walks Policy.MembershipRelation (default "member-of") for
// group membership. These tests pin: a configured relation is followed
// (and attributed as a group source), the default relation is NOT
// followed when a non-default is configured, the default applies when
// the field is blank, and transitivity holds under a renamed relation.

// hasAttribution reports whether attrs contains the given (role, source)
// attribution.
func hasAttribution(attrs []RoleAttribution, want RoleAttribution) bool {
	return slices.Contains(attrs, want)
}

func TestMembershipRelation_Configured_ConfersGroupRole(t *testing.T) {
	t.Parallel()
	// AC1: with membership_relation=heeft_rol, a heeft_rol edge into a
	// group named in assignments confers that group's role, attributed as
	// a SourceGroup.
	g := newFakeGraph()
	g.add("alice", "heeft_rol", "engineering")

	d := newTestDeclarative(t, &Policy{
		MembershipRelation: "heeft_rol",
		Roles:              map[string]RoleDef{"editor": {Create: []string{"ticket"}, Update: []string{"ticket"}, Delete: []string{"ticket"}, Read: []string{"ticket"}}},
		Assignments:        map[string]string{"engineering": "editor"},
	}, g)

	req, err := d.ForPrincipal(aliceDataEntry())
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}

	attrs := req.Globals(context.Background()).Attributions
	want := RoleAttribution{Role: "editor", Source: Source{Kind: SourceGroup, Group: "engineering"}}
	if !hasAttribution(attrs, want) {
		t.Errorf("expected %+v in attributions, got %+v", want, attrs)
	}
	// The walk must have queried heeft_rol, never member-of.
	if n := g.outgoingByRel["heeft_rol"]; n == 0 {
		t.Errorf("expected the walk to query heeft_rol, but it did not (%v)", g.outgoingByRel)
	}
	if n := g.outgoingByRel["member-of"]; n != 0 {
		t.Errorf("walk queried member-of %d times; with a configured relation it must not", n)
	}
}

func TestMembershipRelation_Default_WhenUnset(t *testing.T) {
	t.Parallel()
	// AC2: an unset MembershipRelation falls back to member-of, so a
	// member-of edge still confers the group role.
	g := newFakeGraph()
	g.add("alice", "member-of", "engineering")

	d := newTestDeclarative(t, &Policy{
		Roles:       map[string]RoleDef{"editor": {Create: []string{"ticket"}, Update: []string{"ticket"}, Delete: []string{"ticket"}, Read: []string{"ticket"}}},
		Assignments: map[string]string{"engineering": "editor"},
	}, g)

	req, err := d.ForPrincipal(aliceDataEntry())
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}

	attrs := req.Globals(context.Background()).Attributions
	want := RoleAttribution{Role: "editor", Source: Source{Kind: SourceGroup, Group: "engineering"}}
	if !hasAttribution(attrs, want) {
		t.Errorf("expected %+v via default member-of, got %+v", want, attrs)
	}
}

func TestMembershipRelation_Configured_DoesNotFollowMemberOf(t *testing.T) {
	t.Parallel()
	// AC3 (negative): with membership_relation=heeft_rol, a member-of edge
	// is NOT followed. (The blank → match-all guard is exercised
	// end-to-end in TestMembershipRelation_BlankNeverQueriesMatchAll, not
	// here — this policy is non-blank, so "" could never arise anyway.)
	g := newFakeGraph()
	g.add("alice", "member-of", "engineering")

	d := newTestDeclarative(t, &Policy{
		MembershipRelation: "heeft_rol",
		Roles:              map[string]RoleDef{"editor": {Create: []string{"ticket"}, Update: []string{"ticket"}, Delete: []string{"ticket"}, Read: []string{"ticket"}}},
		Assignments:        map[string]string{"engineering": "editor"},
	}, g)

	req, err := d.ForPrincipal(aliceDataEntry())
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}

	attrs := req.Globals(context.Background()).Attributions
	notWant := RoleAttribution{Role: "editor", Source: Source{Kind: SourceGroup, Group: "engineering"}}
	if hasAttribution(attrs, notWant) {
		t.Errorf("member-of edge must not confer a role when membership_relation=heeft_rol; got %+v", attrs)
	}
	// The walk must have queried heeft_rol (the configured relation), and
	// must not have queried member-of.
	if n := g.outgoingByRel["heeft_rol"]; n == 0 {
		t.Errorf("expected the walk to query heeft_rol, but it did not (%v)", g.outgoingByRel)
	}
	if n := g.outgoingByRel["member-of"]; n != 0 {
		t.Errorf("walk queried member-of %d times; with membership_relation=heeft_rol it must not", n)
	}
}

func TestMembershipRelation_BlankNeverQueriesMatchAll(t *testing.T) {
	t.Parallel()
	// RR-WG5NY1: the end-to-end guard. A blank/whitespace MembershipRelation
	// must drive the *resolver* to walk member-of, never issue a Type==""
	// match-all query. This is the one that would fail if the resolver
	// stopped reading through Policy.membershipRelation(); the accessor unit
	// test (TestPolicy_membershipRelation_EffectiveName) can't catch that.
	for _, blank := range []string{"", "   ", "\t"} {
		t.Run("blank="+strconv.Quote(blank), func(t *testing.T) {
			t.Parallel()
			g := newFakeGraph()
			g.add("alice", "member-of", "engineering")

			d := newTestDeclarative(t, &Policy{
				MembershipRelation: blank,
				Roles:              map[string]RoleDef{"editor": {Read: []string{"ticket"}}},
				Assignments:        map[string]string{"engineering": "editor"},
			}, g)

			req, err := d.ForPrincipal(aliceDataEntry())
			if err != nil {
				t.Fatalf("ForPrincipal: %v", err)
			}

			attrs := req.Globals(context.Background()).Attributions
			want := RoleAttribution{Role: "editor", Source: Source{Kind: SourceGroup, Group: "engineering"}}
			if !hasAttribution(attrs, want) {
				t.Errorf("blank membership relation must walk member-of; expected %+v, got %+v", want, attrs)
			}
			if n := g.outgoingByRel[""]; n != 0 {
				t.Errorf("resolver issued a match-all query (Type==\"\") %d times; a blank relation must never reach the store", n)
			}
			if n := g.outgoingByRel["member-of"]; n == 0 {
				t.Errorf("resolver did not walk member-of for a blank relation (%v)", g.outgoingByRel)
			}
		})
	}
}

func TestMembershipRelation_Configured_Transitive(t *testing.T) {
	t.Parallel()
	// AC4: transitivity holds under a renamed membership relation.
	// A --heeft_rol--> B --heeft_rol--> C, assignments{C: editor} → A editor.
	g := newFakeGraph()
	g.add("A", "heeft_rol", "B")
	g.add("B", "heeft_rol", "C")

	d := newTestDeclarative(t, &Policy{
		MembershipRelation: "heeft_rol",
		Roles:              map[string]RoleDef{"editor": {Read: []string{"ticket"}}},
		Assignments:        map[string]string{"C": "editor"},
	}, g)

	req, err := d.ForPrincipal(principal.Principal{User: "A", Tool: principal.ToolDataEntry})
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}

	attrs := req.Globals(context.Background()).Attributions
	want := RoleAttribution{Role: "editor", Source: Source{Kind: SourceGroup, Group: "C"}}
	if !hasAttribution(attrs, want) {
		t.Errorf("expected %+v via transitive heeft_rol, got %+v", want, attrs)
	}
}

func TestPolicy_membershipRelation_EffectiveName(t *testing.T) {
	t.Parallel()
	// AC6: blank/whitespace collapses to the default; a real value passes
	// through. The accessor is the single guard against a blank relation
	// reaching the store as a Type=="" match-all query.
	cases := []struct {
		name string
		set  string
		want string
	}{
		{"unset", "", "member-of"},
		{"spaces", "   ", "member-of"},
		{"tab", "\t", "member-of"},
		{"newline", "\n", "member-of"},
		{"explicit default", "member-of", "member-of"},
		{"configured", "heeft_rol", "heeft_rol"},
		{"configured with padding", "  heeft_rol  ", "heeft_rol"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &Policy{MembershipRelation: tc.set}
			if got := p.membershipRelation(); got != tc.want {
				t.Errorf("membershipRelation() with %q = %q, want %q", tc.set, got, tc.want)
			}
		})
	}
}

// ---- Helpers ------------------------------------------------------------

// chainID names the i'th node in a synthetic chain used by depth-cap
// tests: g1, g2, ... .
func chainID(i int) string {
	return "g" + strconv.Itoa(i)
}
