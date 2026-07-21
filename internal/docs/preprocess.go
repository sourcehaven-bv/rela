// Package docs implements the rela-docs "doc language": a Markdown file with
// embedded Lua "islands" that pull reference fragments from the schema and a
// seeded in-memory graph.
//
// Two island forms (see segment):
//   - Statement island — a fenced ```rela block. Its Lua runs for side effects;
//     doc.* emit calls (h1/h2/md/…) and resolvers append markdown to an output
//     buffer that is spliced in at the island's position.
//   - Echo island — an inline `rela <expr>` code span. The expression is
//     evaluated and its string value substituted mid-text.
//
// The preprocessor here is pure text→segments; execution lives in runtime.go.
package docs

import (
	"strings"
)

// segmentKind classifies one parsed piece of a manual.
type segmentKind int

const (
	segLiteral   segmentKind = iota // verbatim markdown, emitted unchanged
	segStatement                    // a ```rela fenced block (runs for side effects)
	segEcho                         // an inline `rela <expr>` span (substituted)
)

// segment is one parsed piece of the manual. Line is the 1-based source line
// where the segment's content begins (the line after the opening ```rela fence
// for a statement island; the line of the span for an echo island) — used to
// report errors against the manual, not the island-internal offset.
type segment struct {
	kind segmentKind
	body string // island Lua source, or literal text
	line int    // 1-based manual line of body's first line
}

const fenceMarker = "```rela"

// parse splits a manual into segments. It is deliberately line-oriented for the
// fenced blocks (mirroring Markdown's own fence handling) and does an inline
// scan for echo spans within literal runs.
//
// Fence rules: a line whose trimmed content is exactly "```rela" opens a
// statement island; the island runs until a line whose trimmed content is
// exactly "```". An unterminated fence is a parse error. A fence that is NOT
// "```rela" (e.g. "```go", "```", or a plain "```rela-ish") is left in the
// literal stream untouched — including any `rela …` spans inside it, so a code
// sample showing the doc language renders literally.
func parse(src string) ([]segment, error) {
	// Normalize CRLF → LF so island bodies handed to Lua carry no stray \r and
	// fence/marker matching is uniform. Output is LF (markdown is agnostic).
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(src, "\n")
	var segs []segment
	var literal strings.Builder
	litStartLine := 1 // manual line where the current literal run began

	flushLiteral := func() {
		if literal.Len() == 0 {
			return
		}
		// Expand echo spans within this literal run into their own segments.
		segs = append(segs, splitEchoes(literal.String(), litStartLine)...)
		literal.Reset()
	}

	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// A generic code fence that is NOT ```rela: consume the whole fenced
		// block verbatim into the literal stream (no island parsing inside).
		if isGenericFenceOpen(trimmed) && trimmed != fenceMarker {
			literal.WriteString(line)
			literal.WriteByte('\n')
			i++
			for i < len(lines) {
				literal.WriteString(lines[i])
				literal.WriteByte('\n')
				closes := strings.TrimSpace(lines[i]) == "```"
				i++
				if closes {
					break
				}
			}
			continue
		}

		if trimmed == fenceMarker {
			flushLiteral()
			bodyStart := i + 2 // 1-based line of the first island body line
			var island strings.Builder
			i++
			closed := false
			for i < len(lines) {
				if strings.TrimSpace(lines[i]) == "```" {
					closed = true
					i++
					break
				}
				island.WriteString(lines[i])
				island.WriteByte('\n')
				i++
			}
			if !closed {
				return nil, &BuildError{
					Line: i, Kind: "parse",
					Msg: "unterminated ```rela block (missing closing ```)",
				}
			}
			segs = append(segs, segment{kind: segStatement, body: island.String(), line: bodyStart})
			litStartLine = i + 1
			continue
		}

		// Ordinary line: accumulate into the literal run.
		if literal.Len() == 0 {
			litStartLine = i + 1
		}
		literal.WriteString(line)
		if i < len(lines)-1 {
			literal.WriteByte('\n')
		}
		i++
	}
	flushLiteral()
	return segs, nil
}

// isGenericFenceOpen reports whether a trimmed line opens a triple-backtick
// fenced code block (```lang or bare ```). It does not match indented lines
// beyond the trim already applied by the caller.
func isGenericFenceOpen(trimmed string) bool {
	return strings.HasPrefix(trimmed, "```")
}

const echoMarker = "`rela "

// isEchoExpr reports whether an inline `rela …` span's content is meant as a
// resolver call rather than prose that merely mentions the word "rela" (e.g.
// `rela docs build`, `rela init`). An echo expression is a function call, so it
// must contain a call marker — "(" or "{". This keeps literal command mentions
// in prose from being mistaken for islands.
func isEchoExpr(expr string) bool {
	return strings.ContainsAny(expr, "({")
}

// splitEchoes scans a literal run for inline echo spans and returns the
// resulting segments (literal + echo, interleaved). An echo span is a single
// backtick-delimited code span whose content starts with "rela " — e.g.
// `rela count{type="x"}`. A code span that does not start with the marker is
// left as literal text. startLine is the manual line of the run's first line;
// echo segments carry the line on which their span begins.
func splitEchoes(run string, startLine int) []segment {
	var segs []segment
	var lit strings.Builder
	line := startLine
	litLine := startLine

	appendLiteral := func() {
		if lit.Len() > 0 {
			segs = append(segs, segment{kind: segLiteral, body: lit.String(), line: litLine})
			lit.Reset()
		}
	}

	i := 0
	for i < len(run) {
		c := run[i]
		if c == '\n' {
			lit.WriteByte(c)
			line++
			i++
			continue
		}
		// Potential echo span: a backtick followed by "rela ".
		if c == '`' && strings.HasPrefix(run[i:], "`"+echoMarker[1:]) {
			// Find the closing backtick (a code span cannot contain a newline).
			end := strings.IndexByte(run[i+1:], '`')
			if end >= 0 {
				spanContent := run[i+1 : i+1+end] // between the backticks
				expr := strings.TrimSpace(strings.TrimPrefix(spanContent, "rela"))
				if !strings.ContainsRune(spanContent, '\n') && isEchoExpr(expr) {
					appendLiteral()
					segs = append(segs, segment{kind: segEcho, body: expr, line: line})
					i = i + 1 + end + 1 // past the closing backtick
					litLine = line
					continue
				}
				// Not a resolver call (e.g. prose mentioning `rela docs build`):
				// leave the whole span as literal text.
			}
		}
		if lit.Len() == 0 {
			litLine = line
		}
		lit.WriteByte(c)
		i++
	}
	appendLiteral()
	return segs
}
