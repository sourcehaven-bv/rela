package markdown

import (
	"strings"
	"testing"
)

// FuzzParseRelationFilename tests relation filename parsing
// Run with: go test -fuzz=FuzzParseRelationFilename -fuzztime=30s ./internal/markdown/
func FuzzParseRelationFilename(f *testing.F) {
	// Seed corpus with interesting cases
	seeds := []string{
		// Valid filenames
		"REQ-001--addresses--DEC-001.md",
		"A--B--C.md",
		"COMP-1--uses--COMP-2.md",

		// Edge cases
		"",
		".md",
		"---.md",
		"-----.md",
		"a--b.md",       // Missing third part
		"a--b--c--d.md", // Extra part
		"a--b--c",       // Missing .md
		"a--b--c.txt",   // Wrong extension
		"--b--c.md",     // Empty from
		"a----c.md",     // Empty relation
		"a--b--.md",     // Empty to
		"-------.md",    // Many delimiters
		"a--b--c.md.md", // Double extension

		// Special characters in parts
		"REQ-001--has space--DEC-001.md",
		"REQ-001--has/slash--DEC-001.md",
		"REQ-001--has\\backslash--DEC-001.md",

		// Unicode
		"日本語--関係--中文.md",
		"🚀--🔗--🎯.md",

		// Long strings
		strings.Repeat("A", 100) + "--rel--" + strings.Repeat("B", 100) + ".md",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should never panic
		from, relationType, to, ok := ParseRelationFilename(input)

		if !ok {
			// Parse failed, which is acceptable
			return
		}

		// If parsing succeeded, verify invariants
		if from == "" || relationType == "" || to == "" {
			t.Errorf("ParseRelationFilename returned empty parts for %q: from=%q rel=%q to=%q",
				input, from, relationType, to)
		}

		// None of the parts should contain the delimiter
		// (this would indicate incorrect parsing)
		if strings.Contains(from, "--") {
			t.Errorf("from part contains delimiter: %q", from)
		}
		if strings.Contains(relationType, "--") {
			t.Errorf("relationType part contains delimiter: %q", relationType)
		}
		if strings.Contains(to, "--") {
			t.Errorf("to part contains delimiter: %q", to)
		}
	})
}

// FuzzRelationFilename tests filename generation
func FuzzRelationFilename(f *testing.F) {
	f.Add("REQ-001", "addresses", "DEC-001")
	f.Add("", "", "")
	f.Add("a", "b", "c")
	f.Add("has--delim", "rel", "to")

	f.Fuzz(func(t *testing.T, from, relationType, to string) {
		// Should never panic
		result := RelationFilename(from, relationType, to)

		// Result should end with .md
		if !strings.HasSuffix(result, ".md") {
			t.Errorf("RelationFilename result doesn't end with .md: %q", result)
		}

		// Result should contain the delimiter pattern
		if !strings.Contains(result, "--") {
			t.Errorf("RelationFilename result doesn't contain delimiter: %q", result)
		}
	})
}

// FuzzRelationFilenameRoundTrip tests that generate and parse are inverse operations
func FuzzRelationFilenameRoundTrip(f *testing.F) {
	// Seed with valid parts that should round-trip
	f.Add("REQ-001", "addresses", "DEC-001")
	f.Add("A", "B", "C")
	f.Add("COMP-1", "uses", "COMP-2")

	f.Fuzz(func(t *testing.T, from, relationType, to string) {
		// Skip inputs that contain the delimiter (can't round-trip)
		if strings.Contains(from, "--") ||
			strings.Contains(relationType, "--") ||
			strings.Contains(to, "--") {

			return
		}

		// Skip empty parts
		if from == "" || relationType == "" || to == "" {
			return
		}

		// Generate filename
		filename := RelationFilename(from, relationType, to)

		// Parse it back
		parsedFrom, parsedRel, parsedTo, ok := ParseRelationFilename(filename)

		if !ok {
			t.Errorf("failed to parse generated filename %q (from=%q, rel=%q, to=%q)",
				filename, from, relationType, to)
			return
		}

		// Should match the original inputs
		if parsedFrom != from {
			t.Errorf("from mismatch: expected %q, got %q", from, parsedFrom)
		}
		if parsedRel != relationType {
			t.Errorf("relationType mismatch: expected %q, got %q", relationType, parsedRel)
		}
		if parsedTo != to {
			t.Errorf("to mismatch: expected %q, got %q", to, parsedTo)
		}
	})
}

// FuzzParseRelationFilenameEdgeCases focuses on edge cases
func FuzzParseRelationFilenameEdgeCases(f *testing.F) {
	// Focus on strings with various numbers of delimiters
	f.Add(0, "a", "b", "c")
	f.Add(1, "a", "b", "c")
	f.Add(2, "a", "b", "c")
	f.Add(5, "a", "b", "c")

	f.Fuzz(func(_ *testing.T, extraDelims int, a, b, c string) {
		if extraDelims < 0 || extraDelims > 10 {
			return
		}

		// Build a filename with varying delimiter counts
		filename := a + strings.Repeat("--", extraDelims+1) + b + "--" + c + ".md"

		// Should never panic
		_, _, _, _ = ParseRelationFilename(filename)
	})
}
