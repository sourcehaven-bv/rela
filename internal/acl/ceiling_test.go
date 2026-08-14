package acl_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// loadCeilingPolicy parses an acl.yaml body and returns the validation error.
// Uses LoadPolicyBytes so the tests exercise the real load path (normalization
// runs before validation, exactly as in production).
func loadCeilingPolicy(t *testing.T, body string) (*acl.Policy, error) {
	t.Helper()
	return acl.LoadPolicyBytes([]byte(body))
}

// TestClientBaselines_DisjointAppliesTo pins AC2. Overlap is the invariant that
// buys "exactly one baseline matches", which is what lets the design have no
// precedence rule at all. If this ever becomes a warning instead of an error,
// the no-combination-rule property is gone and a more-specific baseline can
// silently widen a narrower one.
func TestClientBaselines_DisjointAppliesTo(t *testing.T) {
	t.Parallel()
	_, err := loadCeilingPolicy(t, `
roles:
  reader:
    read: ["*"]
client_baselines:
  apps:
    applies_to: [app, pat]
    deny_write: ["*"]
  automation:
    applies_to: [pat, service]
    deny_write: ["*"]
`)
	if err == nil {
		t.Fatal("overlapping applies_to loaded clean; want a startup error")
	}
	// The message must name BOTH blocks — an operator staring at four baselines
	// needs to know which pair collided, not just that one did.
	for _, want := range []string{"pat", "apps", "automation", "disjoint"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// TestClientBaselines_DisjointErrorIsDeterministic guards against a map-order
// flake: with three-way overlap the reported pair must be stable, or the same
// broken policy produces different errors on different runs and an operator
// chasing one collision sees another.
func TestClientBaselines_DisjointErrorIsDeterministic(t *testing.T) {
	t.Parallel()
	const body = `
client_baselines:
  zeta:
    applies_to: [app]
    deny_write: ["*"]
  alpha:
    applies_to: [app]
    deny_write: ["*"]
  mid:
    applies_to: [app]
    deny_write: ["*"]
`
	var first string
	for i := range 20 {
		_, err := loadCeilingPolicy(t, body)
		if err == nil {
			t.Fatal("overlapping applies_to loaded clean")
		}
		if i == 0 {
			first = err.Error()
			continue
		}
		if err.Error() != first {
			t.Fatalf("non-deterministic error:\n run 0: %s\n run %d: %s", first, i, err)
		}
	}
}

// TestRestriction_OneSpellingPerAxis pins AC5. Declaring both an allowlist and
// a denylist for one axis has no defensible merge semantics — whatever we chose
// would be a coin-flip an operator has to memorize — so it must fail loud.
func TestRestriction_OneSpellingPerAxis(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "read and deny_read",
			body: `
client_baselines:
  apps:
    applies_to: [app]
    read: [ticket]
    deny_read: [person]
`,
			want: []string{"read", "deny_read"},
		},
		{
			name: "visible and redact on the same type",
			body: `
client_baselines:
  apps:
    applies_to: [app]
    visible:
      person: [name]
    redact:
      person: [salary]
`,
			want: []string{"visible", "redact", "person"},
		},
		{
			name: "deny_write and deny_update overlap",
			body: `
client_baselines:
  apps:
    applies_to: [app]
    deny_write: ["*"]
    deny_update: [ticket]
`,
			want: []string{"deny_write", "deny_update"},
		},
		{
			name: "deny_write and allowlist create",
			body: `
client_baselines:
  apps:
    applies_to: [app]
    deny_write: ["*"]
    create: [ticket]
`,
			want: []string{"deny_write", "create"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := loadCeilingPolicy(t, tc.body)
			if err == nil {
				t.Fatal("conflicting spellings loaded clean; want a load error")
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not mention %q", err, w)
				}
			}
		})
	}
}

// TestRestriction_VisibleAndRedactOnDifferentTypes is the counterpart to the
// same-type rejection above: mixing postures ACROSS types is legitimate and
// must keep working. A type with two sensitive columns wants redact; a type
// whose safe set is small wants visible. Rejecting this would force one posture
// on the whole schema.
func TestRestriction_VisibleAndRedactOnDifferentTypes(t *testing.T) {
	t.Parallel()
	p, err := loadCeilingPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    visible:
      ticket: [title, status]
    redact:
      person: [salary]
`)
	if err != nil {
		t.Fatalf("mixing visible and redact across types should load: %v", err)
	}
	b := p.ClientBaselines["apps"]
	if got := b.Visible["ticket"]; len(got) != 2 {
		t.Errorf("visible[ticket] = %v, want 2 fields", got)
	}
	if got := b.Redact["person"]; len(got) != 1 {
		t.Errorf("redact[person] = %v, want 1 field", got)
	}
}

// TestDenyWrite_ExpandsToThreeVerbs pins AC6. deny_write exists so the most
// common ceiling ("read-only client") is one line; the expansion happens at
// load so every downstream consumer sees one canonical shape rather than
// re-deriving the shorthand.
func TestDenyWrite_ExpandsToThreeVerbs(t *testing.T) {
	t.Parallel()
	p, err := loadCeilingPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    deny_write: ["*"]
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	b := p.ClientBaselines["apps"]
	for name, got := range map[string][]string{
		"deny_create": b.DenyCreate,
		"deny_update": b.DenyUpdate,
		"deny_delete": b.DenyDelete,
	} {
		if len(got) != 1 || got[0] != "*" {
			t.Errorf("%s = %v, want [*]", name, got)
		}
	}
	if len(b.DenyWrite) != 0 {
		t.Errorf("DenyWrite = %v, want cleared after expansion", b.DenyWrite)
	}
}

// TestDenyWrite_ExpansionRunsAfterValidation is a regression test for an
// ordering bug found while writing these tests: normalization originally
// expanded deny_write into the three per-verb lists BEFORE validation, so the
// "deny_write alongside deny_update" collision check found deny_write already
// gone, passed the policy, and silently discarded the operator's deny_update.
//
// Silently narrowing differently from what was written is the worst outcome
// available to a ceiling: it neither fails loud nor does what the file says.
func TestDenyWrite_ExpansionRunsAfterValidation(t *testing.T) {
	t.Parallel()
	_, err := loadCeilingPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    deny_write: ["*"]
    deny_update: [ticket]
`)
	if err == nil {
		t.Fatal("deny_write + deny_update loaded clean — the collision check ran " +
			"after expansion and the deny_update was silently discarded")
	}
}

// TestDenyWrite_ExpansionDoesNotAliasBackingArray guards a subtle aliasing bug:
// if the three verb slices shared one backing array, a later in-place edit of
// one (say, a compiler step filtering wildcards) would silently mutate the
// others. The clone is cheap; the bug would be near-invisible.
func TestDenyWrite_ExpansionDoesNotAliasBackingArray(t *testing.T) {
	t.Parallel()
	p, err := loadCeilingPolicy(t, `
client_baselines:
  apps:
    applies_to: [app]
    deny_write: [ticket]
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	b := p.ClientBaselines["apps"]
	b.DenyCreate[0] = "mutated"
	if b.DenyUpdate[0] != "ticket" || b.DenyDelete[0] != "ticket" {
		t.Errorf("deny_write expansion shares a backing array: update=%v delete=%v",
			b.DenyUpdate, b.DenyDelete)
	}
}

// TestScopeGrants_RejectDenySpellings pins the re-open-only asymmetry. A scope
// exists to widen within a ceiling; "deny" on a re-opener means "re-open
// nothing", which omitting the scope already says. Rejecting rather than
// ignoring stops an operator believing a scope tightens something.
func TestScopeGrants_RejectDenySpellings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, field, body string
	}{
		{
			name:  "deny_read",
			field: "deny_read",
			body: `
scope_grants:
  rela.read:
    deny_read: [person]
`,
		},
		{
			name:  "redact",
			field: "redact",
			body: `
scope_grants:
  rela.read:
    redact:
      person: [salary]
`,
		},
		{
			// deny_write survives to validation unexpanded (expansion runs only
			// after every collision check passes), so the error names it directly.
			name:  "deny_write",
			field: "deny_write",
			body: `
scope_grants:
  rela.read:
    deny_write: ["*"]
`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := loadCeilingPolicy(t, tc.body)
			if err == nil {
				t.Fatalf("scope_grants with %s loaded clean; want a load error", tc.field)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Errorf("error %q does not name the offending field %q", err, tc.field)
			}
		})
	}
}

// TestScopeGrants_AllowSpellingsLoad confirms the positive path still works —
// the rejection above must not have been implemented as "reject everything".
func TestScopeGrants_AllowSpellingsLoad(t *testing.T) {
	t.Parallel()
	p, err := loadCeilingPolicy(t, `
scope_grants:
  rela.people.read:
    read: [person]
    visible:
      person: [name, email]
  rela.tickets.write:
    update: [ticket]
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := p.ScopeGrants["rela.people.read"].Read; len(got) != 1 || got[0] != "person" {
		t.Errorf("read = %v, want [person]", got)
	}
	if got := p.ScopeGrants["rela.tickets.write"].Update; len(got) != 1 || got[0] != "ticket" {
		t.Errorf("update = %v, want [ticket]", got)
	}
}

// TestCeiling_WhitespacePaddedKeysAreMatchable is the RR-IK355A trap, ported.
// A padded key that loads clean but can never match any real claim value is the
// worst failure mode available here: it looks like protection and is inert.
// Normalization must make the padded form equivalent to the bare one.
func TestCeiling_WhitespacePaddedKeysAreMatchable(t *testing.T) {
	t.Parallel()
	p, err := loadCeilingPolicy(t, `
client_baselines:
  "  apps  ":
    applies_to: ["  app  "]
    deny_write: ["  ticket  "]
scope_grants:
  "  rela.read  ":
    read: ["  person  "]
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, ok := p.ClientBaselines["apps"]; !ok {
		t.Errorf("baseline key not trimmed; got keys %v", keysOf(p.ClientBaselines))
	}
	if got := p.ClientBaselines["apps"].AppliesTo; len(got) != 1 || got[0] != "app" {
		t.Errorf("applies_to = %q, want [app]", got)
	}
	if got := p.ClientBaselines["apps"].DenyCreate; len(got) != 1 || got[0] != "ticket" {
		t.Errorf("deny_create = %q, want [ticket] (trimmed + expanded)", got)
	}
	if _, ok := p.ScopeGrants["rela.read"]; !ok {
		t.Errorf("scope key not trimmed")
	}
	if got := p.ScopeGrants["rela.read"].Read; len(got) != 1 || got[0] != "person" {
		t.Errorf("scope read = %q, want [person]", got)
	}
}

// TestCeiling_BlankKeysRejected: a blank key can never match a claim, so the
// mapping is silently inert — same reasoning as the asserted_role_assignments
// guard. Fail loud instead.
func TestCeiling_BlankKeysRejected(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, body string }{
		{"blank baseline name", `
client_baselines:
  "   ":
    applies_to: [app]
    deny_write: ["*"]
`},
		{"blank applies_to entry", `
client_baselines:
  apps:
    applies_to: ["  "]
    deny_write: ["*"]
`},
		{"blank scope key", `
scope_grants:
  "   ":
    read: [person]
`},
		{"blank type in visible", `
client_baselines:
  apps:
    applies_to: [app]
    visible:
      "  ": [name]
`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := loadCeilingPolicy(t, tc.body); err == nil {
				t.Fatal("blank key loaded clean; want a load error")
			}
		})
	}
}

// TestCeiling_AbsentIsUnrestricted pins AC3 at the policy layer: a policy with
// no attenuation config must parse to empty maps, not to some implicit
// everything-denied default. The ceiling is opt-in; every existing deployment
// must be byte-for-byte unaffected.
func TestCeiling_AbsentIsUnrestricted(t *testing.T) {
	t.Parallel()
	p, err := loadCeilingPolicy(t, `
roles:
  reader:
    read: ["*"]
assignments:
  alice: reader
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(p.ClientBaselines) != 0 {
		t.Errorf("ClientBaselines = %v, want empty", p.ClientBaselines)
	}
	if len(p.ScopeGrants) != 0 {
		t.Errorf("ScopeGrants = %v, want empty", p.ScopeGrants)
	}
}

// TestCeiling_UnknownKeyStillWarnsNotFails confirms the new keys joined the
// allowlist without turning the loader strict: acl.yaml is tolerant by design
// (operators iterate on it) and a typo must not brick the server.
func TestCeiling_UnknownKeyStillWarnsNotFails(t *testing.T) {
	t.Parallel()
	if _, err := loadCeilingPolicy(t, `
client_baseline:
  apps:
    applies_to: [app]
`); err != nil {
		t.Fatalf("a typo'd top-level key must warn, not fail: %v", err)
	}
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
