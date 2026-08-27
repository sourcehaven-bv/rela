package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestCleanup_CalendarBothArms covers a hazard specific to how cascade cleanup
// is structured: removing a config section needs a case in BOTH the plan phase
// (kind → Change) and the apply phase (Action → YAML edit).
//
// Implementing only one half is silent. Plan-only produces a Change nothing
// executes; apply-only is dead code. Either way deleting an entity type leaves
// a dangling calendar behind, and the next boot fails config validation with an
// error pointing at the calendar rather than at the deletion that orphaned it.
//
// The test drives the real path end to end — reference discovery, planning,
// execution — rather than asserting on either half alone.
func TestCleanup_CalendarBothArms(t *testing.T) {
	tmpDir := t.TempDir()

	dataEntry := `lists:
  keep-list:
    entity_type: keeper
calendars:
  doomed-calendar:
    title: Doomed
    sources:
      - entity_type: to-remove
        date: due
  keep-calendar:
    title: Keep
    sources:
      - entity_type: keeper
        date: due
`
	dataEntryPath := filepath.Join(tmpDir, "data-entry.yaml")
	if err := os.WriteFile(dataEntryPath, []byte(dataEntry), 0o644); err != nil {
		t.Fatal(err)
	}

	metamodel := `entities:
  keeper:
    properties:
      due:
        type: date
  to-remove:
    properties:
      due:
        type: date
`
	metamodelPath := filepath.Join(tmpDir, "schema.yaml")
	if err := os.WriteFile(metamodelPath, []byte(metamodel), 0o644); err != nil {
		t.Fatal(err)
	}

	// Plan phase: the calendar referencing the doomed type must be discovered
	// and turned into a remove_calendar change.
	analysis := &Analysis{
		UnusedEntityTypes: []TypeUsage{{
			Name:  "to-remove",
			Count: 0,
			References: []Reference{{
				File:    "data-entry.yaml",
				Section: "calendars.doomed-calendar",
				Kind:    "calendar",
			}},
		}},
	}
	plan := PlanCleanup(analysis, "schema.yaml")

	var found bool
	for _, c := range plan.DataEntryChanges {
		if c.Action == "remove_calendar" && c.Target == "doomed-calendar" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("plan phase produced no remove_calendar change: %+v", plan.DataEntryChanges)
	}

	// Apply phase: the change must actually edit the YAML.
	if err := ExecuteCleanup(plan, metamodelPath, tmpDir, false); err != nil {
		t.Fatalf("ExecuteCleanup failed: %v", err)
	}

	content, err := os.ReadFile(dataEntryPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(content)

	if strings.Contains(got, "doomed-calendar:") {
		t.Errorf("calendar referencing the removed type should be gone:\n%s", got)
	}
	if !strings.Contains(got, "keep-calendar:") {
		t.Errorf("unrelated calendar must survive:\n%s", got)
	}
	if !strings.Contains(got, "keep-list:") {
		t.Errorf("unrelated list must survive:\n%s", got)
	}
}

// TestFindEntityTypeReferences_Calendar checks reference discovery directly,
// including that a calendar naming the same type in several sources is
// reported once rather than per source.
func TestFindEntityTypeReferences_Calendar(t *testing.T) {
	cfg := &dataentryconfig.Config{
		Calendars: map[string]dataentryconfig.Calendar{
			"multi": {Sources: []dataentryconfig.CalendarSource{
				{EntityType: "task", Date: "due"},
				{EntityType: "task", Date: "starts_at"},
				{EntityType: "meeting", Date: "starts_at"},
			}},
			"other": {Sources: []dataentryconfig.CalendarSource{
				{EntityType: "meeting", Date: "starts_at"},
			}},
		},
	}
	meta := &metamodel.Metamodel{}

	refs := findEntityTypeReferences("task", meta, cfg)

	var calRefs int
	for _, r := range refs {
		if r.Kind == "calendar" {
			calRefs++
			if r.Section != "calendars.multi" {
				t.Errorf("section = %q, want calendars.multi", r.Section)
			}
		}
	}
	if calRefs != 1 {
		t.Errorf("a calendar naming the type twice must be reported once, got %d", calRefs)
	}
}
