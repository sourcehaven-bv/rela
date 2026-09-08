package docs

import (
	"strings"
	"testing"
)

// TestRegionAnchors pins the region table's CONTRACT rather than its contents:
// every entry resolves, names itself, and — the load-bearing part — any entry
// that falls back to a class name explains why.
//
// The class fallbacks are the debt in this table. An ARIA anchor breaks loudly
// when the accessibility contract breaks, which is a bug either way; a class
// anchor can be renamed by someone with no idea a manual depends on it, and the
// symptom is a region that matches nothing. Requiring a stated reason at each
// one keeps the count visible and the justification reviewable.
func TestRegionAnchors(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range regions {
		t.Run(r.Name, func(t *testing.T) {
			if r.Name == "" {
				t.Fatal("region has no name")
			}
			if seen[r.Name] {
				t.Fatalf("duplicate region name %q — the later entry would shadow the earlier", r.Name)
			}
			seen[r.Name] = true

			if r.Selector == "" {
				t.Fatalf("region %q has no selector", r.Name)
			}
			if r.Why == "" {
				t.Fatalf("region %q has no Why — every anchor states why it is the right one", r.Name)
			}

			// A class-based selector must say so. Detected structurally rather
			// than by a hand-maintained list, so a NEW class fallback added
			// later cannot slip in undocumented.
			if strings.Contains(r.Selector, ".") && !strings.Contains(r.Why, "CLASS FALLBACK") {
				t.Errorf("region %q anchors on a class (%q) but its Why does not say "+
					"CLASS FALLBACK. A class anchor is debt: state why no honest role exists",
					r.Name, r.Selector)
			}

			if _, err := lookupRegion(r.Name); err != nil {
				t.Errorf("declared region %q does not resolve: %v", r.Name, err)
			}
		})
	}
}

// TestRegionVocabularyIsClosed is the rule that makes the whole design work: a
// manual names a region, never a selector. An unknown name must be refused with
// the valid set, because a region that silently resolved to nothing would make
// every absent= claim pass for the wrong reason.
func TestRegionVocabularyIsClosed(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"typo", "kanban-cards"},
		{"a CSS selector", ".kanban-card"},
		{"an id selector", "#main-sidebar"},
		{"empty", ""},
		{"an XPath", "//div[@class='x']"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := lookupRegion(tc.input)
			if err == nil {
				t.Fatalf("lookupRegion(%q) succeeded; a non-vocabulary name must be refused", tc.input)
			}
			// The message must be actionable: it names the valid set.
			for _, want := range []string{"kanban-card", "menu", "badge"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error does not list region %q: %s", want, err)
				}
			}
			if !strings.Contains(err.Error(), "never a CSS selector") {
				t.Errorf("error should say a region is not a selector: %s", err)
			}
		})
	}
}

// TestRegionsCoverVocabulary pins the names the design doc committed to, so
// removing one is a deliberate act rather than a rename nobody noticed.
func TestRegionsCoverVocabulary(t *testing.T) {
	want := []string{
		"menu", "main", "list", "table-row", "kanban", "kanban-column",
		"kanban-card", "detail", "detail-section", "search-results",
		"analyze", "dashboard", "banner", "badge",
	}
	for _, name := range want {
		if _, err := lookupRegion(name); err != nil {
			t.Errorf("vocabulary region %q is missing: %v", name, err)
		}
	}
}
