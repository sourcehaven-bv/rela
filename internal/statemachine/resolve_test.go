package statemachine

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Performable resolves outgoing transitions from the entity's current status,
// evaluating guard + when: for the (principal, entity), sorted by To.
func TestPerformable_ShapeAndGates(t *testing.T) {
	set := mustCompile(t, snapshotMeta())
	ctx := context.Background()
	// approved has two out-edges in snapshotMeta: →established (guard establish,
	// when count_relations(signed-by)>0) and →in-review (guard approve).
	e := ent("SNAP-1", "snapshot", "approved")

	tests := []struct {
		name   string
		guard  Guard
		counts map[string]int
		want   map[string]TransitionVerdict // keyed by To
	}{
		{
			name:   "all held and met",
			guard:  fakeGuard{perms: map[string]bool{"establish": true, "approve": true}},
			counts: map[string]int{"signed-by": 1},
			want: map[string]TransitionVerdict{
				"established": {To: "established", Guard: "establish", Allowed: true, Reason: VerdictAllowed},
				"in-review":   {To: "in-review", Guard: "approve", Allowed: true, Reason: VerdictAllowed},
			},
		},
		{
			name:   "guard denied on establish, approve ok",
			guard:  fakeGuard{perms: map[string]bool{"approve": true}},
			counts: map[string]int{"signed-by": 1},
			want: map[string]TransitionVerdict{
				"established": {To: "established", Guard: "establish", Allowed: false, Reason: VerdictGuard},
				"in-review":   {To: "in-review", Guard: "approve", Allowed: true, Reason: VerdictAllowed},
			},
		},
		{
			name:   "guard held but precondition unmet on establish",
			guard:  fakeGuard{perms: map[string]bool{"establish": true, "approve": true}},
			counts: map[string]int{"signed-by": 0},
			want: map[string]TransitionVerdict{
				"established": {To: "established", Guard: "establish", Allowed: false, Reason: VerdictPrecondition},
				"in-review":   {To: "in-review", Guard: "approve", Allowed: true, Reason: VerdictAllowed},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := set.Performable(ctx, e, "status", tc.guard, fakeLookup{counts: tc.counts})
			if len(got) != len(tc.want) {
				t.Fatalf("got %d verdicts, want %d: %+v", len(got), len(tc.want), got)
			}
			// Sorted by To.
			for i := 1; i < len(got); i++ {
				if got[i-1].To > got[i].To {
					t.Fatalf("verdicts not sorted by To: %+v", got)
				}
			}
			for _, v := range got {
				want, ok := tc.want[v.To]
				if !ok {
					t.Fatalf("unexpected verdict for %q", v.To)
				}
				if v != want {
					t.Errorf("verdict %q = %+v, want %+v", v.To, v, want)
				}
			}
		})
	}
}

func TestPerformable_NilCases(t *testing.T) {
	set := mustCompile(t, snapshotMeta())
	ctx := context.Background()
	allow := fakeGuard{perms: map[string]bool{"establish": true, "approve": true}}

	t.Run("non-machine property", func(t *testing.T) {
		if got := set.Performable(ctx, ent("SNAP-1", "snapshot", "approved"), "title", allow, fakeLookup{}); got != nil {
			t.Fatalf("non-machine prop should be nil, got %+v", got)
		}
	})
	t.Run("terminal state has no out-edges", func(t *testing.T) {
		if got := set.Performable(ctx, ent("SNAP-1", "snapshot", "obsolete"), "status", allow, fakeLookup{}); got != nil {
			t.Fatalf("terminal state should be nil, got %+v", got)
		}
	})
	t.Run("nil set", func(t *testing.T) {
		var s *Set
		if got := s.Performable(ctx, ent("A", "snapshot", "approved"), "status", allow, fakeLookup{}); got != nil {
			t.Fatalf("nil set should be nil, got %+v", got)
		}
	})
	t.Run("nil entity", func(t *testing.T) {
		if got := set.Performable(ctx, nil, "status", allow, fakeLookup{}); got != nil {
			t.Fatalf("nil entity should be nil, got %+v", got)
		}
	})
}

// driftMeta is purpose-built to EXPOSE read/write divergence, not hide it: it
// has (a) a `when:` that reads entity.value (must see the TARGET value on both
// paths — RR against critical#1), and (b) a self-loop edge (a no-op on write;
// must not be reported performable on read — critical#2). If Performable
// evaluated against the pre-transition entity, or reported self-loops, the
// parity assertion below would fail.
func driftMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"s": {
				Values:  []string{"a", "b", "c"},
				Initial: "a",
				Transitions: []metamodel.TransitionDef{
					// value-dependent precondition: only legal when the NEW value is "b".
					{From: "a", To: "b", When: `entity.value == "b"`},
					// a plain edge and a guarded edge.
					{From: "a", To: "c", Guard: "g"},
					// self-loop: no-op on write, must not be "performable".
					{From: "a", To: "a"},
				},
			},
		},
		Entities: map[string]metamodel.EntityDef{
			"x": {Properties: map[string]metamodel.PropertyDef{"s": {Type: "s"}}},
		},
	}
}

// TestPerformable_MatchesEnforceUpdate is the drift guard (AC4): for the same
// (entity, target, guard, graph), Performable's Allowed verdict MUST agree with
// whether EnforceUpdate accepts the write. Uses BOTH snapshotMeta (graph
// precondition) and driftMeta (value-dependent precondition + self-loop) so the
// two divergence classes the reviewer found can never regress.
func TestPerformable_MatchesEnforceUpdate(t *testing.T) {
	ctx := context.Background()

	type probe struct {
		meta   *metamodel.Metamodel
		etype  string
		prop   string
		from   string
		guard  Guard
		lookup GraphLookup
		// targets to also enforce-check even if Performable omits them (e.g.
		// self-loops, which Performable must NOT list but which are no-ops on
		// write) — asserts the omission is correct.
		omittedTargets []string
	}
	probes := []struct {
		name string
		p    probe
	}{
		{"snapshot allow", probe{snapshotMeta(), "snapshot", "status", "approved",
			fakeGuard{perms: map[string]bool{"establish": true, "approve": true}}, fakeLookup{counts: map[string]int{"signed-by": 1}}, nil}},
		{"snapshot guard-denied", probe{snapshotMeta(), "snapshot", "status", "approved",
			fakeGuard{perms: map[string]bool{}}, fakeLookup{counts: map[string]int{"signed-by": 1}}, nil}},
		{"snapshot precond-unmet", probe{snapshotMeta(), "snapshot", "status", "approved",
			fakeGuard{perms: map[string]bool{"establish": true, "approve": true}}, fakeLookup{counts: map[string]int{"signed-by": 0}}, nil}},
		// driftMeta: entity.value precondition (a→b legal because target is "b"),
		// a→c guarded, a→a self-loop omitted by read but a no-op on write.
		{"drift value-precond + guard", probe{driftMeta(), "x", "s", "a",
			fakeGuard{perms: map[string]bool{"g": true}}, fakeLookup{}, []string{"a"}}},
		{"drift guard-denied", probe{driftMeta(), "x", "s", "a",
			fakeGuard{perms: map[string]bool{}}, fakeLookup{}, []string{"a"}}},
	}
	for _, tc := range probes {
		t.Run(tc.name, func(t *testing.T) {
			set := mustCompile(t, tc.p.meta)
			from := ent("E-1", tc.p.etype, "")
			from.SetString(tc.p.prop, tc.p.from)
			verdicts := set.Performable(ctx, from, tc.p.prop, tc.p.guard, tc.p.lookup)

			assertParity := func(to string, allowed bool, reason VerdictGate) {
				updated := from.Clone()
				updated.SetString(tc.p.prop, to)
				err := set.EnforceUpdate(ctx, from, updated, tc.p.guard, tc.p.lookup)
				if (err == nil) != allowed {
					t.Errorf("drift on %s→%s: Performable.Allowed=%v but EnforceUpdate allowed=%v (err=%v)",
						tc.p.from, to, allowed, err == nil, err)
				}
				switch reason {
				case VerdictAllowed:
					// already covered by the (err == nil) == allowed assertion
				case VerdictGuard:
					if !errors.Is(err, ErrGuardDenied) {
						t.Errorf("%s: verdict says guard but EnforceUpdate err=%v", to, err)
					}
				case VerdictPrecondition:
					if !errors.Is(err, ErrPreconditionFailed) {
						t.Errorf("%s: verdict says precondition but EnforceUpdate err=%v", to, err)
					}
				}
			}

			for _, v := range verdicts {
				assertParity(v.To, v.Allowed, v.Reason)
			}
			// A self-loop must be omitted from verdicts AND be a no-op (allowed)
			// on write — assert both, so "read omits it" stays correct.
			for _, to := range tc.p.omittedTargets {
				for _, v := range verdicts {
					if v.To == to {
						t.Errorf("self-loop %s→%s must not be reported performable, got %+v", tc.p.from, to, v)
					}
				}
				updated := from.Clone()
				updated.SetString(tc.p.prop, to)
				if err := set.EnforceUpdate(ctx, from, updated, tc.p.guard, tc.p.lookup); err != nil {
					t.Errorf("self-loop %s→%s should be a no-op on write, got %v", tc.p.from, to, err)
				}
			}
		})
	}
}
