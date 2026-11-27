package triggers

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

const unsetSchema = `
types:
  phase:
    values: [a, b]
    transitions:
      - {from: a, to: b}
entities:
  todo:
    label: To-do
    id_prefix: "TODO-"
    properties:
      title: {type: string, required: true}
      status: {type: enum, values: [open, done]}
      completed_on: {type: date}
      score: {type: integer, computed: "1"}
      scan: {type: file}
      stage: {type: phase}
      ref: {type: external_ref, system: tracker}
relations:
  follows: {from: [todo], to: [todo]}
`

// TestSetUnset_ClearsCompletedOnReopen pins the behavior the output exists
// for: reopening a to-do removes its completion date.
func TestSetUnset_ClearsCompletedOnReopen(t *testing.T) {
	t.Parallel()
	meta := mustMeta(t, unsetSchema)
	rules := mustParse(t, meta, `
triggers:
  reopen:
    on: {entity: todo, property: status, becomes: open}
    outputs:
      - set:
          unset: [completed_on]
`)
	engine, err := New(meta, rules)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	old := &entity.Entity{ID: "TODO-1", Type: "todo", Properties: map[string]any{
		"title": "Buy milk", "status": "done", "completed_on": "2026-11-20",
	}}
	next := old.Clone()
	next.Properties["status"] = "open"
	res := engine.Process(context.Background(), Event{Type: EventEntityUpdated, Entity: next, OldEntity: old})
	if !slices.Equal(res.PropertiesUnset, []string{"completed_on"}) {
		t.Fatalf("PropertiesUnset = %v, want [completed_on]", res.PropertiesUnset)
	}
	if !res.HasWrites() {
		t.Error("HasWrites = false for a result that removes a property")
	}
	res.Writes().Apply(next)
	if _, ok := next.Properties["completed_on"]; ok {
		t.Errorf("completed_on survived Apply: %v", next.Properties)
	}
}

// TestSetUnset_LastOutputWins pins that a name is never both set and
// removed: the output rendered last decides.
func TestSetUnset_LastOutputWins(t *testing.T) {
	t.Parallel()
	meta := mustMeta(t, unsetSchema)
	rules := mustParse(t, meta, `
triggers:
  clear:
    on: {entity: todo, updated: true}
    outputs:
      - set: {properties: {completed_on: "2026-01-01"}}
      - set: {unset: [completed_on]}
      - set: {unset: [status]}
      - set: {properties: {status: done}}
`)
	engine, err := New(meta, rules)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	old := &entity.Entity{ID: "TODO-1", Type: "todo", Properties: map[string]any{"title": "x"}}
	e := old.Clone()
	e.Properties["title"] = "y"
	res := engine.Process(context.Background(), Event{Type: EventEntityUpdated, Entity: e, OldEntity: old})
	if !slices.Equal(res.PropertiesUnset, []string{"completed_on"}) {
		t.Errorf("PropertiesUnset = %v, want [completed_on]", res.PropertiesUnset)
	}
	if got := res.PropertiesSet; len(got) != 1 || got["status"] != "done" {
		t.Errorf("PropertiesSet = %v, want only status=done", got)
	}
}

func TestSetUnset_LoadErrors(t *testing.T) {
	t.Parallel()
	meta := mustMeta(t, unsetSchema)
	cases := []struct {
		name, set, want string
	}{
		{"unknown property", `{unset: [nope]}`, `"nope" is not a property of "todo"`},
		{"set and unset", `{properties: {completed_on: x}, unset: [completed_on]}`, "under both"},
		{"twice", `{unset: [completed_on, completed_on]}`, "twice"},
		{"empty name", `{unset: [""]}`, "empty property name"},
		{"required", `{unset: [title]}`, "is required"},
		{"computed", `{unset: [score]}`, "is computed"},
		{"file", `{unset: [scan]}`, "file property"},
		{"state machine", `{unset: [stage]}`, "not a declared transition"},
		{"external ref", `{unset: [ref]}`, "is an external ref"},
		{"nothing", `{}`, "needs `properties:` or `unset:`"},
	}
	// A relation rule writes its source as an update rule writes its
	// subject, so it is refused the same names.
	events := []string{
		"{entity: todo, updated: true}",
		"{entity: todo, relation_created: follows}",
		"{entity: todo, relation_removed: follows}",
	}
	for _, on := range events {
		for _, tc := range cases {
			t.Run(on+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				_, err := Parse([]byte(`
triggers:
  r:
    on: `+on+`
    outputs:
      - set: `+tc.set+"\n"), meta)
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("err = %v, want it to contain %q", err, tc.want)
				}
			})
		}
	}
}

// TestSetUnset_TypelessRuleNeedsADeclaringType refuses a name no type
// declares when the rule names no `on.entity:`.
func TestSetUnset_TypelessRuleNeedsADeclaringType(t *testing.T) {
	t.Parallel()
	meta := mustMeta(t, unsetSchema)
	_, err := Parse([]byte(`
triggers:
  r:
    on: {updated: true}
    outputs:
      - set: {unset: [nope]}
`), meta)
	if err == nil || !strings.Contains(err.Error(), "not a property of any entity type") {
		t.Fatalf("err = %v", err)
	}
}
