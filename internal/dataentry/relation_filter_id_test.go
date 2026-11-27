package dataentry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// sameTitleApp is relationFilterApp's graph plus a second "Apollo".
func sameTitleApp(t *testing.T) *App {
	t.Helper()
	return withSameTitle(relationFilterApp(t))
}

// withSameTitle adds PRJ-003, a second project titled "Apollo", with
// TAAK-004 belonging to it.
func withSameTitle(app *App) *App {
	seedEntity(app, &entity.Entity{ID: "PRJ-003", Type: "project", Properties: map[string]any{"name": "Apollo"}})
	seedRelation(app, entity.NewRelation("TAAK-004", "belongs_to", "PRJ-003"))
	return app
}

// TestV1ListRelationFilter_ByID pins TKT-ZKPA1E: a relation filter value
// that is a target id matches that target only, so two targets with the
// same title filter separately.
func TestV1ListRelationFilter_ByID(t *testing.T) {
	tests := []struct {
		name    string
		query   url.Values
		wantIDs []string
	}{
		{
			name:    "outgoing eq by id",
			query:   url.Values{"filter[belongs_to]": {"PRJ-001"}, "sort": {"title"}},
			wantIDs: []string{"TAAK-001", "TAAK-003"},
		},
		{
			name:    "same-titled target by id",
			query:   url.Values{"filter[belongs_to]": {"PRJ-003"}},
			wantIDs: []string{"TAAK-004"},
		},
		{
			name:    "outgoing ne by id",
			query:   url.Values{"filter[belongs_to][ne]": {"PRJ-001"}, "sort": {"title"}},
			wantIDs: []string{"TAAK-002", "TAAK-004"},
		},
		{
			name:    "incoming eq by id",
			query:   url.Values{"filter[verantwoordelijk_voor]": {"PER-002"}},
			wantIDs: []string{"TAAK-002"},
		},
		{
			name:    "incoming ne by id",
			query:   url.Values{"filter[verantwoordelijk_voor][ne]": {"PER-001"}, "sort": {"title"}},
			wantIDs: []string{"TAAK-002", "TAAK-004"},
		},
		{
			name:    "legacy title matches every same-titled target",
			query:   url.Values{"filter[belongs_to]": {"Apollo"}, "sort": {"title"}},
			wantIDs: []string{"TAAK-001", "TAAK-003", "TAAK-004"},
		},
		{
			name:    "unknown id matches nothing",
			query:   url.Values{"filter[belongs_to]": {"PRJ-999"}},
			wantIDs: []string{},
		},
		{
			name:    "id filter paged by the store keeps order and set",
			query:   url.Values{"filter[belongs_to]": {"PRJ-001"}, "sort": {"title"}, "per_page": {"1"}},
			wantIDs: []string{"TAAK-001"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := listTaken(t, sameTitleApp(t), tt.query)
			if !equalStringSlices(got, tt.wantIDs) {
				t.Errorf("ids = %v, want %v", got, tt.wantIDs)
			}
		})
	}
}

// TestV1ListRelationFilter_ByIDSurvivesRename pins that renaming the target
// leaves an id filter's result unchanged.
func TestV1ListRelationFilter_ByIDSurvivesRename(t *testing.T) {
	app := sameTitleApp(t)
	q := url.Values{"filter[belongs_to]": {"PRJ-001"}, "sort": {"title"}}
	before := listTaken(t, app, q)
	if err := app.store.UpdateEntity(context.Background(), &entity.Entity{
		ID: "PRJ-001", Type: "project", Properties: map[string]any{"name": "Artemis"},
	}); err != nil {
		t.Fatal(err)
	}
	after := listTaken(t, app, q)
	if !equalStringSlices(before, after) || !equalStringSlices(after, []string{"TAAK-001", "TAAK-003"}) {
		t.Errorf("before %v, after %v, want [TAAK-001 TAAK-003] both", before, after)
	}
}

// TestV1ListRelationFilter_UnreadableIDIsNotAnOracle pins that an id the
// caller cannot read filters exactly like a value that names nothing: it
// falls back to a title match against readable neighbors only.
func TestV1ListRelationFilter_UnreadableIDIsNotAnOracle(t *testing.T) {
	app := sameTitleApp(t)
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"taak", "project"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)

	listAs := func(q url.Values) []string {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/taaks?"+q.Encode(), http.NoBody)
		req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
		rec := httptest.NewRecorder()
		app.entities.handleV1ListEntities(rec, req, "taak", "taaks")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		var resp v1.ListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		ids := make([]string, 0, len(resp.Data))
		for _, e := range resp.Data {
			ids = append(ids, e.ID)
		}
		return ids
	}

	for _, op := range []string{"", "[ne]"} {
		hidden := listAs(url.Values{"filter[verantwoordelijk_voor]" + op: {"PER-001"}, "sort": {"title"}})
		missing := listAs(url.Values{"filter[verantwoordelijk_voor]" + op: {"PER-999"}, "sort": {"title"}})
		if !equalStringSlices(hidden, missing) {
			t.Errorf("op %q: hidden id %v differs from missing id %v", op, hidden, missing)
		}
	}
	if got := listAs(url.Values{"filter[belongs_to]": {"PRJ-003"}}); !equalStringSlices(got, []string{"TAAK-004"}) {
		t.Errorf("readable id under ACL = %v, want [TAAK-004]", got)
	}
}

// TestListPushdown_RelationFilterByID pins that the store serves an id
// relation filter and returns the Go path's page and total, while a title
// value keeps the Go path.
func TestListPushdown_RelationFilterByID(t *testing.T) {
	assertRelationIDPushdown(t, sameTitleApp(t))
}

// assertRelationIDPushdown runs TestListPushdown_RelationFilterByID's
// comparison on app, so each backend can run it on its own store.
func assertRelationIDPushdown(t *testing.T, app *App) {
	t.Helper()
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"*"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)
	const perPage = 2
	for _, tc := range []struct {
		query  string
		pushed bool
	}{
		{"filter%5Bbelongs_to%5D=PRJ-001&sort=title", true},
		{"filter%5Bbelongs_to%5D%5Bne%5D=PRJ-001&sort=title", true},
		{"filter%5Bverantwoordelijk_voor%5D=PER-001&sort=title", true},
		{"filter%5Bverantwoordelijk_voor%5D%5Bne%5D=PER-001&sort=-title", true},
		{"filter%5Bbelongs_to%5D=PRJ-001&filter%5Bverantwoordelijk_voor%5D=PER-001", true},
		{"filter%5Bbelongs_to%5D=PRJ-999&sort=title", false},
		{"filter%5Bbelongs_to%5D=Apollo&sort=title", false},
	} {
		for page := 1; page <= 2; page++ {
			t.Run(fmt.Sprintf("%s page=%d", tc.query, page), func(t *testing.T) {
				q := fmt.Sprintf("%s&per_page=%d&page=%d", tc.query, perPage, page)
				query := parseQuery(q)
				ctx := gateCtxFor(aliceCtx(), t, d)
				state := app.State()
				gated, err := gateAppListQuery(ctx, app, state, "taak", query)
				if err != nil {
					t.Fatal(err)
				}
				nctx, n, err := resolveListNarrowing(ctx, app, state, "taak", query)
				if err != nil {
					t.Fatal(err)
				}
				if _, _, ok := n.pushdownPlan(nctx, "taak", gated, page, perPage); ok != tc.pushed {
					t.Errorf("pushed = %v, want %v", ok, tc.pushed)
				}
				resp, rec := listEntitiesAs(aliceCtx(), t, app, d, "taak", "taaks", q)
				if rec.Code != http.StatusOK {
					t.Fatalf("status %d: %s", rec.Code, rec.Body)
				}
				wantIDs, wantTotal := goPath(ctx, t, app, "taak", query, page, perPage)
				gotIDs := make([]string, 0, len(resp.Data))
				for _, e := range resp.Data {
					gotIDs = append(gotIDs, e.ID)
				}
				if !equalStringSlices(gotIDs, wantIDs) || resp.Meta.Total != wantTotal {
					t.Errorf("pushed %v (total %d), go %v (total %d)", gotIDs, resp.Meta.Total, wantIDs, wantTotal)
				}
			})
		}
	}
}

// TestPushableRelationFilter pins which relation filters the store may
// answer: one eq or ne value naming a served id over an identity-scoped
// relation.
func TestPushableRelationFilter(t *testing.T) {
	meta := relationFilterMeta()
	content := meta.Relations["belongs_to"]
	content.Scope = metamodel.ScopeContent
	meta.Relations["owned_by"] = content
	isRel := func(string) bool { return true }
	byID := relationTarget{id: "PRJ-001", entityType: "project"}
	for _, tc := range []struct {
		name   string
		key    string
		values []string
		target *relationTarget
		want   bool
	}{
		{"id eq", "filter[belongs_to]", []string{"PRJ-001"}, &byID, true},
		{"id ne", "filter[belongs_to][ne]", []string{"PRJ-001"}, &byID, true},
		{"title", "filter[belongs_to]", []string{"Apollo"}, &relationTarget{title: "Apollo"}, false},
		{"unresolved", "filter[belongs_to]", []string{"PRJ-001"}, nil, false},
		{"two values", "filter[belongs_to]", []string{"PRJ-002", "PRJ-001"}, &byID, false},
		{"content-scoped", "filter[owned_by]", []string{"PRJ-001"}, &byID, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := testGatedQuery(map[string][]string{tc.key: tc.values}, isRel)
			if tc.target != nil {
				q.relationTargets = map[string]relationTarget{tc.key: *tc.target}
			}
			fk, err := parseFilterKey(tc.key)
			if err != nil {
				t.Fatal(err)
			}
			if got := pushableRelationFilter(meta, q, tc.key, fk, tc.values); got != tc.want {
				t.Errorf("pushable = %v, want %v", got, tc.want)
			}
		})
	}
}
