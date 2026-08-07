package migration

import (
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// mockMetamodel implements MetamodelProvider for testing.
type mockMetamodel struct {
	entities  map[string]mockEntityDef
	relations map[string]mockRelationDef
	types     map[string]mockCustomType
}

type mockEntityDef struct {
	properties map[string]mockPropertyDef
}

type mockPropertyDef struct {
	propType string
	required bool
	defValue string
}

type mockRelationDef struct {
	label string
	from  []string
	to    []string
}

type mockCustomType struct {
	values   []string
	defValue string
}

func (m *mockMetamodel) GetPropertyType(entityType, property string) string {
	if ent, ok := m.entities[entityType]; ok {
		if prop, ok := ent.properties[property]; ok {
			return prop.propType
		}
	}
	return ""
}

func (m *mockMetamodel) IsPropertyRequired(entityType, property string) bool {
	if ent, ok := m.entities[entityType]; ok {
		if prop, ok := ent.properties[property]; ok {
			return prop.required
		}
	}
	return false
}

func (m *mockMetamodel) GetPropertyDefault(entityType, property string) string {
	if ent, ok := m.entities[entityType]; ok {
		if prop, ok := ent.properties[property]; ok {
			return prop.defValue
		}
	}
	return ""
}

func (m *mockMetamodel) GetTypeDefault(typeName string) string {
	if ct, ok := m.types[typeName]; ok {
		return ct.defValue
	}
	return ""
}

func (m *mockMetamodel) IsEnumType(typeName string) bool {
	if ct, ok := m.types[typeName]; ok {
		return len(ct.values) > 0
	}
	return false
}

func (m *mockMetamodel) GetRelationLabel(relation string) string {
	if rel, ok := m.relations[relation]; ok {
		return rel.label
	}
	return ""
}

func (m *mockMetamodel) GetRelationFrom(relation string) []string {
	if rel, ok := m.relations[relation]; ok {
		return rel.from
	}
	return nil
}

func (m *mockMetamodel) GetRelationTo(relation string) []string {
	if rel, ok := m.relations[relation]; ok {
		return rel.to
	}
	return nil
}

func (m *mockMetamodel) ResolveWidgetFromType(propType string) string {
	switch propType {
	case "string":
		return "text"
	case "date":
		return "date"
	case "integer":
		return "number"
	case "boolean":
		return "checkbox"
	case "enum":
		return "select"
	default:
		if ct, ok := m.types[propType]; ok && len(ct.values) > 0 {
			return "select"
		}
		return "text"
	}
}

func TestDataEntryCleanupMigration_Detect(t *testing.T) {
	m := &DataEntryCleanupMigration{}

	tests := []struct {
		name   string
		yaml   string
		expect bool
	}{
		{
			// DEC-6C1NAA: a title-cased label is NOT redundant. It used to be
			// stripped, which silently downgraded the field to its raw
			// identifier (BUG-8N2WT2).
			name: "does not detect title-cased label in form field",
			yaml: `
forms:
  create_ticket:
    fields:
      - property: title
        label: "Title"
`,
			expect: false,
		},
		{
			name: "does not detect title-cased label in list column",
			yaml: `
lists:
  all_tickets:
    columns:
      - property: status
        label: "Status"
`,
			expect: false,
		},
		{
			name: "detects redundant widget: select in relation",
			yaml: `
forms:
  create_ticket:
    relations:
      - relation: belongs-to
        widget: select
`,
			expect: true,
		},
		{
			name: "does not detect custom label",
			yaml: `
forms:
  create_ticket:
    fields:
      - property: assignee
        label: "Assign to"
`,
			expect: false,
		},
		{
			name: "does not detect non-default widget",
			yaml: `
forms:
  create_ticket:
    relations:
      - relation: tagged
        widget: multi-select
`,
			expect: false,
		},
		{
			// The multi-word case is where the old behavior was most
			// damaging: "Due Date" was stripped and rendered as `due_date`.
			name: "does not detect snake_case property label",
			yaml: `
forms:
  create_ticket:
    fields:
      - property: due_date
        label: "Due Date"
`,
			expect: false,
		},
		{
			name: "does not match partial label",
			yaml: `
forms:
  create_ticket:
    fields:
      - property: due_date
        label: "Due"
`,
			expect: false,
		},
		{
			name: "empty config returns false",
			yaml: `
version: "1.0"
`,
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.yaml), &doc); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			got := m.Detect(&doc)
			if got != tt.expect {
				t.Errorf("Detect() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestDataEntryCleanupMigration_DetectWithMetamodel(t *testing.T) {
	meta := &mockMetamodel{
		types: map[string]mockCustomType{
			"priority": {values: []string{"low", "medium", "high"}, defValue: "medium"},
			"status":   {values: []string{"open", "closed"}, defValue: "open"},
		},
		entities: map[string]mockEntityDef{
			"ticket": {
				properties: map[string]mockPropertyDef{
					"title":    {propType: "string", required: true},
					"priority": {propType: "priority", required: true},
					"status":   {propType: "status"},
					"due_date": {propType: "date"},
					"count":    {propType: "integer"},
					"active":   {propType: "boolean"},
				},
			},
		},
		relations: map[string]mockRelationDef{
			"belongs-to": {label: "belongs to", from: []string{"ticket"}, to: []string{"category"}},
			"blocks":     {label: "blocks", from: []string{"ticket"}, to: []string{"ticket"}},
		},
	}

	tests := []struct {
		name   string
		yaml   string
		expect bool
	}{
		{
			name: "detects redundant widget matching type",
			yaml: `
forms:
  create_ticket:
    entity_type: ticket
    fields:
      - property: due_date
        widget: date
`,
			expect: true,
		},
		{
			name: "detects redundant widget for integer",
			yaml: `
forms:
  create_ticket:
    entity_type: ticket
    fields:
      - property: count
        widget: number
`,
			expect: true,
		},
		{
			name: "detects redundant widget for boolean",
			yaml: `
forms:
  create_ticket:
    entity_type: ticket
    fields:
      - property: active
        widget: checkbox
`,
			expect: true,
		},
		{
			name: "detects redundant widget for custom type (select)",
			yaml: `
forms:
  create_ticket:
    entity_type: ticket
    fields:
      - property: priority
        widget: select
`,
			expect: true,
		},
		{
			name: "detects redundant required matching metamodel",
			yaml: `
forms:
  create_ticket:
    entity_type: ticket
    fields:
      - property: title
        required: true
`,
			expect: true,
		},
		{
			name: "detects redundant default matching type default",
			yaml: `
forms:
  create_ticket:
    entity_type: ticket
    fields:
      - property: priority
        default: medium
`,
			expect: true,
		},
		{
			name: "detects redundant direction when unambiguous",
			yaml: `
forms:
  create_ticket:
    entity_type: ticket
    relations:
      - relation: belongs-to
        direction: outgoing
`,
			expect: true,
		},
		{
			name: "detects redundant target_type when single target",
			yaml: `
forms:
  create_ticket:
    entity_type: ticket
    relations:
      - relation: belongs-to
        target_type: category
`,
			expect: true,
		},
		{
			name: "detects redundant relation label matching metamodel",
			yaml: `
forms:
  create_ticket:
    entity_type: ticket
    relations:
      - relation: belongs-to
        label: "belongs to"
`,
			expect: true,
		},
		{
			name: "does not detect non-redundant direction (ambiguous)",
			yaml: `
forms:
  create_ticket:
    entity_type: ticket
    relations:
      - relation: blocks
        direction: outgoing
`,
			expect: false,
		},
		{
			name: "does not detect non-redundant widget (textarea for string)",
			yaml: `
forms:
  create_ticket:
    entity_type: ticket
    fields:
      - property: title
        widget: textarea
`,
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &DataEntryCleanupMigration{}
			m.SetMetamodel(meta)

			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.yaml), &doc); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			got := m.Detect(&doc)
			if got != tt.expect {
				t.Errorf("Detect() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestDataEntryCleanupMigration_Apply(t *testing.T) {
	m := &DataEntryCleanupMigration{}

	tests := []struct {
		name       string
		input      string
		wantAbsent []string
		wantKeep   []string
	}{
		{
			// DEC-6C1NAA: labels survive, whatever they look like.
			name: "keeps title-cased label on form field",
			input: `
forms:
  create_ticket:
    fields:
      - property: title
        label: "Title"
        placeholder: "Enter title"
`,
			wantAbsent: nil,
			wantKeep:   []string{"property: title", "placeholder:", "Title"},
		},
		{
			name: "keeps title-cased label on list column",
			input: `
lists:
  all_tickets:
    columns:
      - property: status
        label: "Status"
        sortable: true
`,
			wantAbsent: nil,
			wantKeep:   []string{"property: status", "sortable: true", "Status"},
		},
		{
			name: "removes widget: select from relation",
			input: `
forms:
  create_ticket:
    relations:
      - relation: belongs-to
        widget: select
        required: true
`,
			wantAbsent: []string{"widget: select"},
			wantKeep:   []string{"relation: belongs-to", "required: true"},
		},
		{
			name: "keeps custom labels",
			input: `
forms:
  create_ticket:
    fields:
      - property: assignee
        label: "Assign to"
`,
			wantAbsent: []string{},
			wantKeep:   []string{"label:", "Assign to", "property: assignee"},
		},
		{
			name: "keeps non-default widgets",
			input: `
forms:
  create_ticket:
    relations:
      - relation: tagged
        widget: multi-select
`,
			wantAbsent: []string{},
			wantKeep:   []string{"widget: multi-select"},
		},
		{
			name: "strips only the redundant widget, never a label",
			input: `
forms:
  create_ticket:
    fields:
      - property: title
        label: "Title"
      - property: assignee
        label: "Assign to"
    relations:
      - relation: belongs-to
        widget: select
      - relation: tagged
        widget: multi-select
`,
			wantAbsent: []string{"widget: select"},
			wantKeep:   []string{"Title", "Assign to", "widget: multi-select"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.input), &doc); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			if err := m.Apply(&doc); err != nil {
				t.Fatalf("Apply() error: %v", err)
			}

			output, err := yaml.Marshal(&doc)
			if err != nil {
				t.Fatalf("failed to marshal result: %v", err)
			}
			result := string(output)

			for _, absent := range tt.wantAbsent {
				if strings.Contains(result, absent) {
					t.Errorf("output should not contain %q:\n%s", absent, result)
				}
			}

			for _, keep := range tt.wantKeep {
				if !strings.Contains(result, keep) {
					t.Errorf("output should contain %q:\n%s", keep, result)
				}
			}
		})
	}
}

func TestDataEntryCleanupMigration_ApplyWithMetamodel(t *testing.T) {
	meta := &mockMetamodel{
		types: map[string]mockCustomType{
			"priority": {values: []string{"low", "medium", "high"}, defValue: "medium"},
		},
		entities: map[string]mockEntityDef{
			"ticket": {
				properties: map[string]mockPropertyDef{
					"title":    {propType: "string", required: true},
					"priority": {propType: "priority"},
					"due_date": {propType: "date"},
				},
			},
		},
		relations: map[string]mockRelationDef{
			"belongs-to": {label: "belongs to", from: []string{"ticket"}, to: []string{"category"}},
		},
	}

	tests := []struct {
		name       string
		input      string
		wantAbsent []string
		wantKeep   []string
	}{
		{
			name: "removes redundant widget for date type",
			input: `
forms:
  create_ticket:
    entity_type: ticket
    fields:
      - property: due_date
        widget: date
        label: "Due"
`,
			wantAbsent: []string{"widget: date"},
			wantKeep:   []string{"property: due_date", "label: "},
		},
		{
			name: "removes redundant required",
			input: `
forms:
  create_ticket:
    entity_type: ticket
    fields:
      - property: title
        required: true
`,
			wantAbsent: []string{"required: true"},
			wantKeep:   []string{"property: title"},
		},
		{
			name: "removes redundant default",
			input: `
forms:
  create_ticket:
    entity_type: ticket
    fields:
      - property: priority
        default: medium
`,
			wantAbsent: []string{"default: medium"},
			wantKeep:   []string{"property: priority"},
		},
		{
			name: "removes redundant direction and target_type",
			input: `
forms:
  create_ticket:
    entity_type: ticket
    relations:
      - relation: belongs-to
        direction: outgoing
        target_type: category
        required: true
`,
			wantAbsent: []string{"direction: outgoing", "target_type: category"},
			wantKeep:   []string{"relation: belongs-to", "required: true"},
		},
		{
			name: "removes redundant relation label",
			input: `
forms:
  create_ticket:
    entity_type: ticket
    relations:
      - relation: belongs-to
        label: "belongs to"
`,
			wantAbsent: []string{"label: belongs to", `label: "belongs to"`},
			wantKeep:   []string{"relation: belongs-to"},
		},
		{
			name: "keeps custom relation label",
			input: `
forms:
  create_ticket:
    entity_type: ticket
    relations:
      - relation: belongs-to
        label: "Category"
`,
			wantAbsent: []string{},
			wantKeep:   []string{"label:", "Category"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &DataEntryCleanupMigration{}
			m.SetMetamodel(meta)

			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.input), &doc); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			if err := m.Apply(&doc); err != nil {
				t.Fatalf("Apply() error: %v", err)
			}

			output, err := yaml.Marshal(&doc)
			if err != nil {
				t.Fatalf("failed to marshal result: %v", err)
			}
			result := string(output)

			for _, absent := range tt.wantAbsent {
				if strings.Contains(result, absent) {
					t.Errorf("output should not contain %q:\n%s", absent, result)
				}
			}

			for _, keep := range tt.wantKeep {
				if !strings.Contains(result, keep) {
					t.Errorf("output should contain %q:\n%s", keep, result)
				}
			}
		})
	}
}

// TestLabelsAreNeverStripped pins DEC-6C1NAA: a label is authored, never
// derived, so the migration must not remove one just because it happens to
// match a title-cased identifier.
//
// This is the direct regression test for BUG-8N2WT2. Previously each of these
// labels satisfied isRedundantLabel and was deleted, and because the server
// refuses to start on unmigrated config the user could not decline the
// downgrade: the field then rendered its raw identifier forever.
func TestLabelsAreNeverStripped(t *testing.T) {
	labels := []struct {
		property string
		label    string
	}{
		{"title", "Title"},
		{"due_date", "Due Date"},
		{"first-name", "First Name"},
		{"estimated_hours", "Estimated Hours"},
		{"is_blocked", "Is Blocked"},
	}

	for _, tt := range labels {
		t.Run(tt.property, func(t *testing.T) {
			yamlStr := `forms:
  test_form:
    entity_type: ticket
    fields:
      - property: ` + tt.property + `
        label: "` + tt.label + `"
`
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(yamlStr), &doc); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			m := &DataEntryCleanupMigration{}
			if m.Detect(&doc) {
				t.Errorf("Detect() = true for label %q on property %q; a title-cased "+
					"label must not be treated as redundant (server would refuse to start)",
					tt.label, tt.property)
			}

			if err := m.Apply(&doc); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			out, err := yaml.Marshal(&doc)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if !strings.Contains(string(out), tt.label) {
				t.Errorf("Apply() stripped label %q; labels are authored, never derived\ngot:\n%s",
					tt.label, out)
			}
		})
	}
}

// TestListColumnLabelsAreNeverStripped covers the same rule for list columns,
// which the migration used to clean up via the same predicate.
func TestListColumnLabelsAreNeverStripped(t *testing.T) {
	yamlStr := `lists:
  tickets:
    columns:
      - property: due_date
        label: "Due Date"
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlStr), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	m := &DataEntryCleanupMigration{}
	if m.Detect(&doc) {
		t.Error("Detect() = true for a title-cased list column label; want false")
	}
	if err := m.Apply(&doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), "Due Date") {
		t.Errorf("Apply() stripped a list column label\ngot:\n%s", out)
	}
}

// TestRelationLabelStrippedOnlyWhenMetamodelSupplied pins the one label the
// migration may still remove: a relation label duplicating the metamodel's own
// label for that relation type. That value is server-authored and served to the
// SPA, which recovers it from relationType.label — a derivation from an
// authored label, not from an identifier.
//
// The titleCase(relation) arm is gone, so a label matching only the
// title-cased relation name must now survive.
func TestRelationLabelStrippedOnlyWhenMetamodelSupplied(t *testing.T) {
	const relYAML = `forms:
  test_form:
    entity_type: ticket
    relations:
      - relation: blocked_by
        label: "%s"
        widget: cards
`

	t.Run("metamodel label is stripped", func(t *testing.T) {
		var doc yaml.Node
		if err := yaml.Unmarshal(fmt.Appendf(nil, relYAML, "Blocked By"), &doc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		m := &DataEntryCleanupMigration{}
		m.SetMetamodel(&mockMetamodel{
			relations: map[string]mockRelationDef{"blocked_by": {label: "Blocked By"}},
		})

		if !m.Detect(&doc) {
			t.Fatal("Detect() = false; a label duplicating the metamodel label is redundant")
		}
		if err := m.Apply(&doc); err != nil {
			t.Fatalf("Apply: %v", err)
		}
		out, _ := yaml.Marshal(&doc)
		if strings.Contains(string(out), "Blocked By") {
			t.Errorf("Apply() kept a label duplicating the metamodel label\ngot:\n%s", out)
		}
	})

	t.Run("title-cased relation name survives without a metamodel match", func(t *testing.T) {
		var doc yaml.Node
		if err := yaml.Unmarshal(fmt.Appendf(nil, relYAML, "Blocked By"), &doc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		// No metamodel label for this relation, so the only thing "Blocked By"
		// could match is titleCase("blocked_by") — which is no longer a reason
		// to strip it.
		m := &DataEntryCleanupMigration{}
		m.SetMetamodel(&mockMetamodel{})

		if err := m.Apply(&doc); err != nil {
			t.Fatalf("Apply: %v", err)
		}
		out, _ := yaml.Marshal(&doc)
		if !strings.Contains(string(out), "Blocked By") {
			t.Errorf("Apply() stripped a relation label that only matched titleCase\ngot:\n%s", out)
		}
	})
}

func TestResolveWidgetFromType(t *testing.T) {
	meta := &mockMetamodel{
		types: map[string]mockCustomType{
			"priority": {values: []string{"low", "high"}},
		},
	}

	tests := []struct {
		propType string
		want     string
	}{
		{"string", "text"},
		{"date", "date"},
		{"integer", "number"},
		{"boolean", "checkbox"},
		{"enum", "select"},
		{"priority", "select"},
		{"unknown", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.propType, func(t *testing.T) {
			got := meta.ResolveWidgetFromType(tt.propType)
			if got != tt.want {
				t.Errorf("ResolveWidgetFromType(%q) = %q, want %q", tt.propType, got, tt.want)
			}
		})
	}
}
