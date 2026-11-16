package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// Pinned read counts for /api/v1/_page_status under [useFieldPolicy].
// Measured, not derived; neither moves with row count.
const (
	pageStatusFieldPolicyBudget        = 7
	pageStatusRelatedFieldPolicyBudget = 8
)

// installDoneWhenStep publishes a steps page with one step over all tickets
// that is judged by doneWhen.
func installDoneWhenStep(app *App, doneWhen string) {
	installPagesConfig(app, map[string]dataentryconfig.List{"tickets": {EntityType: "ticket"}},
		map[string]dataentryconfig.Page{"workshop": {
			PageHeader: dataentryconfig.PageHeader{Label: "Workshop"},
			Mode:       dataentryconfig.PageModeSteps,
			Tabs:       []dataentryconfig.PageTab{{ID: "tickets", Label: "Tickets", List: "tickets", DoneWhen: doneWhen}},
		}}, nil)
}

// Each step is one gated read of its list and, for a done_when that
// traverses, one batch of related() answers: never a lookup per row.
func TestQueryBudget_PageStatusIsSizeIndependent(t *testing.T) {
	for _, tc := range []struct {
		name, doneWhen string
		pinned         int
	}{
		// Reads the status that only `lead` may see, so every row is redacted.
		{"property", "entity.status == 'done'", pageStatusFieldPolicyBudget},
		{"related", "related(entity, 'assigned-to', { id = current_user.id })", pageStatusRelatedFieldPolicyBudget},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, detail := fieldPolicyReads(t,
				func(app *App) { installDoneWhenStep(app, tc.doneWhen) },
				func(app *App, ctx context.Context) {
					if _, ok := pageStatusAs(ctx, t, app, "workshop", "")["tickets"]; !ok {
						t.Fatal("the step was not judged, so this budget proves nothing")
					}
				})
			// TODO(TKT-D3RAB0E): calls only. Like the navigation status, the
			// step reads its list through configuredListRows, whose type scan
			// grows with the store.
			assertCallBudget(t, tc.name, b, tc.pinned, detail)
		})
	}
}
