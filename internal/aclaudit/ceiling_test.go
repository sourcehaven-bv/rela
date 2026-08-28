package aclaudit

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// Client-attenuation audit rules (TKT-IAC8TX).
//
// Every rule here targets a ceiling that is SILENTLY inert. That is the whole
// justification for auditing them: a broken grant denies something and someone
// complains, but a broken restriction just... doesn't restrict, and nobody
// files a bug about access they still have.

func mustLoad(t *testing.T, body string) *acl.Policy {
	t.Helper()
	p, err := acl.LoadPolicyBytes([]byte(body))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	return p
}

// personMeta declares a person type so the tier-B ceiling checks have a schema
// to cross-check against.
var personMeta = fakeMetamodel{
	types: map[string]bool{"person": true, "ticket": true},
	fields: map[string]map[string][]string{
		"person": {"name": nil, "email": nil, "salary": nil},
		"ticket": {"title": nil, "status": nil},
	},
}

// TestAudit_A11_InertBaseline: a baseline that narrows nothing. The operator
// believes a client is restricted; it holds its user's full access.
func TestAudit_A11_InertBaseline(t *testing.T) {
	t.Parallel()
	p := mustLoad(t, `
client_baselines:
  apps:
    applies_to: [app]
    description: "locked down"
`)
	f := Audit(p, personMeta, allPerms{})
	if !hasRule(f, "A11-inert-client-baseline") {
		t.Fatal("a baseline with no restriction produced no finding")
	}
	// Medium, not Low: this is a believed-but-absent control, not cosmetic drift.
	if got := ruleSeverity(f, "A11-inert-client-baseline"); got != Medium {
		t.Errorf("severity = %v, want Medium", got)
	}
}

func TestAudit_A11_RestrictiveBaseline_NoFinding(t *testing.T) {
	t.Parallel()
	p := mustLoad(t, `
client_baselines:
  apps:
    applies_to: [app]
    redact:
      person: [salary]
`)
	if f := Audit(p, personMeta, allPerms{}); hasRule(f, "A11-inert-client-baseline") {
		t.Error("a baseline with a redact block was reported inert")
	}
}

// TestAudit_A13_EmptyAppliesTo: the restriction is real but selects nothing.
func TestAudit_A13_EmptyAppliesTo(t *testing.T) {
	t.Parallel()
	p := mustLoad(t, `
client_baselines:
  apps:
    applies_to: []
    deny_write: ["*"]
`)
	f := Audit(p, personMeta, allPerms{})
	if !hasRule(f, "A13-baseline-matches-nothing") {
		t.Fatal("a baseline with empty applies_to produced no finding")
	}
}

// TestAudit_A12_ScopeReopensNothing is the subtle one. The scope APPEARS to
// work — the capability is present — so an operator concludes the mechanism is
// wired correctly, then writes a second scope that really does depend on a
// baseline closing something first and finds it inert.
func TestAudit_A12_ScopeReopensNothing(t *testing.T) {
	t.Parallel()
	p := mustLoad(t, `
client_baselines:
  apps:
    applies_to: [app]
    redact:
      person: [salary]
scope_grants:
  rela.tickets.write:
    update: [ticket]
`)
	f := Audit(p, personMeta, allPerms{})
	if !hasRule(f, "A12-scope-reopens-nothing") {
		t.Fatal("a scope re-opening a capability no baseline closes produced no finding")
	}
	// The message must name what is unreachable, or the operator cannot act.
	var detail string
	for _, x := range f {
		if x.Rule == "A12-scope-reopens-nothing" {
			detail = x.Detail
		}
	}
	if !strings.Contains(detail, "update ticket") {
		t.Errorf("detail %q does not name the unreachable target", detail)
	}
}

func TestAudit_A12_ScopeReopensSomething_NoFinding(t *testing.T) {
	t.Parallel()
	p := mustLoad(t, `
client_baselines:
  apps:
    applies_to: [app]
    deny_write: ["*"]
scope_grants:
  rela.tickets.write:
    update: [ticket]
`)
	if f := Audit(p, personMeta, allPerms{}); hasRule(f, "A12-scope-reopens-nothing") {
		t.Error("a scope carving out of deny_write was reported unreachable")
	}
}

// TestAudit_A12_FieldReopen covers the field axis of the same rule, including
// the closed-world case: a `visible:` block that omits a field IS closing it,
// so a scope re-opening that field is reachable.
func TestAudit_A12_FieldReopen(t *testing.T) {
	t.Parallel()
	t.Run("redact closes it", func(t *testing.T) {
		t.Parallel()
		p := mustLoad(t, `
client_baselines:
  apps:
    applies_to: [app]
    redact:
      person: [salary]
scope_grants:
  rela.payroll:
    visible:
      person: [salary]
`)
		if f := Audit(p, personMeta, allPerms{}); hasRule(f, "A12-scope-reopens-nothing") {
			t.Error("a scope re-opening a redacted field was reported unreachable")
		}
	})

	t.Run("closed-world visible closes it", func(t *testing.T) {
		t.Parallel()
		p := mustLoad(t, `
client_baselines:
  apps:
    applies_to: [app]
    visible:
      person: [name]
scope_grants:
  rela.payroll:
    visible:
      person: [salary]
`)
		if f := Audit(p, personMeta, allPerms{}); hasRule(f, "A12-scope-reopens-nothing") {
			t.Error("a `visible:` block omitting salary does close it; the scope is reachable")
		}
	})

	t.Run("nothing closes it", func(t *testing.T) {
		t.Parallel()
		p := mustLoad(t, `
client_baselines:
  apps:
    applies_to: [app]
    deny_write: ["*"]
scope_grants:
  rela.payroll:
    visible:
      person: [salary]
`)
		if f := Audit(p, personMeta, allPerms{}); !hasRule(f, "A12-scope-reopens-nothing") {
			t.Error("no baseline constrains person fields; the scope re-opens nothing")
		}
	})
}

// TestAudit_B8_CeilingUndeclaredType: a typo'd type in a DENY position is worse
// than in a grant — it silently fails to PROTECT rather than failing to permit.
func TestAudit_B8_CeilingUndeclaredType(t *testing.T) {
	t.Parallel()
	p := mustLoad(t, `
client_baselines:
  apps:
    applies_to: [app]
    deny_read: [persno]
`)
	f := Audit(p, personMeta, allPerms{})
	if !hasRule(f, "B8-ceiling-undeclared-type") {
		t.Fatal("a deny_read on an undeclared type produced no finding")
	}
	if got := ruleSeverity(f, "B8-ceiling-undeclared-type"); got != High {
		t.Errorf("severity = %v, want High — a typo'd denial protects nothing", got)
	}
}

func TestAudit_B8_RedactKeyUndeclaredType(t *testing.T) {
	t.Parallel()
	p := mustLoad(t, `
client_baselines:
  apps:
    applies_to: [app]
    redact:
      persno: [salary]
`)
	if f := Audit(p, personMeta, allPerms{}); !hasRule(f, "B8-ceiling-undeclared-type") {
		t.Error("a redact block keyed on an undeclared type produced no finding")
	}
}

// TestAudit_B9_CeilingUndeclaredField: the field-level equivalent.
func TestAudit_B9_CeilingUndeclaredField(t *testing.T) {
	t.Parallel()
	p := mustLoad(t, `
client_baselines:
  apps:
    applies_to: [app]
    redact:
      person: [salaryy]
`)
	f := Audit(p, personMeta, allPerms{})
	if !hasRule(f, "B9-ceiling-undeclared-field") {
		t.Fatal("a redact naming an undeclared field produced no finding")
	}
}

func TestAudit_B9_DeclaredFields_NoFinding(t *testing.T) {
	t.Parallel()
	p := mustLoad(t, `
client_baselines:
  apps:
    applies_to: [app]
    redact:
      person: [salary]
    visible:
      ticket: [title]
`)
	f := Audit(p, personMeta, allPerms{})
	if hasRule(f, "B9-ceiling-undeclared-field") || hasRule(f, "B8-ceiling-undeclared-type") {
		t.Error("a correct ceiling produced a drift finding")
	}
}

// TestAudit_NoCeilings_NoFindings confirms the new rules are silent on every
// existing policy: this feature is opt-in, and an operator who has never heard
// of it must not suddenly see findings.
func TestAudit_NoCeilings_NoFindings(t *testing.T) {
	t.Parallel()
	p := mustLoad(t, `
roles:
  reader:
    read: ["*"]
assignments:
  alice: reader
`)
	for _, rule := range []string{
		"A11-inert-client-baseline",
		"A12-scope-reopens-nothing",
		"A13-baseline-matches-nothing",
		"B8-ceiling-undeclared-type",
		"B9-ceiling-undeclared-field",
	} {
		if hasRule(Audit(p, personMeta, allPerms{}), rule) {
			t.Errorf("policy with no attenuation config produced %s", rule)
		}
	}
}
