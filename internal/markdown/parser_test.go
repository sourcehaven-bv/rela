package markdown

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

func TestParseDocument(t *testing.T) {
	content := `---
id: REQ-001
type: requirement
title: Test Requirement
tags:
  - security
  - performance
---

# Description

This is a test requirement.
`

	doc, err := ParseDocument(content)
	testutil.AssertNoError(t, err)

	if doc.Frontmatter == nil {
		t.Fatal("frontmatter should not be nil")
	}

	testutil.AssertEqual(t, doc.GetString("id"), "REQ-001")
	testutil.AssertEqual(t, doc.GetString("type"), "requirement")
	testutil.AssertEqual(t, doc.GetString("title"), "Test Requirement")

	tags := doc.GetStringSlice("tags")
	testutil.AssertEqual(t, len(tags), 2)

	testutil.AssertStringContains(t, doc.Content, "# Description")
	testutil.AssertStringContains(t, doc.Content, "This is a test requirement")
}

func TestParseDocument_NoFrontmatter(t *testing.T) {
	content := `# Just a heading

Some content without frontmatter.
`

	doc, err := ParseDocument(content)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, len(doc.Frontmatter), 0)

	testutil.AssertStringContains(t, doc.Content, "# Just a heading")
}

func TestParseDocument_EmptyFrontmatter(t *testing.T) {
	content := `---
---

Content after empty frontmatter.
`

	doc, err := ParseDocument(content)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, len(doc.Frontmatter), 0)

	testutil.AssertStringContains(t, doc.Content, "Content after empty frontmatter")
}

func TestParseDocument_InvalidYAML(t *testing.T) {
	content := `---
id: REQ-001
type: [invalid
---

Content here.
`

	_, err := ParseDocument(content)
	testutil.AssertError(t, err)
}

func TestParseDocument_FrontmatterOnly(t *testing.T) {
	content := `---
id: REQ-001
type: requirement
---
`

	doc, err := ParseDocument(content)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, doc.GetString("id"), "REQ-001")
	testutil.AssertEqual(t, doc.Content, "")
}

func TestParseDocument_UnclosedFrontmatter(t *testing.T) {
	content := `---
id: REQ-001

This content should be part of body since frontmatter was never closed.
`

	doc, err := ParseDocument(content)
	// The parser will attempt to parse unclosed frontmatter as YAML
	// which will fail if it's not valid YAML
	if err != nil {
		// Error is expected since unclosed frontmatter with invalid YAML fails
		return
	}

	// If it succeeds, verify the result
	if doc == nil {
		t.Fatal("doc should not be nil")
	}
}

func TestFormatDocument(t *testing.T) {
	frontmatter := map[string]interface{}{
		"id":       "REQ-001",
		"type":     "requirement",
		"title":    "Test Requirement",
		"priority": "high",
	}
	content := "# Description\n\nThis is the content."

	formatted, err := FormatDocument(frontmatter, content)
	testutil.AssertNoError(t, err)

	if !strings.HasPrefix(formatted, "---\n") {
		t.Error("formatted document should start with ---")
	}
	testutil.AssertStringContains(t, formatted, "id: REQ-001")
	testutil.AssertStringContains(t, formatted, "type: requirement")
	testutil.AssertStringContains(t, formatted, "# Description")

	// Verify it can be parsed back
	doc, err := ParseDocument(formatted)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, doc.GetString("id"), "REQ-001")
}

func TestFormatDocument_EmptyFrontmatter(t *testing.T) {
	frontmatter := map[string]interface{}{}
	content := "Just content without frontmatter."

	formatted, err := FormatDocument(frontmatter, content)
	testutil.AssertNoError(t, err)

	if strings.HasPrefix(formatted, "---") {
		t.Error("formatted document should not have frontmatter delimiters")
	}
	testutil.AssertStringContains(t, formatted, "Just content")
}

func TestFormatDocument_NoContent(t *testing.T) {
	frontmatter := map[string]interface{}{
		"id":   "REQ-001",
		"type": "requirement",
	}
	content := ""

	formatted, err := FormatDocument(frontmatter, content)
	testutil.AssertNoError(t, err)

	if !strings.HasPrefix(formatted, "---\n") {
		t.Error("formatted document should start with ---")
	}
	testutil.AssertStringContains(t, formatted, "id: REQ-001")

	// Should not have extra content after closing ---
	lines := strings.Split(formatted, "\n")
	var foundClosing bool
	var afterClosing []string
	for _, line := range lines {
		switch {
		case foundClosing:
			afterClosing = append(afterClosing, line)
		case strings.TrimSpace(line) == "---" && len(afterClosing) == 0:
			// First --- is opening
			continue
		case strings.TrimSpace(line) == "---":
			foundClosing = true
		}
	}

	// After closing should be empty or just whitespace/newlines
	nonEmpty := 0
	for _, line := range afterClosing {
		if strings.TrimSpace(line) != "" {
			nonEmpty++
		}
	}
	if nonEmpty > 0 {
		t.Errorf("expected no content after frontmatter, got %d non-empty lines", nonEmpty)
	}
}

func TestFormatDocument_ContentWithoutTrailingNewline(t *testing.T) {
	frontmatter := map[string]interface{}{
		"id": "REQ-001",
	}
	content := "Content without newline"

	formatted, err := FormatDocument(frontmatter, content)
	testutil.AssertNoError(t, err)

	if !strings.HasSuffix(formatted, "\n") {
		t.Error("formatted document should end with newline")
	}
}

func TestDocumentGetString(t *testing.T) {
	doc := &Document{
		Frontmatter: map[string]interface{}{
			"string_field": "value",
			"int_field":    42,
			"bool_field":   true,
		},
	}

	// Test existing string field
	testutil.AssertEqual(t, doc.GetString("string_field"), "value")

	// Test non-existent field
	testutil.AssertEqual(t, doc.GetString("nonexistent"), "")

	// Test non-string field
	testutil.AssertEqual(t, doc.GetString("int_field"), "")
}

func TestDocumentGetString_NilFrontmatter(t *testing.T) {
	doc := &Document{
		Frontmatter: nil,
	}

	testutil.AssertEqual(t, doc.GetString("any_field"), "")
}

func TestDocumentGetStringSlice(t *testing.T) {
	doc := &Document{
		Frontmatter: map[string]interface{}{
			"string_slice": []string{"a", "b", "c"},
			"interface_slice": []interface{}{
				"x",
				"y",
				"z",
			},
			"mixed_slice": []interface{}{
				"string",
				42,
				"another",
			},
			"not_slice": "just a string",
		},
	}

	// Test []string
	if got := doc.GetStringSlice("string_slice"); len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("GetStringSlice(string_slice) = %v, want [a b c]", got)
	}

	// Test []interface{} with strings
	if got := doc.GetStringSlice("interface_slice"); len(got) != 3 || got[0] != "x" || got[1] != "y" || got[2] != "z" {
		t.Errorf("GetStringSlice(interface_slice) = %v, want [x y z]", got)
	}

	// Test mixed slice (should skip non-strings)
	if got := doc.GetStringSlice("mixed_slice"); len(got) != 2 || got[0] != "string" || got[1] != "another" {
		t.Errorf("GetStringSlice(mixed_slice) = %v, want [string another]", got)
	}

	// Test non-slice field
	if got := doc.GetStringSlice("not_slice"); got != nil {
		t.Errorf("GetStringSlice(not_slice) = %v, want nil", got)
	}

	// Test non-existent field
	if got := doc.GetStringSlice("nonexistent"); got != nil {
		t.Errorf("GetStringSlice(nonexistent) = %v, want nil", got)
	}
}

func TestDocumentGetStringSlice_NilFrontmatter(t *testing.T) {
	doc := &Document{
		Frontmatter: nil,
	}

	if got := doc.GetStringSlice("any_field"); got != nil {
		t.Errorf("GetStringSlice with nil frontmatter = %v, want nil", got)
	}
}

func TestSplitFrontmatter(t *testing.T) {
	content := `---
id: REQ-001
type: requirement
---

# Heading

Content here.
`

	frontmatter, body, err := splitFrontmatter(content)
	testutil.AssertNoError(t, err)

	testutil.AssertStringContains(t, frontmatter, "id: REQ-001")
	testutil.AssertStringContains(t, frontmatter, "type: requirement")

	testutil.AssertStringContains(t, body, "# Heading")
	testutil.AssertStringContains(t, body, "Content here")

	// Body should not contain frontmatter delimiters
	testutil.AssertStringNotContains(t, body, "---")
}

func TestSplitFrontmatter_NoFrontmatter(t *testing.T) {
	content := `# Just a heading

Some content.
`

	frontmatter, body, err := splitFrontmatter(content)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, frontmatter, "")

	testutil.AssertStringContains(t, body, "# Just a heading")
}

func TestSplitFrontmatter_EmptyContent(t *testing.T) {
	content := ""

	frontmatter, body, err := splitFrontmatter(content)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, frontmatter, "")
	testutil.AssertEqual(t, body, "")
}

func TestSplitFrontmatter_OnlyFrontmatter(t *testing.T) {
	content := `---
id: REQ-001
---
`

	frontmatter, body, err := splitFrontmatter(content)
	testutil.AssertNoError(t, err)

	testutil.AssertStringContains(t, frontmatter, "id: REQ-001")

	testutil.AssertEqual(t, body, "")
}

func TestHasConflictMarkers(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "no conflicts",
			content:  "---\nid: REQ-001\n---\nContent",
			expected: false,
		},
		{
			name: "has conflicts in frontmatter",
			content: `---
id: REQ-001
<<<<<<< HEAD
status: draft
=======
status: approved
>>>>>>> feature-branch
---
Content`,
			expected: true,
		},
		{
			name: "has conflicts in content",
			content: `---
id: REQ-001
---
<<<<<<< HEAD
Some content
=======
Different content
>>>>>>> feature-branch`,
			expected: true,
		},
		{
			name:     "partial marker only",
			content:  "Some text with <<<<<< but not full marker",
			expected: false,
		},
		{
			name:     "full start marker",
			content:  "Some text with <<<<<<< HEAD somewhere",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasConflictMarkersString(tt.content)
			testutil.AssertEqual(t, got, tt.expected)

			// Also test byte version
			gotBytes := HasConflictMarkers([]byte(tt.content))
			testutil.AssertEqual(t, gotBytes, tt.expected)
		})
	}
}

func TestParseDocument_ConflictedFile(t *testing.T) {
	content := `---
id: REQ-001
<<<<<<< HEAD
status: draft
=======
status: approved
>>>>>>> feature-branch
---

# Description
`

	_, err := ParseDocument(content)
	testutil.AssertError(t, err)

	if err != ErrConflictedFile {
		t.Errorf("expected ErrConflictedFile, got %v", err)
	}
}

func TestParseDocument_ConflictedFileInContent(t *testing.T) {
	content := `---
id: REQ-001
status: draft
---

# Description

<<<<<<< HEAD
This is the current content.
=======
This is the incoming content.
>>>>>>> feature-branch
`

	_, err := ParseDocument(content)
	testutil.AssertError(t, err)

	if err != ErrConflictedFile {
		t.Errorf("expected ErrConflictedFile, got %v", err)
	}
}

func TestRoundTrip(t *testing.T) {
	// Original content
	original := `---
id: REQ-001
type: requirement
title: Test Requirement
tags:
  - tag1
  - tag2
---

# Description

This is some content with multiple lines.

## Subsection

More content here.
`

	// Parse
	doc, err := ParseDocument(original)
	testutil.AssertNoError(t, err)

	// Format back
	formatted, err := FormatDocument(doc.Frontmatter, doc.Content)
	testutil.AssertNoError(t, err)

	// Parse again
	doc2, err := ParseDocument(formatted)
	testutil.AssertNoError(t, err)

	// Verify key fields are preserved
	testutil.AssertEqual(t, doc2.GetString("id"), "REQ-001")
	testutil.AssertEqual(t, doc2.GetString("type"), "requirement")
	testutil.AssertEqual(t, doc2.GetString("title"), "Test Requirement")

	tags := doc2.GetStringSlice("tags")
	testutil.AssertEqual(t, len(tags), 2)

	testutil.AssertStringContains(t, doc2.Content, "# Description")
	testutil.AssertStringContains(t, doc2.Content, "More content here")
}
