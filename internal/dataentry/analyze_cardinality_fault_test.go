package dataentry

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// Every reader gatedScriptReader can return has the strict relation read, so
// lateGatedReader's missing-method fallback is unreachable in production.
var (
	_ interface {
		ListRelationsStrict(context.Context, store.RelationQuery) iter.Seq2[*entity.Relation, error]
	} = (*visibility.ScriptReader)(nil)
	_ interface {
		ListRelationsStrict(context.Context, store.RelationQuery) iter.Seq2[*entity.Relation, error]
	} = (*visibility.UnrestrictedReader)(nil)
	_ interface {
		ListRelationsStrict(context.Context, store.RelationQuery) iter.Seq2[*entity.Relation, error]
	} = visibility.DenyReader{}
)

// faultingRelationsReader fails the strict relation read, as a gate fault
// does.
type faultingRelationsReader struct{ analyzeReader }

func (faultingRelationsReader) ListRelationsStrict(
	context.Context, store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	return func(yield func(*entity.Relation, error) bool) { yield(nil, errors.New("gate down")) }
}

// TestAnalyzeCardinality_GateFaultReportsOneIssue pins TKT-5LW875: a gate
// fault reports that the check did not run, never partial findings and never
// the fault's text.
func TestAnalyzeCardinality_GateFaultReportsOneIssue(t *testing.T) {
	one := 1
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{"doc": {}},
		Relations: map[string]metamodel.RelationDef{
			"cites": {From: []string{"doc"}, To: []string{"doc"}, MinOutgoing: &one},
		},
	}
	g := newFixture()
	g.AddNode(&entity.Entity{ID: "D-1", Type: "doc", Properties: map[string]any{}})
	svc := newAnalyzeService(t, g, meta)
	svc.reads = faultingRelationsReader{svc.reads}

	section := svc.analyzeCardinality(context.Background(), meta)
	if len(section.Issues) != 1 {
		t.Fatalf("issues = %+v, want exactly one", section.Issues)
	}
	issue := section.Issues[0]
	if issue.EntityID != "" || issue.Message != "Cardinality could not be checked; see the server log" {
		t.Errorf("issue = %+v, want the generic failure issue", issue)
	}
}
