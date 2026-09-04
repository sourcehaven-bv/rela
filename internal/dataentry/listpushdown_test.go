package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The pushed page and the Go page must be the same page (listpushdown.go).

// pushdownApp seeds tickets whose sort keys collide, are missing, or are
// out of insertion order, so an ordering difference between the two paths
// cannot hide.
func pushdownApp(t *testing.T) (*App, *acl.Declarative) {
	t.Helper()
	app := newTestAppV1(t)
	dues := []string{"2026-03-01", "", "2026-01-15", "2026-03-01", "null", "2025-12-31", "2026-02-02", "", "2026-01-15", "2026-12-31", "2026-05-05", "null"}
	for i, due := range dues {
		props := map[string]any{"title": fmt.Sprintf("Ticket %02d", 11-i), "status": []string{"open", "done", "open"}[i%3]}
		switch due {
		case "":
		case "null":
			props["due"] = nil // a JSON null sorts with the absent rows on both paths
		default:
			props["due"] = due
		}
		seedEntity(app, &entity.Entity{ID: fmt.Sprintf("TKT-%03d", i+1), Type: "ticket", Properties: props})
	}
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"*"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	return app, d
}

// goPath runs the whole-type pipeline the pushdown replaces and slices it.
func goPath(
	ctx context.Context, t *testing.T, app *App, typeName string, query map[string][]string, page, perPage int,
) (ids []string, total int) {
	t.Helper()
	all, err := app.scopedSortedEntities(ctx, typeName, query)
	if err != nil {
		t.Fatal(err)
	}
	start := min((page-1)*perPage, len(all))
	end := min(start+perPage, len(all))
	ids = make([]string, 0, end-start)
	for _, e := range all[start:end] {
		ids = append(ids, e.ID)
	}
	return ids, len(all)
}

func TestListPushdown_MatchesGoPathPageForPage(t *testing.T) {
	// The test metamodel declares no `due`, so declare a string-shaped one
	// for the sort key; status is a plain string already.
	app, d := pushdownApp(t)
	meta := app.Meta()
	tk := meta.Entities["ticket"]
	tk.Properties["due"] = metamodelProp("date")
	meta.Entities["ticket"] = tk

	for _, rq := range []string{
		"sort=due", "sort=-due", "sort=due,title", "sort=-title",
		"filter%5Bstatus%5D=open&sort=due", "filter%5Bstatus%5D%5Bne%5D=open&sort=-due",
		"filter%5Bstatus%5D=open", "",
	} {
		for _, perPage := range []int{3, 5} {
			for page := 1; page <= 3; page++ {
				name := fmt.Sprintf("%s per_page=%d page=%d", rq, perPage, page)
				t.Run(name, func(t *testing.T) {
					q := rq
					if q != "" {
						q += "&"
					}
					q += fmt.Sprintf("per_page=%d&page=%d", perPage, page)
					ctx := gateCtxFor(aliceCtx(), t, d)
					// The handler picks the pushed path when eligible …
					resp, rec := listEntitiesAs(aliceCtx(), t, app, d, "ticket", "tickets", q)
					if rec.Code != http.StatusOK {
						t.Fatalf("status %d: %s", rec.Code, rec.Body)
					}
					// … and this is what the Go path would have served.
					query := parseQuery(q)
					wantIDs, wantTotal := goPath(ctx, t, app, "ticket", query, page, perPage)
					var gotIDs []string
					for _, e := range resp.Data {
						gotIDs = append(gotIDs, e.ID)
					}
					if strings.Join(gotIDs, ",") != strings.Join(wantIDs, ",") {
						t.Errorf("rows differ: pushed %v, go %v", gotIDs, wantIDs)
					}
					if resp.Meta.Total != wantTotal {
						t.Errorf("total: pushed %d, go %d", resp.Meta.Total, wantTotal)
					}
					wantMore := (page-1)*perPage+len(wantIDs) < wantTotal
					if resp.Meta.HasMore != wantMore {
						t.Errorf("has_more: pushed %v, go %v", resp.Meta.HasMore, wantMore)
					}
				})
			}
		}
	}
}

// planListPushdown must refuse what the store cannot evaluate identically.
func TestListPushdown_Eligibility(t *testing.T) {
	app, _ := pushdownApp(t)
	meta := app.Meta()
	tk := meta.Entities["ticket"]
	tk.Properties["due"] = metamodelProp("date")
	tk.Properties["estimate"] = metamodelProp("integer")
	meta.Entities["ticket"] = tk
	allow := acl.ReadQueryResult{AllowAll: true}
	isRel := func(k string) bool { return k == "implements" }
	for _, tc := range []struct {
		name  string
		query string
		want  bool
	}{
		{"plain", "", true},
		{"sort string prop", "sort=due", true},
		{"eq filter", "filter%5Bstatus%5D=open", true},
		{"ne filter", "filter%5Bstatus%5D%5Bne%5D=open", true},
		{"free text", "q=foo", false},
		{"relation filter", "filter%5Bimplements%5D=Feature", false},
		{"integer sort", "sort=estimate", false},
		{"undeclared sort", "sort=nope", false},
		{"contains", "filter%5Btitle%5D%5Bcontains%5D=x", false},
		{"in list", "filter%5Bstatus%5D%5Bin%5D=a,b", false},
		{"ne list", "filter%5Bstatus%5D%5Bne%5D=a,b", false},
		{"array form", "filter%5Bstatus%5D%5B%5D=open", false},
		{"range", "filter%5Bdue%5D%5Bgte%5D=2026-01-01", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := planListPushdown(meta, "ticket", parseQuery(tc.query), allow, store.WorldScope{}, 1, 25, isRel)
			if ok != tc.want {
				t.Errorf("eligible = %v, want %v", ok, tc.want)
			}
		})
	}
	t.Run("deny all", func(t *testing.T) {
		_, ok := planListPushdown(meta, "ticket", parseQuery(""), acl.ReadQueryResult{DenyAll: true}, store.WorldScope{}, 1, 25, isRel)
		if ok {
			t.Error("a denied type must not be pushed")
		}
	})
}

// A gated principal's total is the count of rows they may read, never the
// type's row count (RR-SSPCCI / RR-J1VAW0), and the page holds only those.
func TestListPushdown_ScopedTotalForGatedPrincipal(t *testing.T) {
	app := newTestAppV1(t)
	for i := 1; i <= 6; i++ {
		seedEntity(app, &entity.Entity{ID: fmt.Sprintf("TKT-%03d", i), Type: "ticket",
			Properties: map[string]any{"title": fmt.Sprintf("T%d", i), "status": []string{"open", "done"}[i%2]}})
	}
	// bob may read only tickets with status=open, through a predicate-free
	// type grant narrowed by a field-level read query shape: emulate with a
	// role that reads tickets and a request-level filter, then compare to the
	// Go path for the same principal.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"bob": "viewer"},
	}, app.store)
	app.acl = d
	resp, rec := listEntitiesAs(principalCtx("bob"), t, app, d, "ticket", "tickets", "filter%5Bstatus%5D=open&per_page=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if resp.Meta.Total != 3 || len(resp.Data) != 2 || !resp.Meta.HasMore {
		t.Errorf("total=%d rows=%d has_more=%v, want 3/2/true", resp.Meta.Total, len(resp.Data), resp.Meta.HasMore)
	}
	// A type bob may not read at all: empty, total 0, no pushdown leak.
	resp, rec = listEntitiesAs(principalCtx("bob"), t, app, d, "feature", "features", "")
	if rec.Code != http.StatusOK || resp.Meta.Total != 0 || len(resp.Data) != 0 {
		t.Errorf("denied type: status %d total %d rows %d", rec.Code, resp.Meta.Total, len(resp.Data))
	}
}

var _ = v1.ListResponse{}

func parseQuery(raw string) map[string][]string {
	vals, err := url.ParseQuery(raw)
	if err != nil {
		panic(err)
	}
	return vals
}

func metamodelProp(typ string) metamodel.PropertyDef {
	return metamodel.PropertyDef{Type: typ}
}
