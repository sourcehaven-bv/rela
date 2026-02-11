package search

import (
	"testing"
)

func TestTrigrams(t *testing.T) {
	tests := []struct {
		input string
		want  int // expected number of unique trigrams
	}{
		{"", 0},
		{"ab", 1},   // "ab" is the sole entry
		{"abc", 1},  // "abc"
		{"abcd", 2}, // "abc", "bcd"
		{"hello", 3},
	}
	for _, tt := range tests {
		got := trigrams(tt.input)
		if len(got) != tt.want {
			t.Errorf("trigrams(%q): got %d trigrams, want %d", tt.input, len(got), tt.want)
		}
	}
}

func TestTrigramsCaseInsensitive(t *testing.T) {
	upper := trigrams("ABC")
	lower := trigrams("abc")
	if len(upper) != len(lower) {
		t.Fatalf("case mismatch: upper=%d lower=%d", len(upper), len(lower))
	}
	for k := range upper {
		if _, ok := lower[k]; !ok {
			t.Errorf("trigram %q from upper not in lower set", k)
		}
	}
}

func TestTrigramSimilarity(t *testing.T) {
	tests := []struct {
		a, b    string
		minSim  float64
		maxSim  float64
		comment string
	}{
		{"hello", "hello", 1.0, 1.0, "identical"},
		{"hello", "hallo", 0.2, 0.6, "one char diff"},
		{"hello", "world", 0.0, 0.1, "completely different"},
		{"authentication", "autentication", 0.5, 1.0, "missing char"},
		{"", "hello", 0.0, 0.0, "empty a"},
		{"hello", "", 0.0, 0.0, "empty b"},
	}
	for _, tt := range tests {
		sim := trigramSimilarity(tt.a, tt.b)
		if sim < tt.minSim || sim > tt.maxSim {
			t.Errorf("trigramSimilarity(%q, %q) = %f, want [%f, %f] (%s)",
				tt.a, tt.b, sim, tt.minSim, tt.maxSim, tt.comment)
		}
	}
}

func TestFuzzyContains(t *testing.T) {
	tests := []struct {
		text      string
		word      string
		threshold float64
		wantMatch bool
		comment   string
	}{
		// Exact substring
		{"the quick brown fox", "quick", 0.4, true, "exact substring"},
		// Typo: missing letter
		{"user authentication system", "autentication", 0.4, true, "missing h"},
		// Typo: swapped letters
		{"requirement specification", "requirment", 0.4, true, "missing e"}, //nolint:misspell // intentional typo for fuzzy test
		// Completely unrelated
		{"the quick brown fox", "elephant", 0.4, false, "no match"},
		// Short words — fall back to exact
		{"hello world", "he", 0.4, true, "short exact match"},
		{"hello world", "zz", 0.4, false, "short no match"},
		// Empty
		{"hello", "", 0.4, false, "empty word"},
		{"", "hello", 0.4, false, "empty text"},
		// Case insensitive
		{"Authentication Required", "authentication", 0.4, true, "case insensitive"},
	}
	for _, tt := range tests {
		score := fuzzyContains(tt.text, tt.word, tt.threshold)
		gotMatch := score > 0
		if gotMatch != tt.wantMatch {
			t.Errorf("fuzzyContains(%q, %q, %f) = %f (match=%v), want match=%v (%s)",
				tt.text, tt.word, tt.threshold, score, gotMatch, tt.wantMatch, tt.comment)
		}
	}
}

func TestFuzzyContainsScoreOrdering(t *testing.T) {
	text := "the authentication module handles user requests"

	exact := fuzzyContains(text, "authentication", 0.4)
	typo := fuzzyContains(text, "autentication", 0.4) // missing h
	noMatch := fuzzyContains(text, "elephant", 0.4)

	if exact <= typo {
		t.Errorf("exact (%f) should score higher than typo (%f)", exact, typo)
	}
	if typo <= noMatch {
		t.Errorf("typo (%f) should score higher than noMatch (%f)", typo, noMatch)
	}
}
