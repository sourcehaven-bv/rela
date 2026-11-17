package triggers

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// buildEntity returns the built entity from a testutil EntityBuilder.
func buildEntity(b *testutil.EntityBuilder) *entity.Entity {
	return b.Build()
}

// buildRelation returns the built relation from a testutil RelationBuilder.
func buildRelation(b *testutil.RelationBuilder) *entity.Relation {
	return b.Build()
}

// useAutomations replaces the engine's automations and compiles them again,
// for tests that build the engine from a metamodel first.
func useAutomations(e *Engine, autos []Rule) {
	e.automations = autos
	e.compile()
}

// mustFilters parses each `where:` clause, failing the test on a bad one.
func mustFilters(t *testing.T, clauses ...string) []*filter.Filter {
	t.Helper()
	out := make([]*filter.Filter, 0, len(clauses))
	for _, c := range clauses {
		f, err := filter.Parse(c)
		if err != nil {
			t.Fatalf("parse where clause %q: %v", c, err)
		}
		out = append(out, f)
	}
	return out
}

// mustParse parses a flows.yaml document against meta, failing the test
// when it does not load.
func mustParse(t *testing.T, meta *metamodel.Metamodel, flows string) []Rule {
	t.Helper()
	rules, err := Parse([]byte(flows), meta)
	if err != nil {
		t.Fatalf("parse flows.yaml: %v", err)
	}
	return rules
}

// mustMeta parses a schema.yaml document, failing the test when it does
// not load.
func mustMeta(t *testing.T, schema string) *metamodel.Metamodel {
	t.Helper()
	meta, err := metamodel.Parse([]byte(schema))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return meta
}
