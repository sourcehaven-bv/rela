package aclmap_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// Conditional (asserted-claim) grants in the who-can report (TKT-RP3X3Q).
//
// The load-bearing property under test is a REPORTING one: an asserted grant
// must never be presented as an everyone grant. `rela acl who-can` is consulted
// post-incident to answer "who could have done this?"; telling an operator that
// everybody holds a role they cannot enumerate is worse than saying nothing.

const conditionalPolicy = `
roles:
  editor:
    read: [incident]
    update: [incident]
  auditor:
    read: [incident]
asserted_role_assignments:
  admin: editor
  compliance: [editor, auditor]
`

func TestWhoCan_ReportsAssertedGrantsSeparately(t *testing.T) {
	w := buildWorld(t, conditionalPolicy, []ent{{"INC-1", "incident"}}, nil)

	got, err := w.eng.WhoCan(context.Background(), acl.VerbUpdate, "INC-1")
	if err != nil {
		t.Fatalf("WhoCan: %v", err)
	}

	// The grant must NOT be reported as an everyone grant.
	if got.Everyone.Granted {
		t.Error("an asserted grant was reported as an everyone grant — this tells " +
			"an operator that every principal holds the role")
	}

	// It must appear in its own section. Both claims map to `editor`, which
	// grants update.
	wantClaims := map[string]bool{"admin": false, "compliance": false}
	for _, c := range got.Conditional {
		if _, ok := wantClaims[c.Claim]; !ok {
			t.Errorf("unexpected conditional claim %q", c.Claim)
			continue
		}
		wantClaims[c.Claim] = true
		if c.Role != "editor" {
			t.Errorf("claim %q → role %q, want editor", c.Claim, c.Role)
		}
	}
	for claim, seen := range wantClaims {
		if !seen {
			t.Errorf("claim %q missing from the conditional section", claim)
		}
	}
}

func TestWhoCan_AssertedGrantsAreNotPrincipals(t *testing.T) {
	w := buildWorld(t, conditionalPolicy, []ent{{"INC-1", "incident"}}, nil)

	got, err := w.eng.WhoCan(context.Background(), acl.VerbUpdate, "INC-1")
	if err != nil {
		t.Fatalf("WhoCan: %v", err)
	}

	// A claim value is not a principal and must never be enumerated as one —
	// it would read as a real, addressable identity that does not exist.
	for _, p := range got.Principals {
		if p.Principal == "admin" || p.Principal == "compliance" {
			t.Errorf("claim %q was enumerated as a principal", p.Principal)
		}
	}
}

func TestWhoCan_AssertedGrantsRespectVerb(t *testing.T) {
	w := buildWorld(t, conditionalPolicy, []ent{{"INC-1", "incident"}}, nil)

	// `auditor` grants read but not update, so the compliance→auditor mapping
	// must appear for read and be absent for update.
	read, err := w.eng.WhoCan(context.Background(), acl.VerbRead, "INC-1")
	if err != nil {
		t.Fatalf("WhoCan(read): %v", err)
	}
	var sawAuditor bool
	for _, c := range read.Conditional {
		if c.Role == "auditor" {
			sawAuditor = true
		}
	}
	if !sawAuditor {
		t.Error("auditor grant missing from the read report")
	}

	update, err := w.eng.WhoCan(context.Background(), acl.VerbUpdate, "INC-1")
	if err != nil {
		t.Fatalf("WhoCan(update): %v", err)
	}
	for _, c := range update.Conditional {
		if c.Role == "auditor" {
			t.Error("auditor reported as granting update, which it does not")
		}
	}
}

func TestWhoCan_UndeclaredAssertedRoleNotReported(t *testing.T) {
	// Matches what the resolver does at attribution time: a mapping to a role
	// the policy never declared grants nothing, so reporting it would overstate
	// the deployment's exposure.
	w := buildWorld(t, `
roles:
  editor:
    read: [incident]
    update: [incident]
asserted_role_assignments:
  admin: editor
  ghost: no-such-role
`, []ent{{"INC-1", "incident"}}, nil)

	got, err := w.eng.WhoCan(context.Background(), acl.VerbUpdate, "INC-1")
	if err != nil {
		t.Fatalf("WhoCan: %v", err)
	}
	for _, c := range got.Conditional {
		if c.Claim == "ghost" {
			t.Error("a mapping to an undeclared role was reported as a grant")
		}
	}
}

func TestWhoCan_NoAssertedMappingsOmitsSection(t *testing.T) {
	// The artifact is a diff input: a policy with no asserted mappings must
	// serialize byte-identically to what earlier versions produced.
	w := buildWorld(t, `
roles:
  editor:
    read: [incident]
    update: [incident]
assignments:
  alice: editor
`, []ent{{"INC-1", "incident"}}, nil)

	got, err := w.eng.WhoCan(context.Background(), acl.VerbUpdate, "INC-1")
	if err != nil {
		t.Fatalf("WhoCan: %v", err)
	}
	if got.Conditional != nil {
		t.Errorf("Conditional = %v, want nil", got.Conditional)
	}

	data, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "conditional") {
		t.Errorf("empty conditional section appeared in the wire format: %s", data)
	}
}

func TestWhoCan_AssertedGrantsAreSorted(t *testing.T) {
	// Deterministic ordering: the report feeds a diff artifact.
	w := buildWorld(t, `
roles:
  a-role: {read: [incident]}
  b-role: {read: [incident]}
asserted_role_assignments:
  zeta: [b-role, a-role]
  alpha: b-role
`, []ent{{"INC-1", "incident"}}, nil)

	got, err := w.eng.WhoCan(context.Background(), acl.VerbRead, "INC-1")
	if err != nil {
		t.Fatalf("WhoCan: %v", err)
	}
	if len(got.Conditional) != 3 {
		t.Fatalf("got %d conditional grants, want 3: %+v", len(got.Conditional), got.Conditional)
	}
	for i := 1; i < len(got.Conditional); i++ {
		prev, cur := got.Conditional[i-1], got.Conditional[i]
		if prev.Claim > cur.Claim || (prev.Claim == cur.Claim && prev.Role > cur.Role) {
			t.Errorf("not sorted at %d: %+v then %+v", i, prev, cur)
		}
	}
}
