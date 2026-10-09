package automation_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestUpdatedTrigger: `updated` fires on any change to a property or the
// body; not on a create, nor on an update that changes nothing (a value
// whose Go type differs counts as unchanged). A rename re-runs every
// entity trigger.
func TestUpdatedTrigger(t *testing.T) {
	engine := automation.NewEngine([]automation.Automation{
		{Name: "any", On: automation.Trigger{Entity: []string{"note"}, Updated: true},
			Do: []automation.Action{{LuaFile: "any.lua", Background: true}}},
		{Name: "status", On: automation.Trigger{Entity: []string{"note"}, Property: "status"},
			Do: []automation.Action{{LuaFile: "status.lua"}}},
	})
	note := func(title, body string) *entity.Entity {
		e := entity.New("NOTE-1", "note")
		e.Properties["title"] = title
		e.Content = body
		return e
	}
	counted := func(n any) *entity.Entity {
		e := note("a", "")
		e.Properties["n"] = n
		return e
	}
	fired := func(ev automation.Event) []string {
		var out []string
		for _, l := range engine.Process(context.Background(), ev).LuaToExecute {
			out = append(out, l.FilePath)
		}
		return out
	}
	tests := []struct {
		name string
		ev   automation.Event
		want []string
	}{
		{"property change", automation.Event{Type: automation.EventEntityUpdated,
			Entity: note("b", ""), OldEntity: note("a", "")}, []string{"any.lua"}},
		{"body change", automation.Event{Type: automation.EventEntityUpdated,
			Entity: note("a", "new"), OldEntity: note("a", "old")}, []string{"any.lua"}},
		{"no change", automation.Event{Type: automation.EventEntityUpdated,
			Entity: note("a", ""), OldEntity: note("a", "")}, nil},
		{"same value, other type", automation.Event{Type: automation.EventEntityUpdated,
			Entity: counted(2.0), OldEntity: counted(int64(2))}, nil},
		{"create", automation.Event{Type: automation.EventEntityCreated, Entity: note("a", "")}, nil},
		{"rename", automation.Event{Type: automation.EventEntityRenamed, Entity: note("a", "")},
			[]string{"any.lua", "status.lua"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := fired(tc.ev); !slices.Equal(got, tc.want) {
				t.Errorf("fired %v, want %v", got, tc.want)
			}
		})
	}
	// The background flag survives the action → LuaToExecute hop.
	res := engine.Process(context.Background(), automation.Event{Type: automation.EventEntityRenamed, Entity: note("a", "")})
	if !res.LuaToExecute[0].Background {
		t.Error("Background dropped on the way to LuaToExecute")
	}
}

// TestConvertCarriesBackgroundAndUpdated: the field-by-field metamodel
// conversion keeps both new fields.
func TestConvertCarriesBackgroundAndUpdated(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`
entities:
  note: {label: Note, id_prefix: "NOTE-", properties: {title: {type: string}}}
automations:
  - name: push
    on: {entity: note, updated: true}
    do:
      - lua_file: push.lua
        background: true
`))
	if err != nil {
		t.Fatal(err)
	}
	engine, err := automation.NewEngineFromMetamodel(meta, meta.Automations)
	if err != nil {
		t.Fatal(err)
	}
	res := engine.Process(context.Background(), automation.Event{
		Type: automation.EventEntityRenamed, Entity: entity.New("NOTE-1", "note"),
	})
	if len(res.LuaToExecute) != 1 || !res.LuaToExecute[0].Background {
		t.Fatalf("LuaToExecute = %+v", res.LuaToExecute)
	}
}
