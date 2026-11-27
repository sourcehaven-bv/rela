package dataentry

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
)

// searchPaged performs GET /api/v1/_search with the given parameters.
func searchPaged(
	ctx context.Context, t *testing.T, app *App, params url.Values,
) (v1.ListResponse, *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?"+params.Encode(), http.NoBody)
	rec := httptest.NewRecorder()
	app.entities.handleV1Search(rec, req.WithContext(ctx))
	var resp v1.ListResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode search response: %v\nbody: %s", err, rec.Body)
		}
	}
	return resp, rec
}

// searchEveryPage reads q page by page at limit rows a page, following
// has_more, and returns the ids in order and the total of the first page.
func searchEveryPage(ctx context.Context, t *testing.T, app *App, q string, limit int) (ids []string, total int) {
	t.Helper()
	for page := 1; ; page++ {
		resp, rec := searchPaged(ctx, t, app, url.Values{
			"q": {q}, "limit": {strconv.Itoa(limit)}, "page": {strconv.Itoa(page)},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("page %d: %d %s", page, rec.Code, rec.Body)
		}
		if resp.Meta.Page != page || resp.Meta.PerPage != limit {
			t.Errorf("page %d: meta %+v", page, resp.Meta)
		}
		if page == 1 {
			total = resp.Meta.Total
		}
		ids = append(ids, limitedIDs(resp)...)
		if !resp.Meta.HasMore {
			return ids, total
		}
		if len(resp.Data) == 0 || page > 20 {
			t.Fatalf("page %d: has_more with %d rows", page, len(resp.Data))
		}
	}
}

func seedSearchPageApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppV1(t)
	for i := 1; i <= 5; i++ {
		seedEntity(app, &entity.Entity{ID: fmt.Sprintf("TKT-%03d", i), Type: "ticket",
			Properties: map[string]any{"title": fmt.Sprintf("alpha ticket %d", i)}})
	}
	for i := 1; i <= 3; i++ {
		seedEntity(app, &entity.Entity{ID: fmt.Sprintf("FEAT-%03d", i), Type: "feature",
			Properties: map[string]any{"title": fmt.Sprintf("alpha feature %d", i)}})
	}
	return app
}

// TestSearchPage_PagesThroughEveryRow pins that following has_more page by
// page returns the result without a limit, once each and in order: for a
// structured query over two types (the plan, which skips whole types by
// count), one the plan declines (the Go path) and free text.
func TestSearchPage_PagesThroughEveryRow(t *testing.T) {
	app := seedSearchPageApp(t)
	ctx := context.Background()
	for _, q := range []string{"type:ticket,feature", "type:ticket,feature sort:title", "alpha"} {
		t.Run(q, func(t *testing.T) {
			all, rec := searchPaged(ctx, t, app, url.Values{"q": {q}})
			if rec.Code != http.StatusOK || len(all.Data) != 8 || all.Meta.HasMore {
				t.Fatalf("unpaged: %d, %d rows, meta %+v", rec.Code, len(all.Data), all.Meta)
			}
			for _, limit := range []int{1, 3, 4} {
				ids, total := searchEveryPage(ctx, t, app, q, limit)
				if want := limitedIDs(all); !slices.Equal(ids, want) {
					t.Errorf("limit %d: pages give %v, want %v", limit, ids, want)
				}
				if q != "alpha" && total != 8 {
					// Free text reports a lower bound; the pages above pin it.
					t.Errorf("limit %d: total = %d, want 8", limit, total)
				}
			}
		})
	}
}

// TestSearchPage_HiddenRowsDoNotShiftPages pins that pages, the total and
// has_more are computed over the rows the principal may read: alice reads
// tickets only, so the features take no slot and no offset.
func TestSearchPage_HiddenRowsDoNotShiftPages(t *testing.T) {
	app := seedSearchPageApp(t)
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)
	ctx := gateCtxFor(aliceCtx(), t, d)

	ids, total := searchEveryPage(ctx, t, app, "type:feature,ticket", 2)
	want := []string{"TKT-001", "TKT-002", "TKT-003", "TKT-004", "TKT-005"}
	if !slices.Equal(ids, want) || total != 5 {
		t.Errorf("got %v total %d, want %v total 5", ids, total, want)
	}
}

// TestSearchPage_PastTheEnd pins that a page after the last one is empty
// and still reports the total.
func TestSearchPage_PastTheEnd(t *testing.T) {
	app := seedSearchPageApp(t)
	resp, rec := searchPaged(context.Background(), t, app,
		url.Values{"q": {"type:ticket"}, "limit": {"2"}, "page": {"9"}})
	if rec.Code != http.StatusOK || len(resp.Data) != 0 || resp.Meta.Total != 5 || resp.Meta.HasMore {
		t.Errorf("got %d, %d rows, meta %+v; want 200, no rows, total 5", rec.Code, len(resp.Data), resp.Meta)
	}
}

func TestSearchPage_Invalid(t *testing.T) {
	app := seedSearchPageApp(t)
	for _, page := range []string{"0", "-1", "abc", "1.5", strconv.Itoa(maxSearchPage + 1)} {
		t.Run(page, func(t *testing.T) {
			_, rec := searchPaged(context.Background(), t, app, url.Values{"q": {"type:ticket"}, "page": {page}})
			if rec.Code != http.StatusBadRequest {
				t.Errorf("page=%s: got %d, want 400", page, rec.Code)
			}
		})
	}
}

// limitedSearcher yields one ticket hit per id, in order, up to the
// query's Limit, as a real index does.
type limitedSearcher struct{ ids []string }

func (s limitedSearcher) Search(_ context.Context, q search.Query) iter.Seq2[search.Hit, error] {
	return func(yield func(search.Hit, error) bool) {
		hits := make([]search.Hit, 0, len(s.ids))
		for _, id := range s.ids {
			if q.Limit > 0 && len(hits) == q.Limit {
				break
			}
			hits = append(hits, search.Hit{ID: id, Type: "ticket"})
		}
		admitted, err := admitHits(q, hits)
		if err != nil {
			yield(search.Hit{}, err)
			return
		}
		for _, h := range admitted {
			if !yield(h, nil) {
				return
			}
		}
	}
}

// TestSearchPage_FreeTextCapIsReported pins that a free-text search with
// more hits than maxSearchResults serves the top maxSearchResults, says so
// with meta.capped, and offers no page past the cap; and that one with
// exactly maxSearchResults hits is not capped.
func TestSearchPage_FreeTextCapIsReported(t *testing.T) {
	for _, tc := range []struct {
		name   string
		hits   int
		capped bool
	}{
		{"over the cap", maxSearchResults + 5, true},
		{"at the cap", maxSearchResults, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			ids := make([]string, tc.hits)
			for i := range ids {
				ids[i] = fmt.Sprintf("TKT-%05d", i)
				seedEntity(app, &entity.Entity{ID: ids[i], Type: "ticket", Properties: map[string]any{"title": "x"}})
			}
			app.searcher = limitedSearcher{ids: ids}
			rebindVisibleSearcher(t, app)

			resp, rec := searchPaged(context.Background(), t, app, url.Values{"q": {"alpha"}})
			if rec.Code != http.StatusOK {
				t.Fatalf("search: %d %s", rec.Code, rec.Body)
			}
			if len(resp.Data) != maxSearchResults || resp.Meta.Total != maxSearchResults ||
				resp.Meta.HasMore || resp.Meta.Capped != tc.capped {

				t.Errorf("%d rows, meta %+v; want %d rows, total %d, no has_more, capped %v",
					len(resp.Data), resp.Meta, maxSearchResults, maxSearchResults, tc.capped)
			}
			if got, want := resp.Data[len(resp.Data)-1].ID, ids[maxSearchResults-1]; got != want {
				t.Errorf("last row %s, want %s: a row past the cap was served", got, want)
			}
		})
	}
}
