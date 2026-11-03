package texthazard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

func TestIsDisplayHazard(t *testing.T) {
	tests := []struct {
		name string
		r    rune
		want bool
	}{
		{"letter", 'a', false},
		{"space", ' ', false},
		{"accented letter", 'é', false},
		{"combining acute", '\u0301', false},
		{"CJK", '中', false},
		{"emoji", '\U0001F600', false},
		{"newline", '\n', true},
		{"tab", '\t', true},
		{"delete", '\u007F', true},
		{"C1 control", '\u0085', true},
		{"zero-width space", '\u200B', true},
		{"zero-width joiner on its own", '\u200D', true},
		{"right-to-left override", '\u202E', true},
		{"Arabic letter mark", '\u061C', true},
		{"soft hyphen", '\u00AD', true},
		{"tag letter A", '\U000E0041', true},
		{"line separator", '\u2028', true},
		{"paragraph separator", '\u2029', true},
		{"combining grapheme joiner", '\u034F', true},
		{"variation selector 16", '\uFE0F', true},
		{"variation selector supplement", '\U000E0100', true},
		{"Hangul choseong filler", '\u115F', true},
		{"Hangul jungseong filler", '\u1160', true},
		{"Hangul filler", '\u3164', true},
		{"halfwidth Hangul filler", '\uFFA0', true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsDisplayHazard(tc.r); got != tc.want {
				t.Errorf("IsDisplayHazard(%U) = %v, want %v", tc.r, got, tc.want)
			}
		})
	}
}

func TestFind(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		want  rune
		found bool
	}{
		{"plain text", "plain text", 0, false},
		{"family emoji", "a \U0001F468\u200D\U0001F469\u200D\U0001F467", 0, false},
		{"skin tone before the joiner", "\U0001F468\U0001F3FD\u200D\U0001F4BB", 0, false},
		{"zero-width space", "ab\u200Bc", '\u200B', true},
		{"joiner inside a word", "Ye\u200Ds", '\u200D', true},
		{"joiner at the start", "\u200DYes", '\u200D', true},
		{"joiner at the end", "\U0001F44D\u200D", '\u200D', true},
		{"joiner between emoji and letter", "\U0001F44D\u200Da", '\u200D', true},
		{"emoji style selector", "✔\uFE0F", '\uFE0F', true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, found := Find(tc.in)
			if r != tc.want || found != tc.found {
				t.Errorf("Find(%+q) = %U, %v; want %U, %v", tc.in, r, found, tc.want, tc.found)
			}
		})
	}
}

func TestDropStyleSelectors(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain", "plain"},
		{"✔\uFE0F Done", "✔ Done"},
		{"❤\uFE0E", "❤"},
		{"a\uFE00", "a\uFE00"},
	}
	for _, tc := range tests {
		if got := DropStyleSelectors(tc.in); got != tc.want {
			t.Errorf("DropStyleSelectors(%+q) = %+q, want %+q", tc.in, got, tc.want)
		}
	}
}

// TestSourceHasNoRawHazards keeps the Go source of the packages that test
// for display hazards free of raw hazards and combining marks. A reviewer
// cannot see such a rune, and an editor that normalizes to NFC would
// silently change a test that compares a decomposed string. Write them as
// \u escapes.
func TestSourceHasNoRawHazards(t *testing.T) {
	dirs := []string{".", "../realtime", "../realtimecfg"}
	for _, dir := range dirs {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for i, line := range strings.Split(string(b), "\n") {
				for _, r := range line {
					if r != '\t' && r != '\r' && (IsDisplayHazard(r) || unicode.Is(unicode.Mn, r)) {
						t.Errorf("%s:%d holds a raw %U; write it as an escape", f, i+1, r)
					}
				}
			}
		}
	}
}
