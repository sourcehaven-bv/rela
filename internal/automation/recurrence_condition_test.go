package automation

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// TestAtlasRecurrenceConditionIsExpressible is the motivating case for
// this whole line of work: the guard at the top of
// atlas/scripts/recurrence.lua — ~30 lines of hand-rolled Lua reading
// properties that were already declarative — written as one condition.
//
// It also pins why days_between returns Int rather than Number: the last
// clause compares it against `doorlooptijd`, an integer property, and
// the engine requires both sides of an ordered comparison to share a
// type. As Number this expression would not compile.
func TestAtlasRecurrenceConditionIsExpressible(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"terugkerend": {
				Properties: map[string]metamodel.PropertyDef{
					"status":         {Type: metamodel.PropertyTypeString},
					"modus":          {Type: metamodel.PropertyTypeString},
					"herhaling":      {Type: metamodel.PropertyTypeRrule},
					"volgende_datum": {Type: metamodel.PropertyTypeDate},
					"doorlooptijd":   {Type: metamodel.PropertyTypeInteger},
				},
			},
		},
	}
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)

	// The Lua guard: status actief, modus fixed, herhaling set, and
	// due within the doorlooptijd lead time.
	cond := "entity.status == 'actief' and entity.modus == 'fixed' and " +
		"entity.herhaling ~= '' and " +
		"days_between(entity.volgende_datum, today()) <= entity.doorlooptijd"

	def := metamodel.AutomationDef{
		Name: "recurrence-fixed",
		On: metamodel.AutomationTrigger{
			Entity:    metamodel.StringOrSlice{"terugkerend"},
			Created:   true,
			Condition: cond,
		},
		Do: []metamodel.AutomationAction{{Set: "spawn", Value: "yes"}},
	}

	engine, err := NewEngineFromMetamodel(meta, []metamodel.AutomationDef{def})
	if err != nil {
		t.Fatalf("recurrence condition does not compile: %v", err)
	}
	engine.ev = predicatefns.NewEvaluatorWithClock(meta, func() time.Time { return now })

	tests := []struct {
		name string
		p    map[string]any
		want bool
	}{
		{"due within lead time", map[string]any{
			"status": "actief", "modus": "fixed", "herhaling": "FREQ=WEEKLY",
			"volgende_datum": "2026-08-20", "doorlooptijd": 3}, true},
		{"not yet due", map[string]any{
			"status": "actief", "modus": "fixed", "herhaling": "FREQ=WEEKLY",
			"volgende_datum": "2026-09-30", "doorlooptijd": 3}, false},
		{"paused", map[string]any{
			"status": "gepauzeerd", "modus": "fixed", "herhaling": "FREQ=WEEKLY",
			"volgende_datum": "2026-08-20", "doorlooptijd": 3}, false},
		{"other mode", map[string]any{
			"status": "actief", "modus": "after", "herhaling": "FREQ=WEEKLY",
			"volgende_datum": "2026-08-20", "doorlooptijd": 3}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ent := buildEntity(testutil.Entity("terugkerend"))
			ent.Properties = tc.p
			res := engine.Process(context.Background(),
				Event{Type: EventEntityCreated, Entity: ent})
			got := res.PropertiesSet["spawn"] == "yes"
			if got != tc.want {
				t.Errorf("fired=%v want %v", got, tc.want)
			}
		})
	}
}
