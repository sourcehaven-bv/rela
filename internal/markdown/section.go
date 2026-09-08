package markdown

import (
	"slices"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// AppendToSection appends line to the end of the markdown section headed by
// title, returning the new document content. Matching is on the heading's
// flattened text, case-insensitively, ignoring surrounding whitespace — an
// operator writing `section: Notifications` should not have to know whether the
// document spells it `## Notifications` or `### notifications`.
//
// # A missing section is CREATED, not an error
//
// When no heading matches, the section is appended to the end of the document
// as a new `## <title>` followed by line. This is deliberate and is the whole
// reason the function cannot fail on a well-formed document.
//
// The motivating caller is an inbound webhook (TKT-1EM4KL) receiving a
// monitoring alert. The producer there — Icinga — executes its notification
// command exactly once and never retries, so an error return means the alert is
// destroyed rather than deferred. Weighed against that, the cost of guessing
// wrong is a heading that an operator did not plan for, which is visible,
// harmless and trivially edited. Losing the payload is neither.
//
// It also makes the common setup work with no ceremony: a freshly created
// entity whose template has no `## Notifications` yet still accumulates them
// from the first delivery, so find-or-create does not need the template and the
// hook config to be kept in sync.
//
// The new heading is level 2 because that is the conventional top-level section
// depth in a rela entity body (the H1 is the title, carried in frontmatter).
//
// # Placement within a found section
//
// line is inserted after the LAST content line of the section — immediately
// before the next heading of the same or higher level, or at end of document if
// the section runs to the end. Trailing blank lines inside the section are
// preserved *after* the inserted line, so repeated appends accumulate in
// chronological order and do not drift a blank-line separator downward.
//
// A nested subsection belongs to its parent, so appending to `## Notifications`
// with a `### Details` beneath it places the line after `### Details`'s content,
// still inside `## Notifications`. That keeps "append to this section" meaning
// the whole section.
func AppendToSection(content, title, line string) string {
	if strings.TrimSpace(title) == "" {
		// No section named: append to the document body. Nothing to locate,
		// and inventing a heading with an empty name would be worse.
		return appendLines(content, line)
	}

	lines := splitContentLines(content)
	start, level := findHeading(content, title)
	if start < 0 {
		// Section absent — create it. See the doc comment: for the webhook
		// caller an error here would discard an unretried alert.
		body := appendLines(content, "## "+strings.TrimSpace(title))
		return appendLines(body, line)
	}

	end := sectionEnd(content, start, level)

	// Walk back over trailing blank lines so the appended line sits directly
	// after the section's real content rather than after its separator.
	insert := end
	for insert > start+1 && strings.TrimSpace(lines[insert-1]) == "" {
		insert--
	}

	// slices.Insert rather than a hand-rolled make(len+1) + three appends:
	// same result, but no capacity arithmetic on a length derived from
	// caller-supplied content. The old form tripped CodeQL's
	// allocation-size-overflow rule — not reachable here, since `lines` is a
	// split of a string already in memory, but the arithmetic is what the rule
	// keys on and removing it is cheaper than justifying it forever.
	return strings.Join(slices.Insert(lines, insert, line), "\n")
}

// findHeading returns the 0-based line index of the first heading whose
// flattened text matches title (case-insensitive, whitespace-trimmed) and its
// level. Returns (-1, 0) when absent.
//
// Parsing goes through goldmark rather than a line scan so a `#` inside a
// fenced code block is not mistaken for a heading — the same reason
// ExtractHeaders parses.
func findHeading(content, title string) (line, level int) {
	source := []byte(content)
	reader := text.NewReader(source)
	doc := goldmark.DefaultParser().Parse(reader)

	want := strings.ToLower(strings.TrimSpace(title))
	found, foundLevel := -1, 0
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || found >= 0 {
			return ast.WalkContinue, nil
		}
		heading, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		if strings.ToLower(strings.TrimSpace(headingText(heading, source))) != want {
			return ast.WalkContinue, nil
		}
		if ln := headingLine(heading, source); ln >= 0 {
			found, foundLevel = ln, heading.Level
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return found, foundLevel
}

// headingText flattens a heading's inline children to plain text.
func headingText(heading *ast.Heading, source []byte) string {
	var b strings.Builder
	for c := heading.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			b.Write(t.Segment.Value(source))
		}
	}
	return b.String()
}

// headingLine returns the 0-based source line index of a heading, or -1 when it
// carries no positional information (which goldmark does not produce for the
// ATX/setext headings this handles).
func headingLine(heading *ast.Heading, source []byte) int {
	lines := heading.Lines()
	if lines.Len() == 0 {
		return -1
	}
	// A setext heading's recorded segment is its TEXT line; the underline
	// follows. Either way the section's content starts after the recorded
	// span, and counting newlines before the segment gives the text line.
	return strings.Count(string(source[:lines.At(0).Start]), "\n")
}

// sectionEnd returns the exclusive 0-based line index at which the section
// headed at line start (of the given level) ends: the next heading at the same
// or a higher level, else the end of the document.
func sectionEnd(content string, start, level int) int {
	lines := splitContentLines(content)

	// Re-parse to get every heading's line and level, so a `#` inside a fenced
	// code block does not terminate the section early.
	source := []byte(content)
	doc := goldmark.DefaultParser().Parse(text.NewReader(source))

	end := len(lines)
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		heading, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		ln := headingLine(heading, source)
		if ln > start && heading.Level <= level && ln < end {
			end = ln
		}
		return ast.WalkContinue, nil
	})
	return end
}

// splitContentLines splits content into lines without a trailing empty element for a
// final newline, so joining reproduces the input.
func splitContentLines(content string) []string {
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

// appendLines appends line to content, separated by a blank line when content
// is non-empty, normalizing trailing whitespace so repeated appends do not
// accumulate blank lines.
func appendLines(content, line string) string {
	trimmed := strings.TrimRight(content, "\n")
	if strings.TrimSpace(trimmed) == "" {
		return line
	}
	return trimmed + "\n\n" + line
}
