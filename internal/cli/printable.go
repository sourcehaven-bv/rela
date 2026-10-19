package cli

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// printable makes text safe to write to a terminal. It replaces with
// U+FFFD the characters that could change how the line displays:
// [unicode.IsControl] (newline included), the line and paragraph
// separators, the bidi controls, which could make a line display in
// another order than it reads, and the zero-width space and BOM, which
// hide a difference between two names. Other format characters, such as
// the soft hyphen, the zero-width joiner and non-joiner and the word
// joiner, occur in real text and are kept.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || isDisplayHazard(r) {
			return unicode.ReplacementChar
		}
		return r
	}, s)
}

// isDisplayHazard reports the non-control runes [printable] replaces.
func isDisplayHazard(r rune) bool {
	switch {
	case r == '\u2028', r == '\u2029': // line and paragraph separator
		return true
	case r == '\u061c', r == '\u200e', r == '\u200f': // bidi marks
		return true
	case r >= '\u202a' && r <= '\u202e', r >= '\u2066' && r <= '\u2069': // bidi embeddings, overrides, isolates
		return true
	case r == '\u200b', r == '\ufeff': // zero-width space, BOM
		return true
	}
	return false
}

// terminalSafeJSON escapes as \uXXXX every control, format, line or
// paragraph separator rune in encoded JSON. encoding/json escapes only the
// C0 controls and U+2028/9, so a C1 control such as U+009B (CSI) or a bidi
// override from another author's message would otherwise reach a terminal
// running `-o json` raw. The result decodes to the same values: outside a
// string, valid JSON has only ASCII, which is left alone.
func terminalSafeJSON(b []byte) []byte {
	var buf bytes.Buffer
	buf.Grow(len(b))
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r >= utf8.RuneSelf && (unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp)) {
			writeJSONEscape(&buf, r)
		} else {
			buf.Write(b[:size])
		}
		b = b[size:]
	}
	return buf.Bytes()
}

// writeJSONEscape writes r as a JSON \uXXXX escape, as a surrogate pair
// outside the Basic Multilingual Plane.
func writeJSONEscape(buf *bytes.Buffer, r rune) {
	if hi, lo := utf16.EncodeRune(r); hi != unicode.ReplacementChar {
		_, _ = fmt.Fprintf(buf, `\u%04x\u%04x`, hi, lo)
		return
	}
	_, _ = fmt.Fprintf(buf, `\u%04x`, r)
}
