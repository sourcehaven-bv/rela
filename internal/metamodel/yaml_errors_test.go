package metamodel

import (
	"strings"
	"testing"
)

func TestHumanizeYAMLErrors(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantMsgs []string // substrings expected in the error
		notWant  []string // substrings that should NOT appear
	}{
		{
			name: "string instead of list for aliases",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    aliases: req
    properties:
      title:
        type: string
`,
			wantMsgs: []string{"expected a list", "got a string"},
			notWant:  []string{"!!str", "!!seq", "[]string"},
		},
		{
			name: "list instead of mapping for properties",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      - title
      - description
`,
			wantMsgs: []string{"expected a mapping", "got a list"},
			notWant:  []string{"!!seq", "PropertyDef"},
		},
		{
			name: "string instead of list for from in relation",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
relations:
  addresses:
    label: addresses
    from: decision
    to: requirement
`,
			wantMsgs: []string{"expected a list", "got a string"},
			notWant:  []string{"!!str", "[]string"},
		},
		{
			name: "string instead of bool for required",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
        required: "true"
`,
			wantMsgs: []string{"expected true/false", "got a string"},
			notWant:  []string{"!!str", "bool"},
		},
		{
			name: "string instead of int for cardinality",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
relations:
  test:
    label: test
    from: [requirement]
    to: [requirement]
    min_outgoing: "one"
`,
			wantMsgs: []string{"expected a number", "got a string"},
			notWant:  []string{"!!str", "int"},
		},
		{
			name: "map instead of list for validations",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
validations:
  my-rule:
    then:
      - "title!="
`,
			wantMsgs: []string{"expected a list", "got a mapping"},
			notWant:  []string{"!!map", "ValidationRule"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			errMsg := err.Error()

			for _, want := range tt.wantMsgs {
				if !strings.Contains(errMsg, want) {
					t.Errorf("expected error to contain %q, got: %s", want, errMsg)
				}
			}

			for _, notWant := range tt.notWant {
				if strings.Contains(errMsg, notWant) {
					t.Errorf("error should not contain Go internal %q, got: %s", notWant, errMsg)
				}
			}
		})
	}
}

func TestHumanizeYAMLError_NonTypeError(t *testing.T) {
	// Non-yaml.TypeError errors should pass through unchanged
	_, err := Parse([]byte(`invalid yaml [`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// The error should still be present (not nil) - just not rewritten
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHumanizeUnmarshalError(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: `line 7: cannot unmarshal !!str ` + "`req`" + ` into []string`,
			want:  "line 7: expected a list, got a string",
		},
		{
			input: `line 12: cannot unmarshal !!seq into map[string]metamodel.PropertyDef`,
			want:  "line 12: expected a mapping, got a list",
		},
		{
			input: `line 5: cannot unmarshal !!str ` + "`true`" + ` into bool`,
			want:  "line 5: expected true/false, got a string",
		},
		{
			input: `line 9: cannot unmarshal !!str ` + "`one`" + ` into int`,
			want:  "line 9: expected a number, got a string",
		},
		{
			input: `line 3: cannot unmarshal !!map into []metamodel.ValidationRule`,
			want:  "line 3: expected a list, got a mapping",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := humanizeUnmarshalError(tt.input)
			if got != tt.want {
				t.Errorf("humanizeUnmarshalError(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHumanizeUnmarshalError_NoMatch(t *testing.T) {
	// Non-matching messages should be sanitized to a safe generic message
	got := humanizeUnmarshalError("some other error message")
	if got == "" {
		t.Error("expected non-empty sanitized message for non-matching input")
	}
	// Should not contain any Go internals
	if strings.Contains(got, "!!") || strings.Contains(got, "metamodel.") {
		t.Errorf("sanitized message should not contain Go internals, got %q", got)
	}
}
