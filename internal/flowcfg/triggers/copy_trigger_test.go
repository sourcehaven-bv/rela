package triggers_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg/triggers"
)

// TestCopyTrigger pins `on.copy:` (TKT-50DZEHI). Alone it fires on every run
// of a named copy, created or updated, whether or not a value changed. Beside
// an event key it narrows that key to the copy's write. A trigger without it
// ignores where a write came from, and a rename re-runs a copy trigger.
func TestCopyTrigger(t *testing.T) {
	t.Parallel()
	engine := triggers.NewEngine([]triggers.Rule{
		{Name: "adopt", On: triggers.Trigger{Entity: []string{"doc"}, Copy: []string{"adopt"}},
			Do: []triggers.Action{{LuaFile: "adopt.lua"}}},
		{Name: "adopt-created", On: triggers.Trigger{Entity: []string{"doc"}, Copy: []string{"adopt", "readopt"}, Created: true},
			Do: []triggers.Action{{LuaFile: "adopt-created.lua"}}},
		{Name: "any-update", On: triggers.Trigger{Entity: []string{"doc"}, Updated: true},
			Do: []triggers.Action{{LuaFile: "any-update.lua"}}},
	})
	doc := func(title string) *entity.Entity {
		e := entity.New("DOC-1", "doc")
		e.Properties["title"] = title
		return e
	}
	tests := []struct {
		name string
		ev   triggers.Event
		want []string
	}{
		{"copy creates", triggers.Event{Type: triggers.EventEntityCreated, Entity: doc("a"), Copy: "adopt"},
			[]string{"adopt-created.lua", "adopt.lua"}},
		{"copy updates", triggers.Event{Type: triggers.EventEntityUpdated,
			Entity: doc("b"), OldEntity: doc("a"), Copy: "adopt"},
			[]string{"adopt.lua", "any-update.lua"}},
		{"copy changes nothing", triggers.Event{Type: triggers.EventEntityUpdated,
			Entity: doc("a"), OldEntity: doc("a"), Copy: "adopt"},
			[]string{"adopt.lua"}},
		{"other copy in the list", triggers.Event{Type: triggers.EventEntityCreated, Entity: doc("a"), Copy: "readopt"},
			[]string{"adopt-created.lua"}},
		{"unnamed copy", triggers.Event{Type: triggers.EventEntityUpdated,
			Entity: doc("b"), OldEntity: doc("a"), Copy: "restore"},
			[]string{"any-update.lua"}},
		{"plain create", triggers.Event{Type: triggers.EventEntityCreated, Entity: doc("a")}, nil},
		{"plain update", triggers.Event{Type: triggers.EventEntityUpdated, Entity: doc("b"), OldEntity: doc("a")},
			[]string{"any-update.lua"}},
		// Without a metamodel the copy targets are unknown, so a rename
		// re-runs no copy trigger. TestCopyTriggerRename covers the targets.
		{"rename", triggers.Event{Type: triggers.EventEntityRenamed, Entity: doc("a")},
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
	engine := newEngine(t, `
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
`, `
triggers:
  push:
    on: {copy: adopt}
    outputs: [{script: {file: push.lua, background: true}}]
`)
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
			got := engine.Process(context.Background(), triggers.Event{Type: triggers.EventEntityRenamed, Entity: e})
			if len(got.LuaToExecute) != tc.want {
				t.Errorf("fired %d actions, want %d", len(got.LuaToExecute), tc.want)
			}
		})
	}
}
