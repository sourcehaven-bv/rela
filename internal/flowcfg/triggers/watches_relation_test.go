package triggers

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// TestEngine_WatchesRelation pins which relation events a write path must
// raise: those a trigger names, and never one on a managed relation.
func TestEngine_WatchesRelation(t *testing.T) {
	t.Parallel()
	engine := NewEngine([]Rule{
		newAutomation("on-implements").OnRelationCreated("implements").Set("linked", "yes").Build(),
		newAutomation("any-create").OnCreate().Set("touched", "yes").Build(),
	})
	tests := []struct {
		name    string
		ev      EventType
		relType string
		want    bool
	}{
		{"watched create", EventRelationCreated, "implements", true},
		{"other relation type", EventRelationCreated, "blocks", false},
		{"remove of a create-only type", EventRelationRemoved, "implements", false},
		{"managed relation", EventRelationCreated, entity.CarriesRelation, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := engine.WatchesRelation(tc.ev, tc.relType); got != tc.want {
				t.Errorf("WatchesRelation(%v, %q) = %v, want %v", tc.ev, tc.relType, got, tc.want)
			}
		})
	}
	var nilEngine *Engine
	if nilEngine.WatchesRelation(EventRelationCreated, "implements") {
		t.Error("a nil engine watches a relation")
	}
}

// TestEngine_SetByNamesTheWritingRules pins the attribution a write path
// reads: each rule that rendered a set, once, and no rule that only matched.
func TestEngine_SetByNamesTheWritingRules(t *testing.T) {
	t.Parallel()
	engine := NewEngine([]Rule{
		newAutomation("first").OnRelationCreated("implements").Set("a", "1").Set("b", "2").Build(),
		newAutomation("second").OnRelationCreated("implements").Set("c", "3").Build(),
		newAutomation("other").OnRelationCreated("blocks").Set("d", "4").Build(),
	})
	task := buildEntity(testutil.Entity("ticket").ID("T-1"))
	rel := buildRelation(testutil.NewRelation("T-1", "implements", "F-1"))
	res := engine.Process(context.Background(), Event{Type: EventRelationCreated, Entity: task, Relation: rel})
	if got := res.SetBy; len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Errorf("SetBy = %v, want [first second]", got)
	}
}
