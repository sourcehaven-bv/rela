package automation

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// TestEngine_SkipsManagedRecords pins that no operator automation fires on a
// flow run or its `_carries` edge, not even a type-less one (TKT-BC5G8RP).
func TestEngine_SkipsManagedRecords(t *testing.T) {
	t.Parallel()
	engine := NewEngine([]Automation{
		newAutomation("any-create").OnCreate().Set("touched", "yes").Build(),
		newAutomation("on-carries").OnRelationCreated(entity.CarriesRelation).Set("linked", "yes").Build(),
	})

	run := buildEntity(testutil.Entity(entity.FlowRunType).ID("RUN-1"))
	if res := engine.Process(context.Background(), Event{Type: EventEntityCreated, Entity: run}); len(res.PropertiesSet) > 0 {
		t.Errorf("a type-less create trigger fired on a flow run: %v", res.PropertiesSet)
	}

	task := buildEntity(testutil.Entity("ticket").ID("T-1"))
	rel := buildRelation(testutil.NewRelation("RUN-1", entity.CarriesRelation, "T-1"))
	res := engine.Process(context.Background(), Event{Type: EventRelationCreated, Entity: task, Relation: rel})
	if len(res.PropertiesSet) > 0 {
		t.Errorf("a relation trigger fired on a _carries edge: %v", res.PropertiesSet)
	}

	if res := engine.Process(context.Background(), Event{Type: EventEntityCreated, Entity: task}); res.PropertiesSet["touched"] != "yes" {
		t.Error("the type-less create trigger did not fire on an operator entity")
	}
}
