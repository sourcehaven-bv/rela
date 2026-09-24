package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// A validation rule's related() is answered under the requester's read gate:
// an edge to an entity the requester cannot read does not exist for them, so
// the rule reports the violation it would report without that edge. Answered
// raw, the verdict would reveal the hidden entity's existence (TKT-205V2N).
func TestGatedValidator_TraversalIgnoresHiddenEntity(t *testing.T) {
	rule := metamodel.ValidationRule{
		Name:          "needs-feature",
		EntityType:    "ticket",
		ThenCondition: "related(entity, 'implements')",
		Severity:      "error",
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket":  {Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}}},
			"feature": {Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}}},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {From: []string{"ticket"}, To: []string{"feature"}},
		},
		Validations: []metamodel.ValidationRule{rule},
	}
	app := newAppFromParts(&Config{}, meta, newFixture())
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "t"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-SECRET", Type: "feature", Properties: map[string]any{"title": "f"}})
	seedRelation(app, &entity.Relation{From: "TKT-001", Type: "implements", To: "FEAT-SECRET"})

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"viewer": {Read: []string{"ticket"}},
			"admin":  {Read: []string{"ticket", "feature"}},
		},
		Assignments: map[string]string{"alice": "viewer", "bob": "admin"},
	}, app.store)
	app.acl = d

	for _, tc := range []struct {
		user string
		want int
	}{{"alice", 1}, {"bob", 0}} {
		t.Run(tc.user, func(t *testing.T) {
			ids, err := app.validator.CheckRule(gateCtxFor(principalCtx(tc.user), t, d), rule)
			if err != nil {
				t.Fatal(err)
			}
			if len(ids) != tc.want {
				t.Fatalf("violations = %v, want %d", ids, tc.want)
			}
		})
	}

	// A caller outside the request middleware carries no read gate. The
	// traversal takes its tier from the policy, as the reads do, rather than
	// running ungated.
	t.Run("no read gate on ctx", func(t *testing.T) {
		ids, err := app.validator.CheckRule(principalCtx("alice"), rule)
		if err != nil {
			t.Fatal(err)
		}
		if len(ids) != 1 {
			t.Fatalf("violations = %v, want 1: the hidden feature must not count", ids)
		}
	})
}
