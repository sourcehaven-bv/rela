package automation_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestCopyTrigger pins `on.copy:` (TKT-50DZEHI). Alone it fires on every run
// of a named copy, created or updated, whether or not a value changed. Beside
// an event key it narrows that key to the copy's write. A trigger without it
// ignores where a write came from, and a rename re-runs a copy trigger.
func TestCopyTrigger(t *testing.T) {
	t.Parallel()
	engine := automation.NewEngine([]automation.Automation{
		{Name: "adopt", On: automation.Trigger{Entity: []string{"doc"}, Copy: []string{"adopt"}},
			Do: []automation.Action{{LuaFile: "adopt.lua"}}},
		{Name: "adopt-created", On: automation.Trigger{Entity: []string{"doc"}, Copy: []string{"adopt", "readopt"}, Created: true},
			Do: []automation.Action{{LuaFile: "adopt-created.lua"}}},
		{Name: "any-update", On: automation.Trigger{Entity: []string{"doc"}, Updated: true},
			Do: []automation.Action{{LuaFile: "any-update.lua"}}},
	})
	doc := func(title string) *entity.Entity {
		e := entity.New("DOC-1", "doc")
		e.Properties["title"] = title
		return e
	}
	tests := []struct {
		name string
		ev   automation.Event
		want []string
	}{
		{"copy creates", automation.Event{Type: automation.EventEntityCreated, Entity: doc("a"), Copy: "adopt"},
			[]string{"adopt-created.lua", "adopt.lua"}},
		{"copy updates", automation.Event{Type: automation.EventEntityUpdated,
			Entity: doc("b"), OldEntity: doc("a"), Copy: "adopt"},
			[]string{"adopt.lua", "any-update.lua"}},
		{"copy changes nothing", automation.Event{Type: automation.EventEntityUpdated,
			Entity: doc("a"), OldEntity: doc("a"), Copy: "adopt"},
			[]string{"adopt.lua"}},
		{"other copy in the list", automation.Event{Type: automation.EventEntityCreated, Entity: doc("a"), Copy: "readopt"},
			[]string{"adopt-created.lua"}},
		{"unnamed copy", automation.Event{Type: automation.EventEntityUpdated,
			Entity: doc("b"), OldEntity: doc("a"), Copy: "restore"},
			[]string{"any-update.lua"}},
		{"plain create", automation.Event{Type: automation.EventEntityCreated, Entity: doc("a")}, nil},
		{"plain update", automation.Event{Type: automation.EventEntityUpdated, Entity: doc("b"), OldEntity: doc("a")},
			[]string{"any-update.lua"}},
		// Without a metamodel the copy targets are unknown, so a rename
		// re-runs no copy trigger. TestCopyTriggerRename covers the targets.
		{"rename", automation.Event{Type: automation.EventEntityRenamed, Entity: doc("a")},
			[]string{"any-update.lua"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, l := range engine.Process(context.Background(), tc.ev).LuaToExecute {
				got = append(got, l.FilePath)
			}
			slices.Sort(got)
			if !slices.Equal(got, tc.want) {
				t.Errorf("fired %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCopyTriggerRename: a rename re-runs a copy trigger only on a face one
// of its copies writes, never on the copy's source face or another type.
func TestCopyTriggerRename(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(`
version: "1"
entities:
  doc:
    label: Doc
    id_prefix: DOC
    faces: {draft: {}, adopted: {}}
    properties: {title: {type: string}}
  note:
    label: Note
    id_prefix: NOTE
    properties: {title: {type: string}}
copies:
  adopt:
    from: doc@draft
    to: doc@adopted
    fields: all
    guard: {permission: adopt}
automations:
  - name: push
    on: {copy: adopt}
    do: [{lua_file: push.lua, background: true}]
`))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	engine, err := automation.NewEngineFromMetamodel(meta, meta.Automations)
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	tests := []struct {
		name string
		typ  string
		face entity.Face
		want int
	}{
		{"target face", "doc", "adopted", 1},
		{"source face", "doc", "draft", 0},
		{"other type", "note", "", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := entity.New("X-1", tc.typ)
			e.Face = tc.face
			got := engine.Process(context.Background(), automation.Event{Type: automation.EventEntityRenamed, Entity: e})
			if len(got.LuaToExecute) != tc.want {
				t.Errorf("fired %d actions, want %d", len(got.LuaToExecute), tc.want)
			}
		})
	}
}
