package acl

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestRoleGrantsVerbOnFace pins the verb question the affordance resolver asks
// for a role that declares no field block (BUG-Y5UF3K). It must agree with the
// row gate on reads and the write gate on updates, and the client ceiling must
// only ever remove.
func TestRoleGrantsVerbOnFace(t *testing.T) {
	t.Parallel()
	d := mustDeclarative(t, `
roles:
  admin:
    read: ["*"]
    update: ["*"]
  reader:
    read: [ticket, "policy@published"]
  drafter:
    read: [policy]
    update: ["policy@draft"]
  none:
    permissions: [history:read]
client_baselines:
  apps:
    applies_to: [app]
    deny_read: [person]
    deny_update: [ticket]
`)
	published, draft := entity.Face("published"), entity.Face("draft")
	// A Request is single-goroutine, so each parallel subtest opens its own.
	user, app := "user", "app"

	tests := []struct {
		name   string
		client string
		role   string
		verb   Verb
		typ    string
		face   entity.Face
		grants bool
	}{
		{"wildcard read covers a faceless type", user, "admin", VerbRead, "ticket", "", true},
		{"wildcard read covers every face", user, "admin", VerbRead, "policy", draft, true},
		{"wildcard update covers a faceless type", user, "admin", VerbUpdate, "ticket", "", true},
		{"wildcard update does not cover a named face", user, "admin", VerbUpdate, "policy", draft, false},
		{"face read grant covers its face", user, "reader", VerbRead, "policy", published, true},
		{"face read grant does not cover another face", user, "reader", VerbRead, "policy", draft, false},
		{"bare read grant covers the type", user, "reader", VerbRead, "ticket", "", true},
		{"read grant is not update", user, "reader", VerbUpdate, "ticket", "", false},
		{"face update grant covers its face", user, "drafter", VerbUpdate, "policy", draft, true},
		{"face update grant does not cover another face", user, "drafter", VerbUpdate, "policy", published, false},
		{"a role without verbs grants nothing", user, "none", VerbRead, "ticket", "", false},
		{"an undeclared role grants nothing", user, "ghost", VerbRead, "ticket", "", false},
		{"an unknown verb grants nothing", user, "admin", Verb("frobnicate"), "ticket", "", false},
		{"ceiling deny_read removes a wildcard read", app, "admin", VerbRead, "person", "", false},
		{"ceiling leaves other reads alone", app, "admin", VerbRead, "ticket", "", true},
		{"ceiling deny_update removes a wildcard update", app, "admin", VerbUpdate, "ticket", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req, err := d.ForPrincipal(verifiedClient("alice", tc.client))
			if err != nil {
				t.Fatalf("ForPrincipal: %v", err)
			}
			// Asked twice: the second answer comes from the memo.
			for range 2 {
				if got := req.RoleGrantsVerbOnFace(tc.role, tc.verb, tc.typ, tc.face); got != tc.grants {
					t.Errorf("RoleGrantsVerbOnFace(%q, %s, %s, %q) = %v, want %v",
						tc.role, tc.verb, tc.typ, tc.face, got, tc.grants)
				}
			}
		})
	}
}
