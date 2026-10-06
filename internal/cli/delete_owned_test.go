package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const ownedCLIMetaYAML = `version: "1.0"
entities:
  plan:
    label: Plan
    id_prefix: "PLAN-"
    properties:
      title: {type: string}
  step:
    label: Step
    id_prefix: "STEP-"
    properties:
      title: {type: string}
relations:
  has_step:
    from: [plan]
    to: [step]
    owning: true
`

// Deleting an owner says how many owned entities went with it (TKT-QO14GB).
func TestDeleteCmd_ReportsOwnedEntities(t *testing.T) {
	meta, err := metamodel.Parse([]byte(ownedCLIMetaYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	b, err := newCLIBundles(appbuildtest.New(meta))
	if err != nil {
		t.Fatalf("build cli services: %v", err)
	}
	ctx := context.Background()
	st := b.write.Store
	for _, e := range []*entity.Entity{
		{ID: "PLAN-1", Type: "plan", Properties: map[string]any{"title": "p"}},
		{ID: "STEP-1", Type: "step", Properties: map[string]any{"title": "a"}},
		{ID: "STEP-2", Type: "step", Properties: map[string]any{"title": "b"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	for _, step := range []string{"STEP-1", "STEP-2"} {
		k := entity.RelationKey{From: "PLAN-1", Type: "has_step", To: step}
		if _, err := st.CreateRelation(ctx, k, &store.RelationData{}); err != nil {
			t.Fatalf("seed edge: %v", err)
		}
	}
	if n, err := countOwned(ctx, st, meta, "PLAN-1"); err != nil || n != 2 {
		t.Fatalf("countOwned = %d, %v; want 2", n, err)
	}

	buf := withOutput(t, output.FormatTable)
	if err := (&DeleteCmd{ID: "PLAN-1", Force: true, Cascade: true}).Run(ctx, b.write); err != nil {
		t.Fatalf("delete PLAN-1: %v", err)
	}
	if !strings.Contains(buf.String(), "Also deleted 2 owned entit(ies)") {
		t.Fatalf("output = %q, want the owned count", buf.String())
	}
}
