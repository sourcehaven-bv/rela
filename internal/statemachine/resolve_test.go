package statemachine

import (
	"context"
	"errors"
	"testing"
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

// TestPerformable_MatchesEnforceUpdate is the drift guard (AC4): for the same
// (entity, target, guard, graph), Performable's Allowed verdict MUST agree with
// whether EnforceUpdate accepts the write. Read and write share evalEdge, so a
// divergence here means someone broke that sharing.
func TestPerformable_MatchesEnforceUpdate(t *testing.T) {
	set := mustCompile(t, snapshotMeta())
	ctx := context.Background()

	scenarios := []struct {
		name   string
		guard  Guard
		counts map[string]int
	}{
		{"all held+met", fakeGuard{perms: map[string]bool{"establish": true, "approve": true}}, map[string]int{"signed-by": 1}},
		{"guard denied", fakeGuard{perms: map[string]bool{}}, map[string]int{"signed-by": 1}},
		{"precondition unmet", fakeGuard{perms: map[string]bool{"establish": true, "approve": true}}, map[string]int{"signed-by": 0}},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			from := ent("SNAP-1", "snapshot", "approved")
			lookup := fakeLookup{counts: sc.counts}
			verdicts := set.Performable(ctx, from, "status", sc.guard, lookup)

			for _, v := range verdicts {
				// Enforce the same transition and check the accept/reject
				// matches the verdict's Allowed.
				updated := ent("SNAP-1", "snapshot", v.To)
				err := set.EnforceUpdate(ctx, from, updated, sc.guard, lookup)
				enforceAllowed := err == nil
				if enforceAllowed != v.Allowed {
					t.Errorf("drift on %q→%q: Performable.Allowed=%v but EnforceUpdate allowed=%v (err=%v)",
						"approved", v.To, v.Allowed, enforceAllowed, err)
				}
				// The gate the verdict names must match the error kind.
				switch v.Reason {
				case VerdictAllowed:
					// Allowed verdict → EnforceUpdate must have succeeded
					// (already asserted above via enforceAllowed).
				case VerdictGuard:
					if !errors.Is(err, ErrGuardDenied) {
						t.Errorf("%q: verdict says guard but EnforceUpdate err=%v", v.To, err)
					}
				case VerdictPrecondition:
					if !errors.Is(err, ErrPreconditionFailed) {
						t.Errorf("%q: verdict says precondition but EnforceUpdate err=%v", v.To, err)
					}
				}
			}
		})
	}
}
