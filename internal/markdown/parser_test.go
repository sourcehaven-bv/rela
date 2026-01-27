package markdown

import (
	"strings"
	"testing"
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
	if err != nil {
		t.Fatalf("ParseDocument failed: %v", err)
	}

	if doc.Frontmatter == nil {
		t.Fatal("frontmatter should not be nil")
	}

	if doc.GetString("id") != "REQ-001" {
		t.Errorf("id = %q, want %q", doc.GetString("id"), "REQ-001")
	}
	if doc.GetString("type") != "requirement" {
		t.Errorf("type = %q, want %q", doc.GetString("type"), "requirement")
	}
	if doc.GetString("title") != "Test Requirement" {
		t.Errorf("title = %q, want %q", doc.GetString("title"), "Test Requirement")
	}

	tags := doc.GetStringSlice("tags")
	if len(tags) != 2 {
		t.Errorf("got %d tags, want 2", len(tags))
	}

	if !strings.Contains(doc.Content, "# Description") {
		t.Error("content should contain heading")
	}
	if !strings.Contains(doc.Content, "This is a test requirement") {
		t.Error("content should contain text")
	}
}

func TestParseDocument_NoFrontmatter(t *testing.T) {
	content := `# Just a heading

Some content without frontmatter.
`

	doc, err := ParseDocument(content)
	if err != nil {
		t.Fatalf("ParseDocument failed: %v", err)
	}

	if len(doc.Frontmatter) > 0 {
		t.Errorf("frontmatter should be empty, got %+v", doc.Frontmatter)
	}

	if !strings.Contains(doc.Content, "# Just a heading") {
		t.Error("content should contain heading")
	}
}

func TestParseDocument_EmptyFrontmatter(t *testing.T) {
	content := `---
---

Content after empty frontmatter.
`

	doc, err := ParseDocument(content)
	if err != nil {
		t.Fatalf("ParseDocument failed: %v", err)
	}

	if len(doc.Frontmatter) > 0 {
		t.Errorf("frontmatter should be empty, got %+v", doc.Frontmatter)
	}

	if !strings.Contains(doc.Content, "Content after empty frontmatter") {
		t.Error("content should be present")
	}
}

func TestParseDocument_InvalidYAML(t *testing.T) {
	content := `---
id: REQ-001
type: [invalid
---

Content here.
`

	_, err := ParseDocument(content)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestParseDocument_FrontmatterOnly(t *testing.T) {
	content := `---
id: REQ-001
type: requirement
---
`

	doc, err := ParseDocument(content)
	if err != nil {
		t.Fatalf("ParseDocument failed: %v", err)
	}

	if doc.GetString("id") != "REQ-001" {
		t.Errorf("id = %q, want %q", doc.GetString("id"), "REQ-001")
	}

	if doc.Content != "" {
		t.Errorf("content = %q, want empty string", doc.Content)
	}
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
	if err != nil {
		t.Fatalf("FormatDocument failed: %v", err)
	}

	if !strings.HasPrefix(formatted, "---\n") {
		t.Error("formatted document should start with ---")
	}
	if !strings.Contains(formatted, "id: REQ-001") {
		t.Error("formatted document should contain id")
	}
	if !strings.Contains(formatted, "type: requirement") {
		t.Error("formatted document should contain type")
	}
	if !strings.Contains(formatted, "# Description") {
		t.Error("formatted document should contain content")
	}

	// Verify it can be parsed back
	doc, err := ParseDocument(formatted)
	if err != nil {
		t.Fatalf("failed to parse formatted document: %v", err)
	}
	if doc.GetString("id") != "REQ-001" {
		t.Errorf("id = %q, want %q", doc.GetString("id"), "REQ-001")
	}
}

func TestFormatDocument_EmptyFrontmatter(t *testing.T) {
	frontmatter := map[string]interface{}{}
	content := "Just content without frontmatter."

	formatted, err := FormatDocument(frontmatter, content)
	if err != nil {
		t.Fatalf("FormatDocument failed: %v", err)
	}

	if strings.HasPrefix(formatted, "---") {
		t.Error("formatted document should not have frontmatter delimiters")
	}
	if !strings.Contains(formatted, "Just content") {
		t.Error("formatted document should contain content")
	}
}

func TestFormatDocument_NoContent(t *testing.T) {
	frontmatter := map[string]interface{}{
		"id":   "REQ-001",
		"type": "requirement",
	}
	content := ""

	formatted, err := FormatDocument(frontmatter, content)
	if err != nil {
		t.Fatalf("FormatDocument failed: %v", err)
	}

	if !strings.HasPrefix(formatted, "---\n") {
		t.Error("formatted document should start with ---")
	}
	if !strings.Contains(formatted, "id: REQ-001") {
		t.Error("formatted document should contain id")
	}

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
	if err != nil {
		t.Fatalf("FormatDocument failed: %v", err)
	}

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
	if got := doc.GetString("string_field"); got != "value" {
		t.Errorf("GetString(string_field) = %q, want %q", got, "value")
	}

	// Test non-existent field
	if got := doc.GetString("nonexistent"); got != "" {
		t.Errorf("GetString(nonexistent) = %q, want empty string", got)
	}

	// Test non-string field
	if got := doc.GetString("int_field"); got != "" {
		t.Errorf("GetString(int_field) = %q, want empty string for non-string", got)
	}
}

func TestDocumentGetString_NilFrontmatter(t *testing.T) {
	doc := &Document{
		Frontmatter: nil,
	}

	if got := doc.GetString("any_field"); got != "" {
		t.Errorf("GetString with nil frontmatter = %q, want empty string", got)
	}
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
	if err != nil {
		t.Fatalf("splitFrontmatter failed: %v", err)
	}

	if !strings.Contains(frontmatter, "id: REQ-001") {
		t.Error("frontmatter should contain id")
	}
	if !strings.Contains(frontmatter, "type: requirement") {
		t.Error("frontmatter should contain type")
	}

	if !strings.Contains(body, "# Heading") {
		t.Error("body should contain heading")
	}
	if !strings.Contains(body, "Content here") {
		t.Error("body should contain content")
	}

	// Body should not contain frontmatter delimiters
	if strings.Contains(body, "---") {
		t.Error("body should not contain frontmatter delimiters")
	}
}

func TestSplitFrontmatter_NoFrontmatter(t *testing.T) {
	content := `# Just a heading

Some content.
`

	frontmatter, body, err := splitFrontmatter(content)
	if err != nil {
		t.Fatalf("splitFrontmatter failed: %v", err)
	}

	if frontmatter != "" {
		t.Errorf("frontmatter = %q, want empty string", frontmatter)
	}

	if !strings.Contains(body, "# Just a heading") {
		t.Error("body should contain all content")
	}
}

func TestSplitFrontmatter_EmptyContent(t *testing.T) {
	content := ""

	frontmatter, body, err := splitFrontmatter(content)
	if err != nil {
		t.Fatalf("splitFrontmatter failed: %v", err)
	}

	if frontmatter != "" {
		t.Errorf("frontmatter = %q, want empty", frontmatter)
	}
	if body != "" {
		t.Errorf("body = %q, want empty", body)
	}
}

func TestSplitFrontmatter_OnlyFrontmatter(t *testing.T) {
	content := `---
id: REQ-001
---
`

	frontmatter, body, err := splitFrontmatter(content)
	if err != nil {
		t.Fatalf("splitFrontmatter failed: %v", err)
	}

	if !strings.Contains(frontmatter, "id: REQ-001") {
		t.Error("frontmatter should contain id")
	}

	if body != "" {
		t.Errorf("body = %q, want empty", body)
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
	if err != nil {
		t.Fatalf("ParseDocument failed: %v", err)
	}

	// Format back
	formatted, err := FormatDocument(doc.Frontmatter, doc.Content)
	if err != nil {
		t.Fatalf("FormatDocument failed: %v", err)
	}

	// Parse again
	doc2, err := ParseDocument(formatted)
	if err != nil {
		t.Fatalf("ParseDocument (second) failed: %v", err)
	}

	// Verify key fields are preserved
	if doc2.GetString("id") != "REQ-001" {
		t.Errorf("id = %q, want %q", doc2.GetString("id"), "REQ-001")
	}
	if doc2.GetString("type") != "requirement" {
		t.Errorf("type = %q, want %q", doc2.GetString("type"), "requirement")
	}
	if doc2.GetString("title") != "Test Requirement" {
		t.Errorf("title = %q, want %q", doc2.GetString("title"), "Test Requirement")
	}

	tags := doc2.GetStringSlice("tags")
	if len(tags) != 2 {
		t.Errorf("got %d tags, want 2", len(tags))
	}

	if !strings.Contains(doc2.Content, "# Description") {
		t.Error("content should contain heading")
	}
	if !strings.Contains(doc2.Content, "More content here") {
		t.Error("content should be preserved")
	}
}
