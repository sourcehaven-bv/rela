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

// maxFormatPasses bounds the fixed-point iteration in
// [FormatMarkdownWithWidth]. Convergence is fast in practice (≤2 passes across
// the fuzz corpus); the bound is a backstop against a pathological input that
// never settles, in which case the last pass is still returned deterministically.
const maxFormatPasses = 8

// FormatMarkdown normalizes markdown content using consistent formatting rules:
// - ATX-style headings (#)
// - Consistent list markers
// - Normalized whitespace and indentation
// - Consistent code fence style
// - Paragraph text wrapped at 80 characters
//
// The result is idempotent: FormatMarkdown(FormatMarkdown(x)) == FormatMarkdown(x).
// Returns the formatted content, or the original content if formatting fails.
func FormatMarkdown(content string) string {
	return FormatMarkdownWithWidth(content, DefaultLineWidth)
}

// FormatMarkdownWithWidth normalizes markdown content with a specific line
// width, iterating to a formatting fixed point so the result is idempotent.
//
// A single goldmark round-trip is NOT idempotent for every input — goldmark can
// re-parse its own output into a different document (e.g. "**\n*" renders to
// "** *", which on the next pass parses as a thematic break "---"). Callers that
// re-normalize already-formatted content rely on idempotency: the canonical
// content hash (internal/canonical) re-formats both fsstore's reflowed body and
// pgstore's raw body, and they must converge in one call. Iterating here makes
// that hold for every caller rather than pushing the loop onto each one.
func FormatMarkdownWithWidth(content string, lineWidth int) string {
	if content == "" {
		return ""
	}
	result := content
	for range maxFormatPasses {
		next := formatOnce(result, lineWidth)
		if next == result {
			break
		}
		result = next
	}
	return result
}

// formatOnce applies one pass of markdown normalization (goldmark render +
// paragraph wrap + blank-line trim). It is not necessarily idempotent on its
// own; [FormatMarkdownWithWidth] iterates it to a fixed point.
func formatOnce(content string, lineWidth int) string {
	// Trim trailing whitespace but preserve structure.
	content = strings.TrimRight(content, " \t")

	r := markdown.NewRenderer(
		markdown.WithHeadingStyle(markdown.HeadingStyleATX),
		markdown.WithIndentStyle(markdown.IndentStyleSpaces),
	)

	md := goldmark.New(goldmark.WithRenderer(r))

	var buf bytes.Buffer
	if err := md.Convert([]byte(content), &buf); err != nil {
		// If formatting fails, return original content.
		return content
	}

	result := wrapParagraphs(buf.String(), lineWidth)

	// Normalize surrounding blank lines to a single trailing newline. Trimming
	// the leading newline (not just the trailing) is part of what lets the
	// iteration converge: goldmark can emit a leading blank line for some inputs.
	return strings.Trim(result, "\n") + "\n"
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
