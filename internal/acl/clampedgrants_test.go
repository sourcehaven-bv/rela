package acl

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// The clamped forms of EveryoneGrants and AssertedGrants (TKT-J3RDJ4). The
// policy-wide forms answer for the policy and must stay unclamped; the Request
// forms answer for one principal and must agree with the ceiling the runtime
// applies.

const clampedGrantsPolicy = `
roles:
  everyone: { read: [project, ticket] }
  editor:   { read: ["*"], update: [ticket, incident] }
asserted_role_assignments:
  staff: editor
client_baselines:
  apps:
    applies_to: [app]
    read: [ticket]
    deny_write: [incident]
`

func TestClampedEveryoneGrants(t *testing.T) {
	t.Parallel()
	d := mustDeclarative(t, clampedGrantsPolicy)
	app, err := OpenRequest(t.Context(), d, verifiedClient("u1", "app"))
	if err != nil {
		t.Fatalf("OpenRequest: %v", err)
	}
	plain := mustRequest(t, d, "u1")

	for _, tc := range []struct {
		name       string
		req        *Request
		typ        string
		wantGrant  bool
		policyWide bool
	}{
		{"unattenuated keeps project", plain, "project", true, true},
		{"ceiling removes project", app, "project", false, true},
		{"ceiling keeps ticket", app, "ticket", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ClampedEveryoneGrants(tc.req, VerbRead, tc.typ).Granted; got != tc.wantGrant {
				t.Errorf("ClampedEveryoneGrants(read, %s) = %v, want %v", tc.typ, got, tc.wantGrant)
			}
			if got := d.EveryoneGrants(VerbRead, tc.typ).Granted; got != tc.policyWide {
				t.Errorf("policy-wide EveryoneGrants(read, %s) = %v, want %v", tc.typ, got, tc.policyWide)
			}
		})
	}
}

func TestClampedAssertedGrants(t *testing.T) {
	t.Parallel()
	d := mustDeclarative(t, clampedGrantsPolicy)
	app, err := OpenRequest(t.Context(), d, principal.VerifiedFrom("u1", principal.ToolDataEntry,
		principal.Claims{PrincipalType: "app"}))
	if err != nil {
		t.Fatalf("OpenRequest: %v", err)
	}
	plain := mustRequest(t, d, "u1")

	if got := ClampedAssertedGrants(plain, VerbUpdate, "incident"); len(got) != 1 || got[0].Claim != "staff" {
		t.Errorf("unattenuated asserted update on incident = %+v, want staff → editor", got)
	}
	if got := ClampedAssertedGrants(app, VerbUpdate, "incident"); len(got) != 0 {
		t.Errorf("asserted update on incident through deny_write = %+v, want none", got)
	}
	if got := ClampedAssertedGrants(app, VerbUpdate, "ticket"); len(got) != 1 {
		t.Errorf("asserted update on ticket = %+v, want staff → editor", got)
	}
	if got := ClampedAssertedGrants(app, VerbRead, "incident"); len(got) != 0 {
		t.Errorf("asserted read on incident outside the read allowlist = %+v, want none", got)
	}
}
