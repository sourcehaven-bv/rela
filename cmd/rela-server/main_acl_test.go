package main

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// AC3.3: shouldWarnNoACL returns true exactly when the operator
// needs the "non-loopback without acl.yaml" nudge — i.e. the active
// ACL is NopACL AND --read-only is not set.
func TestShouldWarnNoACL(t *testing.T) {
	tests := []struct {
		name     string
		acl      acl.ACL
		readOnly bool
		want     bool
	}{
		{"nop + no read-only → warn", acl.NopACL{}, false, true},
		{"nop + read-only → silent (read-only is the stronger guarantee)", acl.NopACL{}, true, false},
		{"read-only ACL + no flag → silent (ReadOnlyACL is not NopACL)", acl.ReadOnlyACL{}, false, false},
		{"read-only ACL + flag → silent (both belt and suspenders)", acl.ReadOnlyACL{}, true, false},
		{"declarative ACL + no flag → silent (operator already configured policy)", mustDeclarative(t), false, false},
		{"declarative ACL + flag → silent (flag wins, no need to nag)", mustDeclarative(t), true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldWarnNoACL(tt.acl, tt.readOnly); got != tt.want {
				t.Errorf("shouldWarnNoACL(%T, readOnly=%v) = %v, want %v",
					tt.acl, tt.readOnly, got, tt.want)
			}
		})
	}
}

func mustDeclarative(t *testing.T) acl.ACL {
	t.Helper()
	d, err := acl.NewDeclarative(&acl.Policy{}, acl.NullGraph{})
	if err != nil {
		t.Fatalf("acl.NewDeclarative: %v", err)
	}
	return d
}
