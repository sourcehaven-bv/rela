package metamodel

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

// forbiddenPatterns are Go/YAML internals that should never leak into user-facing errors.
// If Parse() returns an error containing any of these, the error message is confusing.
var forbiddenPatterns = []*regexp.Regexp{
	// YAML tag internals
	regexp.MustCompile(`!!\w+`), // !!str, !!seq, !!map, !!int, !!float, !!bool, !!null

	// Go type names that leak through yaml.TypeError
	regexp.MustCompile(`metamodel\.\w+`),           // metamodel.PropertyDef, metamodel.EntityDef, etc.
	regexp.MustCompile(`\bmap\[string\]\w+`),       // map[string]PropertyDef, map[string]interface{}
	regexp.MustCompile(`\[\]\w+\.\w+`),             // []metamodel.ValidationRule
	regexp.MustCompile(`cannot unmarshal`),         // raw yaml.TypeError language
	regexp.MustCompile(`\binto\b.*\b(map|struct)`), // "into map[string]..." or "into struct"
}

// allowedErrorPrefixes are patterns that user-friendly errors should match.
// Every error from Parse() should contain at least one of these to be considered friendly.
var allowedErrorPrefixes = []string{
	// Structural YAML issues (humanized)
	"line ",             // "line 7: expected a list, got a string"
	"yaml:",             // "yaml: ..." (syntax errors from yaml library)
	"did not find",      // yaml syntax error
	"invalid YAML",      // fallback sanitized message
	"invalid value for", // fallback sanitized message with line number

	// Schema validation (our custom messages)
	"entity ",           // entity "x": missing 'label' / property issues
	"relation ",         // relation "x": references unknown entity type
	"metamodel ",        // metamodel has no entity types / metamodel validation errors
	"unknown key ",      // unknown key "entity" (did you mean "entities"?)
	"cannot define",     // cannot define custom type "string": name is reserved
	"invalid id_type",   // invalid id_type for entity x
	"no ID prefix",      // no ID prefix defined
	"missing ",          // missing 'label', missing required property
	"no properties",     // no properties defined
	"no entity types",   // no entity types defined
	"unknown type",      // property x has unknown type "y"
	"no type specified", // property "x" has no type specified
	"has unknown type",  // property x has unknown type
	"is type \"enum\"",  // is type "enum" but has no 'values' list
	"expected ",         // "expected a list, got a string" (humanized yaml errors)
	"invalid ",          // invalid value, invalid date, etc.
	"property ",         // property x has unknown type
	"references ",       // references unknown entity type
	"specifies both",    // specifies both id_prefix and id_prefixes
}

// FuzzParse tests that Parse never panics and always returns user-friendly errors.
// Run with: go test -fuzz=FuzzParse -fuzztime=30s ./internal/metamodel/
func FuzzParse(f *testing.F) {
	// Seed corpus: valid metamodels
	seeds := []string{
		// Minimal valid
		`version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    properties:
      title:
        type: string
`,
		// Full valid metamodel
		DefaultMetamodelYAML(),

		// Empty
		"",

		// Just a scalar
		"hello",

		// YAML list instead of mapping
		"- item1\n- item2",

		// Completely wrong structure
		`entities: "not a map"`,

		// Properties as a list instead of a map
		`version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    properties:
      - title
      - status
`,

		// Type mismatch: aliases as string instead of list
		`version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    aliases: not-a-list
    properties:
      title:
        type: string
`,

		// Type mismatch: from as string instead of list
		`version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    properties:
      title:
        type: string
relations:
  depends:
    label: depends
    from: task
    to: task
`,

		// Misspelled top-level keys
		`version: "1.0"
entity:
  task:
    label: Task
`,
		`version: "1.0"
type:
  status:
    values: [draft]
`,
		`version: "1.0"
relation:
  depends:
    from: [task]
`,

		// Unknown property type
		`version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    properties:
      title:
        type: string
      severity:
        type: nonexistent
`,

		// Enum without values
		`version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    properties:
      title:
        type: string
      size:
        type: enum
`,

		// Relation referencing nonexistent entity
		`version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    properties:
      title:
        type: string
relations:
  depends:
    label: depends
    from: [ghost]
    to: [task]
`,

		// Missing label
		`version: "1.0"
entities:
  task:
    id_prefix: "TASK-"
    properties:
      title:
        type: string
`,

		// Missing properties
		`version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
`,

		// Missing id_prefix for auto
		`version: "1.0"
entities:
  task:
    label: Task
    properties:
      title:
        type: string
`,

		// Both id_prefix and id_prefixes
		`version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    id_prefixes: ["TASK-", "T-"]
    properties:
      title:
        type: string
`,

		// Invalid id_type
		`version: "1.0"
entities:
  task:
    label: Task
    id_type: bogus
    properties:
      title:
        type: string
`,

		// Reserved property name
		`version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    properties:
      id:
        type: string
`,

		// Reserved type name
		`version: "1.0"
types:
  string:
    values: [a, b]
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    properties:
      title:
        type: string
`,

		// Empty entities
		`version: "1.0"
entities: {}
`,

		// Deeply nested nonsense
		`version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    properties:
      title:
        type: string
        required:
          nested: true
`,

		// Validations as a map instead of a list
		`version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    properties:
      title:
        type: string
validations:
  rule1:
    name: test
`,

		// Unicode entity names
		`version: "1.0"
entities:
  要件:
    label: 要件
    id_prefix: "要件-"
    properties:
      タイトル:
        type: string
`,

		// Very long entity name
		`version: "1.0"
entities:
  ` + strings.Repeat("a", 200) + `:
    label: Long
    id_prefix: "L-"
    properties:
      title:
        type: string
`,

		// Null values
		`version: "1.0"
entities:
  task:
    label: ~
    id_prefix: ~
    properties: ~
`,

		// Boolean where string expected
		`version: "1.0"
entities:
  task:
    label: true
    id_prefix: "TASK-"
    properties:
      title:
        type: string
`,

		// Number where string expected
		`version: "1.0"
entities:
  task:
    label: 42
    id_prefix: "TASK-"
    properties:
      title:
        type: string
`,

		// Duplicate keys (YAML spec says last wins)
		`version: "1.0"
entities:
  task:
    label: First
    id_prefix: "TASK-"
    properties:
      title:
        type: string
  task:
    label: Second
    id_prefix: "T-"
    properties:
      name:
        type: string
`,

		// Tab indentation (common YAML mistake)
		"version: \"1.0\"\nentities:\n\ttask:\n\t\tlabel: Task\n",

		// Anchor/alias usage
		`version: "1.0"
entities:
  task: &task_def
    label: Task
    id_prefix: "TASK-"
    properties:
      title:
        type: string
  cloned_task: *task_def
`,
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Parse should NEVER panic, regardless of input
		meta, err := Parse([]byte(input))

		if err == nil {
			// Valid parse — verify basic invariants
			if meta == nil {
				t.Fatal("Parse returned nil metamodel without error")
			}
			return
		}

		// Error path: verify the error message is user-friendly
		errMsg := err.Error()

		// Check for forbidden Go/YAML internals
		for _, pattern := range forbiddenPatterns {
			if pattern.MatchString(errMsg) {
				t.Errorf("error contains Go/YAML internal %q:\n  %s", pattern.String(), errMsg)
			}
		}
	})
}

// FuzzParseErrorQuality is a property-based test that validates error messages
// are always comprehensible. It uses structured fuzzing to generate YAML-like
// documents that are more likely to exercise validation paths.
func FuzzParseErrorQuality(f *testing.F) {
	// Seeds are (version, entityName, label, idPrefix, propName, propType, extraKey)
	// These are combined to generate metamodel YAML documents with controlled variation.
	f.Add("1.0", "task", "Task", "TASK-", "title", "string", "")
	f.Add("1.0", "req", "Requirement", "REQ-", "title", "nonexistent", "")
	f.Add("1.0", "task", "", "TASK-", "title", "string", "")           // missing label
	f.Add("1.0", "task", "Task", "", "title", "string", "")            // missing prefix
	f.Add("1.0", "task", "Task", "TASK-", "title", "enum", "")         // enum without values
	f.Add("1.0", "task", "Task", "TASK-", "id", "string", "")          // reserved prop
	f.Add("1.0", "task", "Task", "TASK-", "title", "string", "entity") // misspelled key
	f.Add("1.0", "task", "Task", "TASK-", "title", "string", "widgets")
	f.Add("", "task", "Task", "TASK-", "title", "string", "")
	f.Add("1.0", "", "Task", "TASK-", "title", "string", "")

	f.Fuzz(func(t *testing.T, version, entityName, label, idPrefix, propName, propType, extraKey string) {
		// Skip inputs that would make invalid YAML (control chars, colons, etc.)
		for _, s := range []string{version, entityName, label, idPrefix, propName, propType, extraKey} {
			if strings.ContainsAny(s, ":\n\r\t{}[]#&*!|>'\",") {
				return
			}
			if len(s) > 100 {
				return
			}
		}

		// Skip empty entity names (not valid YAML keys)
		if entityName == "" {
			return
		}

		// Build a metamodel YAML document
		var b strings.Builder
		if version != "" {
			b.WriteString("version: \"" + version + "\"\n")
		}

		if extraKey != "" && extraKey != "version" && extraKey != "entities" {
			b.WriteString(extraKey + ": {}\n")
		}

		b.WriteString("entities:\n")
		b.WriteString("  " + entityName + ":\n")
		if label != "" {
			b.WriteString("    label: " + label + "\n")
		}
		if idPrefix != "" {
			b.WriteString("    id_prefix: \"" + idPrefix + "\"\n")
		}
		if propName != "" {
			b.WriteString("    properties:\n")
			b.WriteString("      " + propName + ":\n")
			if propType != "" {
				b.WriteString("        type: " + propType + "\n")
			}
		}

		input := b.String()

		meta, err := Parse([]byte(input))

		if err == nil {
			if meta == nil {
				t.Fatal("Parse returned nil metamodel without error")
			}
			return
		}

		errMsg := err.Error()

		// PROPERTY 1: No Go/YAML internal leakage
		for _, pattern := range forbiddenPatterns {
			if pattern.MatchString(errMsg) {
				t.Errorf("error leaks Go/YAML internal %q for input:\n%s\nerror: %s",
					pattern.String(), input, errMsg)
			}
		}

		// PROPERTY 2: Error is not empty
		if errMsg == "" {
			t.Errorf("Parse returned error with empty message for input:\n%s", input)
		}

		// PROPERTY 3: Error must match at least one known-friendly pattern.
		// This ensures we never return a raw/confusing error that we haven't accounted for.
		assertFriendlyError(t, err, errMsg, input)
	})
}

// assertFriendlyError verifies that an error message matches at least one known-friendly pattern.
// For SchemaValidationError, it checks each sub-error individually.
func assertFriendlyError(t *testing.T, err error, errMsg, input string) {
	t.Helper()

	if matchesAnyFriendlyPattern(errMsg) {
		return
	}

	// Check if it's a SchemaValidationError with multiple sub-errors
	var schemaErr *SchemaValidationError
	if errors.As(err, &schemaErr) {
		for _, subErr := range schemaErr.Errors {
			if !matchesAnyFriendlyPattern(subErr) {
				t.Errorf("sub-error does not match any friendly pattern: %q\nfull error: %s\ninput:\n%s",
					subErr, errMsg, input)
			}
		}
		return
	}

	t.Errorf("error does not match any friendly pattern: %q\ninput:\n%s", errMsg, input)
}

// matchesAnyFriendlyPattern checks if an error message contains any known-friendly substring.
func matchesAnyFriendlyPattern(errMsg string) bool {
	for _, prefix := range allowedErrorPrefixes {
		if strings.Contains(errMsg, prefix) {
			return true
		}
	}
	return false
}
