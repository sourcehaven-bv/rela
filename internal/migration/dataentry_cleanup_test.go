package migration

import (
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

func TestDataEntryCleanupMigration_Detect(t *testing.T) {
	m := &DataEntryCleanupMigration{}

	tests := []struct {
		name   string
		yaml   string
		expect bool
	}{
		{
			name: "detects redundant label in form field",
			yaml: `
forms:
  create_ticket:
    fields:
      - property: title
        label: "Title"
`,
			expect: true,
		},
		{
			name: "detects redundant label in list column",
			yaml: `
lists:
  all_tickets:
    columns:
      - property: status
        label: "Status"
`,
			expect: true,
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
			name: "handles snake_case property correctly",
			yaml: `
forms:
  create_ticket:
    fields:
      - property: due_date
        label: "Due Date"
`,
			expect: true,
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
			name: "removes redundant label from form field",
			input: `
forms:
  create_ticket:
    fields:
      - property: title
        label: "Title"
        placeholder: "Enter title"
`,
			wantAbsent: []string{`label: "Title"`, `label: Title`},
			wantKeep:   []string{"property: title", "placeholder:"},
		},
		{
			name: "removes redundant label from list column",
			input: `
lists:
  all_tickets:
    columns:
      - property: status
        label: "Status"
        sortable: true
`,
			wantAbsent: []string{`label: "Status"`, `label: Status`},
			wantKeep:   []string{"property: status", "sortable: true"},
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
			name: "handles mixed redundant and custom",
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
			wantAbsent: []string{`label: "Title"`, `label: Title`},
			wantKeep:   []string{"Assign to", "widget: multi-select"},
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

func TestTitleCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"title", "Title"},
		{"due_date", "Due Date"},
		{"first-name", "First Name"},
		{"status", "Status"},
		{"estimated_hours", "Estimated Hours"},
		{"is_blocked", "Is Blocked"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := titleCase(tt.input)
			if got != tt.want {
				t.Errorf("titleCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
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
			got := resolveWidgetFromType(tt.propType, meta)
			if got != tt.want {
				t.Errorf("resolveWidgetFromType(%q) = %q, want %q", tt.propType, got, tt.want)
			}
		})
	}
}
