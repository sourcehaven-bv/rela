package appbuild

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestTagPermissionGuard pins where the guard takes its subject from: the
// acl.Request on ctx when there is one, else the ctx principal.
func TestTagPermissionGuard(t *testing.T) {
	t.Parallel()
	policy := &acl.Policy{
		Roles:       map[string]acl.RoleDef{"tagger": {Read: []string{"page"}, Permissions: []string{"tag:sync"}}},
		Assignments: map[string]string{"alice": "tagger"},
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("policy must load: %v", err)
	}
	d, err := acl.NewDeclarative(policy, acl.NullGraph{}, acl.NullGraphQueryer{})
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	request := func(user string) *acl.Request {
		r, rerr := d.ForPrincipal(principal.Principal{User: user, Tool: principal.ToolCLI})
		if rerr != nil {
			t.Fatalf("ForPrincipal(%s): %v", user, rerr)
		}
		return r
	}
	stamped := func(user string) func() context.Context {
		return func() context.Context {
			return principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolCLI})
		}
	}
	withRequest := func(base func() context.Context, user string) func() context.Context {
		return func() context.Context { return acl.WithRequest(base(), request(user)) }
	}

	cases := []struct {
		name  string
		guard tagPermissionGuard
		ctx   func() context.Context
		want  bool
	}{
		{"no policy allows", tagPermissionGuard{}, context.Background, true},
		{"stamped holder", tagPermissionGuard{d: d}, stamped("alice"), true},
		{"stamped non-holder", tagPermissionGuard{d: d}, stamped("bob"), false},
		{"unstamped, no request", tagPermissionGuard{d: d}, context.Background, false},
		{"attached request is reused", tagPermissionGuard{d: d},
			withRequest(context.Background, "alice"), true},
		{"attached request wins over the principal", tagPermissionGuard{d: d},
			withRequest(stamped("alice"), "bob"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.guard.HoldsPermission(tc.ctx(), "PAGE-1", "tag:sync"); got != tc.want {
				t.Errorf("HoldsPermission = %v, want %v", got, tc.want)
			}
		})
	}
}
