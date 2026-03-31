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
	}

	for _, input := range inputs {
		first := FormatMarkdown(input)
		second := FormatMarkdown(first)
		if first != second {
			t.Errorf("FormatMarkdown is not idempotent:\nFirst:  %q\nSecond: %q", first, second)
		}
	}
}
