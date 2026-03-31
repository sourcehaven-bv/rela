package markdown

import (
	"testing"
)

func TestFormatMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty content",
			input:    "",
			expected: "",
		},
		{
			name:     "plain text",
			input:    "Hello world",
			expected: "Hello world\n",
		},
		{
			name:     "ATX heading preserved",
			input:    "## Heading",
			expected: "## Heading\n",
		},
		{
			name:     "setext h1 converted to ATX",
			input:    "Heading\n=======",
			expected: "# Heading\n",
		},
		{
			name:     "setext h2 converted to ATX",
			input:    "Heading\n-------",
			expected: "## Heading\n",
		},
		{
			name:     "trailing whitespace removed",
			input:    "Hello world   \t",
			expected: "Hello world\n",
		},
		{
			name:     "multiple trailing newlines normalized",
			input:    "Hello world\n\n\n",
			expected: "Hello world\n",
		},
		{
			name:     "unordered list normalized",
			input:    "- item 1\n- item 2",
			expected: "- item 1\n- item 2\n",
		},
		{
			name:     "ordered list normalized",
			input:    "1. item 1\n2. item 2",
			expected: "1. item 1\n2. item 2\n",
		},
		{
			name:     "code block preserved",
			input:    "```go\nfunc main() {}\n```",
			expected: "```go\nfunc main() {}\n```\n",
		},
		{
			name:     "inline code preserved",
			input:    "Use `code` here",
			expected: "Use `code` here\n",
		},
		{
			name:     "link preserved",
			input:    "[text](http://example.com)",
			expected: "[text](http://example.com)\n",
		},
		{
			name:     "emphasis preserved",
			input:    "*italic* and **bold**",
			expected: "*italic* and **bold**\n",
		},
		{
			name:     "blockquote preserved",
			input:    "> quoted text",
			expected: "> quoted text\n",
		},
		{
			name:     "paragraph separation preserved",
			input:    "Paragraph 1\n\nParagraph 2",
			expected: "Paragraph 1\n\nParagraph 2\n",
		},
		{
			name:     "complex document",
			input:    "## Description\n\nSome text here.\n\n- Item 1\n- Item 2\n\n## Notes\n\n> A quote",
			expected: "## Description\n\nSome text here.\n\n- Item 1\n- Item 2\n\n## Notes\n\n> A quote\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMarkdown(tt.input)
			if result != tt.expected {
				t.Errorf("FormatMarkdown() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatMarkdown_Idempotent(t *testing.T) {
	// Formatting the same content twice should give identical results
	inputs := []string{
		"## Heading\n\nSome content.\n\n- List item\n",
		"Heading\n=======\n\nParagraph",
		"> Quote\n\n```\ncode\n```\n",
		"This is a very long paragraph that should be wrapped at the default line width of eighty characters.",
	}

	for _, input := range inputs {
		first := FormatMarkdown(input)
		second := FormatMarkdown(first)
		if first != second {
			t.Errorf("FormatMarkdown is not idempotent:\nFirst:  %q\nSecond: %q", first, second)
		}
	}
}

func TestFormatMarkdown_LineWrapping(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected string
	}{
		{
			name:     "short paragraph not wrapped",
			input:    "Short text.",
			width:    80,
			expected: "Short text.\n",
		},
		{
			name:     "long paragraph wrapped",
			input:    "This is a very long paragraph that exceeds the line width and should be wrapped at word boundaries.",
			width:    40,
			expected: "This is a very long paragraph that\nexceeds the line width and should be\nwrapped at word boundaries.\n",
		},
		{
			name:     "code block not wrapped",
			input:    "```\nThis is a very long line of code that should not be wrapped even if it exceeds the line width.\n```",
			width:    40,
			expected: "```\nThis is a very long line of code that should not be wrapped even if it exceeds the line width.\n```\n",
		},
		{
			name:     "heading not wrapped",
			input:    "## This is a very long heading that should not be wrapped even if it exceeds the line width",
			width:    40,
			expected: "## This is a very long heading that should not be wrapped even if it exceeds the line width\n",
		},
		{
			name:     "inline code preserved in wrapped text",
			input:    "Use the `FormatMarkdown` function to format your content and make it look nice.",
			width:    40,
			expected: "Use the `FormatMarkdown` function to\nformat your content and make it look\nnice.\n",
		},
		{
			name:     "link preserved in wrapped text",
			input:    "Visit the [documentation](https://example.com/docs) for more information about this feature.",
			width:    40,
			expected: "Visit the\n[documentation](https://example.com/docs)\nfor more information about this feature.\n",
		},
		{
			name:     "emphasis preserved in wrapped text",
			input:    "This is *important* and this is **very important** information that you should read.",
			width:    40,
			expected: "This is *important* and this is **very\nimportant** information that you should\nread.\n",
		},
		{
			name:     "multiple paragraphs wrapped independently",
			input:    "First paragraph with some text that should be wrapped.\n\nSecond paragraph with different text that also needs wrapping.",
			width:    40,
			expected: "First paragraph with some text that\nshould be wrapped.\n\nSecond paragraph with different text\nthat also needs wrapping.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMarkdownWithWidth(tt.input, tt.width)
			if result != tt.expected {
				t.Errorf("FormatMarkdownWithWidth() =\n%q\nwant:\n%q", result, tt.expected)
			}
		})
	}
}

func TestFormatMarkdownWithWidth_DefaultWidth(t *testing.T) {
	// Test that FormatMarkdown uses 80 char width
	longText := "This is a paragraph that is exactly one hundred characters long and should wrap."
	result := FormatMarkdown(longText)

	// Should be wrapped since it exceeds 80 chars
	if result != "" && result[0] != '\n' {
		lines := splitLines(result)
		for _, line := range lines {
			if len(line) > 80 {
				t.Errorf("Line exceeds 80 chars: %q (len=%d)", line, len(line))
			}
		}
	}
}

func TestFormatMarkdownWithWidth_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected string
	}{
		{
			name:     "zero width falls back to default",
			input:    "Short text.",
			width:    0,
			expected: "Short text.\n",
		},
		{
			name:     "negative width falls back to default",
			input:    "Short text.",
			width:    -10,
			expected: "Short text.\n",
		},
		{
			name:     "unclosed code block handled gracefully",
			input:    "```\ncode here\nmore code",
			width:    40,
			expected: "```\ncode here\nmore code\n```\n", // goldmark auto-closes
		},
		{
			name:     "long list item not wrapped (intentional limitation)",
			input:    "- This is a very long list item that exceeds the line width significantly.",
			width:    40,
			expected: "- This is a very long list item that exceeds the line width significantly.\n",
		},
		{
			name:     "long blockquote not wrapped (intentional limitation)",
			input:    "> This is a very long blockquote that exceeds the line width significantly.",
			width:    40,
			expected: "> This is a very long blockquote that exceeds the line width significantly.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMarkdownWithWidth(tt.input, tt.width)
			if result != tt.expected {
				t.Errorf("FormatMarkdownWithWidth() =\n%q\nwant:\n%q", result, tt.expected)
			}
		})
	}
}

func splitLines(s string) []string {
	var lines []string
	var current string
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(r)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
