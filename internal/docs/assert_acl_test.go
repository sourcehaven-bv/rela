package docs

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// aclFixturePolicy binds two users to roles so an authorization claim in a
// manual has something real to resolve against: `ed` may write, `vi` may only
// read.
func aclFixturePolicy() *acl.Policy {
	return &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"editor": {Read: []string{"*"}, Create: []string{"*"}, Update: []string{"*"}, Delete: []string{"*"}},
			"viewer": {Read: []string{"*"}},
		},
		Assignments: map[string]string{"ed": "editor", "vi": "viewer"},
	}
}

func TestAuthzIsland(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		policy  *acl.Policy
		wantErr string
	}{
		{
			name: "a true refusal builds",
			body: `refuses{who="vi", op="update", type="risico"}`,
		},
		{
			name: "a true permission builds",
			body: `permits{who="ed", op="update", type="risico"}`,
		},
		{
			// The claim a policy manual most needs to not rot: if someone
			// widens `viewer` to include writes, this manual stops building.
			name:    "a refusal that is actually permitted fails",
			body:    `refuses{who="ed", op="update", type="risico"}`,
			wantErr: "claimed: refused\n  actual:  PERMITTED",
		},
		{
			name:    "a permission that is actually refused fails and names the rule",
			body:    `permits{who="vi", op="delete", type="risico"}`,
			wantErr: "claimed: permitted\n  actual:  REFUSED",
		},
		{
			name:    "missing who is refused",
			body:    `refuses{op="update", type="risico"}`,
			wantErr: "`who`, `op` and `type` are all required",
		},
		{
			name:    "an unknown op is refused rather than silently denying",
			body:    `refuses{who="vi", op="frobnicate", type="risico"}`,
			wantErr: "unknown op",
		},
		{
			// Without this, a manual written against a project with no acl.yaml
			// would assert "refused" and pass for the wrong reason.
			name:    "a claim with no policy at all is refused",
			body:    `refuses{who="vi", op="update", type="risico"}`,
			policy:  nil,
			wantErr: "no acl.yaml",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pol := tc.policy
			if pol == nil && !strings.Contains(tc.wantErr, "no acl.yaml") {
				pol = aclFixturePolicy()
			}
			src := "```rela\n" + tc.body + "\n```\n"
			_, err := Build(context.Background(), src, Options{Meta: fixtureMeta(t), Policy: pol})
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want build to succeed, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want failure containing %q, got success", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error does not contain %q:\n%v", tc.wantErr, err)
			}
		})
	}
}

// TestCheckAuthzBecause covers the `because=` claim without an evaluator: a
// decision that is right for the wrong reason must not pass.
func TestCheckAuthzBecause(t *testing.T) {
	deny := acl.Decision{Allow: false, RuleKind: "role-grant", RuleID: "viewer", Reason: "role viewer may not update risico"}

	t.Run("a matching reason passes", func(t *testing.T) {
		if msg := checkAuthz("refuses", "vi", "update", "risico", false, "role-grant", deny); msg != "" {
			t.Fatalf("want pass, got: %s", msg)
		}
	})
	t.Run("a reason matching the prose passes", func(t *testing.T) {
		if msg := checkAuthz("refuses", "vi", "update", "risico", false, "may not update", deny); msg != "" {
			t.Fatalf("want pass, got: %s", msg)
		}
	})
	t.Run("a wrong reason fails even though the decision was right", func(t *testing.T) {
		msg := checkAuthz("refuses", "vi", "update", "risico", false, "unmatched-principal", deny)
		if msg == "" {
			t.Fatal("want failure: the deny came from a different rule than claimed")
		}
		for _, want := range []string{"right but the", "claimed because: unmatched-principal", "role-grant/viewer"} {
			if !strings.Contains(msg, want) {
				t.Errorf("message missing %q:\n%s", want, msg)
			}
		}
	})
	t.Run("no because claim means the rule is not checked", func(t *testing.T) {
		if msg := checkAuthz("refuses", "vi", "update", "risico", false, "", deny); msg != "" {
			t.Fatalf("want pass, got: %s", msg)
		}
	})
}
