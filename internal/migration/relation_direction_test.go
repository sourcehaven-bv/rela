package migration

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func directionTestMetamodel() *mockMetamodel {
	return &mockMetamodel{
		relations: map[string]mockRelationDef{
			// task is only ever the FROM; project only ever the TO.
			"belongs-to": {label: "belongs to", from: []string{"task"}, to: []string{"project"}},
			// self-referencing: task is on BOTH sides.
			"depends-on": {label: "depends on", from: []string{"task"}, to: []string{"task"}},
		},
	}
}

func applyDirectionMigration(t *testing.T, input string) (string, bool) {
	t.Helper()
	m := &RelationDirectionMigration{}
	m.SetMetamodel(directionTestMetamodel())

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	detected := m.Detect(&doc)
	if err := m.Apply(&doc); err != nil {
		t.Fatalf("apply: %v", err)
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(out), detected
}

func TestRelationDirectionMigration(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantDetect   bool
		wantContains []string
		wantAbsent   []string
	}{
		{
			name: "from-side binding gains outgoing",
			input: `
forms:
  edit_task:
    entity_type: task
    relations:
      - relation: belongs-to
`,
			wantDetect:   true,
			wantContains: []string{"direction: outgoing"},
		},
		{
			name: "to-side binding gains incoming",
			input: `
forms:
  edit_project:
    entity_type: project
    relations:
      - relation: belongs-to
`,
			wantDetect:   true,
			wantContains: []string{"direction: incoming"},
		},
		{
			name: "self-referencing binding is left for the author",
			input: `
forms:
  edit_task:
    entity_type: task
    relations:
      - relation: depends-on
`,
			wantDetect: false,
			wantAbsent: []string{"direction:"},
		},
		{
			name: "explicit direction is preserved",
			input: `
forms:
  edit_task:
    entity_type: task
    relations:
      - relation: belongs-to
        direction: incoming
`,
			wantDetect:   false,
			wantContains: []string{"direction: incoming"},
			wantAbsent:   []string{"direction: outgoing"},
		},
		{
			name: "wizard step relations are migrated too",
			input: `
forms:
  wizard_task:
    entity_type: task
    steps:
      - title: Step one
        relations:
          - relation: belongs-to
`,
			wantDetect:   true,
			wantContains: []string{"direction: outgoing"},
		},
		{
			name: "list relation column gains a direction",
			input: `
lists:
  projects:
    entity_type: project
    columns:
      - relation: belongs-to
`,
			wantDetect:   true,
			wantContains: []string{"direction: incoming"},
		},
		{
			name: "list filter control gains a direction",
			input: `
lists:
  tasks:
    entity_type: task
    filter_controls:
      - relation: belongs-to
`,
			wantDetect:   true,
			wantContains: []string{"direction: outgoing"},
		},
		{
			name: "kanban card field gains a direction",
			input: `
kanbans:
  board:
    entity_type: project
    card:
      fields:
        - relation: belongs-to
`,
			wantDetect:   true,
			wantContains: []string{"direction: incoming"},
		},
		{
			name: "kanban filter control gains a direction",
			input: `
kanbans:
  board:
    entity_type: task
    filter_controls:
      - relation: belongs-to
`,
			wantDetect:   true,
			wantContains: []string{"direction: outgoing"},
		},
		{
			// The edge runs member→driver, so entity_type (the member) anchors
			// the inference — NOT driver_type.
			name: "caldav dynamic collection gains a direction",
			input: `
caldav:
  dynamic:
    tasks-per-project:
      entity_type: task
      driver_type: project
      relation: belongs-to
`,
			wantDetect:   true,
			wantContains: []string{"direction: outgoing"},
		},
		{
			name: "self-referencing list column is left for the author",
			input: `
lists:
  tasks:
    entity_type: task
    columns:
      - relation: depends-on
`,
			wantDetect: false,
			wantAbsent: []string{"direction:"},
		},
		{
			name: "self-referencing caldav collection is left for the author",
			input: `
caldav:
  dynamic:
    blockers:
      entity_type: task
      driver_type: task
      relation: depends-on
`,
			wantDetect: false,
			wantAbsent: []string{"direction:"},
		},
		{
			name: "property-only list column is untouched",
			input: `
lists:
  tasks:
    entity_type: task
    columns:
      - property: title
`,
			wantDetect: false,
			wantAbsent: []string{"direction:"},
		},
		{
			name: "unknown relation is left alone",
			input: `
forms:
  edit_task:
    entity_type: task
    relations:
      - relation: no-such-relation
`,
			wantDetect: false,
			wantAbsent: []string{"direction:"},
		},
		{
			name: "form without entity_type is left alone",
			input: `
forms:
  edit_task:
    relations:
      - relation: belongs-to
`,
			wantDetect: false,
			wantAbsent: []string{"direction:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, detected := applyDirectionMigration(t, tt.input)
			if detected != tt.wantDetect {
				t.Errorf("Detect() = %v, want %v", detected, tt.wantDetect)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(out, want) {
					t.Errorf("output should contain %q:\n%s", want, out)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(out, absent) {
					t.Errorf("output should not contain %q:\n%s", absent, out)
				}
			}
		})
	}
}

// Without a metamodel nothing is inferable, so the migration must be inert
// rather than guessing.
func TestRelationDirectionMigration_NoMetamodel(t *testing.T) {
	m := &RelationDirectionMigration{}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(`
forms:
  edit_task:
    entity_type: task
    relations:
      - relation: belongs-to
`), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Detect(&doc) {
		t.Error("Detect() = true without a metamodel, want false")
	}
}
