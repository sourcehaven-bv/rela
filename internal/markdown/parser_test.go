package markdown

import (
	"errors"
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

	// Unclosed frontmatter is a parse error: the body text gets
	// consumed as (invalid) YAML. Previously this test accepted
	// either outcome and pinned nothing.
	doc, err := ParseDocument(content)
	if err == nil {
		t.Fatalf("unclosed frontmatter must fail to parse, got doc %+v", doc)
	}
	testutil.AssertStringContains(t, err.Error(), "frontmatter")
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

// Frontmatter-split unit tests moved to internal/frontmatter
// (TestSplit) along with the split implementation. The tests below
// still exercise the split end-to-end via ParseDocument.

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
			// BUG-WN6D: a substring `<<<<<<<` mid-line is NOT a
			// conflict — git only writes the marker at column 0.
			// Detection is line-anchored.
			name:     "full marker mid-line is not a conflict (BUG-WN6D)",
			content:  "Some text with <<<<<<< HEAD somewhere",
			expected: false,
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

	if !errors.Is(err, ErrConflictedFile) {
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

	if !errors.Is(err, ErrConflictedFile) {
		t.Errorf("expected ErrConflictedFile, got %v", err)
	}
}

// BUG-WN6D: the conflict-marker detector matched the substring
// anywhere, false-positiving on legitimate content (markdown
// codespans, quoted prose). Detection must be line-anchored:
// the marker is meaningful only at column 0.
func TestParseDocument_ConflictMarkerInCodespan_NotAConflict(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "marker inside an inline code span",
			content: `---
id: REQ-001
title: How conflict markers look
status: draft
---

# Spec

The detector triggers on ` + "`<<<<<<<`" + ` at column 0.
`,
		},
		{
			name: "marker quoted in prose, indented two spaces",
			content: `---
id: REQ-002
title: Detector spec
status: draft
---

Notes:

  Git writes ` + "`<<<<<<<`" + ` at the start of a line when a merge
  conflicts. We must not treat the substring inline as one.
`,
		},
		{
			name: "marker mid-line in YAML scalar value",
			content: `---
id: REQ-003
title: contains <<<<<<< as a literal description
status: draft
---

# Body

Plain text.
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := ParseDocument(tt.content)
			if err != nil {
				t.Fatalf("ParseDocument returned %v, want nil (substring should not trigger conflict detection)", err)
			}
			if doc == nil {
				t.Fatal("ParseDocument returned nil doc")
			}
		})
	}
}

// Pin the line-anchor predicate directly so a future refactor that
// drops the helper doesn't quietly regress the semantic.
func TestHasConflictMarkers_LineAnchored(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"empty", "", false},
		{"plain prose", "hello world\n", false},
		{"marker at column 0", "<<<<<<< HEAD\nfoo\n=======\nbar\n>>>>>>> branch\n", true},
		{"marker at column 0 of second line", "first line\n<<<<<<< HEAD\nbody\n", true},
		{"marker preceded by spaces (indented)", "  <<<<<<< HEAD\n", false},
		{"marker inside codespan", "use `<<<<<<<` to spot conflicts\n", false},
		{"marker mid-line", "the prefix <<<<<<< appears here\n", false},
		{"marker after CRLF (Windows line ending)", "line\r\n<<<<<<< HEAD\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasConflictMarkersString(tt.content); got != tt.want {
				t.Errorf("HasConflictMarkersString(%q) = %v, want %v", tt.content, got, tt.want)
			}
			if got := HasConflictMarkers([]byte(tt.content)); got != tt.want {
				t.Errorf("HasConflictMarkers([]byte(%q)) = %v, want %v", tt.content, got, tt.want)
			}
		})
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

func TestFormatDocumentOrdered(t *testing.T) {
	frontmatter := map[string]interface{}{
		"id":       "REQ-001",
		"type":     "requirement",
		"title":    "Some title",
		"status":   "draft",
		"priority": "high",
	}
	keyOrder := []string{"id", "type", "title"}

	output, err := FormatDocumentOrdered(frontmatter, "body text", keyOrder)
	if err != nil {
		t.Fatalf("FormatDocumentOrdered error: %v", err)
	}

	// Ordered keys come first in sequence.
	idPos := strings.Index(output, "id:")
	typePos := strings.Index(output, "type:")
	titlePos := strings.Index(output, "title:")
	priorityPos := strings.Index(output, "priority:")
	statusPos := strings.Index(output, "status:")
	if idPos >= typePos || typePos >= titlePos {
		t.Errorf("expected id < type < title, got positions %d %d %d", idPos, typePos, titlePos)
	}
	// Remaining keys appear alphabetically after the ordered ones.
	if titlePos >= priorityPos || priorityPos >= statusPos {
		t.Errorf("expected title < priority < status (alphabetical after ordered), got %d %d %d",
			titlePos, priorityPos, statusPos)
	}

	// Body content is preserved.
	if !strings.Contains(output, "body text") {
		t.Errorf("body missing in output:\n%s", output)
	}
}

func TestFormatDocumentOrdered_EmptyKeyOrder(t *testing.T) {
	// No keyOrder should fall back to default yaml.Marshal.
	fm := map[string]interface{}{"a": "1", "b": "2"}
	output, err := FormatDocumentOrdered(fm, "body", nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(output, "a: ") || !strings.Contains(output, "b: ") {
		t.Errorf("expected both keys, got:\n%s", output)
	}
}

func TestFormatDocumentOrdered_KeyNotInData(t *testing.T) {
	// Ordered keys that don't exist in data should be silently skipped.
	fm := map[string]interface{}{"a": "1"}
	output, err := FormatDocumentOrdered(fm, "", []string{"missing", "a"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(output, "a:") {
		t.Errorf("missing 'a' in output:\n%s", output)
	}
	if strings.Contains(output, "missing") {
		t.Errorf("'missing' key should not appear in output:\n%s", output)
	}
}
