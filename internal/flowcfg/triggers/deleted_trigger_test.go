package triggers_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg/triggers"
)

// TestDeletedTrigger: `deleted` matches a delete event and nothing else, and
// no other trigger matches a delete event. `where:` and `faces:` decide on
// the deleted face's last state.
func TestDeletedTrigger(t *testing.T) {
	engine := newEngine(t, `
entities:
  page:
    label: Page
    id_prefix: "PAGE-"
    faces: {draft: {}, published: {}}
    properties: {title: {type: string}, status: {type: string}}
`, `
triggers:
  gone:
    on: {entity: page, deleted: true}
    outputs: [{script: {file: gone.lua, background: true}}]
  gone-published:
    on: {entity: page, deleted: true, faces: [published], where: ["status=live"]}
    outputs: [{script: {file: gone-published.lua, background: true}}]
  made:
    on: {entity: page, created: true}
    outputs: [{script: {file: made.lua, background: true}}]
  changed:
    on: {entity: page, updated: true}
    outputs: [{script: {file: changed.lua, background: true}}]
`)
	page := func(face entity.Face, status string) *entity.Entity {
		e := entity.New("PAGE-1", "page")
		e.Face = face
		e.Properties["status"] = status
		return e
	}
	tests := []struct {
		name string
		ev   triggers.Event
		want []string
	}{
		{"delete of a draft", triggers.Event{Type: triggers.EventEntityDeleted, Entity: page("draft", "live")},
			[]string{"gone.lua"}},
		{"delete of a live published face", triggers.Event{Type: triggers.EventEntityDeleted,
			Entity: page("published", "live")}, []string{"gone.lua", "gone-published.lua"}},
		{"delete of a retired published face", triggers.Event{Type: triggers.EventEntityDeleted,
			Entity: page("published", "retired")}, []string{"gone.lua"}},
		{"create", triggers.Event{Type: triggers.EventEntityCreated, Entity: page("draft", "")},
			[]string{"made.lua"}},
		{"update", triggers.Event{Type: triggers.EventEntityUpdated, Entity: page("draft", "b"),
			OldEntity: page("draft", "a")}, []string{"changed.lua"}},
		{"rename", triggers.Event{Type: triggers.EventEntityRenamed, Entity: page("draft", "")},
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
