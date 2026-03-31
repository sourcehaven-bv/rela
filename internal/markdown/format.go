package markdown

import (
	"bytes"
	"regexp"
	"strings"

	wordwrap "github.com/mitchellh/go-wordwrap"
	markdown "github.com/teekennedy/goldmark-markdown"
	"github.com/yuin/goldmark"
)

// orderedListPattern matches ordered list items (e.g., "1. ", "2. ").
var orderedListPattern = regexp.MustCompile(`^\d+\.\s`)

// DefaultLineWidth is the default line width for paragraph wrapping.
const DefaultLineWidth = 80

// FormatMarkdown normalizes markdown content using consistent formatting rules:
// - ATX-style headings (#)
// - Consistent list markers
// - Normalized whitespace and indentation
// - Consistent code fence style
// - Paragraph text wrapped at 80 characters
//
// Returns the formatted content, or the original content if formatting fails.
func FormatMarkdown(content string) string {
	return FormatMarkdownWithWidth(content, DefaultLineWidth)
}

// FormatMarkdownWithWidth normalizes markdown content with a specific line width.
func FormatMarkdownWithWidth(content string, lineWidth int) string {
	if content == "" {
		return ""
	}

	// Trim trailing whitespace but preserve structure
	content = strings.TrimRight(content, " \t")

	r := markdown.NewRenderer(
		markdown.WithHeadingStyle(markdown.HeadingStyleATX),
		markdown.WithIndentStyle(markdown.IndentStyleSpaces),
	)

	md := goldmark.New(goldmark.WithRenderer(r))

	var buf bytes.Buffer
	if err := md.Convert([]byte(content), &buf); err != nil {
		// If formatting fails, return original content
		return content
	}

	result := buf.String()

	// Post-process: wrap paragraphs at line width
	result = wrapParagraphs(result, lineWidth)

	// Ensure single trailing newline
	result = strings.TrimRight(result, "\n") + "\n"

	return result
}

// wrapParagraphs wraps paragraph text while preserving code blocks, lists, headings, etc.
func wrapParagraphs(content string, lineWidth int) string {
	lines := strings.Split(content, "\n")
	var result []string
	paragraphLines := make([]string, 0, 10)
	inCodeBlock := false
	codeBlockMarker := ""

	// Ensure lineWidth is positive to avoid overflow
	if lineWidth <= 0 {
		lineWidth = DefaultLineWidth
	}

	flushParagraph := func() {
		if len(paragraphLines) > 0 {
			// Join lines and wrap
			text := strings.Join(paragraphLines, " ")
			text = strings.TrimSpace(text)
			if text != "" {
				wrapped := wordwrap.WrapString(text, uint(lineWidth)) //nolint:gosec // lineWidth is validated positive
				result = append(result, wrapped)
			}
			paragraphLines = paragraphLines[:0]
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for code block markers
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			flushParagraph()
			switch {
			case !inCodeBlock:
				inCodeBlock = true
				codeBlockMarker = trimmed[:3]
				result = append(result, line)
			case strings.HasPrefix(trimmed, codeBlockMarker):
				inCodeBlock = false
				codeBlockMarker = ""
				result = append(result, line)
			default:
				result = append(result, line)
			}
			continue
		}

		// Inside code block - preserve exactly
		if inCodeBlock {
			result = append(result, line)
			continue
		}

		// Check for special lines that shouldn't be wrapped
		if isSpecialLine(trimmed) {
			flushParagraph()
			result = append(result, line)
			continue
		}

		// Empty line - end of paragraph
		if trimmed == "" {
			flushParagraph()
			result = append(result, "")
			continue
		}

		// Regular paragraph content
		paragraphLines = append(paragraphLines, trimmed)
	}

	flushParagraph()

	return strings.Join(result, "\n")
}

// isSpecialLine returns true for lines that shouldn't be wrapped.
func isSpecialLine(line string) bool {
	if line == "" {
		return true
	}

	// Headings
	if strings.HasPrefix(line, "#") {
		return true
	}

	// List items (unordered)
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return true
	}

	// List items (ordered) - matches "1. ", "2. ", etc.
	if orderedListPattern.MatchString(line) {
		return true
	}

	// Blockquotes
	if strings.HasPrefix(line, ">") {
		return true
	}

	// Thematic breaks
	if line == "---" || line == "***" || line == "___" {
		return true
	}

	// Indented code (4+ spaces or tab)
	if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
		return true
	}

	// HTML-style comments
	if strings.HasPrefix(line, "<!--") {
		return true
	}

	// Tables (pipe-delimited)
	if strings.HasPrefix(line, "|") {
		return true
	}

	return false
}
