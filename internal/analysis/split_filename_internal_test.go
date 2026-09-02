package analysis

import "testing"

// TestSplitRelationFilename_MatchesIndexer pins this function against the SAME
// table as fsstore's TestParseRelationFilename_PinnedForAnalysisCopy.
//
// splitRelationFilename is a deliberate behavioral copy of
// fsstore.parseRelationFilename — unexported there, and arch-lint forbids
// analysis -> store/fsstore. The copy is justified only while it is exact:
// this check's whole claim is that it splits names the way the indexer does,
// so a divergence reports a false mismatch on a good file whose relation type
// contains "--".
//
// Keep the two tables identical. If one changes, the other must.
func TestSplitRelationFilename_MatchesIndexer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		in                string
		from, relType, to string
	}{
		{"plain", "A--rel--B", "A", "rel", "B"},
		{"separator inside the type", "A--we--ird--B", "A", "we--ird", "B"},
		{"type with several separators", "A--r--e--l--B", "A", "r--e--l", "B"},
		{"id containing a single dash", "A-1--rel--B-2", "A-1", "rel", "B-2"},
		{"no separator", "notatriple", "", "", ""},
		{"one separator", "A--B", "", "", ""},
		{"empty from", "--rel--B", "", "", ""},
		{"empty to", "A--rel--", "", "", ""},
		{"adjacent separators", "A----B", "", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			from, relType, to := splitRelationFilename(tc.in)
			if from != tc.from || relType != tc.relType || to != tc.to {
				t.Errorf("splitRelationFilename(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tc.in, from, relType, to, tc.from, tc.relType, tc.to)
			}
		})
	}
}
