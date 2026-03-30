package testutil

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestEntity_Build_RandomID(t *testing.T) {
	e := Entity("ticket").Build()

	if e.ID == "" {
		t.Error("Entity().Build() returned empty ID")
	}
	if e.Type != "ticket" {
		t.Errorf("Type = %q, want 'ticket'", e.Type)
	}
}

func TestEntity_With_SetsProperty(t *testing.T) {
	e := Entity("ticket").
		With("status", "open").
		With("priority", "high").
		Build()

	if e.GetString("status") != "open" {
		t.Errorf("status = %q, want 'open'", e.GetString("status"))
	}
	if e.GetString("priority") != "high" {
		t.Errorf("priority = %q, want 'high'", e.GetString("priority"))
	}
}

func TestEntity_WithList_StoresAsSlice(t *testing.T) {
	e := Entity("ticket").
		WithList("tags", "bug", "urgent").
		Build()

	tags := e.GetAttributeStrings("tags")
	if len(tags) != 2 {
		t.Errorf("tags length = %d, want 2", len(tags))
	}
	if tags[0] != "bug" || tags[1] != "urgent" {
		t.Errorf("tags = %v, want [bug urgent]", tags)
	}
}

func TestEntity_ID_SetsExplicitID(t *testing.T) {
	e := Entity("ticket").ID("TKT-001").Build()

	if e.ID != "TKT-001" {
		t.Errorf("ID = %q, want 'TKT-001'", e.ID)
	}
}

func TestEntity_Content_SetsContent(t *testing.T) {
	e := Entity("ticket").WithContent("# Title\n\nDescription").Build()

	if e.Content != "# Title\n\nDescription" {
		t.Errorf("Content = %q, want '# Title\\n\\nDescription'", e.Content)
	}
}

func fixtureTestMetamodel() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"ticket_status": {
				Values: []string{"open", "closed", "pending"},
			},
			"priority": {
				Values: []string{"low", "medium", "high"},
			},
		},
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				IDPrefixes: []string{"TKT-"},
				Properties: map[string]metamodel.PropertyDef{
					"title": {
						Type:     "string",
						Required: true,
					},
					"status": {
						Type:     "ticket_status",
						Required: true,
					},
					"priority": {
						Type:     "priority",
						Required: false, // optional
					},
					"description": {
						Type:     "string",
						Required: false,
					},
					"tags": {
						Type:     "string",
						Required: false,
						List:     true,
					},
				},
			},
			"feature": {
				IDPrefix: "FEAT-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {
						Type:     "string",
						Required: true,
					},
				},
			},
		},
	}
}

func TestEntityFor_AutoFillsRequired(t *testing.T) {
	meta := fixtureTestMetamodel()
	e := EntityFor(meta, "ticket").Build()

	// Should have auto-filled title (required string)
	if e.GetString("title") == "" {
		t.Error("title should be auto-filled")
	}

	// Should have auto-filled status (required enum)
	status := e.GetString("status")
	if status == "" {
		t.Error("status should be auto-filled")
	}
	// Status should be a valid value
	validStatuses := []string{"open", "closed", "pending"}
	found := false
	for _, v := range validStatuses {
		if status == v {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("status = %q, want one of %v", status, validStatuses)
	}

	// Should NOT have auto-filled optional properties
	if e.GetString("priority") != "" {
		t.Errorf("priority should not be auto-filled, got %q", e.GetString("priority"))
	}
}

func TestEntityFor_WithOverridesAutoFill(t *testing.T) {
	meta := fixtureTestMetamodel()
	e := EntityFor(meta, "ticket").
		With("status", "closed").
		Build()

	if e.GetString("status") != "closed" {
		t.Errorf("status = %q, want 'closed'", e.GetString("status"))
	}
}

func TestEntityFor_UsesIDPrefix(t *testing.T) {
	meta := fixtureTestMetamodel()

	// Test with IDPrefixes
	e1 := EntityFor(meta, "ticket").Build()
	if !strings.HasPrefix(e1.ID, "TKT-") {
		t.Errorf("ticket ID = %q, want prefix 'TKT-'", e1.ID)
	}

	// Test with IDPrefix (singular)
	e2 := EntityFor(meta, "feature").Build()
	if !strings.HasPrefix(e2.ID, "FEAT-") {
		t.Errorf("feature ID = %q, want prefix 'FEAT-'", e2.ID)
	}
}

func TestEntityFor_Without_SkipsAutoFill(t *testing.T) {
	meta := fixtureTestMetamodel()
	e := EntityFor(meta, "ticket").
		Without("title").
		Build()

	// title should NOT be auto-filled since we skipped it
	if e.GetString("title") != "" {
		t.Errorf("title should not be auto-filled when skipped, got %q", e.GetString("title"))
	}

	// status should still be auto-filled
	if e.GetString("status") == "" {
		t.Error("status should be auto-filled")
	}
}

func TestEntityFor_PanicsOnNilMetamodel(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("EntityFor(nil, ...) did not panic")
		}
	}()
	EntityFor(nil, "ticket")
}

func TestEntityFor_PanicsOnUnknownType(t *testing.T) {
	meta := fixtureTestMetamodel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("EntityFor(meta, unknown) did not panic")
		}
	}()
	EntityFor(meta, "unknown-type")
}

func TestRelation_Build(t *testing.T) {
	r := Relation("implements").
		From("TKT-001").
		To("FEAT-001").
		Build()

	if r.From != "TKT-001" {
		t.Errorf("From = %q, want 'TKT-001'", r.From)
	}
	if r.Type != "implements" {
		t.Errorf("Type = %q, want 'implements'", r.Type)
	}
	if r.To != "FEAT-001" {
		t.Errorf("To = %q, want 'FEAT-001'", r.To)
	}
}

func TestRelation_WithContent(t *testing.T) {
	r := Relation("implements").
		From("TKT-001").
		To("FEAT-001").
		WithContent("Implementation notes").
		Build()

	if r.Content != "Implementation notes" {
		t.Errorf("Content = %q, want 'Implementation notes'", r.Content)
	}
}

func TestRelation_Fluent(t *testing.T) {
	// Verify fluent API returns the builder
	b := Relation("test")
	if b.From("A") != b {
		t.Error("From() should return the builder")
	}
	if b.To("B") != b {
		t.Error("To() should return the builder")
	}
	if b.WithContent("C") != b {
		t.Error("WithContent() should return the builder")
	}
}

func TestRelation_Build_PanicsOnMissingFrom(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Build() should panic when From is not set")
		}
	}()
	Relation("test").To("B").Build()
}

func TestRelation_Build_PanicsOnMissingTo(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Build() should panic when To is not set")
		}
	}()
	Relation("test").From("A").Build()
}
