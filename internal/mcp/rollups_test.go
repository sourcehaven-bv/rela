package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const rollupMCPMeta = `version: "1.0"
entities:
  risk:
    label: Risk
    id_prefix: "RIS-"
    properties:
      title: {type: string}
      open_plans:
        type: integer
        rollup: { via: treatedBy, count: true, where: { done: "false" } }
  plan:
    label: Plan
    id_prefix: "PLN-"
    properties:
      title: {type: string}
      done: {type: boolean}
relations:
  treats:
    from: [plan]
    to: [risk]
    inverse: treatedBy
`

// TestShowEntity_Rollups pins that show_entity serves a roll-up
// (TKT-6I2OPG) computed on read, and never a value stored under its name.
func TestShowEntity_Rollups(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	meta, err := metamodel.Parse([]byte(rollupMCPMeta))
	if err != nil {
		t.Fatal(err)
	}
	st := memstore.New()
	risk := entity.New("RIS-1", "risk")
	risk.Properties["title"] = "r"
	risk.Properties["open_plans"] = 99
	if err = st.CreateEntity(ctx, risk); err != nil {
		t.Fatal(err)
	}
	for id, done := range map[string]bool{"PLN-1": false, "PLN-2": false, "PLN-3": true} {
		p := entity.New(id, "plan")
		p.Properties["done"] = done
		if err = st.CreateEntity(ctx, p); err != nil {
			t.Fatal(err)
		}
		if _, err = st.CreateRelation(ctx, entity.RelationKey{From: id, Type: "treats", To: "RIS-1"}, nil); err != nil {
			t.Fatal(err)
		}
	}
	deps := newTestDeps(t, meta, st)
	if deps.Related.Rollups == nil {
		t.Fatal("the gated read bundle carries no roll-up reader")
	}
	srv := &Server{logger: slog.New(slog.DiscardHandler)}
	setDeps(srv, deps)

	result, err := srv.handleShowEntity(ctx, makeToolRequest(map[string]any{"id": "RIS-1"}))
	if err != nil {
		t.Fatal(err)
	}
	var got entityJSON
	if err = json.Unmarshal([]byte(getResultText(t, result)), &got); err != nil {
		t.Fatal(err)
	}
	if v := got.Properties["open_plans"]; v != float64(2) {
		t.Errorf("open_plans = %v, want 2 (the stored 99 is never served)", v)
	}
}
