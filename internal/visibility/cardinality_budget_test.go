package visibility_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/schema"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// TestScriptReader_CardinalityReadBudget pins TKT-5LW875 on a gated reader:
// the edge gate judges a relation type's endpoints in one batch, so the
// store reads of a cardinality check do not grow with the number of
// subjects, and neither does the hidden neighbor count toward the result.
func TestScriptReader_CardinalityReadBudget(t *testing.T) {
	one := 1
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket":  {Label: "Ticket", IDPrefixes: []string{"TKT-"}},
			"concept": {Label: "Concept", IDPrefixes: []string{"CON-"}},
		},
		Relations: map[string]metamodel.RelationDef{
			"affects": {From: []string{"ticket"}, To: []string{"concept"}, MinOutgoing: &one},
		},
	}
	run := func(n int) (reads, violations int) {
		ctx := context.Background()
		counting := storetest.NewCounting(memstore.New())
		for i := range n {
			tkt, con := fmt.Sprintf("TKT-%03d", i), fmt.Sprintf("CON-%03d", i)
			for _, e := range []*entity.Entity{{ID: tkt, Type: "ticket"}, {ID: con, Type: "concept"}} {
				if err := counting.CreateEntity(ctx, e); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := counting.CreateRelation(ctx, entity.RelationKey{From: tkt, Type: "affects", To: con}, nil); err != nil {
				t.Fatal(err)
			}
		}
		// Every ticket affects a concept, but CON-000 is hidden, so
		// TKT-000 counts no edge.
		gate := resolverGate{deny: map[string]bool{"CON-000": true}}
		pr, err := visibility.NewPolicyReader(gate, visibility.NopRedactor{}, counting)
		if err != nil {
			t.Fatal(err)
		}
		sr, err := visibility.NewScriptReader(pr, counting, nil)
		if err != nil {
			t.Fatal(err)
		}
		counting.Reset()
		found, err := schema.CheckCardinality(ctx, sr, meta, nil)
		if err != nil {
			t.Fatal(err)
		}
		return counting.Reads(), len(found)
	}

	small, smallV := run(10)
	large, largeV := run(50)
	if small != large {
		t.Errorf("gated reads grow with subjects: %d at 10, %d at 50", small, large)
	}
	if smallV != 1 || largeV != 1 {
		t.Errorf("violations = %d at 10, %d at 50; want 1 (TKT-000, whose only neighbor is hidden)", smallV, largeV)
	}
}
