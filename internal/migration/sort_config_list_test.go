package migration

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSortConfigListMigration_Detect(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected bool
	}{
		{
			name: "detects single sort object in list",
			yaml: `
lists:
  tickets:
    entity_type: ticket
    sort:
      property: priority
      direction: desc
`,
			expected: true,
		},
		{
			name: "detects single sort object in dashboard card",
			yaml: `
dashboard:
  cards:
    - title: Recent
      query: "type:ticket"
      display: table
      sort:
        property: modified
        direction: desc
`,
			expected: true,
		},
		{
			name: "no detection when sort is already a list",
			yaml: `
lists:
  tickets:
    entity_type: ticket
    sort:
      - property: priority
        direction: desc
`,
			expected: false,
		},
		{
			name: "no detection when no sort key",
			yaml: `
lists:
  tickets:
    entity_type: ticket
    columns:
      - property: title
`,
			expected: false,
		},
		{
			name: "detects sort object in one of multiple lists",
			yaml: `
lists:
  tickets:
    entity_type: ticket
    sort:
      - property: priority
        direction: desc
  bugs:
    entity_type: bug
    sort:
      property: severity
      direction: desc
`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.yaml), &doc); err != nil {
				t.Fatalf("failed to parse yaml: %v", err)
			}

			migration := &SortConfigListMigration{}
			result := migration.Detect(&doc)

			if result != tt.expected {
				t.Errorf("Detect() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSortConfigListMigration_Apply(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "converts single sort object to list",
			input: `lists:
  tickets:
    sort:
      property: priority
      direction: desc
`,
			expected: `lists:
    tickets:
        sort:
            - property: priority
              direction: desc
`,
		},
		{
			name: "converts sort in dashboard card",
			input: `dashboard:
  cards:
    - title: Recent
      sort:
        property: modified
        direction: desc
`,
			expected: `dashboard:
    cards:
        - title: Recent
          sort:
            - property: modified
              direction: desc
`,
		},
		{
			name: "preserves existing list format",
			input: `lists:
  tickets:
    sort:
      - property: priority
        direction: desc
`,
			expected: `lists:
    tickets:
        sort:
            - property: priority
              direction: desc
`,
		},
		{
			name: "converts multiple sort objects across sections",
			input: `lists:
  tickets:
    sort:
      property: priority
      direction: desc
dashboard:
  cards:
    - title: Recent
      sort:
        property: modified
        direction: desc
`,
			expected: `lists:
    tickets:
        sort:
            - property: priority
              direction: desc
dashboard:
    cards:
        - title: Recent
          sort:
            - property: modified
              direction: desc
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.input), &doc); err != nil {
				t.Fatalf("failed to parse input yaml: %v", err)
			}

			migration := &SortConfigListMigration{}
			if err := migration.Apply(&doc); err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			result, err := yaml.Marshal(&doc)
			if err != nil {
				t.Fatalf("failed to marshal result: %v", err)
			}

			if string(result) != tt.expected {
				t.Errorf("Apply() result mismatch\nGot:\n%s\nWant:\n%s", string(result), tt.expected)
			}
		})
	}
}
