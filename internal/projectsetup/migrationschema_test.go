package projectsetup

import (
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestMigrationSchema(t *testing.T) {
	meta := &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"priority": {Values: []string{"low", "high"}, Default: "low"},
		},
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Aliases: []string{"tkt"},
				Properties: map[string]metamodel.PropertyDef{
					"title":    {Type: metamodel.PropertyTypeString, Required: true},
					"priority": {Type: "priority", Default: "high"},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"blocks": {Label: "blocks", From: []string{"ticket"}, To: []string{"ticket"}},
		},
	}
	meta.InitAliases()
	s := migrationSchema{meta: meta}

	t.Run("property queries", func(t *testing.T) {
		if got := s.GetPropertyType("tkt", "priority"); got != "priority" {
			t.Errorf("GetPropertyType via alias = %q, want priority", got)
		}
		if s.GetPropertyType("ticket", "missing") != "" || s.GetPropertyType("nope", "title") != "" {
			t.Error("GetPropertyType unknown: want empty")
		}
		if !s.IsPropertyRequired("ticket", "title") || s.IsPropertyRequired("ticket", "priority") {
			t.Error("IsPropertyRequired: want title required, priority optional")
		}
		if s.IsPropertyRequired("nope", "title") {
			t.Error("IsPropertyRequired unknown type: want false")
		}
		if got := s.GetPropertyDefault("ticket", "priority"); got != "high" {
			t.Errorf("GetPropertyDefault = %q, want high", got)
		}
		if s.GetPropertyDefault("nope", "priority") != "" {
			t.Error("GetPropertyDefault unknown type: want empty")
		}
	})

	t.Run("type queries", func(t *testing.T) {
		if got := s.GetTypeDefault("priority"); got != "low" {
			t.Errorf("GetTypeDefault = %q, want low", got)
		}
		if s.GetTypeDefault("nope") != "" {
			t.Error("GetTypeDefault unknown: want empty")
		}
		if got := s.ResolveWidgetFromType("priority"); got != "select" {
			t.Errorf("ResolveWidgetFromType(priority) = %q, want select", got)
		}
	})

	t.Run("relation queries", func(t *testing.T) {
		if got := s.GetRelationLabel("blocks"); got != "blocks" {
			t.Errorf("GetRelationLabel = %q, want blocks", got)
		}
		if !slices.Equal(s.GetRelationFrom("blocks"), []string{"ticket"}) ||
			!slices.Equal(s.GetRelationTo("blocks"), []string{"ticket"}) {

			t.Error("GetRelationFrom/To: want [ticket]")
		}
		if s.GetRelationLabel("nope") != "" || s.GetRelationFrom("nope") != nil || s.GetRelationTo("nope") != nil {
			t.Error("unknown relation: want empty label and nil endpoints")
		}
	})
}

// A schema that fails to load must reach the migrations as a nil interface,
// so they take their no-metamodel path, not as an adapter around nil.
func TestLoadMigrationMetamodel_MissingSchemaIsNilInterface(t *testing.T) {
	ctx := &project.Context{SchemaPath: "/nonexistent/schema.yaml"}
	if got := loadMigrationMetamodel(ctx, storage.NewMemFS()); got != nil {
		t.Fatalf("loadMigrationMetamodel = %#v, want nil interface", got)
	}
}
