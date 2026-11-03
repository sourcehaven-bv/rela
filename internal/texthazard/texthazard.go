// Package texthazard names the runes that make text show something other
// than what it holds. Such a rune renders as nothing, reorders the text
// around it, or makes two different strings look the same.
//
// The source writes every such rune as a \u escape, so a reader can see
// it. TestSourceHasNoRawHazards keeps it that way.
//
// It is a leaf: it imports nothing from rela.
package texthazard

import (
	"strings"
	"unicode"
)

const (
	zeroWidthJoiner = '\u200D'
	textStyle       = '\uFE0E' // variation selector 15
	emojiStyle      = '\uFE0F' // variation selector 16
)

// IsDisplayHazard reports whether r, on its own, is a display hazard:
//
//   - a control character (Cc), newline and tab included;
//   - a format character (Cf), which covers zero-width characters, every
//     bidi control, the zero-width joiner U+200D and the tag block that
//     can spell hidden ASCII;
//   - the line and paragraph separators U+2028 and U+2029;
//   - the combining grapheme joiner U+034F;
//   - the variation selectors, so two strings cannot differ by one alone;
//   - the Hangul fillers U+115F, U+1160, U+3164 and U+FFA0, which render
//     blank.
//
// U+061C (Arabic letter mark) is a format character, so the Cf rule covers
// it. [Find] allows a zero-width joiner inside an emoji sequence.
func IsDisplayHazard(r rune) bool {
	switch {
	case unicode.In(r, unicode.Cc, unicode.Cf, unicode.Variation_Selector):
		return true
	case r == '\u2028', r == '\u2029', r == '\u034F',
		r == '\u115F', r == '\u1160', r == '\u3164', r == '\uFFA0':
		return true
	}
	return false
}

// Find returns the first display hazard in s and true, or false when s has
// none. A zero-width joiner is no hazard when it joins two emoji, such as
// in a family emoji, because there it changes how the emoji look, not
// what the text says. Anywhere else it is invisible, so "Yes" with a
// joiner after "Ye" would look like "Yes".
func Find(s string) (rune, bool) {
	runes := []rune(s)
	for i, r := range runes {
		if r == zeroWidthJoiner && joinsEmoji(runes, i) {
			continue
		}
		if IsDisplayHazard(r) {
			return r, true
		}
	}
	return 0, false
}

// joinsEmoji reports whether the zero-width joiner at runes[i] stands
// between two emoji. An emoji is a symbol (So) here; the rune before may
// also be a skin tone modifier. Go has no Extended_Pictographic table, and
// So covers every pictograph an emoji sequence joins.
func joinsEmoji(runes []rune, i int) bool {
	if i == 0 || i == len(runes)-1 {
		return false
	}
	before, after := runes[i-1], runes[i+1]
	isSkinTone := before >= '\U0001F3FB' && before <= '\U0001F3FF'
	return (unicode.Is(unicode.So, before) || isSkinTone) && unicode.Is(unicode.So, after)
}

// DropStyleSelectors removes the variation selectors U+FE0E and U+FE0F
// from s. They only choose between the text and the emoji style of the
// symbol before them, and models emit U+FE0F after many symbols, such as
// in a check mark emoji. Without them, two strings that differ by a style
// selector alone compare equal, so they cannot make two different items
// look the same. The other variation selectors stay hazards.
func DropStyleSelectors(s string) string {
	if !strings.ContainsAny(s, string([]rune{textStyle, emojiStyle})) {
		return s
	}
	return strings.Map(func(r rune) rune {
		if r == textStyle || r == emojiStyle {
			return -1
		}
		return r
	}, s)
}
