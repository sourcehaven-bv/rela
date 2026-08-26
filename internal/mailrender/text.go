package mailrender

import (
	"html"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// renderText produces the text/plain alternative.
//
// It is generated from the MODEL, not by stripping tags from the HTML. Two
// reasons: a tag-stripped table is unreadable (cells run together with no
// alignment), and deriving one part from the other means a rendering bug in the
// HTML silently corrupts the text too.
//
// Every message carries this part. It is what plain-text clients show, and its
// absence is a well-known spam signal.
//
// # Markdown is parsed, not pattern-matched
//
// Prose goes through goldmark and this file walks the resulting AST. An earlier
// version hand-rolled the string work — regex tag-stripping, ReplaceAll on
// emphasis markers, index arithmetic for links — and every one of those was
// wrong in a way the first round of tests missed:
//
//   - Unescaping AFTER tag-stripping re-materialized entity-encoded markup, so
//     "&lt;script&gt;" in an entity property arrived as a live tag in the body.
//   - ReplaceAll("**", "") turned "5*3*2" into "532" and "__init__" into "init".
//   - Taking the first ")" as a link terminator truncated any URL containing
//     parens and left stray text in the body.
//
// The parser already knows what is emphasis, what is a link, and what is
// literal text. Asking it is both correct and less code than approximating it.
func (r *Renderer) renderText(m *Message) []byte {
	var b strings.Builder

	if m.Subject != "" {
		s := plainText(m.Subject)
		b.WriteString(s)
		b.WriteString("\n")
		b.WriteString(strings.Repeat("=", displayWidth(s)))
		b.WriteString("\n\n")
	}

	if s := r.plainMarkdown(m.Intro); s != "" {
		b.WriteString(s)
		b.WriteString("\n\n")
	}

	for i := range m.Sections {
		r.writeTextSection(&b, &m.Sections[i])
	}

	if s := r.plainMarkdown(m.Footer); s != "" {
		b.WriteString("--\n")
		b.WriteString(s)
		b.WriteString("\n")
	}

	return []byte(b.String())
}

func (r *Renderer) writeTextSection(b *strings.Builder, s *Section) {
	if s.Title != "" {
		t := plainText(s.Title)
		b.WriteString(t)
		b.WriteString("\n")
		b.WriteString(strings.Repeat("-", displayWidth(t)))
		b.WriteString("\n")
	}
	if body := r.plainMarkdown(s.Body); body != "" {
		b.WriteString(body)
		b.WriteString("\n")
	}

	if len(s.Rows) == 0 {
		if len(s.Columns) > 0 {
			b.WriteString("Nothing to show.\n")
		}
		b.WriteString("\n")
		return
	}

	// One "label: value" block per row rather than a fixed-width table: column
	// alignment breaks the moment a cell is wider than expected, and a mail
	// client using a proportional font destroys it regardless.
	for i, row := range s.Rows {
		for j, cell := range row {
			label := ""
			if j < len(s.Columns) {
				label = s.Columns[j]
			}
			if label != "" {
				b.WriteString(plainText(label))
				b.WriteString(": ")
			}
			b.WriteString(plainText(cell))
			b.WriteString("\n")
		}
		if i < len(s.Links) && s.Links[i] != "" {
			if href, ok := r.safeHref(s.Links[i]); ok {
				b.WriteString(href)
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
}

// tagRe matches an HTML tag.
var tagRe = regexp.MustCompile(`(?s)<[^>]*>`)

// scriptStyleRe matches a script/style/iframe/object/embed element together
// with its content. Explicit alternations rather than a backreference, which
// RE2 does not support.
var scriptStyleRe = regexp.MustCompile(`(?is)` +
	`<script\b[^>]*>.*?</script\s*>|<script\b[^>]*>|` +
	`<style\b[^>]*>.*?</style\s*>|<style\b[^>]*>|` +
	`<iframe\b[^>]*>.*?</iframe\s*>|<iframe\b[^>]*>|` +
	`<object\b[^>]*>.*?</object\s*>|<object\b[^>]*>|` +
	`<embed\b[^>]*>.*?</embed\s*>|<embed\b[^>]*>`)

// maxUnescapePasses bounds the decode/strip loop in stripMarkup.
const maxUnescapePasses = 4

// plainText neutralizes a bare string that is NOT markdown — a subject, a
// column label, a table cell.
//
// These are values, not documents, so they are not parsed as markdown. They are
// still UNTRUSTED, and the text part is not exempt from that: markup echoed
// into a mail body is a phishing surface even where a client renders it inert.
func plainText(s string) string {
	return stripMarkup(s)
}

// stripMarkup removes HTML from an untrusted value.
//
// Decoding happens BEFORE stripping, and the pair repeats to a fixed point.
// Both details are load-bearing: stripping first and decoding after simply
// re-materializes whatever was entity-encoded ("&lt;script&gt;" becomes a live
// tag), and a single pass leaves double-encoded input ("&amp;lt;script&amp;gt;")
// one decode away from the same outcome. The loop is bounded so a pathological
// value cannot spin.
func stripMarkup(s string) string {
	for range maxUnescapePasses {
		decoded := html.UnescapeString(s)
		stripped := tagRe.ReplaceAllString(scriptStyleRe.ReplaceAllString(decoded, ""), "")
		if stripped == s {
			return stripped
		}
		s = stripped
	}
	return s
}

// plainMarkdown renders markdown prose as readable plain text by walking the
// parsed AST.
func (r *Renderer) plainMarkdown(src string) string {
	if strings.TrimSpace(src) == "" {
		return ""
	}

	source := []byte(src)
	doc := r.md.Parser().Parse(text.NewReader(source))

	var b strings.Builder
	r.walkBlocks(&b, doc, source, 0)

	return strings.TrimSpace(b.String())
}

// walkBlocks renders block-level nodes. depth tracks list nesting for indent.
func (r *Renderer) walkBlocks(b *strings.Builder, n ast.Node, src []byte, depth int) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch node := c.(type) {
		case *ast.Heading, *ast.Paragraph, *ast.TextBlock:
			b.WriteString(r.inlineText(c, src))
			b.WriteString("\n\n")

		case *ast.List:
			r.walkBlocks(b, node, src, depth)
			b.WriteString("\n")

		case *ast.ListItem:
			// A list item wraps blocks; flatten them so the bullet and its
			// text stay on one line.
			var inner strings.Builder
			r.walkBlocks(&inner, node, src, depth+1)
			b.WriteString(strings.Repeat("  ", depth))
			b.WriteString("- ")
			b.WriteString(strings.TrimSpace(inner.String()))
			b.WriteString("\n")

		case *ast.FencedCodeBlock, *ast.CodeBlock:
			b.WriteString(rawLines(c, src))
			b.WriteString("\n\n")

		case *ast.Blockquote:
			var inner strings.Builder
			r.walkBlocks(&inner, node, src, depth)
			for line := range strings.SplitSeq(strings.TrimSpace(inner.String()), "\n") {
				b.WriteString("> ")
				b.WriteString(line)
				b.WriteString("\n")
			}
			b.WriteString("\n")

		case *ast.ThematicBreak:
			b.WriteString("---\n\n")

		case *ast.HTMLBlock:
			// Raw HTML in markdown is DROPPED, never echoed — the text-part
			// counterpart of bluemonday on the HTML side.

		default:
			// An extension node — a GFM table is the one that matters here.
			// Its rows are block children holding inline cells, so recursing
			// as blocks loses the row structure; render each row's cells on
			// one line instead. Anything else falls back to its inline text
			// rather than being dropped silently.
			r.writeExtensionBlock(b, c, src)
		}
	}
}

// writeExtensionBlock renders a node goldmark's core does not define — in
// practice a GFM table.
//
// A table's rows are block nodes whose children are inline cells. Recursing
// through walkBlocks would emit each cell as its own paragraph and lose the
// row; joining a row's cells with a separator keeps it legible.
func (r *Renderer) writeExtensionBlock(b *strings.Builder, n ast.Node, src []byte) {
	if n.Type() != ast.TypeBlock {
		b.WriteString(r.inlineText(n, src))
		return
	}

	// A row-like node: every child is a cell holding inline content.
	if isRowLike(n) {
		cells := make([]string, 0, n.ChildCount())
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			cells = append(cells, r.inlineText(c, src))
		}
		b.WriteString(strings.Join(cells, " | "))
		b.WriteString("\n")
		return
	}

	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		r.writeExtensionBlock(b, c, src)
	}
	if n.Parent() != nil && n.Parent().Type() == ast.TypeDocument {
		b.WriteString("\n")
	}
}

// isRowLike reports whether every child of n is an inline-bearing cell, which
// is how a GFM table row is shaped.
func isRowLike(n ast.Node) bool {
	if n.ChildCount() == 0 {
		return false
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Type() != ast.TypeBlock || c.ChildCount() == 0 {
			return false
		}
		if c.FirstChild().Type() != ast.TypeInline {
			return false
		}
	}
	return true
}

// inlineText flattens inline nodes to plain text.
//
// Emphasis markers vanish because the PARSER identified them as emphasis, which
// is exactly why "5*3*2" and "__init__" survive intact: goldmark does not treat
// those as emphasis, so there is nothing to remove.
func (r *Renderer) inlineText(n ast.Node, src []byte) string {
	var b strings.Builder
	r.writeInline(&b, n, src)
	return strings.TrimSpace(b.String())
}

func (r *Renderer) writeInline(b *strings.Builder, n ast.Node, src []byte) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch node := c.(type) {
		case *ast.Text:
			b.Write(node.Segment.Value(src))
			if node.SoftLineBreak() || node.HardLineBreak() {
				b.WriteString(" ")
			}

		case *ast.String:
			b.Write(node.Value)

		case *ast.Link:
			label := inlineOnly(node, src)
			b.WriteString(label)
			// The parser hands over the whole destination, so a URL containing
			// parens survives — the case the old index arithmetic truncated.
			if href, ok := r.safeHref(string(node.Destination)); ok && href != label {
				b.WriteString(" (")
				b.WriteString(href)
				b.WriteString(")")
			}

		case *ast.AutoLink:
			if href, ok := r.safeHref(string(node.URL(src))); ok {
				b.WriteString(href)
			}

		case *ast.Image:
			// An image has no plain-text equivalent beyond its alt text.
			b.WriteString(inlineOnly(node, src))

		case *ast.RawHTML:
			// Dropped, as above.

		default:
			// CodeSpan, Emphasis and anything else: keep the words, drop the
			// markers.
			r.writeInline(b, c, src)
		}
	}
}

// inlineOnly renders a node's children without following links, used for link
// and image labels.
func inlineOnly(n ast.Node, src []byte) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch node := c.(type) {
		case *ast.Text:
			b.Write(node.Segment.Value(src))
		case *ast.String:
			b.Write(node.Value)
		default:
			b.WriteString(inlineOnly(c, src))
		}
	}
	return strings.TrimSpace(b.String())
}

// rawLines returns a code block's literal content.
func rawLines(n ast.Node, src []byte) string {
	var b strings.Builder
	lines := n.Lines()
	for i := range lines.Len() {
		seg := lines.At(i)
		b.Write(seg.Value(src))
	}
	return strings.TrimRight(b.String(), "\n")
}

// displayWidth counts runes, so an underline under a non-ASCII heading is not
// stretched by multi-byte characters.
func displayWidth(s string) int {
	n := utf8.RuneCountInString(s)
	if n > maxUnderlineWidth {
		return maxUnderlineWidth
	}
	return n
}

// maxUnderlineWidth caps a heading underline so a very long title does not
// produce a rule that wraps in a narrow terminal.
const maxUnderlineWidth = 72
