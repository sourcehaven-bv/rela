package automation

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestWhenClauseCompatibility bounds the upgrade impact of making an
// unparseable `when:` clause fatal (it used to be silently skipped).
//
// The worry is that a project which loads today starts failing on
// upgrade. It cannot, unless its clause was already broken: filter.Parse
// rejects only an empty string, a missing operator, or an empty property
// name. The accepted set below is deliberately odd-looking — those forms
// parse, so they must keep loading.
//
// The accepted list is drawn from the shapes that appear in real
// schema.yaml files (all 70 distinct clauses in this repo's own
// schema.yaml parse), plus edge shapes that merely look malformed.
func TestWhenClauseCompatibility(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"taak": {Properties: map[string]metamodel.PropertyDef{
				"status": {Type: metamodel.PropertyTypeString},
			}},
		},
	}

	accepted := []string{
		"status=todo",     // the common form
		"status!=done",    // negation
		"status=",         // "is empty"
		"status!=",        // "is not empty"
		"a=b=c",           // value containing the operator
		"spaces in tag=x", // spaces in the property name
		"UPPER=x",         // case
		"dot.prop=x",      // dotted property
		"status = ready",  // padded operator
	}
	for _, clause := range accepted {
		t.Run("accepted/"+clause, func(t *testing.T) {
			_, err := NewEngineFromMetamodel(meta,
				[]metamodel.AutomationDef{condAutomation("", clause)})
			if err != nil {
				t.Errorf("REGRESSION: %q used to load and now fails: %v", clause, err)
			}
		})
	}

	// These were always broken — they parsed to nothing and the clause was
	// dropped, silently widening the automation. Now they fail the load.
	rejected := []string{
		"",                  // empty
		"   ",               // blank
		"novalueoroperator", // no operator
		"=novalue",          // no property
	}
	for _, clause := range rejected {
		t.Run("rejected/"+clause, func(t *testing.T) {
			_, err := NewEngineFromMetamodel(meta,
				[]metamodel.AutomationDef{condAutomation("", clause)})
			if err == nil {
				t.Errorf("%q should fail the load, not be silently dropped", clause)
			}
		})
	}
}
