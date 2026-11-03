package appbuild

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestTagPermissionGuard pins where the guard takes its subject from: the
// acl.Request the operation's edge attached, for the principal on ctx. A
// stamped principal with no attached Request, or a Request bound to another
// principal, is denied (fail closed, like acl.Declarative.AuthorizeWrite).
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
	stamped := func(user string) func() context.Context {
		return func() context.Context {
			return principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolCLI})
		}
	}
	bound := func(user string) func() context.Context {
		return func() context.Context { return acltest.Bind(stamped(user)(), t, d) }
	}
	foreign := func(ctxUser, reqUser string) func() context.Context {
		return func() context.Context {
			req := acltest.RequestFor(context.Background(), t, d,
				principal.Principal{User: reqUser, Tool: principal.ToolCLI})
			return acl.WithRequest(stamped(ctxUser)(), req)
		}
	}

	cases := []struct {
		name  string
		guard tagPermissionGuard
		ctx   func() context.Context
		want  bool
	}{
		{"no policy allows", tagPermissionGuard{}, context.Background, true},
		{"bound holder", tagPermissionGuard{d: d}, bound("alice"), true},
		{"bound non-holder", tagPermissionGuard{d: d}, bound("bob"), false},
		{"stamped, no request", tagPermissionGuard{d: d}, stamped("alice"), false},
		{"unstamped, no request", tagPermissionGuard{d: d}, context.Background, false},
		{"request for another principal", tagPermissionGuard{d: d}, foreign("alice", "bob"), false},
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
