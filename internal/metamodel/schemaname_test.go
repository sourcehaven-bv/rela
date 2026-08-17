package metamodel

import (
	"strings"
	"testing"
)

// unique:true is only valid on string-valued property types (the application
// scan reads values as strings; the PostgreSQL derived index would otherwise
// diverge from it) — TKT-3Q0GP1.
func TestUniqueRequiresStringType(t *testing.T) {
	base := `
version: "1.0"
entities:
  thing:
    label: Thing
    id_prefix: "THG-"
    id_type: sequential
    properties:
      key:
        type: %s
        unique: true
`
	stringTypes := []string{"string", "date", "datetime", "rrule"}
	for _, ty := range stringTypes {
		if _, err := Parse([]byte(strings.Replace(base, "%s", ty, 1))); err != nil {
			t.Errorf("unique on string-valued type %q should load, got: %v", ty, err)
		}
	}

	nonStringTypes := []string{"integer", "boolean"}
	for _, ty := range nonStringTypes {
		_, err := Parse([]byte(strings.Replace(base, "%s", ty, 1)))
		if err == nil {
			t.Errorf("unique on non-string type %q should be rejected", ty)
		} else if !strings.Contains(err.Error(), "unique") {
			t.Errorf("error for type %q should mention unique, got: %v", ty, err)
		}
	}
}

// ValidateSchemaName guards the entity-type / property names the derived-schema
// reconciler interpolates into DDL (TKT-3Q0GP1). It is a blocklist of dangerous
// characters, not an allowlist, so legitimate dashed/spaced names survive.
func TestValidateSchemaName(t *testing.T) {
	valid := []string{
		"email", "org_id", "review-response", "doc-task", "some property",
		"MixedCase", "with.dot", "ünïcode", "a1b2",
	}
	for _, name := range valid {
		if err := ValidateSchemaName(name); err != nil {
			t.Errorf("ValidateSchemaName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{
		"",               // empty
		"bad'name",       // single quote (SQL literal breakout)
		"back\\slash",    // backslash
		"tab\tname",      // control char
		"new\nline",      // newline
		"null\x00byte",   // NUL
		" leading",       // leading whitespace
		"trailing ",      // trailing whitespace
		"email'OR'1'='1", // injection attempt
	}
	for _, name := range invalid {
		if err := ValidateSchemaName(name); err == nil {
			t.Errorf("ValidateSchemaName(%q) = nil, want error", name)
		}
	}
}
