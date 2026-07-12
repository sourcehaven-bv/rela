package metamodel

import "testing"

// baseMetamodel builds a small metamodel with two entity types, an enum custom
// type, and the render-relevant fields populated, plus churny non-render fields
// (automations, validations, colors) that must NOT affect the projection hash.
func baseMetamodel() *Metamodel {
	return &Metamodel{
		Version:   "1.0",
		Namespace: "test",
		Types: map[string]CustomType{
			"status": {Values: []string{"open", "closed"}, Default: "open", Description: "churny"},
		},
		Entities: map[string]EntityDef{
			"ticket": {
				Label:           "Ticket",
				Color:           "#ff0000", // churny (render-irrelevant to a stored value)
				DisplayProperty: "title",
				PropertyOrder:   []string{"title", "status"},
				Properties: map[string]PropertyDef{
					"title":  {Type: "string", Required: true, Description: "churny"},
					"status": {Type: "status", Required: false},
				},
			},
			"note": {
				Label: "Note",
				Properties: map[string]PropertyDef{
					"body": {Type: "string"},
				},
			},
		},
		Validations: []ValidationRule{}, // churny
	}
}

func TestRenderProjectionHash_Deterministic(t *testing.T) {
	m1 := baseMetamodel()
	m2 := baseMetamodel()
	h1 := m1.RenderProjection().Hash()
	h2 := m2.RenderProjection().Hash()
	if h1 != h2 {
		t.Fatalf("hash not deterministic across equal metamodels: %s != %s", h1, h2)
	}
	// Recomputing on the same metamodel is stable (guards against map-order leakage).
	for i := range 20 {
		if got := m1.RenderProjection().Hash(); got != h1 {
			t.Fatalf("hash unstable on recompute %d: %s != %s", i, got, h1)
		}
	}
}

func TestRenderProjectionHash_StableAcrossChurnyEdits(t *testing.T) {
	base := baseMetamodel().RenderProjection().Hash()

	tests := []struct {
		name   string
		mutate func(*Metamodel)
	}{
		{"add automation", func(m *Metamodel) {
			m.Automations = append(m.Automations, AutomationDef{})
		}},
		{"add validation rule", func(m *Metamodel) {
			m.Validations = append(m.Validations, ValidationRule{})
		}},
		{"change entity color", func(m *Metamodel) {
			d := m.Entities["ticket"]
			d.Color = "#00ff00"
			m.Entities["ticket"] = d
		}},
		{"change property description", func(m *Metamodel) {
			d := m.Entities["ticket"]
			p := d.Properties["title"]
			p.Description = "totally different prose"
			d.Properties["title"] = p
			m.Entities["ticket"] = d
		}},
		{"change custom-type default and description", func(m *Metamodel) {
			ct := m.Types["status"]
			ct.Default = "closed"
			ct.Description = "changed"
			m.Types["status"] = ct
		}},
		{"change property default", func(m *Metamodel) {
			d := m.Entities["ticket"]
			p := d.Properties["status"]
			p.Default = "closed"
			d.Properties["status"] = p
			m.Entities["ticket"] = d
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := baseMetamodel()
			tc.mutate(m)
			if got := m.RenderProjection().Hash(); got != base {
				t.Errorf("churny edit %q changed the projection hash (want stable):\n base=%s\n got =%s", tc.name, base, got)
			}
		})
	}
}

func TestRenderProjectionHash_ChangesOnRenderRelevantEdits(t *testing.T) {
	base := baseMetamodel().RenderProjection().Hash()

	tests := []struct {
		name   string
		mutate func(*Metamodel)
	}{
		{"change display_property", func(m *Metamodel) {
			d := m.Entities["ticket"]
			d.DisplayProperty = "status"
			m.Entities["ticket"] = d
		}},
		{"change property type", func(m *Metamodel) {
			d := m.Entities["ticket"]
			p := d.Properties["title"]
			p.Type = "text"
			d.Properties["title"] = p
			m.Entities["ticket"] = d
		}},
		{"change property required", func(m *Metamodel) {
			d := m.Entities["ticket"]
			p := d.Properties["status"]
			p.Required = true
			d.Properties["status"] = p
			m.Entities["ticket"] = d
		}},
		{"change property list flag", func(m *Metamodel) {
			d := m.Entities["ticket"]
			p := d.Properties["status"]
			p.List = true
			d.Properties["status"] = p
			m.Entities["ticket"] = d
		}},
		{"change property format", func(m *Metamodel) {
			d := m.Entities["ticket"]
			p := d.Properties["title"]
			p.Format = "2006-01-02"
			d.Properties["title"] = p
			m.Entities["ticket"] = d
		}},
		{"change enum values", func(m *Metamodel) {
			ct := m.Types["status"]
			ct.Values = []string{"open", "closed", "wont-fix"}
			m.Types["status"] = ct
		}},
		{"change property order", func(m *Metamodel) {
			d := m.Entities["ticket"]
			d.PropertyOrder = []string{"status", "title"}
			m.Entities["ticket"] = d
		}},
		{"add a property", func(m *Metamodel) {
			d := m.Entities["ticket"]
			d.Properties["priority"] = PropertyDef{Type: "string"}
			m.Entities["ticket"] = d
		}},
		{"add an entity type", func(m *Metamodel) {
			m.Entities["task"] = EntityDef{Label: "Task", Properties: map[string]PropertyDef{"x": {Type: "string"}}}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := baseMetamodel()
			tc.mutate(m)
			if got := m.RenderProjection().Hash(); got == base {
				t.Errorf("render-relevant edit %q did not change the projection hash", tc.name)
			}
		})
	}
}

// TestRenderProjectionHash_InlineEnumValues guards that inline enum value lists
// on a property (Values) are part of the render projection.
func TestRenderProjectionHash_InlineEnumValues(t *testing.T) {
	m := baseMetamodel()
	d := m.Entities["note"]
	p := d.Properties["body"]
	p.Values = []string{"a", "b"}
	d.Properties["body"] = p
	m.Entities["note"] = d
	withValues := m.RenderProjection().Hash()

	m2 := baseMetamodel()
	d2 := m2.Entities["note"]
	p2 := d2.Properties["body"]
	p2.Values = []string{"a", "b", "c"}
	d2.Properties["body"] = p2
	m2.Entities["note"] = d2
	if withValues == m2.RenderProjection().Hash() {
		t.Fatal("inline enum values are not reflected in the projection hash")
	}
}
