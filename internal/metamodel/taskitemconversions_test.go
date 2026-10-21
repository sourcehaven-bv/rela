package metamodel

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const taskItemSchema = `version: "1"
entities:
  ticket:
    label: Ticket
    id_prefix: TKT-
    properties:
      title: {type: string}
      project: {type: string}
      score: {type: string, computed: "1"}
  task:
    label: Task
    id_prefix: TASK-
    properties:
      title: {type: string}
      project: {type: string}
      score: {type: string, computed: "1"}
  policy:
    label: Policy
    id_prefix: POL-
    faces:
      draft: {label: Draft}
    properties:
      title: {type: string}
relations:
  has-subtask:
    label: Has subtask
    from: [ticket]
    to: [task, policy]
task_item_conversions:
  ticket-subtask:
    from: ticket
    to: task
    label: Convert to task
    relation: {type: has-subtask, as: to}
    fields:
      title: "{{item.text}}"
      project: "{{ source.project }}"
`

func TestTaskItemConversionValid(t *testing.T) {
	m, err := Parse([]byte(taskItemSchema))
	if err != nil {
		t.Fatalf("valid recipe rejected: %v", err)
	}
	d := m.TaskItemConversions["ticket-subtask"]
	if d.ItemMode() != TaskItemLink {
		t.Fatalf("default item mode = %q, want %q", d.ItemMode(), TaskItemLink)
	}
}

func TestTaskItemConversionValidation(t *testing.T) {
	tests := []struct {
		name    string
		old     string
		new     string
		wantErr string
	}{
		{"unknown from", "from: ticket\n    to: task", "from: nope\n    to: task", `from: entity type "nope"`},
		{"unknown to", "to: task\n    label", "to: nope\n    label", `to: entity type "nope"`},
		{"missing label", "    label: Convert to task\n", "", "label is required"},
		{"bad item", "label: Convert to task\n", "label: Convert to task\n    item: keep\n", `item must be "link" or "remove"`},
		{"unknown relation", "type: has-subtask", "type: nope", `relation "nope" is not declared`},
		{"bad as", "as: to}", "as: sideways}", `relation.as must be`},
		{"relation ends do not fit as", "as: to}", "as: from}", `cannot originate from "task"`},
		{"no fields", "    fields:\n      title: \"{{item.text}}\"\n      project: \"{{ source.project }}\"\n", "", "requires a `fields:` mapping"},
		{"unknown target field", "title: \"{{item.text}}\"", "nope: \"{{item.text}}\"", `has no property "nope"`},
		{"computed target field", "title: \"{{item.text}}\"", "score: \"{{item.text}}\"", `"score" is computed`},
		{"unknown source property", "source.project", "source.nope", `source type "ticket" has no property "nope"`},
		{"disallowed namespace", "{{ source.project }}", "{{today}}", "{{today}} is not allowed"},
		{"new namespace refused", "{{ source.project }}", "{{new.project}}", "{{new.project}} is not allowed"},
		{"unbalanced braces", "{{ source.project }}", "{{ source.project", "unbalanced"},
		{"faced target without face", "to: task\n", "to: policy\n", `target type "policy" has faces`},
		{"face on faceless target", "label: Convert to task\n", "label: Convert to task\n    face: draft\n", `target type "task" has no faces`},
		{"bad name", "  ticket-subtask:", "  Ticket_Subtask:", "name must match"},
		{"no item text", "title: \"{{item.text}}\"", "title: \"fixed\"", "no field uses {{item.text}}"},
		{"text into integer", "id_prefix: TASK-\n    properties:\n      title: {type: string}", "id_prefix: TASK-\n    properties:\n      title: {type: integer}", "has type integer"},
		{"text into list", "id_prefix: TASK-\n    properties:\n      title: {type: string}", "id_prefix: TASK-\n    properties:\n      title: {type: string, list: true}", "is a list"},
		{"text into enum", "id_prefix: TASK-\n    properties:\n      title: {type: string}", "id_prefix: TASK-\n    properties:\n      title: {type: enum, values: [a, b]}", "takes one of a, b"},
		{"manual-id target", "id_prefix: TASK-\n", "id_prefix: TASK-\n    id_type: manual\n", "uses manual ids"},
		{
			"required file on target", "id_prefix: TASK-\n    properties:\n",
			"id_prefix: TASK-\n    properties:\n      doc: {type: file, required: true}\n", "required file",
		},
		{
			"required external reference on target", "id_prefix: TASK-\n    properties:\n",
			"id_prefix: TASK-\n    properties:\n      ext: {type: external_ref, required: true, external_ref: {provider: x}}\n",
			"required external_ref",
		},
		{
			"file target field", "id_prefix: TASK-\n    properties:\n      title: {type: string}",
			"id_prefix: TASK-\n    properties:\n      title: {type: file}", "\"title\" is a file and cannot be mapped",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(taskItemSchema, tc.old) {
				t.Fatalf("fixture does not contain %q", tc.old)
			}
			schema := strings.Replace(taskItemSchema, tc.old, tc.new, 1)
			_, err := Parse([]byte(schema))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestTaskItemConversionFacedTarget(t *testing.T) {
	schema := strings.Replace(taskItemSchema, "to: task\n", "to: policy\n    face: draft\n", 1)
	schema = strings.Replace(schema, "      project: \"{{ source.project }}\"\n", "", 1)
	if _, err := Parse([]byte(schema)); err != nil {
		t.Fatalf("faced target with face rejected: %v", err)
	}
	bad := strings.Replace(schema, "face: draft", "face: published", 1)
	if _, err := Parse([]byte(bad)); err == nil || !strings.Contains(err.Error(), `has no face "published"`) {
		t.Fatalf("undeclared face error = %v", err)
	}
}

func TestParseTaskItemTemplate(t *testing.T) {
	tests := []struct {
		template string
		want     []TemplateToken
		wantErr  bool
	}{
		{"{{item.text}}", []TemplateToken{{Kind: TokenItemText}}, false},
		{"Fix: {{ item.text }} ({{source.project}})", []TemplateToken{
			{Kind: TokenLiteral, Value: "Fix: "},
			{Kind: TokenItemText},
			{Kind: TokenLiteral, Value: " ("},
			{Kind: TokenSource, Value: "project"},
			{Kind: TokenLiteral, Value: ")"},
		}, false},
		{"plain", []TemplateToken{{Kind: TokenLiteral, Value: "plain"}}, false},
		{"", nil, false},
		{"{{user.name}}", nil, true},
		{"{{item.text", nil, true},
		{"}}", nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.template, func(t *testing.T) {
			got, err := ParseTaskItemTemplate(tc.template)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("tokens = %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("token %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
			if UsesItemText(got) != strings.Contains(tc.template, "item.text") {
				t.Fatalf("UsesItemText = %v", UsesItemText(got))
			}
		})
	}
}

func TestTaskItemConversionsAreRootOnly(t *testing.T) {
	dir := t.TempDir()
	root := "version: \"1\"\nincludes: [part.yaml]\nentities:\n  note: {label: Note, id_prefix: N-}\nrelations: {}\n"
	part := "task_item_conversions:\n  x:\n    from: note\n    to: note\n"
	if err := os.WriteFile(filepath.Join(dir, "schema.yaml"), []byte(root), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "part.yaml"), []byte(part), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := Load(filepath.Join(dir, "schema.yaml"), testMetaFS)
	var rootFieldErr *IncludeHasRootFieldError
	if !errors.As(err, &rootFieldErr) || rootFieldErr.Field != "task_item_conversions" {
		t.Fatalf("include error = %v", err)
	}
}
