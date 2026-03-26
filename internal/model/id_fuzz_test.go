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
		"ISO-CA-001",
		"ISO-CA-XX-042",
		"A-B-1",

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
	f.Add("ISO-CA", "ISO-CA-001,ISO-CA-002,ISO-CA-003")

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
	f.Add("ISO-CA-", "ISO-CA-001,ISO-CA-002")

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

// FuzzGenerateShortID tests short random ID generation
func FuzzGenerateShortID(f *testing.F) {
	// Seed with various prefixes and entity counts
	f.Add("TKT", "", 0, "upper")
	f.Add("REQ-", "REQ-A1B2", 10, "upper")
	f.Add("DEC", "DEC-xxxx,DEC-yyyy,DEC-zzzz", 100, "lower")
	f.Add("COMP-", "", 1000, "upper")
	f.Add("A", "", 10000, "lower")

	f.Fuzz(func(t *testing.T, prefix, existingJoined string, entityCount int, caps string) {
		// Skip very long prefixes
		if len(prefix) > 50 {
			return
		}

		// Skip invalid prefixes - must match ID character rules (alphanumeric, dash, underscore)
		if prefix == "" || !utf8.ValidString(prefix) {
			return
		}
		for _, c := range prefix {
			if !isAllowedIDChar(c) {
				return
			}
		}

		// Limit entity count to reasonable values
		if entityCount < 0 {
			entityCount = 0
		}
		if entityCount > 100000 {
			entityCount = 100000
		}

		// Normalize caps to valid values
		if caps != "lower" {
			caps = "upper"
		}

		var existing []string
		if existingJoined != "" {
			existing = strings.Split(existingJoined, ",")
		}

		// Should never panic
		result := GenerateShortID(existing, prefix, entityCount, caps)

		// Result should be non-empty
		if result == "" {
			t.Error("GenerateShortID returned empty string")
			return
		}

		// Result should start with prefix (as-is) + dash if not already present
		expectedPrefix := strings.TrimSuffix(prefix, "-") + "-"
		if !strings.HasPrefix(result, expectedPrefix) {
			t.Errorf("GenerateShortID result %q doesn't have expected prefix %q",
				result, expectedPrefix)
		}

		// Result should not be in the existing set
		for _, ex := range existing {
			if result == ex {
				t.Errorf("GenerateShortID returned existing ID %q", result)
			}
		}

		// Result should be valid according to ValidateID
		if err := ValidateID(result); err != nil {
			t.Errorf("GenerateShortID returned invalid ID %q: %v", result, err)
		}

		// Random part should only contain base36 characters with correct case
		randomPart := strings.TrimPrefix(result, expectedPrefix)
		for _, c := range randomPart {
			if caps == "upper" {
				if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z')) {
					t.Errorf("GenerateShortID random part %q contains invalid char %q (expected uppercase)",
						randomPart, c)
				}
			} else {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z')) {
					t.Errorf("GenerateShortID random part %q contains invalid char %q (expected lowercase)",
						randomPart, c)
				}
			}
		}
	})
}

// FuzzGenerateShortIDCollision stress tests collision resistance
func FuzzGenerateShortIDCollision(f *testing.F) {
	f.Add("TKT", 100, "upper")
	f.Add("REQ", 500, "lower")
	f.Add("X", 1000, "upper")

	f.Fuzz(func(t *testing.T, prefix string, count int, caps string) {
		// Skip invalid inputs
		if prefix == "" || len(prefix) > 20 {
			return
		}
		if count < 1 || count > 1000 {
			return
		}

		// Normalize caps to valid values
		if caps != "lower" {
			caps = "upper"
		}

		// Generate many IDs and verify no collisions
		seen := make(map[string]struct{}, count)
		for i := 0; i < count; i++ {
			// Convert seen map to slice
			existing := make([]string, 0, len(seen))
			for id := range seen {
				existing = append(existing, id)
			}

			id := GenerateShortID(existing, prefix, len(seen), caps)
			if _, exists := seen[id]; exists {
				t.Errorf("GenerateShortID produced collision: %q (iteration %d)", id, i)
				return
			}
			seen[id] = struct{}{}
		}
	})
}
