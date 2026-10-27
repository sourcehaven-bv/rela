package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// listExportBudget pins the list export over the budget fixture's tickets
// list, with two outgoing relation columns and one incoming. Before
// TKT-VEIMO3 the export read each row's edges with its own queries: one per
// row and direction a column uses. The count is in store calls; on pgstore
// one call may page through several SQL statements.
const listExportBudget = 6

// The list export renders the WHOLE scoped set, so a per-row lookup here
// scales with the type, not with a page.
func TestQueryBudget_ListExportIsSizeIndependent(t *testing.T) {
	requireCp(t)
	small, large, detail := readsFor(t, listExportOp)
	assertBudget(t, "list export", small, large, listExportBudget, detail)
}

// featureCell matches a row whose Feature column resolved. The cell is
// followed by the Assignee cell, so "Feature 1" cannot match "Feature 10".
var featureCell = regexp.MustCompile(`(?m)^\| Ticket TKT-\d+ \| \w+ \| Feature \d \| Person \d \|`)

var listExportOp budgetOp = func(t *testing.T, app *App, d *acl.Declarative, ctx context.Context) {
	t.Helper()
	// budgetMeta and budgetConfig build fresh values per app, so changing
	// them here does not leak into another test.
	app.Meta().Transforms = map[string]metamodel.TransformDef{
		"copy": {From: "markdown", Command: []string{"cp", "{in}", "{out}"}, Produces: "text/plain"},
	}
	list := app.State().Cfg.Lists["tickets"]
	list.Columns = append(append([]dataentryconfig.ListColumn(nil), list.Columns...),
		dataentryconfig.ListColumn{Relation: "blocks", Direction: dataentryconfig.DirectionIncoming, Label: "Blocked by"})
	app.State().Cfg.Lists["tickets"] = list

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/_export?transform=copy&list=tickets", http.NoBody)
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.export.handleV1ExportList(rec, req, "ticket")
	if rec.Code != http.StatusOK {
		t.Fatalf("export: %d %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	// Guard against a vacuous pass: every row resolves its outgoing
	// columns, and the incoming column resolves on TKT-0001.
	resolved := len(featureCell.FindAllString(body, -1))
	rows := len(regexp.MustCompile(`(?m)^\| Ticket TKT-`).FindAllString(body, -1))
	if rows < 10 || resolved != rows {
		t.Fatalf("%d of %d rows resolved their relation columns, so this budget proves nothing:\n%s", resolved, rows, body)
	}
	if !regexp.MustCompile(`(?m)^\| Ticket TKT-0001 \|.*\| Ticket TKT-0002, `).MatchString(body) {
		t.Fatalf("the incoming column did not resolve on TKT-0001:\n%s", body)
	}
}
