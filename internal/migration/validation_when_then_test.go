package migration

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidationWhenThenMigration_Detect(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantFind bool
	}{
		{
			name: "detects match field",
			yaml: `
version: "1.0"
validations:
  - match:
      - entity_type: requirement
      - status: approved
    require:
      - has_outgoing: implements
`,
			wantFind: true,
		},
		{
			name: "detects require field",
			yaml: `
version: "1.0"
validations:
  - require:
      - has_outgoing: implements
`,
			wantFind: true,
		},
		{
			name: "detects both fields",
			yaml: `
version: "1.0"
validations:
  - match:
      - entity_type: requirement
    require:
      - has_outgoing: implements
  - match:
      - status: draft
    require:
      - has_property: description
`,
			wantFind: true,
		},
		{
			name: "no detection for when/then",
			yaml: `
version: "1.0"
validations:
  - when:
      - entity_type: requirement
      - status: approved
    then:
      - has_outgoing: implements
`,
			wantFind: false,
		},
		{
			name: "no detection when no validations section",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
`,
			wantFind: false,
		},
		{
			name: "no detection for empty validations",
			yaml: `
version: "1.0"
validations: []
`,
			wantFind: false,
		},
	}

	m := &ValidationWhenThenMigration{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.yaml), &doc); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			got := m.Detect(&doc)
			if got != tt.wantFind {
				t.Errorf("Detect() = %v, want %v", got, tt.wantFind)
			}
		})
	}
}

func TestValidationWhenThenMigration_Apply(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantWhen    bool
		wantThen    bool
		wantNoMatch bool
		wantNoReq   bool
	}{
		{
			name: "renames match to when",
			yaml: `
version: "1.0"
validations:
  - match:
      - entity_type: requirement
    require:
      - has_outgoing: implements
`,
			wantWhen:    true,
			wantThen:    true,
			wantNoMatch: true,
			wantNoReq:   true,
		},
		{
			name: "renames require to then",
			yaml: `
version: "1.0"
validations:
  - require:
      - has_outgoing: implements
`,
			wantThen:  true,
			wantNoReq: true,
		},
		{
			name: "handles multiple rules",
			yaml: `
version: "1.0"
validations:
  - match:
      - entity_type: requirement
    require:
      - has_outgoing: implements
  - match:
      - status: draft
    require:
      - has_property: description
`,
			wantWhen:    true,
			wantThen:    true,
			wantNoMatch: true,
			wantNoReq:   true,
		},
		{
			name: "leaves when/then unchanged",
			yaml: `
version: "1.0"
validations:
  - when:
      - entity_type: requirement
    then:
      - has_outgoing: implements
`,
			wantWhen:    true,
			wantThen:    true,
			wantNoMatch: true,
			wantNoReq:   true,
		},
	}

	m := &ValidationWhenThenMigration{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.yaml), &doc); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			if err := m.Apply(&doc); err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			// Re-encode and check the result
			output, err := yaml.Marshal(&doc)
			if err != nil {
				t.Fatalf("failed to marshal YAML: %v", err)
			}

			outputStr := string(output)

			// Check for presence of when: field
			if tt.wantWhen && !strings.Contains(outputStr, "when:") {
				t.Error("output should contain 'when:' field")
			}

			// Check for presence of then: field
			if tt.wantThen && !strings.Contains(outputStr, "then:") {
				t.Error("output should contain 'then:' field")
			}

			// Verify deprecated fields are not present
			if tt.wantNoMatch && strings.Contains(outputStr, "match:") {
				t.Error("output still contains 'match:' field")
			}
			if tt.wantNoReq && strings.Contains(outputStr, "require:") {
				t.Error("output still contains 'require:' field")
			}
		})
	}
}

func TestValidationWhenThenMigration_PreservesComments(t *testing.T) {
	input := `# Metamodel config
version: "1.0"

validations:
  # Requirement validation
  - match:  # Match conditions
      - entity_type: requirement
    require:  # Requirements
      - has_outgoing: implements
`

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	m := &ValidationWhenThenMigration{}
	if err := m.Apply(&doc); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	// Re-encode and check for comments
	output, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatalf("failed to marshal YAML: %v", err)
	}

	outputStr := string(output)

	// Check that the fields were changed
	if !strings.Contains(outputStr, "when:") {
		t.Error("match was not changed to when")
	}
	if !strings.Contains(outputStr, "then:") {
		t.Error("require was not changed to then")
	}

	// Check that comments are preserved (yaml.v3 preserves head comments)
	if !strings.Contains(outputStr, "# Metamodel config") {
		t.Error("top comment was not preserved")
	}
	if !strings.Contains(outputStr, "# Requirement validation") {
		t.Error("validation comment was not preserved")
	}
}

func TestValidationWhenThenMigration_NoValidationsSection(t *testing.T) {
	input := `
version: "1.0"
entities:
  requirement:
    label: Requirement
`

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	m := &ValidationWhenThenMigration{}

	// Should not detect
	if m.Detect(&doc) {
		t.Error("Detect() should return false when no validations section exists")
	}

	// Should not error on Apply
	if err := m.Apply(&doc); err != nil {
		t.Errorf("Apply() should not error when no validations section exists: %v", err)
	}
}
