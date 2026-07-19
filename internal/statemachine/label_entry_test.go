package statemachine

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// labelMeta is a two-value machine whose single edge carries a move label, plus
// a distinct Initial, exercising Performable's Label passthrough and EntryValue.
func labelMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"task-status": {
				Values:  []string{"todo", "doing", "done"},
				Initial: "todo",
				Default: "done", // deliberately != Initial: EntryValue must prefer Initial.
				Transitions: []metamodel.TransitionDef{
					{From: "todo", To: "doing", Label: "Start progress"},
					{From: "doing", To: "done"}, // no label: Performable surfaces ""
				},
			},
		},
		Entities: map[string]metamodel.EntityDef{
			"task": {Properties: map[string]metamodel.PropertyDef{
				"status": {Type: "task-status"},
			}},
		},
	}
}

// Performable surfaces the edge's move label verbatim (empty when unset); the
// display-layer fallback to a state label is the SPA's job, not the machine's.
func TestPerformable_SurfacesLabel(t *testing.T) {
	set := mustCompile(t, labelMeta())
	ctx := context.Background()

	got := set.Performable(ctx, ent("T-1", "task", "todo"), "status", inertGuard{}, fakeLookup{})
	if len(got) != 1 {
		t.Fatalf("expected 1 verdict from todo, got %d: %+v", len(got), got)
	}
	if got[0].To != "doing" || got[0].Label != "Start progress" {
		t.Errorf("labeled move mismatch: %+v", got[0])
	}

	// An unlabeled edge surfaces an empty Label (fallback happens downstream).
	got = set.Performable(ctx, ent("T-1", "task", "doing"), "status", inertGuard{}, fakeLookup{})
	if len(got) != 1 || got[0].To != "done" || got[0].Label != "" {
		t.Errorf("unlabeled move must surface empty Label, got %+v", got)
	}
}

// EntryValue returns the machine's entry (Initial, else Default) for a machine
// field, and "" for a non-machine field or unknown type.
func TestEntryValue(t *testing.T) {
	set := mustCompile(t, labelMeta())

	if got := set.EntryValue("task", "status"); got != "todo" {
		t.Errorf("EntryValue(task,status) = %q, want %q (Initial, not Default)", got, "todo")
	}
	if got := set.EntryValue("task", "title"); got != "" {
		t.Errorf("EntryValue for non-machine prop = %q, want \"\"", got)
	}
	if got := set.EntryValue("nonexistent", "status"); got != "" {
		t.Errorf("EntryValue for unknown type = %q, want \"\"", got)
	}
	if got := EmptySet().EntryValue("task", "status"); got != "" {
		t.Errorf("EntryValue on empty set = %q, want \"\"", got)
	}
}
