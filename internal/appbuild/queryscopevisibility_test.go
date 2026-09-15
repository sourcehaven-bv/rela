package appbuild_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/scopes"
)

// scopeVisibilityMetamodel declares one type with three properties and a set
// of scopes reading different ones, so a test can tell WHICH scope a report
// names rather than only that it reported something.
const scopeVisibilityMetamodel = `version: "1.0"
entities:
  person:
    label: Person
    plural: people
    id_prefix: "PERS-"
    id_type: sequential
    properties:
      name:
        type: string
      status:
        type: string
      salary:
        type: string
    query_scopes:
      default: "entity.status ~= 'gearchiveerd'"
      rich: "entity.salary == '100000'"
      named: "entity.name == 'bob'"
relations: {}
`

func mustScopeFixture(t *testing.T, schema, policy string) (
	*scopes.Compiled, *metamodel.Metamodel, *acl.Policy,
) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(schema))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	compiled, err := scopes.Compile(meta)
	if err != nil {
		t.Fatalf("compile scopes: %v", err)
	}
	var pol *acl.Policy
	if policy != "" {
		pol, err = acl.LoadPolicyBytes([]byte(policy))
		if err != nil {
			t.Fatalf("parse policy: %v", err)
		}
	}
	return &compiled, meta, pol
}

// TestQueryScopeVisibility_ReportsOverlap is AC9's core case: a scope reading a
// property a role cannot see is named, with the scope, the property and the
// role all present.
//
// The `viewer` role grants only `name`, so `visible:`'s closed-world semantics
// restrict BOTH `status` and `salary` — which is why the report should name the
// `default` and `rich` scopes but not `named`.
func TestQueryScopeVisibility_ReportsOverlap(t *testing.T) {
	const policy = `roles:
  viewer:
    read: ["*"]
    visible:
      person:
        - field: name
assignments:
  bob: viewer
`
	compiled, meta, pol := mustScopeFixture(t, scopeVisibilityMetamodel, policy)
	got := appbuild.QueryScopeVisibilityConflicts(compiled, meta, pol)

	if len(got) != 2 {
		t.Fatalf("want 2 conflicts, got %d: %v", len(got), got)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		`query scope "default"`, `"status"`,
		`query scope "rich"`, `"salary"`,
		`role "viewer"`, `entity "person"`,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("report missing %q:\n%s", want, joined)
		}
	}
	// `named` reads only the one property the role CAN see, so naming it
	// would be the false positive that makes an operator stop reading the
	// warnings at all.
	if strings.Contains(joined, `query scope "named"`) {
		t.Errorf("scope over a granted property must not be reported:\n%s", joined)
	}
}

// TestQueryScopeVisibility_ClosedWorldNotDenylist pins the load-bearing half of
// restrictedFields: a property the metamodel declares and the role's `visible:`
// list omits is RESTRICTED, even though nothing denies it by name.
//
// Reverting the complement to a denylist makes this test the only thing that
// fails — the report would still run, still format, and still find nothing,
// which is exactly how a check like this rots unnoticed.
func TestQueryScopeVisibility_ClosedWorldNotDenylist(t *testing.T) {
	const schema = `version: "1.0"
entities:
  person:
    label: Person
    plural: people
    id_prefix: "PERS-"
    id_type: sequential
    properties:
      name:
        type: string
      added_later:
        type: string
    query_scopes:
      default: "entity.added_later == 'x'"
relations: {}
`
	// The role names ONLY `name`. `added_later` is never mentioned anywhere
	// in the policy — the case a denylist implementation cannot see.
	const policy = `roles:
  viewer:
    read: ["*"]
    visible:
      person:
        - field: name
assignments:
  bob: viewer
`
	compiled, meta, pol := mustScopeFixture(t, schema, policy)
	got := appbuild.QueryScopeVisibilityConflicts(compiled, meta, pol)
	if len(got) != 1 || !strings.Contains(got[0], `"added_later"`) {
		t.Fatalf("a property absent from visible: must count as restricted, got: %v", got)
	}
}

// TestQueryScopeVisibility_ConditionalGrantRestricts pins that a `when:` grant
// does NOT count as granting. It hides the field for some rows, which is the
// row-shaped disagreement the report exists to name.
func TestQueryScopeVisibility_ConditionalGrantRestricts(t *testing.T) {
	const schema = `version: "1.0"
entities:
  person:
    label: Person
    plural: people
    id_prefix: "PERS-"
    id_type: sequential
    properties:
      status:
        type: string
    query_scopes:
      default: "entity.status ~= 'gearchiveerd'"
relations: {}
`
	const policy = `roles:
  viewer:
    read: ["*"]
    visible:
      person:
        - field: status
          when: "entity.status == 'open'"
assignments:
  bob: viewer
`
	compiled, meta, pol := mustScopeFixture(t, schema, policy)
	got := appbuild.QueryScopeVisibilityConflicts(compiled, meta, pol)
	if len(got) != 1 || !strings.Contains(got[0], `"status"`) {
		t.Fatalf("a when:-conditional grant must count as restricting, got: %v", got)
	}
}

// TestQueryScopeVisibility_Quiet covers the configurations that must produce
// NOTHING. A warning surface that fires on ordinary projects is one operators
// learn to ignore, so the silent cases deserve pinning as much as the loud one.
func TestQueryScopeVisibility_Quiet(t *testing.T) {
	const openPolicy = `roles:
  viewer:
    read: ["*"]
assignments:
  bob: viewer
`
	tests := []struct {
		name          string
		schema        string
		policy        string
		nilMeta       bool
		nilCompiled   bool
		wantNoReports bool
	}{
		{
			name:          "no visible: block anywhere",
			schema:        scopeVisibilityMetamodel,
			policy:        openPolicy,
			wantNoReports: true,
		},
		{
			name:          "nil policy",
			schema:        scopeVisibilityMetamodel,
			policy:        "",
			wantNoReports: true,
		},
		{
			name:          "no scopes declared",
			schema:        strings.ReplaceAll(scopeVisibilityMetamodel, "    query_scopes:", "    x_unused:"),
			policy:        openPolicy,
			wantNoReports: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			compiled, meta, pol := mustScopeFixture(t, tc.schema, tc.policy)
			if got := appbuild.QueryScopeVisibilityConflicts(compiled, meta, pol); len(got) != 0 {
				t.Fatalf("want no conflicts, got: %v", got)
			}
		})
	}
}

// TestQueryScopeVisibility_NilSafe pins the nil contract, since this runs
// during prepare() where a project may legitimately have no acl.yaml.
func TestQueryScopeVisibility_NilSafe(t *testing.T) {
	if got := appbuild.QueryScopeVisibilityConflicts(nil, nil, nil); got != nil {
		t.Fatalf("nil inputs must report nothing, got: %v", got)
	}
}
