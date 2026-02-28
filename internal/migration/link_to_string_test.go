package migration

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLinkToStringMigration_Name(t *testing.T) {
	m := &LinkToStringMigration{}
	if got := m.Name(); got != "link-to-string" {
		t.Errorf("Name() = %q, want %q", got, "link-to-string")
	}
}

func TestLinkToStringMigration_FileTypes(t *testing.T) {
	m := &LinkToStringMigration{}
	ft := m.FileTypes()
	if len(ft) != 1 || ft[0] != FileTypeDataEntry {
		t.Errorf("FileTypes() = %v, want [FileTypeDataEntry]", ft)
	}
}

func TestLinkToStringMigration_Detect(t *testing.T) {
	tests := []struct {
		name   string
		yaml   string
		expect bool
	}{
		{
			name: "list with link: true",
			yaml: `
lists:
  tickets:
    columns:
      - property: title
        link: true
`,
			expect: true,
		},
		{
			name: "list with link: detail (already migrated)",
			yaml: `
lists:
  tickets:
    columns:
      - property: title
        link: detail
`,
			expect: false,
		},
		{
			name: "list without link",
			yaml: `
lists:
  tickets:
    columns:
      - property: title
`,
			expect: false,
		},
		{
			name: "view section with link: true",
			yaml: `
views:
  ticket_detail:
    sections:
      - heading: Items
        link: true
`,
			expect: true,
		},
		{
			name: "view section columns with link: true",
			yaml: `
views:
  ticket_detail:
    sections:
      - heading: Items
        columns:
          - property: status
            link: true
`,
			expect: true,
		},
		{
			name: "view with link: detail (already migrated)",
			yaml: `
views:
  ticket_detail:
    sections:
      - heading: Items
        link: detail
`,
			expect: false,
		},
		{
			name: "mixed - some migrated some not",
			yaml: `
lists:
  tickets:
    columns:
      - property: title
        link: detail
      - property: id
        link: true
`,
			expect: true,
		},
		{
			name:   "empty document",
			yaml:   `{}`,
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.yaml), &doc); err != nil {
				t.Fatalf("failed to parse yaml: %v", err)
			}

			m := &LinkToStringMigration{}
			got := m.Detect(&doc)
			if got != tt.expect {
				t.Errorf("Detect() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestLinkToStringMigration_Apply(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "list columns link: true to detail",
			input: `
lists:
  tickets:
    columns:
      - property: title
        link: true
      - property: status
`,
			expect: `link: detail`,
		},
		{
			name: "view section link: true to detail",
			input: `
views:
  ticket_detail:
    sections:
      - heading: Items
        link: true
`,
			expect: `link: detail`,
		},
		{
			name: "view section columns link: true to detail",
			input: `
views:
  ticket_detail:
    sections:
      - heading: Items
        columns:
          - property: status
            link: true
`,
			expect: `link: detail`,
		},
		{
			name: "preserves link: detail unchanged",
			input: `
lists:
  tickets:
    columns:
      - property: title
        link: detail
`,
			expect: `link: detail`,
		},
		{
			name: "multiple conversions",
			input: `
lists:
  tickets:
    columns:
      - property: title
        link: true
      - property: id
        link: true
views:
  detail:
    sections:
      - heading: X
        link: true
`,
			expect: `link: detail`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.input), &doc); err != nil {
				t.Fatalf("failed to parse yaml: %v", err)
			}

			m := &LinkToStringMigration{}
			if err := m.Apply(&doc); err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			// Marshal back and check
			out, err := yaml.Marshal(&doc)
			if err != nil {
				t.Fatalf("failed to marshal yaml: %v", err)
			}

			// Should contain expected string
			if !strings.Contains(string(out), tt.expect) {
				t.Errorf("Apply() result doesn't contain %q:\n%s", tt.expect, out)
			}

			// Should not contain "link: true" after migration
			if strings.Contains(string(out), "link: true") {
				t.Errorf("Apply() result still contains 'link: true':\n%s", out)
			}
		})
	}
}

func TestLinkToStringMigration_ApplyPreservesOtherFields(t *testing.T) {
	input := `
lists:
  tickets:
    entity_type: ticket
    columns:
      - property: title
        link: true
        sortable: true
        label: Title
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatalf("failed to parse yaml: %v", err)
	}

	m := &LinkToStringMigration{}
	if err := m.Apply(&doc); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatalf("failed to marshal yaml: %v", err)
	}

	result := string(out)
	// Check other fields are preserved
	if !strings.Contains(result, "sortable: true") {
		t.Errorf("Apply() lost sortable field:\n%s", result)
	}
	if !strings.Contains(result, "label: Title") {
		t.Errorf("Apply() lost label field:\n%s", result)
	}
	if !strings.Contains(result, "entity_type: ticket") {
		t.Errorf("Apply() lost entity_type field:\n%s", result)
	}
}
