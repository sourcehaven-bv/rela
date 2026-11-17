package appbuild_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

const grantCondSchema = `version: "1.0"
entities:
  person:
    label: Person
    plural: people
    id_prefix: "P-"
    properties:
      email: {type: string, unique: true}
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "TKT-"
    properties:
      title: {type: string}
relations:
  grants:
    label: grants
    from: [person]
    to: [ticket]
    properties:
      valid_until: {type: date}
      until: {type: date}
      starts: {type: date}
      at: {type: datetime}
      eu: {type: date, format: "DD-MM-YYYY"}
      kind: {type: string}
      tags: {type: string, list: true}
`

// TestValidateACLPolicy_GrantConditions pins what a role relation's `when:`
// may say (TKT-TKSCSBT): only what lowers exactly into a store query.
func TestValidateACLPolicy_GrantConditions(t *testing.T) {
	meta, err := metamodel.Parse([]byte(grantCondSchema))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	tests := []struct {
		name, when, wantErr string
		terms               int
	}{
		{"valid_until today", "edge.valid_until >= today()", "", 1},
		{"window", "edge.starts <= today() and edge.valid_until >= today()", "", 2},
		{"flipped operands", "today() <= edge.valid_until", "", 1},
		{"date literal", "edge.valid_until >= '2026-12-31'", "", 1},
		{"datetime literal", "edge.valid_until >= '2026-12-31T10:00:00Z'", "time of day", 0},
		{"string equality", "edge.kind == 'audit' and edge.valid_until >= today()", "", 2},
		{"or", "edge.valid_until >= today() or edge.kind == 'x'", "AND of comparisons", 0},
		{"not", "not (edge.valid_until < today())", "AND of comparisons", 0},
		{"two attributes", "edge.starts <= edge.valid_until", "AND of comparisons", 0},
		{"undeclared property", "edge.expires >= today()", "", 0},
		{"datetime", "edge.at >= today()", "datetime", 0},
		{"custom format", "edge.eu >= today()", "format", 0},
		{"ordered string", "edge.kind >= 'a'", "only a date property", 0},
		{"empty string", "edge.kind == ''", "only a date property", 0},
		{"list", "edge.tags == 'a'", "", 0},
		{"reserved word", "edge.until >= today()", "reserved word", 0},
		{"syntax error", "edge.valid_until >=", "", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &acl.Policy{
				UserEntityType:    "person",
				PrincipalProperty: "email",
				Roles:             map[string]acl.RoleDef{"auditor": {Read: []string{"ticket"}}},
				RoleRelations: map[string]acl.RoleRelationDef{
					"grants": {Confers: "auditor", RequiresPermission: "delegate", When: tc.when},
				},
			}
			err := appbuild.ValidateACLPolicy(p, meta)
			if tc.terms == 0 {
				if err == nil {
					t.Fatalf("when %q must fail the load", tc.when)
				}
				if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not mention %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("when %q: %v", tc.when, err)
			}
			cond := p.RoleRelations["grants"].Condition
			if cond == nil || len(cond.Terms) != tc.terms || cond.Source != tc.when {
				t.Fatalf("condition = %+v, want %d terms from %q", cond, tc.terms, tc.when)
			}
		})
	}
}

// TestValidateACLPolicy_GrantConditionsRace pins that validating a policy a
// running Declarative reads writes nothing to it (the Configure preview
// validates the live policy). Run under -race.
func TestValidateACLPolicy_GrantConditionsRace(t *testing.T) {
	meta, err := metamodel.Parse([]byte(grantCondSchema))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	p := &acl.Policy{
		UserEntityType:    "person",
		PrincipalProperty: "email",
		Roles:             map[string]acl.RoleDef{"auditor": {Read: []string{"ticket"}}},
		RoleRelations: map[string]acl.RoleRelationDef{
			"grants": {Confers: "auditor", RequiresPermission: "delegate", When: "edge.valid_until >= today()"},
		},
	}
	if err := appbuild.ValidateACLPolicy(p, meta); err != nil {
		t.Fatal(err)
	}
	compiled := p.RoleRelations["grants"].Condition

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 200 {
			for _, def := range p.RoleRelations {
				_ = def.Condition
			}
		}
	}()
	for range 200 {
		if err := appbuild.ValidateACLPolicy(p, meta); err != nil {
			t.Error(err)
			break
		}
	}
	<-done
	if p.RoleRelations["grants"].Condition != compiled {
		t.Error("validating a compiled policy must not replace its condition")
	}
}
