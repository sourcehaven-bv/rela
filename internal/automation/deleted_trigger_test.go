package automation_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestDeletedTrigger: `deleted` matches a delete event and nothing else, and
// no other trigger matches a delete event. `when:` and `faces:` decide on the
// deleted face's last state.
func TestDeletedTrigger(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`
entities:
  page:
    label: Page
    id_prefix: "PAGE-"
    faces: {draft: {}, published: {}}
    properties: {title: {type: string}, status: {type: string}}
automations:
  - name: gone
    on: {entity: page, deleted: true}
    do: [{lua_file: gone.lua, background: true}]
  - name: gone-published
    on: {entity: page, deleted: true, faces: [published], when: ["status=live"]}
    do: [{lua_file: gone-published.lua, background: true}]
  - name: made
    on: {entity: page, created: true}
    do: [{lua_file: made.lua, background: true}]
  - name: changed
    on: {entity: page, updated: true}
    do: [{lua_file: changed.lua, background: true}]
`))
	if err != nil {
		t.Fatal(err)
	}
	engine, err := automation.NewEngineFromMetamodel(meta, meta.Automations)
	if err != nil {
		t.Fatal(err)
	}
	page := func(face entity.Face, status string) *entity.Entity {
		e := entity.New("PAGE-1", "page")
		e.Face = face
		e.Properties["status"] = status
		return e
	}
	tests := []struct {
		name string
		ev   automation.Event
		want []string
	}{
		{"delete of a draft", automation.Event{Type: automation.EventEntityDeleted, Entity: page("draft", "live")},
			[]string{"gone.lua"}},
		{"delete of a live published face", automation.Event{Type: automation.EventEntityDeleted,
			Entity: page("published", "live")}, []string{"gone.lua", "gone-published.lua"}},
		{"delete of a retired published face", automation.Event{Type: automation.EventEntityDeleted,
			Entity: page("published", "retired")}, []string{"gone.lua"}},
		{"create", automation.Event{Type: automation.EventEntityCreated, Entity: page("draft", "")},
			[]string{"made.lua"}},
		{"update", automation.Event{Type: automation.EventEntityUpdated, Entity: page("draft", "b"),
			OldEntity: page("draft", "a")}, []string{"changed.lua"}},
		{"rename", automation.Event{Type: automation.EventEntityRenamed, Entity: page("draft", "")},
			[]string{"made.lua", "changed.lua"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			res := engine.Process(context.Background(), tc.ev)
			for _, l := range res.LuaToExecute {
				got = append(got, l.FilePath)
				if !l.Background {
					t.Errorf("%s: Background dropped", l.FilePath)
				}
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("fired %v, want %v", got, tc.want)
			}
		})
	}
}
