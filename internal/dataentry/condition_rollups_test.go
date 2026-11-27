package dataentry

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// withRollupListCondition adds the list "cond" over maatregel with the given
// `condition:` to app and wires the real view-condition compiler.
func withRollupListCondition(t *testing.T, app *App, condition string) {
	t.Helper()
	app.State().Cfg.Lists["cond"] = dataentryconfig.List{
		EntityType: "maatregel",
		Columns:    []dataentryconfig.ListColumn{{Property: "title"}},
		Condition:  condition,
	}
	wireRealViewConditions(t, app)
}

// TestListCondition_ReadsRollups pins that a list `condition:` judges a
// roll-up as the reader computes it (newRollupSortApp), after the gate:
//   - PERS-B cannot read BEW-4, so MAAT-2 counts two bewijs for it, not
//     three, and the list drops MAAT-2 for PERS-B only.
//   - PERS-B cannot see test.resultaat, so the latest pick is absent for it
//     and reads as unset.
//   - A value stored under a roll-up name (MAAT-3's 99) never matches.
//
// The total counts the rows the condition keeps, so it is computed after it.
func TestListCondition_ReadsRollups(t *testing.T) {
	tests := []struct {
		condition, user string
		want            []string
	}{
		{"entity.bewijs_aantal >= 3", "PERS-A", []string{"MAAT-2"}},
		{"entity.bewijs_aantal >= 3", "PERS-B", nil},
		{"entity.bewijs_aantal >= 2", "PERS-B", []string{"MAAT-2"}},
		{"entity.bewijs_aantal >= 1", "PERS-A", []string{"MAAT-1", "MAAT-2"}},
		{"entity.bewijs_aantal == 99", "PERS-A", nil},
		{"entity.laatste_testresultaat == 'gefaald'", "PERS-A", []string{"MAAT-1"}},
		{"entity.laatste_testresultaat == 'gefaald'", "PERS-B", nil},
		{"entity.laatste_testresultaat ~= 'gefaald'", "PERS-A", []string{"MAAT-2", "MAAT-3"}},
		{"entity.title ~= 'MAAT-3' and entity.bewijs_aantal == 0", "PERS-A", nil},
		{"entity.title == 'MAAT-3' and entity.bewijs_aantal == 0", "PERS-A", []string{"MAAT-3"}},
	}
	for _, tc := range tests {
		t.Run(tc.user+" "+tc.condition, func(t *testing.T) {
			app, d := newRollupSortApp(t)
			withRollupListCondition(t, app, tc.condition)
			resp, rec := listEntitiesAs(principalCtx(tc.user), t, app, d, "maatregel", "maatregels",
				"list_id=cond&sort=title")
			if rec.Code != http.StatusOK {
				t.Fatalf("list = %d %s", rec.Code, rec.Body)
			}
			var got []string
			for _, row := range resp.Data {
				got = append(got, row.ID)
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Errorf("rows = %v, want %v", got, tc.want)
			}
			if resp.Meta.Total != len(tc.want) {
				t.Errorf("total = %d, want %d", resp.Meta.Total, len(tc.want))
			}
		})
	}
}

// TestListCondition_RollupTotalIsPostCondition pins that a paged list over a
// roll-up condition counts and pages the kept rows only.
func TestListCondition_RollupTotalIsPostCondition(t *testing.T) {
	app, d := newRollupSortApp(t)
	withRollupListCondition(t, app, "entity.bewijs_aantal >= 1")
	resp, rec := listEntitiesAs(principalCtx("PERS-A"), t, app, d, "maatregel", "maatregels",
		"list_id=cond&sort=title&per_page=1&page=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "MAAT-2" {
		t.Errorf("page 2 = %v, want [MAAT-2]", resp.Data)
	}
	if resp.Meta.Total != 2 || resp.Meta.HasMore {
		t.Errorf("meta = %+v, want total 2 and no more", resp.Meta)
	}
}

// TestListCondition_RollupNeverPushedDown pins that a condition reading a
// roll-up is never lowered: the store holds no roll-up value.
func TestListCondition_RollupNeverPushedDown(t *testing.T) {
	app, _ := newRollupSortApp(t)
	withRollupListCondition(t, app, "entity.bewijs_aantal >= 1")
	m, err := viewCondition(app.viewConditions, app.State(), viewKindList, "cond")
	if err != nil || m == nil {
		t.Fatalf("viewCondition = %v, %v", m, err)
	}
	if _, ok := m.(interface{ Lower() }); ok {
		t.Error("a roll-up condition offers a store lowering")
	}
	if _, ok := m.(rollupConditionMatcher); !ok {
		t.Errorf("matcher = %T, want rollupConditionMatcher", m)
	}
}

// TestQueryBudget_ListConditionRollups pins the cost of a list condition
// that reads roll-ups. It is the Go path (TestNoNewUnpagedReads,
// `withQueriedRollups (condition roll-ups)`): store calls grow with the
// candidate population divided by rollupChunk, and rows read grow with the
// population. Within one chunk the calls are the same at 10 and 50 rows.
func TestQueryBudget_ListConditionRollups(t *testing.T) {
	measure := func(n int) storetest.Cost {
		counting := storetest.NewCounting(memstore.New())
		app, d := newRollupWireApp(t, n, counting)
		withRollupListCondition(t, app, "entity.bewijs_aantal >= 2")
		counting.Reset()
		resp, rec := listEntitiesAs(principalCtx("PERS-B"), t, app, d, "maatregel", "maatregels",
			"list_id=cond&per_page=100&rollups=false")
		if rec.Code != http.StatusOK || len(resp.Data) != n {
			t.Fatalf("list: %d, %d rows (%s)", rec.Code, len(resp.Data), rec.Body)
		}
		return counting.Cost()
	}
	small, large := measure(10), measure(50)
	if small.Calls != large.Calls {
		t.Errorf("calls grow within one chunk: %d at 10 rows, %d at 50", small.Calls, large.Calls)
	}
	if large.Rows <= small.Rows {
		t.Errorf("rows = %d at 50, %d at 10; the Go path reads the population", large.Rows, small.Rows)
	}
}

// TestRollupValuesChunked_CallsPerChunk pins that the roll-up statement is
// asked about at most rollupChunk sources at a time: one call per chunk.
func TestRollupValuesChunked_CallsPerChunk(t *testing.T) {
	app, d := newRollupWireApp(t, 0, memstore.New())
	ctx := gateCtxFor(principalCtx("PERS-A"), t, d)
	for _, n := range []int{1, rollupChunk, rollupChunk + 1, 2*rollupChunk + 1} {
		counting := storetest.NewCounting(app.store)
		src := newRollupSource(app)
		src.st = counting
		rows := make([]rollupRow, n)
		for i := range rows {
			rows[i] = rollupRow{
				ref:   entity.Ref{ID: fmt.Sprintf("MAAT-%d", i)},
				props: map[string]any{},
			}
		}
		src.add(ctx, "maatregel", rows, worldScopeFrom(ctx), func(p string) bool { return p == "bewijs_aantal" })
		chunks := (n + rollupChunk - 1) / rollupChunk
		perChunk := counting.Cost().Calls / chunks
		if perChunk*chunks != counting.Cost().Calls || perChunk == 0 {
			t.Errorf("n=%d: %d calls over %d chunks (%s)", n, counting.Cost().Calls, chunks, counting)
		}
	}
}
