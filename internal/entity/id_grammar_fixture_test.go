package entity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The entity-ID grammar is enforced in two languages: [ValidateID] here, and
// isValidEntityRefId in the markdown editor, which decides whether a code span
// renders as an entity reference.
//
// They must agree. A looser editor renders links to entities the store would
// refuse to create; a stricter one leaves real references as plain code spans.
// The drift is invisible in either codebase alone, which is why both sides read
// the same fixture — see frontend/src/components/forms/milkdown/
// entityRefIdGrammar.test.ts for the other half.
func TestIDGrammarFixtureMatchesFrontend(t *testing.T) {
	// internal/entity -> repo root.
	path := filepath.Join("..", "..", "frontend", "src", "components", "forms",
		"milkdown", "testdata", "entity-id-grammar.json")

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shared id-grammar fixture: %v", err)
	}

	var fixture struct {
		Valid   []string `json:"valid"`
		Invalid []string `json:"invalid"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse shared id-grammar fixture: %v", err)
	}

	// Guards a fixture that silently emptied: a passing test over zero cases
	// would prove nothing.
	if len(fixture.Valid) < 5 || len(fixture.Invalid) < 10 {
		t.Fatalf("fixture too small to be meaningful: %d valid, %d invalid",
			len(fixture.Valid), len(fixture.Invalid))
	}

	for _, id := range fixture.Valid {
		t.Run("valid/"+id, func(t *testing.T) {
			if err := ValidateID(id); err != nil {
				t.Errorf("ValidateID(%q) = %v, want nil "+
					"(the editor renders this id as a reference)", id, err)
			}
		})
	}

	for _, id := range fixture.Invalid {
		t.Run("invalid/"+id, func(t *testing.T) {
			if err := ValidateID(id); err == nil {
				t.Errorf("ValidateID(%q) = nil, want an error "+
					"(the editor refuses to render this id)", id)
			}
		})
	}
}
