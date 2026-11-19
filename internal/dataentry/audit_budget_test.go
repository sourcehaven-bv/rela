package dataentry

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// changesBudget is the pinned store-read budget of one change-list page:
// the subject's gated read, one readability batch for the ids the page's
// records name, and the ACL membership walk.
const changesBudget = 4

// The change list reads the same number of times whatever the page holds:
// the ids its records name are checked in one batch per source read, never
// one read per record.
func TestQueryBudget_Changes(t *testing.T) {
	measure := func(n, pad int) (storetest.Cost, string) {
		app, counting, d, ctx := newBudgetAppPadded(t, n, pad, memstore.New(), budgetMeta())
		log := &fakeAuditLog{}
		for i := n; i >= 2; i-- {
			log.entries = append(log.entries,
				relationEntry(int64(i), "TKT-0001", fmt.Sprintf("TKT-%04d", i)))
		}
		log.entries = append(log.entries, entityEntry(1, audit.OpCreateEntity, "ticket", "TKT-0001"))
		if err := SetAuditLog(app, log); err != nil {
			t.Fatal(err)
		}
		counting.Reset()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/_changes/ticket/TKT-0001?limit=200", http.NoBody)
		req = req.WithContext(gateCtxFor(ctx, t, d))
		rec := httptest.NewRecorder()
		handleV1Changes(app, rec, req)
		if rec.Code != http.StatusOK || !bodyMentions(rec, fmt.Sprintf("TKT-%04d", n)) {
			t.Fatalf("changes: %d %s", rec.Code, rec.Body)
		}
		return counting.Cost(), counting.String()
	}
	b, detail := measureBudget(measure)
	assertBudget(t, "GET /_changes", b, changesBudget, detail)
}
