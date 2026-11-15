package triggers_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/flowcfg/triggers"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// loadProject parses a schema.yaml and a flows.yaml document, failing the
// test when either does not load.
func loadProject(t *testing.T, schema, flows string) (*metamodel.Metamodel, []triggers.Rule) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(schema))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	rules, err := triggers.Parse([]byte(flows), meta)
	if err != nil {
		t.Fatalf("parse flows.yaml: %v", err)
	}
	return meta, rules
}

// newEngine loads a project and builds its engine, failing the test when
// any step fails.
func newEngine(t *testing.T, schema, flows string) *triggers.Engine {
	t.Helper()
	meta, rules := loadProject(t, schema, flows)
	engine, err := triggers.New(meta, rules)
	if err != nil {
		t.Fatalf("build engine: %v", err)
	}
	return engine
}
