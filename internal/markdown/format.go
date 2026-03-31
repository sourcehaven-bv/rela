package markdown

import (
	"bytes"
	"strings"

	markdown "github.com/teekennedy/goldmark-markdown"
	"github.com/yuin/goldmark"
)

// FormatMarkdown normalizes markdown content using consistent formatting rules:
// - ATX-style headings (#)
// - Consistent list markers
// - Normalized whitespace and indentation
// - Consistent code fence style
//
// Returns the formatted content, or the original content if formatting fails.
func FormatMarkdown(content string) string {
	if content == "" {
		return ""
	}

	// Trim trailing whitespace but preserve structure
	content = strings.TrimRight(content, " \t")

	renderer := markdown.NewRenderer(
		markdown.WithHeadingStyle(markdown.HeadingStyleATX),
		markdown.WithIndentStyle(markdown.IndentStyleSpaces),
	)
	md := goldmark.New(goldmark.WithRenderer(renderer))

	var buf bytes.Buffer
	if err := md.Convert([]byte(content), &buf); err != nil {
		// If formatting fails, return original content
		return content
	}

	result := buf.String()

	// Ensure single trailing newline
	result = strings.TrimRight(result, "\n") + "\n"

	return result
}
