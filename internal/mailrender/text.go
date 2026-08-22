package mailrender

import (
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
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
func (r *Renderer) renderText(m *Message) []byte {
	var b strings.Builder

	if m.Subject != "" {
		b.WriteString(m.Subject)
		b.WriteString("\n")
		b.WriteString(strings.Repeat("=", displayWidth(m.Subject)))
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
		b.WriteString(s.Title)
		b.WriteString("\n")
		b.WriteString(strings.Repeat("-", displayWidth(s.Title)))
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
				b.WriteString(label)
				b.WriteString(": ")
			}
			b.WriteString(stripHTML(cell))
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

// tagRe matches an HTML tag. Used to STRIP markup from the text part.
var tagRe = regexp.MustCompile(`(?s)<[^>]*>`)

// stripHTML removes tags and decodes entities.
//
// The text part is NOT exempt from sanitization. Content here is the same
// untrusted entity markdown the HTML part carries, and markdown permits inline
// HTML, so raw <script> and onerror= reach this function. They must not reach
// the recipient: a text/plain body is still rendered by a mail client, some of
// which will happily interpret markup, and a body echoing an attacker's script
// tag is a phishing surface even where it is inert.
//
// Tags are dropped whole rather than escaped, because the goal is legibility:
// a reader wants the words, not the markup.
func stripHTML(s string) string {
	// Drop script/style CONTENT as well as their tags — leaving the body text
	// of a <script> behind would put "alert(1)" in the mail.
	s = scriptStyleRe.ReplaceAllString(s, "")
	return html.UnescapeString(tagRe.ReplaceAllString(s, ""))
}

// scriptStyleRe matches a script/style/iframe/object/embed element together
// with its content. Written as explicit alternations rather than a
// backreference, which RE2 does not support.
var scriptStyleRe = regexp.MustCompile(`(?is)` +
	`<script\b[^>]*>.*?</script\s*>|<script\b[^>]*>|` +
	`<style\b[^>]*>.*?</style\s*>|<style\b[^>]*>|` +
	`<iframe\b[^>]*>.*?</iframe\s*>|<iframe\b[^>]*>|` +
	`<object\b[^>]*>.*?</object\s*>|<object\b[^>]*>|` +
	`<embed\b[^>]*>.*?</embed\s*>|<embed\b[^>]*>`)

// plainMarkdown reduces markdown to readable plain text.
//
// Deliberately small: it unwraps the inline emphasis and link syntax that reads
// as noise in a terminal, strips any HTML the markdown carried, and leaves
// everything else alone. It is NOT a markdown parser and does not need to be —
// the text part's job is to be legible, not to round-trip.
func (r *Renderer) plainMarkdown(src string) string {
	s := stripHTML(strings.TrimSpace(src))
	if strings.TrimSpace(s) == "" {
		return ""
	}

	out := make([]string, 0, 8)
	for line := range strings.SplitSeq(s, "\n") {
		l := strings.TrimRight(line, " \t")
		trimmed := strings.TrimLeft(l, " \t")

		// ATX headings: drop the marker, keep the text.
		if h := strings.TrimLeft(trimmed, "#"); h != trimmed {
			l = strings.TrimSpace(h)
		}
		// Bullets: normalize -/*/+ to a single dash.
		if len(trimmed) > 1 && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') && trimmed[1] == ' ' {
			l = "- " + strings.TrimSpace(trimmed[2:])
		}

		out = append(out, r.unlink(stripEmphasis(l)))
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// emphasisRe matches *single* or _single_ emphasis around non-space text. Run
// after the ** and __ pairs so a bold marker is not half-consumed.
var emphasisRe = regexp.MustCompile(`(?:\*([^*\n]+)\*)|(?:\b_([^_\n]+)_\b)`)

// stripEmphasis removes ** __ * _ and backtick markers, leaving the words.
func stripEmphasis(s string) string {
	for _, m := range []string{"**", "__"} {
		s = strings.ReplaceAll(s, m, "")
	}
	s = emphasisRe.ReplaceAllString(s, "$1$2")
	return strings.Map(func(r rune) rune {
		if r == '`' {
			return -1
		}
		return r
	}, s)
}

// unlink rewrites [text](href) as "text (href)".
//
// The href goes through safeHref, so a relative link is resolved against
// BaseURL and an unsafe scheme is dropped — the same treatment the HTML part
// gives it. Without this the text alternative would carry "/board", which is
// meaningless in a mail client.
func (r *Renderer) unlink(s string) string {
	for {
		open := strings.Index(s, "](")
		if open == -1 {
			break
		}
		start := strings.LastIndex(s[:open], "[")
		if start == -1 {
			break
		}
		end := strings.Index(s[open:], ")")
		if end == -1 {
			break
		}
		end += open

		text := s[start+1 : open]
		href := s[open+2 : end]

		// Resolve exactly as the HTML part does, so a relative link is not
		// left dead in the text alternative — mail is read outside the app.
		if resolved, ok := r.safeHref(href); ok {
			href = resolved
		} else {
			href = ""
		}

		repl := text
		if href != "" && href != text {
			repl = text + " (" + href + ")"
		}
		s = s[:start] + repl + s[end+1:]
	}
	return s
}

// displayWidth counts runes, so an underline under a non-ASCII heading is not
// stretched by multi-byte characters.
func displayWidth(s string) int {
	n := utf8.RuneCountInString(s)
	if n > 72 {
		return 72
	}
	return n
}
