package ingest

import "strings"

// Interpolate substitutes every {{ref}} in tmpl with resolve(ref), flattening
// each substituted value with [FlattenToLine].
//
// An unresolved reference becomes the EMPTY STRING rather than being left
// literal. A literal `{{body.host}}` stored in an entity is a silent corruption
// that looks like a template bug forever; empty is visibly missing and cannot
// be mistaken for data. It also means an absent optional field simply omits.
//
// A substituted value is never re-scanned, so a value cannot smuggle in a
// second reference. An unterminated `{{` is emitted verbatim so the operator
// sees the typo.
func Interpolate(tmpl string, resolve func(ref string) string) string {
	return interpolateWith(tmpl, func(ref string) string { return FlattenToLine(resolve(ref)) })
}

// interpolateWith is [Interpolate] with the per-value step left to the
// caller: value receives each reference and returns what to substitute.
func interpolateWith(tmpl string, value func(ref string) string) string {
	if !strings.Contains(tmpl, "{{") {
		return tmpl
	}
	var b strings.Builder
	rest := tmpl
	for {
		before, after, found := strings.Cut(rest, "{{")
		b.WriteString(before)
		if !found {
			break
		}
		ref, remainder, closed := strings.Cut(after, "}}")
		if !closed {
			b.WriteString("{{")
			b.WriteString(after)
			break
		}
		b.WriteString(value(strings.TrimSpace(ref)))
		rest = remainder
	}
	return b.String()
}

// markdownEscaper backslash-escapes the punctuation that can open markdown
// or HTML: images and links, raw HTML and character references, emphasis,
// code, ATX and setext headings, thematic breaks, tables and strikethrough. CommonMark allows a backslash
// before any ASCII punctuation, so escaping more than needed is harmless.
// A Replacer is single-pass, so an escape is never escaped again.
var markdownEscaper = strings.NewReplacer(
	`\`, `\\`, "`", "\\`", "*", `\*`, "_", `\_`, "[", `\[`, "]", `\]`,
	"<", `\<`, ">", `\>`, "&", `\&`, "!", `\!`, "#", `\#`, "|", `\|`, "~", `\~`,
	"-", `\-`, "=", `\=`, "+", `\+`,
)

// EscapeMarkdown returns s as markdown that renders as the literal text s.
func EscapeMarkdown(s string) string {
	return markdownEscaper.Replace(s)
}

// FlattenToLine collapses newlines and NULs in ONE INTERPOLATED VALUE, so a
// source cannot emit its own markdown structure through a template
// substitution.
//
// The threat is the payload, not the template. A `## Heading` arriving inside
// {{body.output}} would become a SIBLING of the section being appended to, so
// every later record targeting that section lands above it and the document
// silently reshapes itself.
//
// Applied per value in [Interpolate] rather than to the finished string,
// because the two are different concerns and the finished string cannot tell
// them apart. An operator writing a multi-line `content:` is describing the
// shape they want (their document, their call), and flattening that was an
// over-reach that also made a structured timeline entry impossible to express.
// What no operator can do, deliberately, is hand that power to their source.
//
// Never returns an error: a value that cannot be represented on one line is
// flattened, not rejected, because refusing would discard an alert whose
// producer will not resend it.
//
// # Only \n and \r, deliberately
//
// Those are the only characters that terminate a line in CommonMark, which is
// what goldmark parses on the write path (markdown.AppendToSection splits on
// \n) and what marked renders on the read path. \v, \f, U+0085, U+2028 and
// U+2029 are left alone because they cannot forge a heading: appended after a
// value they stay on the same line and no new heading is parsed.
//
// The safety therefore comes from the PARSER's definition of a line ending, not
// from this function. A future path that splits on Unicode line boundaries,
// notably anything using unicode.IsSpace, which does match \v, \f and U+0085,
// would put those survivors back in play and need more than this.
//
// Both parsers are held to that definition by tests, because one of them
// diverging is the only way the survivors become dangerous: goldmark by
// TestAppendToSection_NonCommonMarkBreaksForgeNoHeading in internal/markdown,
// marked by frontend/src/utils/markdownLineEndings.test.ts.
func FlattenToLine(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r':
			return ' '
		case 0:
			return -1
		}
		return r
	}, s)
}
