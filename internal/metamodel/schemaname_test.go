package metamodel

import "testing"

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
