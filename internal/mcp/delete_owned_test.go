package mcp

import (
	"context"
	"log/slog"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// delete_entity on an owner says how many owned entities went with it
// (TKT-QO14GB).
func TestHandleDeleteEntity_ReportsOwnedEntities(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  plan: {label: Plan, id_prefix: "PLAN-", properties: {title: {type: string}}}
  step: {label: Step, id_prefix: "STEP-", properties: {title: {type: string}}}
relations:
  has_step: {from: [plan], to: [step], owning: true}
`))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	st := memstore.New()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "PLAN-1", Type: "plan", Properties: map[string]any{"title": "p"}},
		{ID: "STEP-1", Type: "step", Properties: map[string]any{"title": "a"}},
		{ID: "STEP-2", Type: "step", Properties: map[string]any{"title": "b"}},
	} {
		seedEntity(t, st, e)
	}
	for _, step := range []string{"STEP-1", "STEP-2"} {
		k := entity.RelationKey{From: "PLAN-1", Type: "has_step", To: step}
		if _, cErr := st.CreateRelation(ctx, k, &store.RelationData{}); cErr != nil {
			t.Fatalf("seed edge: %v", cErr)
		}
	}
	s := &Server{logger: slog.New(slog.DiscardHandler)}
	setDeps(s, newTestDeps(t, meta, st))

	res, err := s.handleDeleteEntity(ctx, makeToolRequest(map[string]any{"id": "PLAN-1", "cascade": true}))
	if err != nil || isErrorResult(res) {
		t.Fatalf("delete PLAN-1: %v %s", err, getResultText(t, res))
	}
	if got, want := getResultText(t, res), "Deleted PLAN-1, the 2 entit(ies) it owns and 2 relation(s)"; got != want {
		t.Errorf("result = %q, want %q", got, want)
	}
}
