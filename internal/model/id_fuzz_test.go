package model

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzParseEntityID tests the entity ID parser with arbitrary input
// Run with: go test -fuzz=FuzzParseEntityID -fuzztime=30s ./internal/model/
func FuzzParseEntityID(f *testing.F) {
	// Seed corpus with interesting cases
	seeds := []string{
		// Valid IDs
		"REQ-001",
		"DEC-42",
		"ADR-1",
		"COMP-999",
		"A1",
		"ABC123",

		// Edge cases
		"",
		"-",
		"-1",
		"1",
		"REQ-",
		"-001",
		"REQ--001",
		"req-001", // lowercase

		// Large numbers
		"REQ-999999999999999999999",
		"REQ-0",
		"REQ-00000001",

		// Path traversal attempts
		"../etc/passwd",
		"..\\windows\\system32",
		"REQ-001/../../../etc/passwd",
		"/etc/passwd",
		"C:\\Windows",

		// Special characters
		"REQ_001",
		"REQ.001",
		"REQ 001",
		"REQ\t001",
		"REQ\n001",
		"REQ-001\x00",

		// Unicode
		"REQ-日本語",
		"требование-001",
		"REQ-🎉",

		// Very long
		strings.Repeat("A", 1000),
		strings.Repeat("A", 1000) + "-1",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// ParseEntityID should never panic
		id, err := ParseEntityID(input)

		if err != nil {
			// Errors are acceptable
			return
		}

		// If parsing succeeded, String() should not panic
		str := id.String()

		// The string representation should be non-empty for valid IDs
		if input != "" && str == "" {
			t.Errorf("String() returned empty for non-empty input %q", input)
		}
	})
}

// FuzzValidateID tests the ID validation function
func FuzzValidateID(f *testing.F) {
	seeds := []string{
		"REQ-001",
		"",
		"..",
		"../",
		"/",
		"\\",
		"a/b",
		"a\\b",
		"valid-id",
		"VALID_ID",
		"spaces not allowed",
		"special!chars@here",
		"unicode-日本語",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// ValidateID should never panic
		err := ValidateID(input)

		if err == nil {
			// If validation passed, verify the invariants we expect

			// Should not be empty
			if input == "" {
				t.Error("ValidateID accepted empty string")
			}

			// Should not contain path traversal
			if strings.Contains(input, "..") {
				t.Errorf("ValidateID accepted path traversal: %q", input)
			}
			if strings.Contains(input, "/") {
				t.Errorf("ValidateID accepted forward slash: %q", input)
			}
			if strings.Contains(input, "\\") {
				t.Errorf("ValidateID accepted backslash: %q", input)
			}

			// Should only contain allowed characters
			for _, r := range input {
				if !isAllowedIDChar(r) {
					t.Errorf("ValidateID accepted disallowed character %q in %q", r, input)
				}
			}
		}
	})
}

func isAllowedIDChar(r rune) bool {
	return (r >= 'A' && r <= 'Z') ||
		(r >= 'a' && r <= 'z') ||
		(r >= '0' && r <= '9') ||
		r == '_' || r == '-'
}

// FuzzParseEntityIDRoundTrip tests that parse and string are inverse operations
func FuzzParseEntityIDRoundTrip(f *testing.F) {
	// Seed with valid standard-format IDs
	seeds := []string{
		"REQ-001",
		"DEC-1",
		"ADR-999",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Parse the input
		id1, err := ParseEntityID(input)
		if err != nil {
			return
		}

		// Get string representation
		str := id1.String()

		// Parse again
		id2, err := ParseEntityID(str)
		if err != nil {
			t.Errorf("failed to parse String() output %q (from input %q): %v",
				str, input, err)
			return
		}

		// The string representations should match
		if id1.String() != id2.String() {
			t.Errorf("round-trip mismatch: %q -> %q -> %q",
				input, id1.String(), id2.String())
		}
	})
}

// FuzzExtractHighestNumber tests number extraction from ID lists
func FuzzExtractHighestNumber(f *testing.F) {
	f.Add("REQ", "REQ-001,REQ-002,REQ-003")
	f.Add("DEC", "DEC-1,DEC-10,DEC-2")
	f.Add("COMP", "")
	f.Add("REQ", "invalid,REQ-001,alsoinvalid")

	f.Fuzz(func(t *testing.T, prefix, idsJoined string) {
		ids := strings.Split(idsJoined, ",")

		// Should never panic
		result := ExtractHighestNumber(ids, prefix)

		// Result should be non-negative
		if result < 0 {
			t.Errorf("ExtractHighestNumber returned negative: %d", result)
		}
	})
}

// FuzzGenerateNextID tests ID generation
func FuzzGenerateNextID(f *testing.F) {
	f.Add("REQ", "REQ-001,REQ-002")
	f.Add("DEC-", "DEC-1")
	f.Add("COMP", "")

	f.Fuzz(func(t *testing.T, prefix, idsJoined string) {
		// Skip very long prefixes to avoid memory issues
		if len(prefix) > 100 {
			return
		}

		// Skip invalid prefixes (need at least one letter)
		if !utf8.ValidString(prefix) {
			return
		}

		ids := strings.Split(idsJoined, ",")

		// Should never panic
		result := GenerateNextID(ids, prefix)

		// Result should be non-empty
		if result == "" {
			t.Error("GenerateNextID returned empty string")
		}

		// Result should contain the prefix (uppercased, with dash)
		expectedPrefix := strings.ToUpper(strings.TrimSuffix(prefix, "-")) + "-"
		if !strings.HasPrefix(result, expectedPrefix) {
			t.Errorf("GenerateNextID result %q doesn't have expected prefix %q",
				result, expectedPrefix)
		}
	})
}

// FuzzFormatWithPadding tests padded formatting
func FuzzFormatWithPadding(f *testing.F) {
	f.Add("REQ-1", 3)
	f.Add("DEC-42", 5)
	f.Add("opaque-id", 3)

	f.Fuzz(func(t *testing.T, input string, width int) {
		// Limit width to reasonable values
		if width < 0 || width > 100 {
			return
		}

		id, err := ParseEntityID(input)
		if err != nil {
			return
		}

		// Should never panic
		result := id.FormatWithPadding(width)

		// Result should be non-empty for valid IDs
		if result == "" && input != "" {
			t.Errorf("FormatWithPadding returned empty for %q", input)
		}
	})
}
