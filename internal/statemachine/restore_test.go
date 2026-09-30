package statemachine

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// BUG-KK1UXH: a restore may bring back any value the principal could have
// reached from the entry value by declared edges whose guards it holds.
// `when:` is not evaluated (approved→established carries one, and a nil
// lookup would fail it if it were).
func TestEnforceRestore(t *testing.T) {
	set := mustCompile(t, snapshotMeta())
	approver := fakeGuard{perms: map[string]bool{"approve": true}}
	establisher := fakeGuard{perms: map[string]bool{"establish": true}}
	both := fakeGuard{perms: map[string]bool{"approve": true, "establish": true}}
	tests := []struct {
		name     string
		status   string
		guard    Guard
		wantErr  error
		wantEdge [3]string // from, to, permission of the refusing edge
	}{
		{name: "absent ok", status: ""},
		{name: "entry value ok without any guard", status: "in-review"},
		{name: "one guarded hop held", status: "approved", guard: approver},
		{name: "every hop held, when: skipped", status: "established", guard: both},
		{name: "two hops past the entry", status: "obsolete", guard: both},
		{
			name: "last hop held, earlier hop not", status: "established", guard: establisher,
			wantErr: ErrGuardDenied, wantEdge: [3]string{"in-review", "approved", "approve"},
		},
		{
			name: "earlier hop held, last hop not", status: "established", guard: approver,
			wantErr: ErrGuardDenied, wantEdge: [3]string{"approved", "established", "establish"},
		},
		{
			name: "nil guard fails closed", status: "approved",
			wantErr: ErrGuardDenied, wantEdge: [3]string{"in-review", "approved", "approve"},
		},
		{name: "undeclared value", status: "shredded", guard: inertGuard{}, wantErr: ErrIllegalEntry},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := set.EnforceRestore(context.Background(), ent("SNAP-9", "snapshot", tc.status), tc.guard)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want Is(%v)", err, tc.wantErr)
			}
			var ge *GuardError
			if tc.wantEdge[2] == "" {
				return
			}
			if !errors.As(err, &ge) || [3]string{ge.From, ge.To, ge.Permission} != tc.wantEdge {
				t.Errorf("error = %v, want a GuardError on %v", err, tc.wantEdge)
			}
		})
	}
}

// Holding the guard of the last edge is not enough: an unguarded edge into a
// state is useless when its source is reachable only through an unheld guard,
// or not at all.
func TestEnforceRestore_PathNotLastHop(t *testing.T) {
	cases := []struct {
		name    string
		extra   metamodel.TransitionDef
		status  string
		wantErr error
	}{
		{
			// in-review -approve-> approved -> obsolete (unguarded, added)
			name:    "unguarded last hop after a guarded hop",
			extra:   metamodel.TransitionDef{From: "approved", To: "obsolete"},
			status:  "obsolete",
			wantErr: ErrGuardDenied,
		},
		{
			// scrapped is declared, entered by no edge, and leaves unguarded.
			name:    "unguarded edge from an unreachable state",
			extra:   metamodel.TransitionDef{From: "scrapped", To: "obsolete"},
			status:  "scrapped",
			wantErr: ErrIllegalEntry,
		},
		{
			// in-review -> established is free, established -establish->
			// obsolete is not.
			name:    "unguarded path, guarded last hop",
			extra:   metamodel.TransitionDef{From: "in-review", To: "established"},
			status:  "obsolete",
			wantErr: ErrGuardDenied,
		},
		{
			name:   "an unguarded path to the value itself",
			extra:  metamodel.TransitionDef{From: "in-review", To: "established"},
			status: "established",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meta := snapshotMeta()
			st := meta.Types["snapshot-status"]
			st.Values = append(st.Values, "scrapped")
			st.Transitions = append(st.Transitions, tc.extra)
			meta.Types["snapshot-status"] = st
			set := mustCompile(t, meta)
			err := set.EnforceRestore(context.Background(), ent("SNAP-9", "snapshot", tc.status), fakeGuard{})
			switch {
			case tc.wantErr == nil && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tc.wantErr != nil && !errors.Is(err, tc.wantErr):
				t.Fatalf("error = %v, want Is(%v)", err, tc.wantErr)
			}
		})
	}
}

func TestEnforceRestore_EmptySet_NoOp(t *testing.T) {
	var set *Set
	if err := set.EnforceRestore(context.Background(), ent("A", "snapshot", "approved"), nil); err != nil {
		t.Fatalf("nil set should be a no-op, got %v", err)
	}
}
