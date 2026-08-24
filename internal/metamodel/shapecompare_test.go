package metamodel

import "testing"

func hasDelta(r ShapeReport, kind string, tier ShapeTier) bool {
	for _, d := range r.Deltas {
		if d.Kind == kind && d.Tier == tier {
			return true
		}
	}
	return false
}

func TestCompareShapes_IdenticalIsEmpty(t *testing.T) {
	from := shapeFixture().ShapeProjection()
	to := shapeFixture().ShapeProjection()
	r := CompareShapes(from, to)
	if len(r.Deltas) != 0 {
		t.Fatalf("identical shapes produced deltas: %+v", r.Deltas)
	}
	if r.Tier() != TierAdditive || !r.Compatible() {
		t.Fatalf("empty report must be additive and compatible")
	}
}

func TestCompareShapes_Classification(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(m *Metamodel)
		kind   string
		tier   ShapeTier
	}{
		{
			name: "new entity type is additive",
			mutate: func(m *Metamodel) {
				m.Entities["note"] = EntityDef{Properties: map[string]PropertyDef{"body": {Type: "string"}}}
			},
			kind: "entity_type_added", tier: TierAdditive,
		},
		{
			name: "new optional property is additive",
			mutate: func(m *Metamodel) {
				m.Entities["task"].Properties["estimate"] = PropertyDef{Type: "integer"}
			},
			kind: "property_added", tier: TierAdditive,
		},
		{
			name: "new required property is drift",
			mutate: func(m *Metamodel) {
				m.Entities["task"].Properties["owner"] = PropertyDef{Type: "string", Required: true}
			},
			kind: "required_property_added", tier: TierDrift,
		},
		{
			name: "deleted property is drift",
			mutate: func(m *Metamodel) {
				delete(m.Entities["task"].Properties, "tags")
			},
			kind: "property_removed", tier: TierDrift,
		},
		{
			name: "deleted entity type is drift",
			mutate: func(m *Metamodel) {
				delete(m.Entities, "person")
			},
			kind: "entity_type_removed", tier: TierDrift,
		},
		{
			name: "property type change needs migration",
			mutate: func(m *Metamodel) {
				p := m.Entities["task"].Properties["due"]
				p.Type = "string"
				m.Entities["task"].Properties["due"] = p
			},
			kind: "property_type_changed", tier: TierMigration,
		},
		{
			name: "property format change needs migration",
			mutate: func(m *Metamodel) {
				p := m.Entities["task"].Properties["due"]
				p.Format = "02.01.2006"
				m.Entities["task"].Properties["due"] = p
			},
			kind: "property_format_changed", tier: TierMigration,
		},
		{
			name: "list flip needs migration",
			mutate: func(m *Metamodel) {
				p := m.Entities["task"].Properties["tags"]
				p.List = false
				m.Entities["task"].Properties["tags"] = p
			},
			kind: "property_list_changed", tier: TierMigration,
		},
		{
			name: "required flip on existing property is drift",
			mutate: func(m *Metamodel) {
				p := m.Entities["task"].Properties["status"]
				p.Required = true
				m.Entities["task"].Properties["status"] = p
			},
			kind: "property_became_required", tier: TierDrift,
		},
		{
			name: "required relaxed is additive",
			mutate: func(m *Metamodel) {
				p := m.Entities["task"].Properties["title"]
				p.Required = false
				m.Entities["task"].Properties["title"] = p
			},
			kind: "property_became_optional", tier: TierAdditive,
		},
		{
			name: "default-only change is additive",
			mutate: func(m *Metamodel) {
				p := m.Entities["task"].Properties["status"]
				p.Default = "doing"
				m.Entities["task"].Properties["status"] = p
			},
			kind: "property_default_changed", tier: TierAdditive,
		},
		{
			name: "enum value added is additive",
			mutate: func(m *Metamodel) {
				ct := m.Types["status"]
				ct.Values = []string{"todo", "doing", "done", "archived"}
				m.Types["status"] = ct
			},
			kind: "enum_values_added", tier: TierAdditive,
		},
		{
			name: "enum value removed is drift",
			mutate: func(m *Metamodel) {
				ct := m.Types["status"]
				ct.Values = []string{"todo", "done"}
				m.Types["status"] = ct
			},
			kind: "enum_values_removed", tier: TierDrift,
		},
		{
			name: "enum value replacement needs migration",
			mutate: func(m *Metamodel) {
				ct := m.Types["status"]
				ct.Values = []string{"open", "doing", "done"}
				m.Types["status"] = ct
			},
			kind: "enum_values_replaced", tier: TierMigration,
		},
		{
			name: "enum reorder is additive",
			mutate: func(m *Metamodel) {
				ct := m.Types["status"]
				ct.Values = []string{"done", "doing", "todo"}
				m.Types["status"] = ct
			},
			kind: "enum_values_reordered", tier: TierAdditive,
		},
		{
			name: "inline enum replacement needs migration",
			mutate: func(m *Metamodel) {
				p := m.Entities["task"].Properties["priority"]
				p.Values = []string{"lowest", "high"}
				m.Entities["task"].Properties["priority"] = p
			},
			kind: "enum_values_replaced", tier: TierMigration,
		},
		{
			name: "relation endpoint widened is additive",
			mutate: func(m *Metamodel) {
				r := m.Relations["assigned-to"]
				r.From = []string{"task", "person"}
				m.Relations["assigned-to"] = r
			},
			kind: "relation_endpoint_widened", tier: TierAdditive,
		},
		{
			name: "relation endpoint narrowed needs migration",
			mutate: func(m *Metamodel) {
				r := m.Relations["assigned-to"]
				r.To = []string{"task"}
				m.Relations["assigned-to"] = r
			},
			kind: "relation_endpoint_narrowed", tier: TierMigration,
		},
		{
			name: "relation cardinality loosened is additive",
			mutate: func(m *Metamodel) {
				r := m.Relations["assigned-to"]
				r.MaxOutgoing = nil
				m.Relations["assigned-to"] = r
			},
			kind: "relation_cardinality_loosened", tier: TierAdditive,
		},
		{
			name: "relation cardinality tightened needs migration",
			mutate: func(m *Metamodel) {
				one := 1
				r := m.Relations["assigned-to"]
				r.MinOutgoing = &one
				m.Relations["assigned-to"] = r
			},
			kind: "relation_cardinality_tightened", tier: TierMigration,
		},
		{
			name: "relation symmetry flip needs migration",
			mutate: func(m *Metamodel) {
				r := m.Relations["assigned-to"]
				r.Symmetric = true
				m.Relations["assigned-to"] = r
			},
			kind: "relation_symmetry_changed", tier: TierMigration,
		},
		{
			name: "relation content removed is drift",
			mutate: func(m *Metamodel) {
				r := m.Relations["assigned-to"]
				r.Content = false
				m.Relations["assigned-to"] = r
			},
			kind: "relation_content_removed", tier: TierDrift,
		},
		{
			name: "relation type removed is drift",
			mutate: func(m *Metamodel) {
				delete(m.Relations, "assigned-to")
			},
			kind: "relation_type_removed", tier: TierDrift,
		},
		{
			name: "relation property type change needs migration",
			mutate: func(m *Metamodel) {
				r := m.Relations["assigned-to"]
				r.Properties["weight"] = PropertyDef{Type: "string"}
				m.Relations["assigned-to"] = r
			},
			kind: "property_type_changed", tier: TierMigration,
		},
		{
			name: "named type added is additive",
			mutate: func(m *Metamodel) {
				m.Types["kind"] = CustomType{Values: []string{"a", "b"}}
			},
			kind: "named_type_added", tier: TierAdditive,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			from := shapeFixture().ShapeProjection()
			m := shapeFixture()
			tc.mutate(m)
			to := m.ShapeProjection()

			r := CompareShapes(from, to)
			if !hasDelta(r, tc.kind, tc.tier) {
				t.Fatalf("expected delta kind=%s tier=%s, got: %+v", tc.kind, tc.tier, r.Deltas)
			}
		})
	}
}

func TestCompareShapes_PossiblePropertyRename(t *testing.T) {
	from := shapeFixture().ShapeProjection()
	m := shapeFixture()
	// Delete "status" (type status) and add "state" with the same shape.
	p := m.Entities["task"].Properties["status"]
	delete(m.Entities["task"].Properties, "status")
	m.Entities["task"].Properties["state"] = p
	to := m.ShapeProjection()

	r := CompareShapes(from, to)
	if !hasDelta(r, "possible_property_rename", TierDrift) {
		t.Fatalf("expected possible_property_rename drift delta, got: %+v", r.Deltas)
	}
	// The pair itself must not force a migration.
	if !r.Compatible() {
		t.Fatalf("delete+add pair alone must stay compatible (auto-adopt with notice)")
	}
}

func TestCompareShapes_PossibleEntityTypeRename(t *testing.T) {
	from := shapeFixture().ShapeProjection()
	m := shapeFixture()
	def := m.Entities["person"]
	delete(m.Entities, "person")
	m.Entities["contact"] = def
	to := m.ShapeProjection()

	r := CompareShapes(from, to)
	if !hasDelta(r, "possible_entity_type_rename", TierDrift) {
		t.Fatalf("expected possible_entity_type_rename drift delta, got: %+v", r.Deltas)
	}
}

func TestShapeReport_TierReduction(t *testing.T) {
	r := ShapeReport{Deltas: []ShapeDelta{
		{Tier: TierAdditive}, {Tier: TierMigration}, {Tier: TierDrift},
	}}
	if r.Tier() != TierMigration {
		t.Fatalf("Tier() = %s, want needs-migration", r.Tier())
	}
	if r.Compatible() {
		t.Fatalf("report with a migration delta must not be compatible")
	}
	if got := len(r.ByTier(TierDrift)); got != 1 {
		t.Fatalf("ByTier(drift) = %d deltas, want 1", got)
	}
}
