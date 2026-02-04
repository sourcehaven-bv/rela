package search

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

// trigramSimilarity returns the Jaccard similarity between the trigram sets of a and b.
// Returns a value between 0.0 (no overlap) and 1.0 (identical trigram sets).
func trigramSimilarity(a, b string) float64 {
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

// fuzzyContains checks whether any substring of text fuzzy-matches word.
// It uses a sliding window approach: for each position in text, it extracts
// windows of length len(word)-2 to len(word)+2 and compares trigram similarity.
// Returns the best similarity score above threshold, or 0 if none.
func fuzzyContains(text, word string, threshold float64) float64 {
	textLower := strings.ToLower(text)
	wordLower := strings.ToLower(word)
	textRunes := []rune(textLower)
	wordLen := len([]rune(wordLower))

	if wordLen == 0 {
		return 0
	}

	// For very short words (1-2 chars), fuzzy matching is not meaningful.
	if wordLen <= 2 {
		if strings.Contains(textLower, wordLower) {
			return 1.0
		}
		return 0
	}

	best := 0.0
	minWin := wordLen - 2
	if minWin < 3 {
		minWin = 3
	}
	maxWin := wordLen + 2

	for winSize := minWin; winSize <= maxWin; winSize++ {
		if winSize > len(textRunes) {
			break
		}
		for i := 0; i <= len(textRunes)-winSize; i++ {
			window := string(textRunes[i : i+winSize])
			sim := trigramSimilarity(window, wordLower)
			if sim > best {
				best = sim
			}
		}
	}

	if best >= threshold {
		return best
	}
	return 0
}
