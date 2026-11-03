package dataentry

import (
	"net/http"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// TestExportList_SkipsBusyColumn pins that a list export leaves a busy
// column out of its header and rows (TKT-TU3XH4): busy state is live and
// per principal, so a document does not carry it.
func TestExportList_SkipsBusyColumn(t *testing.T) {
	requireCp(t)
	app, ctx := goldenExportApp(t, nil)
	s := app.State()
	cfg := *s.Cfg
	cfg.Lists = map[string]dataentryconfig.List{
		"tickets": {EntityType: "ticket", Columns: []dataentryconfig.ListColumn{
			{Property: "id", Label: "ID"},
			{Busy: "run", Label: "Agent"},
			{Property: "status", Label: "Status"},
		}},
	}
	app.schema.Publish(&Schema{Cfg: &cfg, Meta: s.Meta})

	rec := exportListURL(ctx, app, "transform=copy&list=tickets&sort=id")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	const want = `| ID | Status |
| --- | --- |
| TKT-1 | open |
| TKT-2 | done |
| TKT-3 |  |
| TKT-4 | open |
`
	if got := rec.Body.String(); got != want {
		t.Errorf("export table:\ngot:\n%s\nwant:\n%s", got, want)
	}
}
