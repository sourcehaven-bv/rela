package search

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestScoreText_ExactSubstring(t *testing.T) {
	text := "the quick brown fox jumps over the lazy dog"
	score := ScoreText(text, []string{"quick"}, nil)
	if score != 1.0 {
		t.Errorf("exact substring: got %f, want 1.0", score)
	}
}

func TestScoreText_MultipleWordsOR(t *testing.T) {
	text := "the quick brown fox"

	// Both words match
	score := ScoreText(text, []string{"quick", "fox"}, nil)
	if score != 2.0 {
		t.Errorf("two matching words: got %f, want 2.0", score)
	}

	// Only one word matches
	score = ScoreText(text, []string{"quick", "elephant"}, nil)
	if score != 1.0 {
		t.Errorf("one matching word: got %f, want 1.0", score)
	}

	// No words match
	score = ScoreText(text, []string{"elephant", "giraffe"}, nil)
	if score != 0.0 {
		t.Errorf("no matching words: got %f, want 0.0", score)
	}
}

func TestScoreText_FuzzyWord(t *testing.T) {
	text := "user authentication system"

	// Exact match
	exact := ScoreText(text, []string{"authentication"}, nil)
	// Fuzzy match (typo)
	fuzzy := ScoreText(text, []string{"autentication"}, nil)
	// No match
	none := ScoreText(text, []string{"elephant"}, nil)

	if exact <= 0 {
		t.Errorf("exact match should score > 0, got %f", exact)
	}
	if fuzzy <= 0 {
		t.Errorf("fuzzy match should score > 0, got %f", fuzzy)
	}
	if exact <= fuzzy {
		t.Errorf("exact (%f) should score higher than fuzzy (%f)", exact, fuzzy)
	}
	if none != 0 {
		t.Errorf("no match should score 0, got %f", none)
	}
}

func TestScoreText_Phrases(t *testing.T) {
	text := "the OAuth 2.0 authentication protocol"

	// Phrase matches
	score := ScoreText(text, nil, []string{"OAuth 2.0"})
	if score != 2.0 {
		t.Errorf("matching phrase: got %f, want 2.0", score)
	}

	// Phrase doesn't match — score must be 0
	score = ScoreText(text, []string{"authentication"}, []string{"does not exist"})
	if score != 0.0 {
		t.Errorf("missing phrase should force score to 0, got %f", score)
	}
}

func TestScoreText_WordsAndPhrases(t *testing.T) {
	text := "the OAuth 2.0 authentication protocol for API access"

	// Phrase matches + words match
	score := ScoreText(text, []string{"API", "authentication"}, []string{"OAuth 2.0"})
	if score != 4.0 { // phrase=2.0 + API=1.0 + authentication=1.0
		t.Errorf("words + phrase: got %f, want 4.0", score)
	}
}

func TestScoreText_CaseInsensitive(t *testing.T) {
	text := "User Authentication System"
	score := ScoreText(text, []string{"authentication"}, nil)
	if score <= 0 {
		t.Errorf("case insensitive match should score > 0, got %f", score)
	}
}

func TestScoreText_EmptyInputs(t *testing.T) {
	// No words or phrases
	score := ScoreText("some text", nil, nil)
	if score != 0.0 {
		t.Errorf("no query terms: got %f, want 0.0", score)
	}

	// Empty text
	score = ScoreText("", []string{"hello"}, nil)
	if score != 0.0 {
		t.Errorf("empty text: got %f, want 0.0", score)
	}
}

func TestScoreEntity(t *testing.T) {
	e := model.NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "User Authentication"
	e.Properties["description"] = "System must support OAuth"
	e.Content = "Detailed requirements for authentication flow"

	// Should match on title
	score := ScoreEntity(e, []string{"authentication"}, nil)
	if score <= 0 {
		t.Errorf("should match entity title/content, got %f", score)
	}

	// Should match on ID
	score = ScoreEntity(e, []string{"REQ-001"}, nil)
	if score <= 0 {
		t.Errorf("should match entity ID, got %f", score)
	}

	// Should match on property value
	score = ScoreEntity(e, []string{"OAuth"}, nil)
	if score <= 0 {
		t.Errorf("should match property value, got %f", score)
	}
}

func TestScoreText_RelevanceRanking(t *testing.T) {
	// Entity that matches both words should score higher than one matching only one
	textBoth := "API authentication service"
	textOne := "API gateway service"

	scoreBoth := ScoreText(textBoth, []string{"API", "authentication"}, nil)
	scoreOne := ScoreText(textOne, []string{"API", "authentication"}, nil)

	if scoreBoth <= scoreOne {
		t.Errorf("both-match (%f) should score higher than one-match (%f)", scoreBoth, scoreOne)
	}
}
