package automation

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// intPropMeta is a metamodel with one entity type ("bug") carrying an
// integer property ("count"), used to exercise type-aware comparison.
func intPropMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"bug": {
				Properties: map[string]metamodel.PropertyDef{
					"count":  {Type: metamodel.PropertyTypeInteger},
					"status": {Type: metamodel.PropertyTypeString},
				},
			},
		},
	}
}

// TestEngine_WhenCondition_IntegerComparisonIsNumeric pins the C2 fix:
// a `when: count>9` condition on an integer property must compare
// numerically (10 > 9 → true), not lexicographically ("10" < "9" →
// false). With the metamodel wired, the automation fires; the old
// string-only path would have skipped it.
func TestEngine_WhenCondition_IntegerComparisonIsNumeric(t *testing.T) {
	auto := newAutomation().
		OnCreate("bug").
		When("count>9").
		Set("status", "escalated").
		Build()

	ent := buildEntity(testutil.Entity("bug").With("count", 10))

	t.Run("with metamodel: numeric comparison fires", func(t *testing.T) {
		engine := NewEngineFromMetamodel(intPropMeta(), nil)
		engine.automations = []Automation{auto}

		result := engine.Process(Event{Type: EventEntityCreated, Entity: ent})
		if result.PropertiesSet["status"] != "escalated" {
			t.Errorf("expected count>9 to match numerically for count=10; PropertiesSet=%v", result.PropertiesSet)
		}
	})

	t.Run("without metamodel: string comparison (legacy) does not fire", func(t *testing.T) {
		// Documents the pre-fix behavior: "10" > "9" is false
		// lexicographically, so the condition wrongly fails. Engines
		// built without a metamodel keep this string-only behavior.
		engine := NewEngine([]Automation{auto})

		result := engine.Process(Event{Type: EventEntityCreated, Entity: ent})
		if result.PropertiesSet["status"] == "escalated" {
			t.Error("string-only engine unexpectedly compared numerically")
		}
	})
}

// TestEngine_Validation_IntegerComparisonIsNumeric pins the same fix on
// the validate: path. `validate: count<100` must pass numerically for
// count=10 (10 < 100), where lexicographically "10" < "100" also holds —
// so use a case where the two disagree: count=10 vs threshold 9.
func TestEngine_Validation_IntegerComparisonIsNumeric(t *testing.T) {
	// validate check "count<9" should FAIL for count=10 (10 is not < 9).
	// Lexicographically "10" < "9" is TRUE, so the buggy path would
	// consider the entity valid and emit no warning.
	auto := newAutomation().
		OnCreate("bug").
		ValidateWarning("count<9", "count must be under 9").
		Build()

	ent := buildEntity(testutil.Entity("bug").With("count", 10))

	engine := NewEngineFromMetamodel(intPropMeta(), nil)
	engine.automations = []Automation{auto}

	result := engine.Process(Event{Type: EventEntityCreated, Entity: ent})
	if !result.HasWarnings() {
		t.Error("expected a warning: count=10 is not < 9 numerically")
	}
}

// TestEngine_WhenCondition_UnknownPropertyFallsBackToString pins that a
// `when:` on a property the metamodel doesn't declare still evaluates
// (via string matching) rather than being silently dropped.
func TestEngine_WhenCondition_UnknownPropertyFallsBackToString(t *testing.T) {
	auto := newAutomation().
		OnCreate("bug").
		When("adhoc=yes").
		Set("status", "tagged").
		Build()

	ent := buildEntity(testutil.Entity("bug").With("adhoc", "yes"))

	engine := NewEngineFromMetamodel(intPropMeta(), nil)
	engine.automations = []Automation{auto}

	result := engine.Process(Event{Type: EventEntityCreated, Entity: ent})
	if result.PropertiesSet["status"] != "tagged" {
		t.Errorf("undeclared property should still match via string fallback; PropertiesSet=%v", result.PropertiesSet)
	}
}
