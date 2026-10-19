package cli

import (
	"strings"
	"unicode"
)

// zeroWidthJoiner joins emoji sequences; it is a format character that
// cannot reorder or hide text, so [printable] keeps it.
const zeroWidthJoiner = '‍'

// printable makes text safe to write to a terminal. It replaces control
// characters (newline included) and format characters with U+FFFD. Format
// characters cover the bidi overrides, which could make a line display in
// another order than it reads. The zero-width joiner is kept.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || (unicode.Is(unicode.Cf, r) && r != zeroWidthJoiner) {
			return unicode.ReplacementChar
		}
		return r
	}, s)
}
