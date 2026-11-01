package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search/searchparser"
)

// searchLimited performs GET /api/v1/_search with a `limit` parameter.
func searchLimited(
	ctx context.Context, t *testing.T, app *App, q, limit string,
) (v1.ListResponse, *httptest.ResponseRecorder) {
	t.Helper()
	params := url.Values{"q": {q}}
	if limit != "" {
		params.Set("limit", limit)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?"+params.Encode(), http.NoBody)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	app.handleV1Search(rec, req)

	var resp v1.ListResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode search response: %v\nbody: %s", err, rec.Body)
		}
	}
	return resp, rec
}

func limitedIDs(resp v1.ListResponse) []string {
	out := make([]string, 0, len(resp.Data))
	for _, e := range resp.Data {
		out = append(out, e.ID)
	}
	return out
}

// TestSearchLimit_RecentlyModified pins the mention menu's "recently modified"
// query: sorted newest first, then cut to `limit`.
func TestSearchLimit_RecentlyModified(t *testing.T) {
	app := newTestAppV1(t)
	for _, id := range []string{"TKT-001", "TKT-002", "TKT-003"} {
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": id}})
	}
	// Touch the oldest so it becomes the newest.
	if err := app.store.UpdateEntity(context.Background(), &entity.Entity{
		ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "touched"},
	}); err != nil {
		t.Fatal(err)
	}

	resp, rec := searchLimited(context.Background(), t, app, "type:ticket sort:modified:desc", "2")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /_search: %d %s", rec.Code, rec.Body)
	}
	got := limitedIDs(resp)
	if len(got) != 2 || got[0] != "TKT-001" || got[1] != "TKT-003" {
		t.Errorf("got %v, want [TKT-001 TKT-003]", got)
	}
	if resp.Meta.Total != 2 {
		t.Errorf("meta.total = %d, want 2", resp.Meta.Total)
	}
}

// TestSearchLimit_CountsOnlyVisibleRows pins that the cut happens after the
// read gate: newer hidden rows must not take the visible rows' slots.
func TestSearchLimit_CountsOnlyVisibleRows(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "a"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "b"}})
	for _, id := range []string{"FEAT-001", "FEAT-002", "FEAT-003"} {
		seedEntity(app, &entity.Entity{ID: id, Type: "feature", Properties: map[string]any{"title": id}})
	}

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)

	ctx := gateCtxFor(aliceCtx(), t, d)
	resp, rec := searchLimited(ctx, t, app, "type:ticket,feature sort:modified:desc", "2")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /_search: %d %s", rec.Code, rec.Body)
	}
	got := limitedIDs(resp)
	if len(got) != 2 || got[0] != "TKT-002" || got[1] != "TKT-001" {
		t.Errorf("got %v, want the two visible tickets [TKT-002 TKT-001]", got)
	}
}

func TestSearchLimit_Invalid(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "a"}})

	for _, q := range []string{"type:ticket", ""} {
		for _, limit := range []string{"0", "-1", "abc", "101"} {
			t.Run(q+"/"+limit, func(t *testing.T) {
				_, rec := searchLimited(context.Background(), t, app, q, limit)
				if rec.Code != http.StatusBadRequest {
					t.Errorf("q=%q limit=%s: got %d, want 400", q, limit, rec.Code)
				}
			})
		}
	}
}

// TestSearchTypeParam_ScopesFreeText pins that `type` scopes a free-text
// search, and that an undeclared type matches nothing rather than widening.
func TestSearchTypeParam_ScopesFreeText(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "alpha"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "alpha"}})

	for _, tc := range []struct {
		name, typ string
		want      []string
	}{
		{"declared type", "ticket", []string{"TKT-001"}},
		{"undeclared type", "ticket sort:title", []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := url.Values{"q": {"alpha"}, "type": {tc.typ}}
			req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?"+params.Encode(), http.NoBody)
			rec := httptest.NewRecorder()
			app.handleV1Search(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET /_search: %d %s", rec.Code, rec.Body)
			}
			var resp v1.ListResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}
			if got := limitedIDs(resp); !slices.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSearchLimit_FreeTextTypeClauseKeepsTheParamType pins the type rule of
// the limit pushdown (TKT-T2I7P2): a `type:` clause in q unions with the
// `type` parameter, and the parameter's exact filter runs after the search.
// Lowering the relevance cap to `limit` there would spend the slots on the
// clause's type and return no row of the parameter's type. The features
// sort first (id ties), so a pushed cap of 2 would hold only features.
func TestSearchLimit_FreeTextTypeClauseKeepsTheParamType(t *testing.T) {
	app := newTestAppV1(t)
	for _, id := range []string{"FEAT-001", "FEAT-002", "FEAT-003"} {
		seedEntity(app, &entity.Entity{ID: id, Type: "feature", Properties: map[string]any{"title": "alpha"}})
	}
	for _, id := range []string{"TKT-001", "TKT-002", "TKT-003"} {
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": "alpha"}})
	}

	params := url.Values{"q": {"type:feature alpha"}, "type": {"ticket"}, "limit": {"2"}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?"+params.Encode(), http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Search(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /_search: %d %s", rec.Code, rec.Body)
	}
	var resp v1.ListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if got := limitedIDs(resp); !slices.Equal(got, []string{"TKT-001", "TKT-002"}) {
		t.Errorf("got %v, want [TKT-001 TKT-002]", got)
	}
}

// TestSearchLimit_FreeTextSortSeesEveryHit pins the sort rule of the limit
// pushdown: `sort:` reorders the capped set, so the cap must stay at its
// full size. The search ranks TKT-001 ("alpha c") last, so a cap of 1 would
// sort a set without it.
func TestSearchLimit_FreeTextSortSeesEveryHit(t *testing.T) {
	app := newTestAppV1(t)
	for _, e := range []struct{ id, title string }{{"TKT-001", "alpha c"}, {"TKT-002", "alpha b"}, {"TKT-003", "alpha a"}} {
		seedEntity(app, &entity.Entity{ID: e.id, Type: "ticket", Properties: map[string]any{"title": e.title}})
	}
	ranked, _ := searchLimited(context.Background(), t, app, "alpha", "1")
	if got := limitedIDs(ranked); slices.Equal(got, []string{"TKT-001"}) {
		t.Fatalf("fixture: the search must not rank TKT-001 first, got %v", got)
	}

	resp, rec := searchLimited(context.Background(), t, app, "alpha sort:title:desc", "1")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /_search: %d %s", rec.Code, rec.Body)
	}
	if got := limitedIDs(resp); !slices.Equal(got, []string{"TKT-001"}) {
		t.Errorf("got %v, want [TKT-001]", got)
	}
}

func TestFreeTextLimit(t *testing.T) {
	for _, tc := range []struct {
		name  string
		query string
		opts  queryOptions
		want  int
	}{
		{"no limit", "alpha", queryOptions{}, maxFreeTextSearchResults},
		{"limit", "alpha", queryOptions{limit: 50}, 50},
		{"limit above the cap", "alpha", queryOptions{limit: 5000}, maxFreeTextSearchResults},
		{"sort clause", "alpha sort:title", queryOptions{limit: 50}, maxFreeTextSearchResults},
		{"property filter", "alpha prop:status=open", queryOptions{limit: 50}, maxFreeTextSearchResults},
		{"exact type, no type clause", "alpha", queryOptions{limit: 50, exactType: "ticket"}, maxFreeTextSearchResults},
		{"exact type equals the clause", "type:ticket alpha", queryOptions{limit: 50, exactType: "ticket"}, 50},
		{"exact type and another clause", "type:ticket type:feature alpha", queryOptions{limit: 50, exactType: "ticket"}, maxFreeTextSearchResults},
		{"type clause without exact type", "type:ticket,feature alpha", queryOptions{limit: 50}, 50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := freeTextLimit(searchparser.ParseQuery(tc.query), tc.opts); got != tc.want {
				t.Errorf("freeTextLimit(%q, %+v) = %d, want %d", tc.query, tc.opts, got, tc.want)
			}
		})
	}
}
