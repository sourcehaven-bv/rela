package statemachine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// snapshotStatus is the running example: a four-state lifecycle with guards and
// one precondition, referenced by the snapshot entity type's `status` property.
func snapshotMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"snapshot-status": {
				Values:  []string{"in-review", "approved", "established", "obsolete"},
				Initial: "in-review",
				Transitions: []metamodel.TransitionDef{
					{From: "in-review", To: "approved", Guard: "approve"},
					{From: "approved", To: "established", Guard: "establish", When: `count_relations(entity, "signed-by") > 0`},
					{From: "approved", To: "in-review", Guard: "approve"},
					{From: "established", To: "obsolete", Guard: "establish"},
				},
			},
		},
		Entities: map[string]metamodel.EntityDef{
			"snapshot": {Properties: map[string]metamodel.PropertyDef{
				"status": {Type: "snapshot-status"},
			}},
		},
	}
}

func ent(id, typ, status string) *entity.Entity {
	e := entity.New(id, typ)
	if status != "" {
		e.SetString("status", status)
	}
	return e
}

// fakeGuard grants a fixed permission set (a nil map grants nothing).
type fakeGuard struct{ perms map[string]bool }

func (g fakeGuard) HoldsPermission(_ context.Context, _, perm string) bool {
	return g.perms[perm]
}

// inertGuard models the direct-CLI/no-policy adapter: it allows everything, so
// guarded edges pass (authorization is meaningless without a principal/policy).
type inertGuard struct{}

func (inertGuard) HoldsPermission(_ context.Context, _, _ string) bool { return true }

// fakeLookup answers OutgoingCounts from a fixed map.
type fakeLookup struct{ counts map[string]int }

func (l fakeLookup) OutgoingCounts(_ context.Context, _ string) map[string]int { return l.counts }

func mustCompile(t *testing.T, m *metamodel.Metamodel) *Set {
	t.Helper()
	set, err := Compile(m)
	if err != nil {
		t.Fatalf("Compile: unexpected error: %v", err)
	}
	return set
}

func TestCompile_WellFormed(t *testing.T) {
	set := mustCompile(t, snapshotMeta())
	if set.Empty() {
		t.Fatal("expected a non-empty set")
	}
	if _, ok := set.machines["snapshot-status"]; !ok {
		t.Fatal("expected snapshot-status machine")
	}
	if got := set.propType["snapshot"]["status"]; got != "snapshot-status" {
		t.Fatalf("propType index = %q, want snapshot-status", got)
	}
}

func TestCompile_Errors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*metamodel.Metamodel)
		wantSub string
	}{
		{
			name: "dangling from",
			mutate: func(m *metamodel.Metamodel) {
				ct := m.Types["snapshot-status"]
				ct.Transitions[0].From = "nope"
				m.Types["snapshot-status"] = ct
			},
			wantSub: `from "nope" is not a declared value`,
		},
		{
			name: "dangling to",
			mutate: func(m *metamodel.Metamodel) {
				ct := m.Types["snapshot-status"]
				ct.Transitions[0].To = "ghost"
				m.Types["snapshot-status"] = ct
			},
			wantSub: `to "ghost" is not a declared value`,
		},
		{
			name: "dangling initial",
			mutate: func(m *metamodel.Metamodel) {
				ct := m.Types["snapshot-status"]
				ct.Initial = "bogus"
				m.Types["snapshot-status"] = ct
			},
			wantSub: `initial "bogus" is not a declared value`,
		},
		{
			name: "duplicate edge",
			mutate: func(m *metamodel.Metamodel) {
				ct := m.Types["snapshot-status"]
				ct.Transitions = append(ct.Transitions, metamodel.TransitionDef{From: "in-review", To: "approved"})
				m.Types["snapshot-status"] = ct
			},
			wantSub: "declared more than once",
		},
		{
			name: "bad when syntax",
			mutate: func(m *metamodel.Metamodel) {
				ct := m.Types["snapshot-status"]
				ct.Transitions[1].When = "this is not valid $$"
				m.Types["snapshot-status"] = ct
			},
			wantSub: "when:",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := snapshotMeta()
			tc.mutate(m)
			_, err := Compile(m)
			if err == nil {
				t.Fatal("expected a compile error")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestCompile_NoTransitions_Empty(t *testing.T) {
	m := &metamodel.Metamodel{
		Types:    map[string]metamodel.CustomType{"plain": {Values: []string{"a", "b"}}},
		Entities: map[string]metamodel.EntityDef{"x": {Properties: map[string]metamodel.PropertyDef{"s": {Type: "plain"}}}},
	}
	set := mustCompile(t, m)
	if !set.Empty() {
		t.Fatal("a metamodel with no transitions should compile to an empty set")
	}
}

func TestCompile_NilMetamodel(t *testing.T) {
	if _, err := Compile(nil); err == nil {
		t.Fatal("expected error on nil metamodel")
	}
}

func TestEnforceUpdate_Legality(t *testing.T) {
	set := mustCompile(t, snapshotMeta())
	ctx := context.Background()
	allow := fakeGuard{perms: map[string]bool{"approve": true, "establish": true}}

	tests := []struct {
		name    string
		from    string
		to      string
		wantErr error
	}{
		{"legal edge", "in-review", "approved", nil},
		{"illegal skip", "in-review", "established", ErrIllegalTransition},
		{"illegal backward", "established", "in-review", ErrIllegalTransition},
		{"no change", "approved", "approved", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			old := ent("SNAP-1", "snapshot", tc.from)
			nw := ent("SNAP-1", "snapshot", tc.to)
			// signed-by present so the approved→established `when:` passes when reached.
			lookup := fakeLookup{counts: map[string]int{"signed-by": 1}}
			err := set.EnforceUpdate(ctx, old, nw, allow, lookup)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want Is(%v)", err, tc.wantErr)
			}
		})
	}
}

func TestEnforceUpdate_Guard(t *testing.T) {
	set := mustCompile(t, snapshotMeta())
	ctx := context.Background()
	lookup := fakeLookup{counts: map[string]int{"signed-by": 1}}

	tests := []struct {
		name    string
		guard   Guard
		wantErr error
	}{
		{"holds permission", fakeGuard{perms: map[string]bool{"approve": true}}, nil},
		{"lacks permission", fakeGuard{perms: map[string]bool{}}, ErrGuardDenied},
		{"nil guard fails closed", nil, ErrGuardDenied},
		{"inert guard (no policy) allows", inertGuard{}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			old := ent("SNAP-1", "snapshot", "in-review")
			nw := ent("SNAP-1", "snapshot", "approved")
			err := set.EnforceUpdate(ctx, old, nw, tc.guard, lookup)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want Is(%v)", err, tc.wantErr)
			}
		})
	}
}

func TestEnforceUpdate_When(t *testing.T) {
	set := mustCompile(t, snapshotMeta())
	ctx := context.Background()
	allow := fakeGuard{perms: map[string]bool{"establish": true}}

	tests := []struct {
		name    string
		counts  map[string]int
		wantErr error
	}{
		{"precondition met", map[string]int{"signed-by": 2}, nil},
		{"precondition fails", map[string]int{"signed-by": 0}, ErrPreconditionFailed},
		{"precondition fails, missing", map[string]int{}, ErrPreconditionFailed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			old := ent("SNAP-1", "snapshot", "approved")
			nw := ent("SNAP-1", "snapshot", "established")
			err := set.EnforceUpdate(ctx, old, nw, allow, fakeLookup{counts: tc.counts})
			if tc.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want Is(%v)", err, tc.wantErr)
			}
		})
	}
}

// Guard is checked before when: a principal lacking the permission gets a 403
// even if the precondition would also fail.
func TestEnforceUpdate_GuardBeforeWhen(t *testing.T) {
	set := mustCompile(t, snapshotMeta())
	old := ent("SNAP-1", "snapshot", "approved")
	nw := ent("SNAP-1", "snapshot", "established")
	err := set.EnforceUpdate(context.Background(), old, nw,
		fakeGuard{perms: map[string]bool{}}, fakeLookup{counts: map[string]int{"signed-by": 0}})
	if !errors.Is(err, ErrGuardDenied) {
		t.Fatalf("expected guard denial to take precedence, got %v", err)
	}
}

func TestEnforceUpdate_NonMachineProperty_NoOp(t *testing.T) {
	set := mustCompile(t, snapshotMeta())
	// An entity type with no machine property.
	old := ent("X-1", "other", "")
	nw := ent("X-1", "other", "")
	nw.SetString("title", "changed")
	if err := set.EnforceUpdate(context.Background(), old, nw, nil, nil); err != nil {
		t.Fatalf("non-machine write should be a no-op, got %v", err)
	}
}

func TestEnforceUpdate_EmptySet_NoOp(t *testing.T) {
	var set *Set
	if err := set.EnforceUpdate(context.Background(), nil, ent("A", "snapshot", "approved"), nil, nil); err != nil {
		t.Fatalf("nil set should be a no-op, got %v", err)
	}
}

func TestEnforceCreate_Entry(t *testing.T) {
	set := mustCompile(t, snapshotMeta())
	tests := []struct {
		name    string
		status  string
		wantErr error
	}{
		{"initial value ok", "in-review", nil},
		{"absent ok", "", nil},
		{"non-initial rejected", "established", ErrIllegalEntry},
		{"approved rejected", "approved", ErrIllegalEntry},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := set.EnforceCreate(context.Background(), ent("SNAP-9", "snapshot", tc.status))
			if tc.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want Is(%v)", err, tc.wantErr)
			}
		})
	}
}
