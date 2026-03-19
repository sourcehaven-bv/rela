package filter

import "strings"

// DefaultFuzzyThreshold is the minimum trigram similarity for a fuzzy match.
const DefaultFuzzyThreshold = 0.4

// trigrams returns the set of 3-character subsequences of s (lowercased).
// For strings shorter than 3 chars, the string itself is used as the sole trigram.
func trigrams(s string) map[string]struct{} {
	s = strings.ToLower(s)
	set := make(map[string]struct{})
	runes := []rune(s)
	if len(runes) < 3 {
		if len(runes) > 0 {
			set[string(runes)] = struct{}{}
		}
		return set
	}
	for i := 0; i <= len(runes)-3; i++ {
		set[string(runes[i:i+3])] = struct{}{}
	}
	return set
}

// TrigramSimilarity returns the Jaccard similarity between the trigram sets of a and b.
// Returns a value between 0.0 (no overlap) and 1.0 (identical trigram sets).
func TrigramSimilarity(a, b string) float64 {
	setA := trigrams(a)
	setB := trigrams(b)
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}

	intersection := 0
	for t := range setA {
		if _, ok := setB[t]; ok {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
