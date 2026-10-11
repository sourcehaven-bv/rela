package dataentry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// pileBudgetOp is one piles request measured at two pile sizes. ids are
// the seeded tickets; pileID is a pile already holding all of them.
type pileBudgetOp func(t *testing.T, app *App, d *acl.Declarative, pileID string, ids []string)

// pileReads runs op over the budget fixture at 10 and 50 tickets, with a
// pile holding every ticket, and returns the store reads op made. The pile
// store is not the counted store, so only entity reads are measured.
func pileReads(t *testing.T, op pileBudgetOp) (small, large int, detail string) {
	t.Helper()
	counts := make([]int, 0, 2)
	for _, n := range []int{10, 50} {
		app, counting, d, ctx := newBudgetAppOn(t, n, memstore.New())
		app.State().Meta.Transforms = map[string]metamodel.TransformDef{
			"copy": {From: "markdown", Command: []string{"cp", "{in}", "{out}"}, Produces: "text/plain"},
		}
		svc := newPilesService(t)
		if err := app.SetPiles(svc, stubScriptPiles{}); err != nil {
			t.Fatal(err)
		}
		ids := make([]string, n)
		refs := make([]entity.Ref, n)
		for i := range ids {
			ids[i] = fmt.Sprintf("TKT-%04d", i+1)
			ref, err := entity.ParseRef(ids[i])
			if err != nil {
				t.Fatal(err)
			}
			refs[i] = ref
		}
		p, err := svc.Create(ctx, piles.CreateRequest{Name: "All", Refs: refs})
		if err != nil {
			t.Fatal(err)
		}
		counting.Reset()
		op(t, app, d, p.ID, ids)
		counts = append(counts, counting.Reads())
		detail = counting.String()
	}
	return counts[0], counts[1], detail
}

// budgetPileCall runs one piles request as the budget fixture's principal.
func budgetPileCall(t *testing.T, app *App, d *acl.Declarative, method, path, body string, want int) {
	t.Helper()
	rec := doPile(t, app, pileCall{user: "P1", d: d, method: method, path: path, body: body})
	if rec.Code != want {
		t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body)
	}
}

func itemsBody(ids []string) string {
	b, _ := json.Marshal(map[string][]string{"items": ids})
	return string(b)
}

// The piles routes read the same number of rows whatever the pile holds:
// one batched resolution per request, never a read per item.
func TestQueryBudget_Piles(t *testing.T) {
	cases := []struct {
		name   string
		pinned int
		op     pileBudgetOp
	}{
		{"GET /_piles", pilesListBudget, func(t *testing.T, app *App, d *acl.Declarative, _ string, _ []string) {
			t.Helper()
			budgetPileCall(t, app, d, http.MethodGet, pilesPath, "", http.StatusOK)
		}},
		{"GET /_piles/{id}", pilesGetBudget, func(t *testing.T, app *App, d *acl.Declarative, id string, _ []string) {
			t.Helper()
			budgetPileCall(t, app, d, http.MethodGet, pilesPath+"/"+id, "", http.StatusOK)
		}},
		{"POST /_piles", pilesCreateBudget, func(t *testing.T, app *App, d *acl.Declarative, _ string, ids []string) {
			t.Helper()
			body := `{"name":"Copy","items":` + strings.TrimPrefix(itemsBody(ids), `{"items":`)
			budgetPileCall(t, app, d, http.MethodPost, pilesPath, body, http.StatusCreated)
		}},
		{"POST /_piles/{id}/items", pilesAddBudget, func(t *testing.T, app *App, d *acl.Declarative, id string, ids []string) {
			t.Helper()
			budgetPileCall(t, app, d, http.MethodPost, pilesPath+"/"+id+"/items", itemsBody(ids), http.StatusOK)
		}},
		{"pile _position", pilesPositionBudget, func(t *testing.T, app *App, d *acl.Declarative, id string, ids []string) {
			t.Helper()
			rec := pilePosition(t, app, pileCall{user: "P1", d: d}, id, ids[len(ids)/2])
			if rec.Code != http.StatusOK {
				t.Fatalf("position: %d %s", rec.Code, rec.Body)
			}
		}},
		{"pile export", pilesExportBudget, func(t *testing.T, app *App, d *acl.Declarative, id string, _ []string) {
			t.Helper()
			requireCp(t)
			budgetPileCall(t, app, d, http.MethodGet, pilesPath+"/"+id+"/_export?transform=copy", "", http.StatusOK)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			small, large, detail := pileReads(t, tc.op)
			assertBudget(t, tc.name, small, large, tc.pinned, detail)
		})
	}
}

// Pinned store-read budgets for the piles routes: one header batch plus the
// two-query ACL membership walk. Create resolves its items and then reads
// the new pile back, so it makes two batches. None depends on the pile size.
const (
	pilesListBudget     = 3
	pilesGetBudget      = 3
	pilesCreateBudget   = 4
	pilesAddBudget      = 3
	pilesPositionBudget = 3
	pilesExportBudget   = 3
)
