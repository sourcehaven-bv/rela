package access

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// holderFunc answers HoldsPermission from a fixed set and counts the calls.
type holderFunc struct {
	held  map[string]bool
	calls *int
}

func (h holderFunc) HoldsPermission(_ context.Context, permission string) bool {
	*h.calls++
	return h.held[permission]
}

// otherACL is an implementation ModeOf does not know.
type otherACL struct{}

func (otherACL) AuthorizeWrite(context.Context, acl.WriteRequest) acl.Decision { return acl.Decision{} }

func mustDeclarative(t *testing.T) *acl.Declarative {
	t.Helper()
	d, err := acl.NewDeclarative(&acl.Policy{}, acl.NullGraph{}, acl.NullGraphQueryer{})
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	return d
}

// TestModeOf_ClassifiesEveryShape covers the value, pointer and nil-pointer
// forms the replaced type switches matched.
func TestModeOf_ClassifiesEveryShape(t *testing.T) {
	var nilNop *acl.NopACL
	var nilReadOnly *acl.ReadOnlyACL
	var nilPolicy *acl.Declarative
	tests := []struct {
		name string
		acl  acl.ACL
		want Mode
	}{
		{"nil interface", nil, modeUnwired},
		{"NopACL value", acl.NopACL{}, modeNoPolicy},
		{"NopACL pointer", &acl.NopACL{}, modeNoPolicy},
		{"NopACL nil pointer", nilNop, modeNoPolicy},
		{"ReadOnlyACL value", acl.ReadOnlyACL{}, modeReadOnly},
		{"ReadOnlyACL pointer", &acl.ReadOnlyACL{}, modeReadOnly},
		{"ReadOnlyACL nil pointer", nilReadOnly, modeReadOnly},
		{"Declarative", mustDeclarative(t), modePolicy},
		{"Declarative nil pointer", nilPolicy, modeBrokenPolicy},
		{"other implementation", otherACL{}, modeUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ModeOf(tc.acl); got != tc.want {
				t.Errorf("ModeOf = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestMode_Decisions pins every arm of every decision, per mode. The rows are
// the arms of the five type switches this type replaced (TKT-0NODIZ).
func TestMode_Decisions(t *testing.T) {
	const perm = "nav:admin"
	tests := []struct {
		mode Mode
		// gated UI and elevated read when the holder holds perm / does not
		uiHeld, uiNotHeld, elevatedHeld, elevatedNotHeld bool
		reveal, configured, refusesWrites                bool
		// project write when the holder holds perm / does not
		projectHeld, projectNotHeld bool
	}{
		{modeUnknown, false, false, false, false, true, false, false, false, false},
		{modeUnwired, false, false, false, false, false, false, false, false, false},
		{modeNoPolicy, true, true, false, false, false, false, false, true, true},
		{modeReadOnly, true, true, false, false, false, false, true, false, false},
		{modePolicy, true, false, true, false, true, true, false, true, false},
		{modeBrokenPolicy, false, false, false, false, true, true, false, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.mode.String(), func(t *testing.T) {
			ctx := context.Background()
			calls := 0
			held := holderFunc{held: map[string]bool{perm: true}, calls: &calls}
			notHeld := holderFunc{held: map[string]bool{}, calls: &calls}

			check := func(what string, got, want bool) {
				t.Helper()
				if got != want {
					t.Errorf("%s = %v, want %v", what, got, want)
				}
			}
			check("PermitsGatedUIElement(held)", tc.mode.PermitsGatedUIElement(ctx, held, perm), tc.uiHeld)
			check("PermitsGatedUIElement(not held)", tc.mode.PermitsGatedUIElement(ctx, notHeld, perm), tc.uiNotHeld)
			check("PermitsElevatedRead(held)", tc.mode.PermitsElevatedRead(ctx, held, perm), tc.elevatedHeld)
			check("PermitsElevatedRead(not held)",
				tc.mode.PermitsElevatedRead(ctx, notHeld, perm), tc.elevatedNotHeld)
			check("RevealIsPrivileged", tc.mode.RevealIsPrivileged(), tc.reveal)
			check("PolicyConfigured", tc.mode.PolicyConfigured(), tc.configured)
			check("RefusesWrites", tc.mode.RefusesWrites(), tc.refusesWrites)
			check("PermitsProjectWrite(held)", tc.mode.PermitsProjectWrite(ctx, held, perm), tc.projectHeld)
			check("PermitsProjectWrite(not held)",
				tc.mode.PermitsProjectWrite(ctx, notHeld, perm), tc.projectNotHeld)

			// Only a configured policy may consult the holder: under the other
			// modes the fallback read gate grants every permission.
			wantCalls := 0
			if tc.mode == modePolicy {
				wantCalls = 6
			}
			if calls != wantCalls {
				t.Errorf("holder consulted %d times, want %d", calls, wantCalls)
			}
		})
	}
}

// TestMode_EmptyPermission: an element without a permission is always shown,
// and an elevated document without one is never rendered, in every mode.
func TestMode_EmptyPermission(t *testing.T) {
	calls := 0
	holder := holderFunc{held: map[string]bool{"": true}, calls: &calls}
	for _, m := range []Mode{modeUnknown, modeUnwired, modeNoPolicy, modeReadOnly, modePolicy, modeBrokenPolicy} {
		t.Run(m.String(), func(t *testing.T) {
			if !m.PermitsGatedUIElement(context.Background(), holder, "") {
				t.Error("PermitsGatedUIElement(\"\") = false, want true")
			}
			if m.PermitsElevatedRead(context.Background(), holder, "") {
				t.Error("PermitsElevatedRead(\"\") = true, want false")
			}
		})
	}
	if calls != 0 {
		t.Errorf("holder consulted %d times for an empty permission, want 0", calls)
	}
}

// TestMode_ZeroValueIsUnknown: a Mode nobody computed takes the unknown arms.
func TestMode_ZeroValueIsUnknown(t *testing.T) {
	var m Mode
	if m != modeUnknown {
		t.Fatalf("zero Mode = %v, want unknown", m)
	}
}
