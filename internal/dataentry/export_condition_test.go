package dataentry

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Exporting a list must render the rows that list SHOWS.
//
// handleV1ExportList reads through scopedSortedEntities, which applies the
// ACL, the filters, the sort and the query scope — but the view `condition:`
// is applied in listPage, one layer above. So an export of a conditioned list
// emitted rows the list itself excludes, and the export handler's own comment
// ("Reuses the exact read path the list view uses, so export can't widen past
// the view") was not true of the condition.
//
// This is a disclosure of the ordinary kind rather than an ACL one: the rows
// are ones the principal may read, so nothing leaks past the gate. What breaks
// is the promise the feature makes — the export is supposed to be the list.
func TestExport_List_AppliesTheViewCondition(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	app.Cfg().Lists["tickets"] = dataentryconfig.List{
		EntityType: "ticket",
		Condition:  "entity.status ~= 'closed'",
		Columns:    []dataentryconfig.ListColumn{{Property: "title"}},
	}
	if err := app.SetViewConditions(
		AdaptViewConditions(appbuild.ViewConditions)); err != nil {
		t.Fatalf("wire view conditions: %v", err)
	}

	for _, tc := range []struct{ id, title, status string }{
		{"TKT-001", "Open work", "open"},
		{"TKT-002", "Finished work", "closed"},
	} {
		seedEntity(app, &entity.Entity{ID: tc.id, Type: "ticket",
			Properties: map[string]any{"title": tc.title, "status": tc.status}})
	}

	rec := exportList(t.Context(), app)
	if rec.Code != 200 {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Open work") {
		t.Errorf("export dropped a row the list shows:\n%s", body)
	}
	if strings.Contains(body, "Finished work") {
		t.Errorf("export included a row the list's condition excludes:\n%s", body)
	}
}

// A condition naming `current_user` needs a query identity on ctx, and the
// export path resolves no scope of its own — so the binding the list path does
// is not reached here. Without it this 500s instead of exporting.
func TestExport_List_ConditionWithCurrentUser(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	app.Cfg().Lists["tickets"] = dataentryconfig.List{
		EntityType: "ticket",
		Condition:  "is_current_user(entity.status)",
		Columns:    []dataentryconfig.ListColumn{{Property: "title"}},
	}
	if err := app.SetViewConditions(
		AdaptViewConditions(appbuild.ViewConditions)); err != nil {
		t.Fatalf("wire view conditions: %v", err)
	}
	if err := app.SetQueryScopeResolver(AdaptQueryScopes(appbuild.QueryScopes)); err != nil {
		t.Fatalf("wire query scopes: %v", err)
	}

	// `status` stands in for an assignee here: newExportApp's ticket type
	// declares it, and what matters is that the value is compared against the
	// caller's identity.
	for _, tc := range []struct{ id, title, who string }{
		{"TKT-001", "Mine", "alice"},
		{"TKT-002", "Theirs", "bob"},
	} {
		seedEntity(app, &entity.Entity{ID: tc.id, Type: "ticket",
			Properties: map[string]any{"title": tc.title, "status": tc.who}})
	}

	rec := exportList(principal.With(t.Context(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry}), app)
	if rec.Code != 200 {
		t.Fatalf("a current_user condition must export, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Mine") || strings.Contains(body, "Theirs") {
		t.Errorf("export must render alice's row only:\n%s", body)
	}
}
