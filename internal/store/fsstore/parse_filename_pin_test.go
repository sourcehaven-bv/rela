package fsstore

import "testing"

// TestParseRelationFilename_PinnedForAnalysisCopy pins the exact splitting
// behavior of parseRelationFilename, because analysis.splitRelationFilename
// is a deliberate behavioral COPY of it (TKT-2P9S72).
//
// The copy exists because this function is unexported and arch-lint forbids
// analysis -> store/fsstore — the right boundary, but it means a change here
// silently desynchronises a checker one package away whose entire claim is
// "I split names the way the indexer does". A divergence makes that checker
// report a false mismatch on a good file.
//
// If you change this function, change internal/analysis/relation_filename.go
// to match, and update both tables. The cases below are the ones where the
// two could plausibly drift: they all turn on WHICH "--" is chosen.
func TestParseRelationFilename_PinnedForAnalysisCopy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		in                string
		from, relType, to string
	}{
		{"plain", "A--rel--B", "A", "rel", "B"},
		// First "--" ends FROM, last "--" starts TO, so a relation type
		// containing the separator round-trips. This is the case that makes
		// a naive Split("--") wrong.
		{"separator inside the type", "A--we--ird--B", "A", "we--ird", "B"},
		{"type with several separators", "A--r--e--l--B", "A", "r--e--l", "B"},
		{"id containing a single dash", "A-1--rel--B-2", "A-1", "rel", "B-2"},
		// Rejections: every one returns all-empty, which callers treat as
		// "not a relation filename".
		{"no separator", "notatriple", "", "", ""},
		{"one separator", "A--B", "", "", ""},
		{"empty from", "--rel--B", "", "", ""},
		{"empty to", "A--rel--", "", "", ""},
		{"adjacent separators", "A----B", "", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			from, relType, to := parseRelationFilename(tc.in)
			if from != tc.from || relType != tc.relType || to != tc.to {
				t.Errorf("parseRelationFilename(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tc.in, from, relType, to, tc.from, tc.relType, tc.to)
			}
		})
	}
}
