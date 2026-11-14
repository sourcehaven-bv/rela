package automation

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestMayFireCoversSelects pins the event index as a superset of
// Engine.selects: an event type the index leaves out of a trigger must
// never select it, or the trigger silently stops firing.
func TestMayFireCoversSelects(t *testing.T) {
	var triggers []Trigger
	for mask := range 1 << 7 {
		on := Trigger{
			Created: mask&1 != 0, Updated: mask&2 != 0, Deleted: mask&4 != 0,
		}
		if mask&8 != 0 {
			on.Property = "status"
		}
		if mask&16 != 0 {
			on.RelationCreated = "part-of"
		}
		if mask&32 != 0 {
			on.RelationRemoved = "part-of"
		}
		if mask&64 != 0 {
			on.Copy = []string{"adopt"}
		}
		triggers = append(triggers, on)
	}
	old := &entity.Entity{ID: "T-1", Type: "task", Properties: map[string]any{"status": "a"}}
	cur := &entity.Entity{ID: "T-1", Type: "task", Properties: map[string]any{"status": "b"}}
	rel := entity.NewRelation("T-1", "part-of", "P-1")
	events := []Event{
		{Type: EventEntityCreated, Entity: cur},
		{Type: EventEntityCreated, Entity: cur, Copy: "adopt"},
		{Type: EventEntityUpdated, Entity: cur, OldEntity: old},
		{Type: EventEntityUpdated, Entity: cur, OldEntity: cur, Copy: "adopt"},
		{Type: EventEntityRenamed, Entity: cur},
		{Type: EventEntityDeleted, Entity: cur},
		{Type: EventRelationCreated, Entity: cur, Relation: rel},
		{Type: EventRelationRemoved, Entity: cur, Relation: rel},
	}
	e := NewEngine(nil)
	for _, on := range triggers {
		for _, ev := range events {
			if e.selects(on, ev) && !mayFire(on, ev.Type) {
				t.Errorf("trigger %+v is selected by event type %d but not indexed under it", on, ev.Type)
			}
		}
	}
}
