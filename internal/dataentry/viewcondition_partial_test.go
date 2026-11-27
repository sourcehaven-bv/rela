package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// The surfaces that judge their own condition per item each fail closed for
// a condition that does not compile, while a sibling with a good condition
// keeps evaluating. Before, any broken condition disabled them all.

const brokenCondition = "entity.no_such_property == 'x'"

func TestPartialConditions_ListGroupLeavesOnlyItsListUngrouped(t *testing.T) {
	broken := dataentryconfig.List{EntityType: "ticket", GroupBy: &dataentryconfig.ListGroupBy{
		Groups: []dataentryconfig.ListGroup{
			{ID: "open", Condition: brokenCondition},
			{ID: "rest"},
		},
	}}
	app, _, _, ctx := newBudgetAppOn(t, 10, memstore.New())
	app.Cfg().Lists["grouped"] = groupedList
	app.Cfg().Lists["broken"] = broken
	wireRealViewConditions(t, app)
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"reader": {Read: []string{"*"}}},
		Assignments: map[string]string{"T1": "reader"},
	}, app.store)
	setTestACL(t, app, d)
	list := func(query string) (v1.ListResponse, int) {
		resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", query)
		return resp, rec.Code
	}

	resp, code := list("per_page=100&list_id=broken")
	if code != http.StatusOK || len(resp.Data) != 10 {
		t.Fatalf("broken list: %d, %d rows", code, len(resp.Data))
	}
	for _, row := range resp.Data {
		if row.Group != "" {
			t.Errorf("%s: _group = %q on a list whose group does not compile", row.ID, row.Group)
		}
	}

	resp, code = list("per_page=100&list_id=grouped")
	if code != http.StatusOK {
		t.Fatalf("good list: %d", code)
	}
	for _, row := range resp.Data {
		if row.Group == "" {
			t.Errorf("%s: no _group on the good list", row.ID)
		}
	}
}

func TestPartialConditions_TraverseStepFailsOnlyItsView(t *testing.T) {
	good := partsView("", "entity.type == 'usecase'")
	broken := partsView("", "entity.nope == 'x'")
	app := conditionViewApp(t, good)
	s := *app.State()
	cfg := *s.Cfg
	cfg.Views = map[string]ViewConfig{"good_view": good, "broken_view": broken}
	s.Cfg = &cfg
	app.schema.Publish(&s)

	result, err := app.views.executeView(context.Background(), good, "BOUWBLOK-001", defaultViewWorld())
	if err != nil {
		t.Fatalf("good view: %v", err)
	}
	if got := collectIDs(result.Collections["parts"]); !slices.Equal(got, []string{"UC-001"}) {
		t.Errorf("good view parts = %v, want [UC-001]", got)
	}
	if _, err := app.views.executeView(context.Background(), broken, "BOUWBLOK-001", defaultViewWorld()); err == nil {
		t.Fatal("broken view: want an error, got a result")
	}
}

func TestPartialConditions_DetailActionWhenHidesOnlyItsAction(t *testing.T) {
	bad := regenerateSoA()
	bad.When = "entity.no_such_property == 'soa'"
	d := newDetailActionApp(t, map[string]dataentryconfig.Action{
		"regenerate-soa": regenerateSoA(),
		"broken":         bad,
	})

	rec := getEntityAs(principalCtx("alice"), t, d.app, d.decl, "document", "documents", "DOC-1@concept", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET: %d %s", rec.Code, rec.Body)
	}
	var got v1.Entity
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Actions[detailActionKeyPrefix+"regenerate-soa"] {
		t.Error("the action with a good when is not offered")
	}
	if got.Actions[detailActionKeyPrefix+"broken"] {
		t.Error("the action whose when does not compile is offered")
	}
	rec = postAction(aliceCtx(), t, d.app, d.decl, "broken", `{"entity_id":"DOC-1@concept"}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("POST broken: %d, want 403: %s", rec.Code, rec.Body)
	}
}

func TestPartialConditions_LLMCandidatesFailOnlyTheirAction(t *testing.T) {
	ents, edges := llmGraphSeed()
	good := dataentryconfig.LLMRelateOutput{Relation: "touches", Candidates: "entity.area == 'auth'"}
	a := buildLLMApp(t, llmFixture{
		meta: llmGraphMeta(t), seed: ents, edges: edges, read: []string{"ticket", "concept", "component"},
		action: llmAction(nil, []string{"priority"}, []dataentryconfig.LLMRelateOutput{good}),
		reply:  `{"set":{"priority":null},"relate":{"touches":[]}}`,
	})
	broken := llmAction(nil, []string{"priority"}, []dataentryconfig.LLMRelateOutput{
		{Relation: "touches", Candidates: brokenCondition},
	})
	s := *a.app.State()
	s.Cfg = &Config{Actions: map[string]dataentryconfig.Action{"triage": s.Cfg.Actions["triage"], "broken": broken}}
	a.app.schema.Publish(&s)

	rec := postAction(principalCtx("alice"), t, a.app, a.decl, "broken", `{"entity_id":"TKT-1"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("broken action: %d, want 500: %s", rec.Code, rec.Body)
	}
	if n := a.chat.calls(); n != 0 {
		t.Fatalf("provider called %d times for an action whose candidates do not compile", n)
	}

	code, _, body := a.run(t, "alice")
	if code != http.StatusOK {
		t.Fatalf("good action: %d %s", code, body)
	}
	if got := itemIDs(a.chat.doc(t).Candidates["touches"].Entities); len(got) == 0 {
		t.Error("the good action offered no candidates")
	}
}

// With no condition compiler wired, a list that declares a condition fails
// its export instead of exporting every row.
func TestExport_List_UnwiredCompilerFails(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	app.Cfg().Lists["tickets"] = dataentryconfig.List{
		EntityType: "ticket",
		Condition:  "entity.status ~= 'closed'",
		Columns:    []dataentryconfig.ListColumn{{Property: "title"}},
	}
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "Finished work", "status": "closed"}})

	rec := exportList(t.Context(), app)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body)
	}
}
