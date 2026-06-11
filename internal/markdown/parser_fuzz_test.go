package markdown

import (
	"testing"
)

// FuzzParseDocument tests the markdown parser with arbitrary input
// Run with: go test -fuzz=FuzzParseDocument -fuzztime=30s ./internal/markdown/
func FuzzParseDocument(f *testing.F) {
	// Seed corpus with interesting cases
	seeds := []string{
		// Valid documents
		"---\ntitle: test\n---\n\nContent here",
		"---\nkey: value\nlist:\n  - item1\n  - item2\n---\n\n# Heading\n\nBody text",
		"",
		"No frontmatter here",
		"---\n---\n",
		"---\ntitle: \"quoted value\"\n---\n",

		// Edge cases
		"---",
		"---\n",
		"---\n---",
		"---\nkey: value\n",
		"------",
		"--- \ntitle: test\n--- ",
		"\n---\ntitle: test\n---\n",
		"  ---  \ntitle: test\n  ---  \n",

		// Potentially problematic YAML
		"---\n: empty key\n---\n",
		"---\nnull: ~\n---\n",
		"---\nkey: |\n  multiline\n  value\n---\n",
		"---\nkey: >\n  folded\n  value\n---\n",
		"---\nanchored: &anchor value\naliased: *anchor\n---\n",
		"---\n!!str 123: typed\n---\n",

		// Unicode
		"---\ntitle: 日本語\n---\n\n中文内容",
		"---\nemoji: 🎉\n---\n\n🚀 rocket",

		// Nested delimiters
		"---\ntitle: test\n---\n\n```\n---\ncode block\n---\n```",
		"---\ncode: \"---\"\n---\n",

		// Large values
		"---\nkey: " + string(make([]byte, 1000)) + "\n---\n",

		// Special characters
		"---\ntitle: \"value with \\n newline\"\n---\n",
		"---\ntitle: 'single quoted'\n---\n",
		"---\npath: C:\\Users\\test\n---\n",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// The parser should never panic on any input
		doc, err := ParseDocument(input)

		if err != nil {
			// Errors are acceptable, panics are not
			return
		}

		// If parsing succeeded, verify basic invariants
		if doc == nil {
			t.Error("ParseDocument returned nil document without error")
			return
		}

		// Frontmatter should be a valid map (possibly empty/nil)
		// This is implicitly checked by the type system
	})
}

// The frontmatter-split fuzz test moved to internal/frontmatter
// (FuzzSplit) along with the split implementation.

// FuzzFormatDocument tests the document formatter
func FuzzFormatDocument(f *testing.F) {
	// We'll fuzz the content string with a fixed frontmatter
	f.Add("")
	f.Add("Simple content")
	f.Add("# Heading\n\nParagraph")
	f.Add("Content with ---\n delimiter")
	f.Add(string(make([]byte, 1000)))

	f.Fuzz(func(t *testing.T, content string) {
		frontmatter := map[string]interface{}{
			"title": "test",
		}

		// Should never panic
		result, err := FormatDocument(frontmatter, content)

		if err != nil {
			return
		}

		if result == "" {
			t.Error("FormatDocument returned empty string for non-empty frontmatter")
		}
	})
}

// FuzzParseFormatRoundTrip tests that parse and format are inverse operations
// This is a property-based test
func FuzzParseFormatRoundTrip(f *testing.F) {
	// Seed with valid documents that should round-trip
	seeds := []string{
		"---\ntitle: test\n---\n\nContent",
		"---\nkey: value\n---\n\n# Heading",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Parse the input
		doc1, err := ParseDocument(input)
		if err != nil || doc1 == nil {
			return // Skip invalid inputs
		}

		// Skip if frontmatter is empty (format behavior differs)
		if len(doc1.Frontmatter) == 0 {
			return
		}

		// Format back to string
		formatted, err := FormatDocument(doc1.Frontmatter, doc1.Content)
		if err != nil {
			return
		}

		// Parse the formatted result
		doc2, err := ParseDocument(formatted)
		if err != nil {
			t.Errorf("failed to parse formatted output: %v", err)
			return
		}

		// Compare: the content should be equivalent
		// Note: exact string comparison may fail due to YAML formatting differences
		// so we compare the parsed structures
		if doc2.Content != doc1.Content {
			// Allow for trailing newline differences
			if trimContent(doc1.Content) != trimContent(doc2.Content) {
				t.Errorf("content mismatch after round-trip:\noriginal: %q\nafter: %q",
					doc1.Content, doc2.Content)
			}
		}
	})
}

func trimContent(s string) string {
	// Normalize content for comparison
	for s != "" && (s[len(s)-1] == '\n' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	for s != "" && (s[0] == '\n' || s[0] == ' ') {
		s = s[1:]
	}
	return s
}
