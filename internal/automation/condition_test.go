package automation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// datePropMeta declares a date property, the shape a recurring-task
// condition runs against.
func datePropMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"taak": {
				Properties: map[string]metamodel.PropertyDef{
					"due":    {Type: metamodel.PropertyTypeDate},
					"status": {Type: metamodel.PropertyTypeString},
				},
			},
		},
	}
}

// condAutomation builds a `condition:`-bearing automation definition.
func condAutomation(cond string, when ...string) metamodel.AutomationDef {
	return metamodel.AutomationDef{
		Name: "due-soon",
		On: metamodel.AutomationTrigger{
			Entity:    metamodel.StringOrSlice{"taak"},
			Created:   true,
			When:      when,
			Condition: cond,
		},
		Do: []metamodel.AutomationAction{{Set: "status", Value: "due-soon"}},
	}
}

// pinClock seeds the lazily-built Evaluator with a fixed clock so
// today() does not drift with the calendar.
func pinClock(e *Engine, meta *metamodel.Metamodel, now time.Time) {
	e.ev = predicatefns.NewEvaluatorWithClock(meta, func() time.Time { return now })
}

// TestEngine_Condition_DateArithmetic is the payoff for TKT-8GD41J: an
// automation condition doing date arithmetic, which no filter-syntax
// `when:` could express.
func TestEngine_Condition_DateArithmetic(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	meta := datePropMeta()

	tests := []struct {
		name     string
		due      string
		wantFire bool
	}{
		{"due today", "2026-08-18", true},
		{"due in 3 days", "2026-08-21", true},
		{"due in exactly 7 days", "2026-08-25", true},
		{"due in 8 days", "2026-08-26", false},
		{"due far out", "2026-12-01", false},
		{"already overdue", "2026-08-10", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine, err := NewEngineFromMetamodel(meta,
				[]metamodel.AutomationDef{
					condAutomation("days_between(entity.due, today()) <= 7"),
				})
			if err != nil {
				t.Fatalf("build engine: %v", err)
			}
			pinClock(engine, meta, now)

			ent := buildEntity(testutil.Entity("taak").With("due", tc.due))
			result := engine.Process(context.Background(),
				Event{Type: EventEntityCreated, Entity: ent})

			fired := result.PropertiesSet["status"] == "due-soon"
			if fired != tc.wantFire {
				t.Errorf("due=%s: fired=%v want %v (PropertiesSet=%v)",
					tc.due, fired, tc.wantFire, result.PropertiesSet)
			}
		})
	}
}

// TestEngine_Condition_AndsWithWhen pins that `when:` and `condition:`
// combine with AND — each side can independently block the automation.
func TestEngine_Condition_AndsWithWhen(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	meta := datePropMeta()

	tests := []struct {
		name     string
		status   string
		due      string
		wantFire bool
	}{
		{"both hold", "todo", "2026-08-20", true},
		{"when fails", "done", "2026-08-20", false},
		{"condition fails", "todo", "2026-12-01", false},
		{"both fail", "done", "2026-12-01", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine, err := NewEngineFromMetamodel(meta,
				[]metamodel.AutomationDef{
					condAutomation(
						"days_between(entity.due, today()) <= 7",
						"status=todo"),
				})
			if err != nil {
				t.Fatalf("build engine: %v", err)
			}
			pinClock(engine, meta, now)

			ent := buildEntity(testutil.Entity("taak").
				With("due", tc.due).
				With("status", tc.status))
			result := engine.Process(context.Background(),
				Event{Type: EventEntityCreated, Entity: ent})

			fired := result.PropertiesSet["status"] == "due-soon"
			if fired != tc.wantFire {
				t.Errorf("status=%s due=%s: fired=%v want %v",
					tc.status, tc.due, fired, tc.wantFire)
			}
		})
	}
}

// TestEngine_Condition_CompileErrorIsFatal is the load-bearing negative
// test. A broken condition must FAIL THE LOAD, not be skipped: a
// dropped constraint makes the automation fire on more entities than
// the operator wrote, which is invisible and unsafe.
func TestEngine_Condition_CompileErrorIsFatal(t *testing.T) {
	tests := []struct {
		name string
		cond string
	}{
		{"syntax error", "days_between(entity.due, today()"},
		{"unknown function", "no_such_fn(entity.due) <= 7"},
		{"unknown property", "days_between(entity.nope, today()) <= 7"},
		{"non-boolean result", "entity.status"},
		{"wrong argument type", "days_between(entity.status, today()) <= 7"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewEngineFromMetamodel(datePropMeta(),
				[]metamodel.AutomationDef{condAutomation(tc.cond)})
			if err == nil {
				t.Fatal("want a load error for a broken condition, got none")
			}
			if !strings.Contains(err.Error(), "due-soon") {
				t.Errorf("error %q does not name the offending automation", err)
			}
		})
	}
}

// TestEngine_WhenParseErrorIsFatal pins the same posture for the legacy
// `when:` key, which used to swallow an unparseable clause and thereby
// widen the automation.
func TestEngine_WhenParseErrorIsFatal(t *testing.T) {
	_, err := NewEngineFromMetamodel(datePropMeta(),
		[]metamodel.AutomationDef{condAutomation("", "!!not a filter!!")})
	if err == nil {
		t.Fatal("want a load error for an unparseable when clause, got none")
	}
}

// TestEngine_Condition_RequiresMetamodel pins that a condition on an
// engine with no schema is a construction error rather than a silent
// no-op — there is no typed env to compile against.
func TestEngine_Condition_RequiresMetamodel(t *testing.T) {
	_, err := NewEngineFromMetamodel(nil,
		[]metamodel.AutomationDef{
			condAutomation("days_between(entity.due, today()) <= 7"),
		})
	if err == nil {
		t.Fatal("want an error for a condition without a metamodel, got none")
	}
}

// TestEngine_Condition_RequiresEntityType pins that a condition needs
// `entity:` to name the type its env is built from.
func TestEngine_Condition_RequiresEntityType(t *testing.T) {
	def := condAutomation("days_between(entity.due, today()) <= 7")
	def.On.Entity = nil
	_, err := NewEngineFromMetamodel(datePropMeta(),
		[]metamodel.AutomationDef{def})
	if err == nil {
		t.Fatal("want an error for a condition without entity:, got none")
	}
}

// TestEngine_NoCondition_Unchanged pins that an automation without a
// condition behaves exactly as before.
func TestEngine_NoCondition_Unchanged(t *testing.T) {
	meta := datePropMeta()
	engine, err := NewEngineFromMetamodel(meta,
		[]metamodel.AutomationDef{condAutomation("", "status=todo")})
	if err != nil {
		t.Fatalf("build engine: %v", err)
	}
	ent := buildEntity(testutil.Entity("taak").With("status", "todo"))
	result := engine.Process(context.Background(),
		Event{Type: EventEntityCreated, Entity: ent})
	if result.PropertiesSet["status"] != "due-soon" {
		t.Errorf("plain when: automation did not fire; PropertiesSet=%v",
			result.PropertiesSet)
	}
}
