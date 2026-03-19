package filter

import (
	"testing"
)

func TestTrigrams(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"a", 1},    // short strings use the string as sole trigram
		{"ab", 1},   // short strings use the string as sole trigram
		{"abc", 1},  // exactly 3 chars = 1 trigram
		{"abcd", 2}, // 4 chars = 2 trigrams: abc, bcd
		{"hello", 3},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := trigrams(tt.input)
			if len(result) != tt.expected {
				t.Errorf("trigrams(%q) = %d trigrams, want %d", tt.input, len(result), tt.expected)
			}
		})
	}
}

func TestTrigrams_CaseInsensitive(t *testing.T) {
	lower := trigrams("hello")
	upper := trigrams("HELLO")
	mixed := trigrams("HeLLo")

	// All should produce the same trigrams
	if len(lower) != len(upper) || len(lower) != len(mixed) {
		t.Error("trigrams should be case-insensitive")
	}

	for tri := range lower {
		if _, ok := upper[tri]; !ok {
			t.Errorf("trigram %q missing from uppercase version", tri)
		}
		if _, ok := mixed[tri]; !ok {
			t.Errorf("trigram %q missing from mixed case version", tri)
		}
	}
}

func TestTrigramSimilarity(t *testing.T) {
	tests := []struct {
		a, b     string
		minScore float64
		maxScore float64
	}{
		// Identical strings
		{"hello", "hello", 1.0, 1.0},
		{"Authentication", "Authentication", 1.0, 1.0},

		// Case insensitive
		{"hello", "HELLO", 1.0, 1.0},

		// Very similar strings (single char difference)
		{"authentication", "autentication", 0.5, 1.0}, // missing 'h'
		{"requirement", "requirment", 0.5, 1.0},       //nolint:misspell // intentional typo for fuzzy test

		// Somewhat similar
		{"hello", "hallo", 0.1, 0.8},
		{"test", "text", 0.0, 0.5}, // actually quite different in trigrams

		// Different strings
		{"hello", "world", 0.0, 0.3},
		{"abc", "xyz", 0.0, 0.1},

		// Empty strings
		{"", "", 0.0, 0.0},
		{"hello", "", 0.0, 0.0},
		{"", "hello", 0.0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			score := TrigramSimilarity(tt.a, tt.b)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("TrigramSimilarity(%q, %q) = %.2f, want %.2f-%.2f",
					tt.a, tt.b, score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestTrigramSimilarity_Symmetric(t *testing.T) {
	pairs := [][2]string{
		{"hello", "world"},
		{"authentication", "authorization"},
		{"test", "testing"},
	}

	for _, pair := range pairs {
		ab := TrigramSimilarity(pair[0], pair[1])
		ba := TrigramSimilarity(pair[1], pair[0])
		if ab != ba {
			t.Errorf("TrigramSimilarity(%q, %q)=%.2f != TrigramSimilarity(%q, %q)=%.2f",
				pair[0], pair[1], ab, pair[1], pair[0], ba)
		}
	}
}

func TestTrigramSimilarity_Threshold(t *testing.T) {
	// Test that similar strings exceed the default threshold
	similar := []struct {
		a, b string
	}{
		{"authentication", "autentication"}, // 0.64
		{"requirement", "requirment"},       //nolint:misspell // intentional typo
		{"important", "importent"},          // intentional typo
	}

	for _, tt := range similar {
		score := TrigramSimilarity(tt.a, tt.b)
		if score < DefaultFuzzyThreshold {
			t.Errorf("TrigramSimilarity(%q, %q) = %.2f, expected >= %.2f (threshold)",
				tt.a, tt.b, score, DefaultFuzzyThreshold)
		}
	}
}
