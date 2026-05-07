package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDetailViewToEntityViewsMigration_Detect(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected bool
	}{
		{
			name: "single list with detail_view",
			yaml: `
lists:
  all_ideas:
    entity_type: idea
    detail_view: idea_detail
`,
			expected: true,
		},
		{
			name: "multiple lists same type same detail_view",
			yaml: `
lists:
  all_ideas:
    entity_type: idea
    detail_view: idea_detail
  active_ideas:
    entity_type: idea
    detail_view: idea_detail
`,
			expected: true,
		},
		{
			name: "multiple lists same type one without detail_view (still migratable)",
			yaml: `
lists:
  all_ideas:
    entity_type: idea
    detail_view: idea_detail
  active_ideas:
    entity_type: idea
`,
			expected: true,
		},
		{
			name: "conflicting detail_view across lists for same type",
			yaml: `
lists:
  all_ideas:
    entity_type: idea
    detail_view: idea_detail
  alt_ideas:
    entity_type: idea
    detail_view: idea_detail_alt
`,
			expected: false,
		},
		{
			name: "no detail_view anywhere",
			yaml: `
lists:
  all_tickets:
    entity_type: ticket
`,
			expected: false,
		},
		{
			name: "already migrated (entity_views set, list-level absent)",
			yaml: `
entity_views:
  idea:
    detail_view: idea_detail
lists:
  all_ideas:
    entity_type: idea
`,
			expected: false,
		},
		{
			name: "list-level matches existing entity_views (migratable: drop list-level)",
			yaml: `
entity_views:
  idea:
    detail_view: idea_detail
lists:
  all_ideas:
    entity_type: idea
    detail_view: idea_detail
`,
			expected: true,
		},
		{
			name: "list-level conflicts with existing entity_views (skip)",
			yaml: `
entity_views:
  idea:
    detail_view: idea_detail
lists:
  all_ideas:
    entity_type: idea
    detail_view: other_detail
`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.yaml), &doc); err != nil {
				t.Fatalf("failed to parse yaml: %v", err)
			}
			migration := &DetailViewToEntityViewsMigration{}
			if got := migration.Detect(&doc); got != tt.expected {
				t.Errorf("Detect() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDetailViewToEntityViewsMigration_Apply(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "single list",
			input: `lists:
  all_ideas:
    entity_type: idea
    detail_view: idea_detail
`,
			wantContains: []string{
				"entity_views:",
				"idea:",
				"detail_view: idea_detail",
				"all_ideas:",
				"entity_type: idea",
			},
			wantNotContain: []string{
				"detail_view: idea_detail\n        entity_type", // no longer on list
			},
		},
		{
			name: "multiple lists, one with detail_view (subset inheritance)",
			input: `lists:
  all_ideas:
    entity_type: idea
    detail_view: idea_detail
  active_ideas:
    entity_type: idea
  game_changers:
    entity_type: idea
`,
			wantContains: []string{
				"entity_views:",
				"detail_view: idea_detail",
				"active_ideas:",
				"game_changers:",
			},
		},
		{
			name: "merge into existing entity_views (matching value)",
			input: `entity_views:
  idea:
    detail_view: idea_detail
lists:
  all_ideas:
    entity_type: idea
    detail_view: idea_detail
`,
			wantContains: []string{
				"entity_views:",
				"detail_view: idea_detail",
			},
		},
		{
			name: "conflict skipped, lists untouched",
			input: `lists:
  all_ideas:
    entity_type: idea
    detail_view: idea_detail
  alt_ideas:
    entity_type: idea
    detail_view: other_detail
`,
			wantContains: []string{
				"detail_view: idea_detail",
				"detail_view: other_detail",
			},
			wantNotContain: []string{
				"entity_views:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.input), &doc); err != nil {
				t.Fatalf("failed to parse yaml: %v", err)
			}
			migration := &DetailViewToEntityViewsMigration{}
			if err := migration.Apply(&doc); err != nil {
				t.Fatalf("Apply() error: %v", err)
			}
			out, err := yaml.Marshal(&doc)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}
			outStr := string(out)
			for _, want := range tt.wantContains {
				if !strings.Contains(outStr, want) {
					t.Errorf("expected output to contain %q, got:\n%s", want, outStr)
				}
			}
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(outStr, notWant) {
					t.Errorf("expected output NOT to contain %q, got:\n%s", notWant, outStr)
				}
			}
		})
	}
}

func TestDetailViewToEntityViewsMigration_RefusesMalformedExistingEntry(t *testing.T) {
	// If entity_views.<type> is hand-written as a non-mapping (e.g. scalar
	// shorthand), Apply must error rather than silently delete the list-level
	// detail_view we'd otherwise strip — that would be data loss.
	input := `entity_views:
  idea: "oops_a_string"
lists:
  all_ideas:
    entity_type: idea
    detail_view: idea_detail
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	migration := &DetailViewToEntityViewsMigration{}
	err := migration.Apply(&doc)
	if err == nil {
		t.Fatal("expected error for malformed entity_views entry")
	}
	if !strings.Contains(err.Error(), "entity_views.idea") {
		t.Errorf("expected error to mention entity_views.idea, got: %v", err)
	}
	// And the list-level detail_view must still be present (not stripped).
	out, _ := yaml.Marshal(&doc)
	if !strings.Contains(string(out), "detail_view: idea_detail") {
		t.Errorf("expected list-level detail_view to remain after refusal, got:\n%s", out)
	}
}

func TestDetailViewToEntityViewsMigration_Idempotent(t *testing.T) {
	// Single migrate-able group + one conflicting group. After Apply,
	// Detect must return false (the conflict stays untouched, but Detect
	// only reports migrate-able groups remain).
	input := `lists:
  all_ideas:
    entity_type: idea
    detail_view: idea_detail
  conflict_a:
    entity_type: ticket
    detail_view: ticket_view_a
  conflict_b:
    entity_type: ticket
    detail_view: ticket_view_b
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	migration := &DetailViewToEntityViewsMigration{}

	if !migration.Detect(&doc) {
		t.Fatal("expected Detect to return true on first pass")
	}
	if err := migration.Apply(&doc); err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	if migration.Detect(&doc) {
		out, _ := yaml.Marshal(&doc)
		t.Errorf("Detect should return false after first Apply (conflict shouldn't keep triggering); output:\n%s", string(out))
	}
	// Second Apply must be a no-op.
	if err := migration.Apply(&doc); err != nil {
		t.Fatalf("second Apply: %v", err)
	}
}

// TestDetailViewToEntityViews_InTreeConfigs_AlreadyMigrated guards against
// regressions where a contributor adds a new lists.<id>.detail_view to an
// in-tree data-entry.yaml without re-running the migration. The CI test
// walks the repo for data-entry.yaml files and asserts Detect()=false on
// each. Excludes prototypes/data-entry/catalog/ which has unrelated
// pre-existing pending migrations (no detail_view in that file).
func TestDetailViewToEntityViews_InTreeConfigs_AlreadyMigrated(t *testing.T) {
	repoRoot := findRepoRoot(t)
	migration := &DetailViewToEntityViewsMigration{}
	skipDirs := map[string]bool{
		"node_modules": true,
		".git":         true,
		// testdata/ is sacrosanct in Go's convention: fixtures may be
		// deliberately unmigrated to exercise migration logic.
		"testdata": true,
	}

	err := filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && skipDirs[info.Name()] {
			return filepath.SkipDir
		}
		if info.IsDir() || info.Name() != "data-entry.yaml" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("read %s: %v", path, readErr)
			return nil
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(data, &doc); err != nil {
			// File doesn't parse as YAML; not our concern. Other tests
			// validate config-file shape — we only care about migration
			// state of files that *do* parse.
			t.Logf("skip non-yaml %s: %v", path, err)
			return nil
		}
		if migration.Detect(&doc) {
			rel, _ := filepath.Rel(repoRoot, path)
			t.Errorf("%s has list-level detail_view that needs migrating; run `rela migrate --project %s`",
				rel, filepath.Dir(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk failed: %v", err)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate go.mod walking up from %s", dir)
		}
		dir = parent
	}
}

func TestDetailViewToEntityViewsMigration_PlacementAfterLists(t *testing.T) {
	input := `version: "1.0"
forms:
  edit_idea:
    entity_type: idea
lists:
  all_ideas:
    entity_type: idea
    detail_view: idea_detail
views:
  idea_detail:
    entry:
      type: idea
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	migration := &DetailViewToEntityViewsMigration{}
	if err := migration.Apply(&doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	out, _ := yaml.Marshal(&doc)
	outStr := string(out)
	listsIdx := strings.Index(outStr, "lists:")
	entityViewsIdx := strings.Index(outStr, "entity_views:")
	viewsIdx := strings.Index(outStr, "views:")
	if listsIdx == -1 || entityViewsIdx == -1 || viewsIdx == -1 {
		t.Fatalf("missing expected sections; output:\n%s", outStr)
	}
	if listsIdx >= entityViewsIdx || entityViewsIdx >= viewsIdx {
		t.Errorf("expected ordering lists < entity_views < views; got:\n%s", outStr)
	}
}
