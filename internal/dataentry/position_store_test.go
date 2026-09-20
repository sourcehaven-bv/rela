package dataentry

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The store-answered position must equal what the Go pipeline says for the
// same scope: same index, same total, same neighbors, same 404 (TKT-U9DYW4).
func TestPosition_StoreMatchesGoPath(t *testing.T) {
	app, d := pushdownApp(t)
	meta := app.Meta()
	tk := meta.Entities["ticket"]
	tk.Properties["due"] = metamodelProp("date")
	meta.Entities["ticket"] = tk

	scopes := []ScopeDescriptor{
		{Source: "list", Type: "ticket"},
		{Source: "list", Type: "ticket", Sort: "due"},
		{Source: "list", Type: "ticket", Sort: "-due"},
		{Source: "list", Type: "ticket", Sort: "due,title"},
		{Source: "list", Type: "ticket", Sort: "-due", Filters: map[string]string{"filter[status]": "open"}},
		{Source: "list", Type: "ticket", Filters: map[string]string{"filter[status][ne]": "open"}},
	}
	for _, scope := range scopes {
		t.Run(fmt.Sprintf("%s %v", scope.Sort, scope.Filters), func(t *testing.T) {
			ctx := gateCtxFor(aliceCtx(), t, d)
			query := scope.toQuery()

			_, n, err := resolveListNarrowing(ctx, app, scope.Type, query)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := n.pushdownPlan(ctx, app, scope.Type, query, 1, 1); !ok {
				t.Fatal("scope is not pushable: the test would compare the Go path to itself")
			}

			want, total := goPath(ctx, t, app, scope.Type, query, 1, 1000)
			inScope := map[string]bool{}
			for i, id := range want {
				inScope[id] = true
				req := httptest.NewRequest(http.MethodGet, positionURL(t, id, scope), http.NoBody).WithContext(ctx)
				rec := httptest.NewRecorder()
				pos, handled := app.storePosition(rec, req, scope, id)
				if !handled || pos == nil {
					t.Fatalf("%s: store path declined or failed: %d %s", id, rec.Code, rec.Body)
				}
				if pos.Current != i+1 || pos.Total != total {
					t.Errorf("%s: store %d/%d, go %d/%d", id, pos.Current, pos.Total, i+1, total)
				}
				wantPrev, wantNext := "", ""
				if i > 0 {
					wantPrev = want[i-1]
				}
				if i < len(want)-1 {
					wantNext = want[i+1]
				}
				gotPrev, gotNext := "", ""
				if pos.Prev != nil {
					gotPrev = pos.Prev.ID
					if pos.Prev.Type != "ticket" {
						t.Errorf("%s: prev type %q", id, pos.Prev.Type)
					}
				}
				if pos.Next != nil {
					gotNext = pos.Next.ID
				}
				if gotPrev != wantPrev || gotNext != wantNext {
					t.Errorf("%s: neighbors store (%s,%s), go (%s,%s)", id, gotPrev, gotNext, wantPrev, wantNext)
				}
			}

			for _, id := range []string{"TKT-404", "TKT-002"} {
				if inScope[id] {
					continue
				}
				req := httptest.NewRequest(http.MethodGet, positionURL(t, id, scope), http.NoBody).WithContext(ctx)
				rec := httptest.NewRecorder()
				pos, handled := app.storePosition(rec, req, scope, id)
				if !handled || pos != nil || rec.Code != http.StatusNotFound {
					t.Errorf("%s: want a written 404, got handled=%v pos=%v code=%d", id, handled, pos, rec.Code)
				}
			}
		})
	}
}

// Shapes the store cannot answer identically must fall to the Go path.
func TestPosition_StoreDeclines(t *testing.T) {
	app, d := pushdownApp(t)
	ctx := gateCtxFor(aliceCtx(), t, d)
	for name, scope := range map[string]ScopeDescriptor{
		"search source": {Source: "search", Type: "ticket", Q: "type:ticket"},
		"free text":     {Source: "list", Type: "ticket", Q: "ticket"},
		"ordered op":    {Source: "list", Type: "ticket", Filters: map[string]string{"filter[title][gt]": "A"}},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/_position", http.NoBody).WithContext(ctx)
			rec := httptest.NewRecorder()
			if pos, handled := app.storePosition(rec, req, scope, "TKT-001"); handled || pos != nil {
				t.Errorf("store path took a scope it must decline")
			}
		})
	}
}
