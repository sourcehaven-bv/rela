package migration

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// labelDerivationPatterns match the *shape* of an identifier→prose transform,
// not the name of a helper that performs one.
//
// Matching behavior rather than names is deliberate. The first version of this
// guard banned four exact declarations (`func titleCase(`, `function
// formatLabel(`, …) and was trivially evadable: an arrow function
// (`const titleCase = (s) => …`) — the dominant idiom in this frontend — passed
// straight through, and so did the two `propertyLabel` helpers in
// HistoryView.vue / RelationHistoryView.vue that this very change deleted. A
// guard that misses the code it was written for is worse than no guard, because
// it reads like coverage.
var labelDerivationPatterns = []struct {
	re   *regexp.Regexp
	what string
}{
	// JS/TS: .replace(/\b\w/g, ... toUpperCase()) — capitalize each word.
	{regexp.MustCompile(`replace\(\s*/\\b\\w/`), `JS per-word capitalization (replace(/\b\w/g, …))`},
	// Go: strings.ToUpper(x[:1]) + x[1:] — capitalize first byte.
	{regexp.MustCompile(`ToUpper\([A-Za-z_][A-Za-z0-9_]*\[:1\]\)`), "Go first-byte capitalization (ToUpper(x[:1]))"},
	// Go: unicode.ToUpper on the first rune of a word slice.
	{regexp.MustCompile(`runes\[0\]\s*=\s*unicode\.ToUpper`), "Go first-rune capitalization (runes[0] = unicode.ToUpper)"},
	// JS/TS: charAt(0).toUpperCase() + slice(1) — capitalize first char.
	{regexp.MustCompile(`charAt\(0\)\.toUpperCase\(\)`), "JS first-char capitalization (charAt(0).toUpperCase())"},
}

// TestNoLabelDerivation is the structural guard for DEC-6C1NAA: a label is
// authored, never derived.
//
// The behavioral tests elsewhere prove the derivation is gone today. This one
// stops it coming back. That matters because re-adding it always looks like a
// small, helpful improvement in isolation — "just title-case the property name
// so the form looks nicer" — and it was previously reintroduced independently
// in eleven places (four Go copies, seven frontend copies) which had already
// drifted apart: the Go copies replaced "-" and the JS copies did not, so
// `kebab-name` became "Kebab Name" on one side and "Kebab-Name" on the other.
//
// Why the rule exists: title-casing an identifier encodes an English
// orthographic convention (split on spaces, upper-case each word's first rune)
// into a platform whose metamodel is explicitly language-neutral. It is correct
// by coincidence for a subset of Latin-script languages and silently wrong
// elsewhere. BUG-8N2WT2 is what that cost in practice — the cleanup migration
// deleted labels assuming the SPA would re-derive them, the SPA rendered raw
// snake_case identifiers instead, and because the server refuses to start on
// unmigrated config the user could not decline the downgrade.
//
// If you are here because this test failed: do not add the derivation back.
// Give the field an explicit `label:` instead, in whatever language the project
// is written in. If you genuinely need to capitalize something that is NOT a
// user-facing label (a schema name, a DOT identifier), add it to allowedFiles
// below with a comment saying why it is not a label.
func TestNoLabelDerivation(t *testing.T) {
	// Paths (repo-relative) permitted to capitalize an identifier because what
	// they produce is not a user-facing label.
	allowedFiles := map[string]string{
		// OpenAPI *schema type names* (e.g. "ticket" -> "Ticket" in $ref), a
		// wire-format identifier, never shown to a user as a label.
		"internal/openapi/paths.go": "OpenAPI schema names, not labels",

		// getTemplateLabel capitalizes an entity-TEMPLATE FILE NAME for the
		// template picker. It is a label derived from an identifier and so is
		// in scope for DEC-6C1NAA on principle, but templates are named by the
		// same author who would write the label, there is no config key to
		// author instead, and removing it is a UX change beyond this bug's
		// scope. Tracked as follow-up; allowlisted rather than silently
		// matched so the exception is visible.
		"frontend/src/components/forms/DynamicForm.vue": "template file name, no authorable label key (follow-up)",
	}

	// Reuses findRepoRoot from detail_view_to_entity_views_test.go — this file
	// is in `package migration` (not `_test`) precisely so it need not carry a
	// second copy of that walk-up helper.
	root := findRepoRoot(t)

	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "dist": true, "testdata": true,
		// Nested agent worktrees hold OTHER branches' copies of the tree.
		".claude": true,
		"static":  true, "app_editor_dist": true, ".ignored": true,
		"coverage": true, "e2e-results": true,
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		switch filepath.Ext(path) {
		case ".go", ".ts", ".vue":
		default:
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)

		// This file necessarily contains the patterns it bans.
		if strings.HasSuffix(rel, "label_derivation_lint_test.go") {
			return nil
		}
		if why, ok := allowedFiles[rel]; ok {
			_ = why
			return nil
		}

		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}

		for i, line := range strings.Split(string(body), "\n") {
			for _, p := range labelDerivationPatterns {
				if p.re.MatchString(line) {
					t.Errorf("%s:%d: %s — labels are authored, never derived (DEC-6C1NAA).\n"+
						"    %s\n"+
						"    Write an explicit label: instead of deriving one from an identifier.",
						rel, i+1, p.what, strings.TrimSpace(line))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}
