package markdown

import (
	"strings"
	"testing"
)

// TestFormatMarkdown_FencedBacktickContentPreserved guards the boundary of the
// code-span whitespace normalization: a fenced code block can legitimately
// contain backtick pairs AND internal trailing spaces, which are literal content
// and must survive FormatMarkdown verbatim. The normalization is applied only to
// prose lines (via wrapParagraphs' fence-awareness), never inside a fence.
func TestFormatMarkdown_FencedBacktickContentPreserved(t *testing.T) {
	in := "Para before.\n\n```\na `b   ` c\n```\n\nPara after.\n"
	out := FormatMarkdown(in)
	if !strings.Contains(out, "a `b   ` c") {
		t.Errorf("fenced backtick content with internal spaces was altered:\n%s", out)
	}
}
