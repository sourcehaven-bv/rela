package search

import (
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// ScoreText returns a relevance score for how well text matches the given query terms.
//
// Scoring rules:
//   - Each word that substring-matches: +1.0
//   - Each word that only fuzzy-matches: +fractional score (0.0–1.0)
//   - Each phrase that substring-matches: +2.0
//   - If any phrase does NOT match: score is forced to 0 (phrases are explicit user intent)
//
// Returns 0 if nothing matches or a required phrase is missing.
func ScoreText(text string, words, phrases []string) float64 {
	textLower := strings.ToLower(text)

	// Phrases are required (AND logic) — if any phrase is missing, no match.
	for _, phrase := range phrases {
		if !strings.Contains(textLower, strings.ToLower(phrase)) {
			return 0
		}
	}

	score := float64(len(phrases)) * 2.0

	// Words use OR logic — each matching word adds to the score.
	for _, word := range words {
		wordLower := strings.ToLower(word)
		if strings.Contains(textLower, wordLower) {
			score += 1.0
		} else if fuzzyScore := fuzzyContains(textLower, wordLower, DefaultFuzzyThreshold); fuzzyScore > 0 {
			score += fuzzyScore
		}
	}

	return score
}

// ScoreEntity builds searchable text from an entity and scores it against the query terms.
func ScoreEntity(e *model.Entity, words, phrases []string) float64 {
	parts := []string{e.ID, e.Title(), e.Description(), e.Content}
	for _, v := range e.Properties {
		// Handle list values (multi-select) by adding each element separately
		switch val := v.(type) {
		case []string:
			parts = append(parts, val...)
		case []interface{}:
			for _, item := range val {
				if s, ok := item.(string); ok {
					parts = append(parts, s)
				}
			}
		default:
			parts = append(parts, fmt.Sprintf("%v", v))
		}
	}
	text := strings.Join(parts, " ")
	return ScoreText(text, words, phrases)
}
