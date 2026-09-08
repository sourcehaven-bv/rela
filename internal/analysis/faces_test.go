package analysis_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/analysis"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The CLI (`rela validate`, `rela analyze`) runs through internal/analysis,
// which is a SECOND validation implementation alongside internal/validator.
// Widening only the latter left the command an operator gates a merge on
// reporting a clean run over faces it never loaded (TKT-4Y6CMV).

func facedPolicyMeta(rules ...metamodel.ValidationRule) *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{"policy": {
			Label: "Policy", BareFace: "draft",
			Faces: map[string]metamodel.FaceDef{"draft": {}, "published": {}},
			Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string"},
				"owner": {Type: "string"},
			},
		}},
		Validations: rules,
	}
}

// A rule scoped to `published` must actually reach the published face.
func TestRunValidations_SeesNonBareFaces(t *testing.T) {
	rule := metamodel.ValidationRule{
		Name: "published-needs-owner", EntityType: "policy",
		Faces: []string{"published"}, Then: []string{"owner!="}, Severity: "error",
	}
	unscoped := metamodel.ValidationRule{
		Name: "any-face-needs-owner", EntityType: "policy",
		Then: []string{"owner!="}, Severity: "error",
	}
	meta := facedPolicyMeta(rule, unscoped)

	svc := newServiceWith(t, meta, func(st store.Store) {
		ctx := context.Background()
		// Draft is fine; the published face has no owner.
		_ = st.CreateEntity(ctx, &entity.Entity{ID: "POL-1", Type: "policy",
			Properties: map[string]any{"title": "P", "owner": "Security"}})
		_ = st.CreateEntity(ctx, &entity.Entity{ID: "POL-1", Type: "policy",
			Face: entity.Face("published"), Properties: map[string]any{"title": "P"}})
	})

	res := svc.RunValidations(context.Background(), analysis.Options{})
	if len(res.Violations) != 2 {
		t.Fatalf("got %d violations, want 2 (the scoped rule AND the unscoped one "+
			"must both catch the published face): %+v", len(res.Violations), res.Violations)
	}
	for _, v := range res.Violations {
		if v.EntityID != "POL-1" {
			t.Errorf("unexpected entity %q", v.EntityID)
		}
		if v.Face != "published" {
			t.Errorf("violation must name the offending face, got %q", v.Face)
		}
	}
}

// A content-scoped relation belongs to ONE face, so cardinality must be counted
// per face: an edge on the draft does not satisfy the published face's bound.
func TestCheckCardinality_CountsContentEdgesPerFace(t *testing.T) {
	one := 1
	meta := facedPolicyMeta()
	meta.Entities["control"] = metamodel.EntityDef{Label: "Control"}
	meta.Relations = map[string]metamodel.RelationDef{"implements": {
		From: []string{"policy"}, To: []string{"control"},
		MinOutgoing: &one, Scope: metamodel.ScopeContent,
	}}

	svc := newServiceWith(t, meta, func(st store.Store) {
		ctx := context.Background()
		_ = st.CreateEntity(ctx, &entity.Entity{ID: "POL-1", Type: "policy",
			Properties: map[string]any{"title": "P"}})
		_ = st.CreateEntity(ctx, &entity.Entity{ID: "POL-1", Type: "policy",
			Face: entity.Face("published"), Properties: map[string]any{"title": "P"}})
		_ = st.CreateEntity(ctx, &entity.Entity{ID: "CTL-1", Type: "control"})
		// Edge tailed on the DRAFT (bare) face only.
		_, _ = st.CreateRelation(ctx, "POL-1", "implements", "CTL-1", &store.RelationData{})
	})

	res, err := svc.CheckCardinality(context.Background(), analysis.Options{})
	if err != nil {
		t.Fatalf("CheckCardinality: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("the published face has no `implements` edge and must be reported — " +
			"a draft's edge does not satisfy a content-scoped bound on another face")
	}
}

// A rule scoped to `published` must say NOTHING about an entity that has no
// published face. Absence is not a violation — it means "not published yet",
// which is the ordinary state of half a project.
//
// The distinction matters because the alternative is indefensible: every draft
// would fail every published-scoped rule from the moment the rule is written,
// so the rule would have to be deleted rather than fixed.
func TestRunValidations_MissingFaceIsNotAViolation(t *testing.T) {
	rule := metamodel.ValidationRule{
		Name: "published-needs-owner", EntityType: "policy",
		Faces: []string{"published"}, Then: []string{"owner!="}, Severity: "error",
	}
	meta := facedPolicyMeta(rule)

	svc := newServiceWith(t, meta, func(st store.Store) {
		ctx := context.Background()
		// Draft only, never published, and missing the owner the rule wants.
		_ = st.CreateEntity(ctx, &entity.Entity{ID: "POL-2", Type: "policy",
			Properties: map[string]any{"title": "Draft only"}})
	})

	res := svc.RunValidations(context.Background(), analysis.Options{})
	if len(res.Violations) != 0 {
		t.Fatalf("an entity with no published face must not violate a "+
			"published-scoped rule, got %+v", res.Violations)
	}
}
