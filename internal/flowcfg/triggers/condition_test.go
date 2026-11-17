package triggers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/filter"
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

// condAutomation builds a rule with an `on.when:` predicate and optional
// `where:` clauses.
func condAutomation(cond string, where ...*filter.Filter) Rule {
	return Rule{
		Name: "due-soon",
		On: Trigger{
			Entity:    []string{"taak"},
			Created:   true,
			When:      where,
			Condition: cond,
		},
		Do: []Action{{Set: "status", Value: "due-soon"}},
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
			engine, err := New(meta,
				[]Rule{
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
			engine, err := New(meta,
				[]Rule{
					condAutomation(
						"days_between(entity.due, today()) <= 7",
						mustFilters(t, "status=todo")...),
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
			_, err := New(datePropMeta(),
				[]Rule{condAutomation(tc.cond)})
			if err == nil {
				t.Fatal("want a load error for a broken condition, got none")
			}
			if !strings.Contains(err.Error(), "due-soon") {
				t.Errorf("error %q does not name the offending automation", err)
			}
		})
	}
}

// TestEngine_WhenParseErrorIsFatal pins the same posture for the `where:`
// key (the old automation `when:`), which used to swallow an unparseable
// clause and thereby widen the automation. Clauses are parsed by [Parse].
func TestEngine_WhenParseErrorIsFatal(t *testing.T) {
	_, err := Parse([]byte(`
triggers:
  due-soon:
    on: {entity: taak, created: true, where: ["=novalue"]}
    outputs:
      - set: {properties: {status: due-soon}}
`), datePropMeta())
	if err == nil {
		t.Fatal("want a load error for an unparseable where clause, got none")
	}
	if !strings.Contains(err.Error(), "due-soon") {
		t.Errorf("error %q does not name the offending trigger", err)
	}
}

// TestEngine_Condition_RequiresMetamodel pins that a condition on an
// engine with no schema is a construction error rather than a silent
// no-op — there is no typed env to compile against.
func TestEngine_Condition_RequiresMetamodel(t *testing.T) {
	_, err := New(nil,
		[]Rule{
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
	_, err := New(datePropMeta(),
		[]Rule{def})
	if err == nil {
		t.Fatal("want an error for a condition without entity:, got none")
	}
}

// TestEngine_NoCondition_Unchanged pins that an automation without a
// condition behaves exactly as before.
func TestEngine_NoCondition_Unchanged(t *testing.T) {
	meta := datePropMeta()
	engine, err := New(meta,
		[]Rule{condAutomation("", mustFilters(t, "status=todo")...)})
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

// TestEngine_Condition_EvalErrorWarns pins that an eval-time failure is
// SURFACED, not swallowed. The automation correctly does not fire — it
// must not act on a condition that could not be shown to hold — but a
// condition that silently never matches is the exact failure this key
// exists to remove, so the reason has to reach the caller.
func TestEngine_Condition_EvalErrorWarns(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	meta := datePropMeta()

	engine, err := New(meta,
		[]Rule{
			condAutomation("days_between(entity.due, today()) <= 7"),
		})
	if err != nil {
		t.Fatalf("build engine: %v", err)
	}
	pinClock(engine, meta, now)

	// No `due` property: the date binds as nil and days_between fails at
	// eval, which the compiler cannot catch.
	ent := buildEntity(testutil.Entity("taak").With("status", "todo"))
	result := engine.Process(context.Background(),
		Event{Type: EventEntityCreated, Entity: ent})

	if result.PropertiesSet["status"] == "due-soon" {
		t.Error("automation fired on a condition that could not be evaluated")
	}
	if !result.HasWarnings() {
		t.Fatal("eval failure produced no warning — it was swallowed")
	}
	if !strings.Contains(result.Warnings[0], "`when`") {
		t.Errorf("warning %q does not identify the `when` predicate", result.Warnings[0])
	}
}

// TestEngine_Condition_NilGuard pins the guarded form the docs
// recommend for an optional date property. Unguarded, a missing `due`
// is an eval error and the entity is skipped; guarded, the condition
// evaluates cleanly to false with no warning.
func TestEngine_Condition_NilGuard(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	meta := datePropMeta()

	engine, err := New(meta,
		[]Rule{
			condAutomation(
				"entity.due ~= nil and days_between(entity.due, today()) <= 7"),
		})
	if err != nil {
		t.Fatalf("build engine: %v", err)
	}
	pinClock(engine, meta, now)

	t.Run("missing due is a clean false", func(t *testing.T) {
		ent := buildEntity(testutil.Entity("taak").With("status", "todo"))
		res := engine.Process(context.Background(),
			Event{Type: EventEntityCreated, Entity: ent})
		if res.PropertiesSet["status"] == "due-soon" {
			t.Error("fired on an entity with no due date")
		}
		if res.HasWarnings() {
			t.Errorf("guarded condition still warned: %v", res.Warnings)
		}
	})

	t.Run("present due still matches", func(t *testing.T) {
		ent := buildEntity(testutil.Entity("taak").With("due", "2026-08-20"))
		res := engine.Process(context.Background(),
			Event{Type: EventEntityCreated, Entity: ent})
		if res.PropertiesSet["status"] != "due-soon" {
			t.Errorf("guard broke the normal path; PropertiesSet=%v", res.PropertiesSet)
		}
	})
}
