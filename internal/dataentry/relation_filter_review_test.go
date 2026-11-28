package dataentry

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// An id of a type the relation cannot reach is read as a title (RR-4CMAYI3):
// a persoon whose id is "Gemini" must not turn filter[belongs_to]=Gemini
// into an id filter that matches nothing.
func TestRelationFilter_IDOfUnreachableTypeIsATitle(t *testing.T) {
	app := sameTitleApp(t)
	seedEntity(app, &entity.Entity{ID: "Gemini", Type: "persoon", Properties: map[string]any{"name": "Decoy"}})
	query := url.Values{"filter[belongs_to]": {"Gemini"}}
	gated, err := gateAppListQuery(context.Background(), app, app.State(), "taak", parseQuery(query.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	if target := gated.relationTargets["filter[belongs_to]"]; target.id != "" || target.title != "Gemini" {
		t.Errorf("target = %+v, want title mode for Gemini", target)
	}
	if got := listTaken(t, app, query); !slices.Equal(got, []string{"TAAK-002"}) {
		t.Errorf("ids = %v, want [TAAK-002]", got)
	}
}

// failingFacesGate fails every row-gate batch, as a failing ACL query would.
type failingFacesGate struct{ nopReadGate }

func (failingFacesGate) ReadableFacesMany(context.Context, string, []string) (acl.FaceVerdicts, error) {
	return acl.FaceVerdicts{}, errors.New(`pq: relation "secret" missing`)
}

// A fault while resolving the filter value fails the list (RR-2Y3O3SE,
// RR-CRS1VRR) instead of reading the id as a title, which would answer
// from a partial view.
func TestRelationFilter_LookupFaultFailsTheList(t *testing.T) {
	for _, key := range []string{"filter[belongs_to]", "filter[belongs_to][ne]"} {
		t.Run("gate fault "+key, func(t *testing.T) {
			app := sameTitleApp(t)
			ctx := withReadGate(context.Background(), failingFacesGate{})
			_, err := gateAppListQuery(ctx, app, app.State(), "taak", parseQuery(url.Values{key: {"PRJ-001"}}.Encode()))
			if !errors.Is(err, errListLoad) {
				t.Errorf("err = %v, want errListLoad", err)
			}
		})
		t.Run("header read fault "+key, func(t *testing.T) {
			app := sameTitleApp(t)
			vr, err := newVisibleReader(failingListStore{app.store}, tokenFamilies)
			if err != nil {
				t.Fatal(err)
			}
			app.visibleReader = vr
			_, err = gateAppListQuery(context.Background(), app, app.State(), "taak",
				parseQuery(url.Values{key: {"PRJ-001"}}.Encode()))
			if !errors.Is(err, errListLoad) {
				t.Fatalf("err = %v, want errListLoad", err)
			}
			req := httptest.NewRequest(http.MethodGet, "/api/v1/taaks?"+url.Values{key: {"PRJ-001"}}.Encode(), http.NoBody)
			rec := httptest.NewRecorder()
			app.entities.handleV1ListEntities(rec, req, "taak", "taaks")
			if rec.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want 500 (%s)", rec.Code, rec.Body)
			}
		})
	}
}

// The operator is checked before the value is looked up (RR-5BP241I), so a
// refused operator is a 400 even when the lookup would fail.
func TestRelationFilter_BadOperatorBeforeLookup(t *testing.T) {
	app := sameTitleApp(t)
	ctx := withReadGate(context.Background(), failingFacesGate{})
	_, err := gateAppListQuery(ctx, app, app.State(), "taak",
		parseQuery(url.Values{"filter[belongs_to][contains]": {"PRJ-001"}}.Encode()))
	if errors.Is(err, errListLoad) {
		t.Errorf("err = %v, want no lookup", err)
	}
}

// Two different values for one relation key are a 400 (RR-ZVOS2K9); the same
// value twice is one value.
func TestRelationFilter_RepeatedValues(t *testing.T) {
	app := sameTitleApp(t)
	status := func(q url.Values) int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/taaks?"+q.Encode(), http.NoBody)
		rec := httptest.NewRecorder()
		app.entities.handleV1ListEntities(rec, req, "taak", "taaks")
		return rec.Code
	}
	if got := status(url.Values{"filter[belongs_to]": {"PRJ-001", "PRJ-002"}}); got != http.StatusBadRequest {
		t.Errorf("two values: status %d, want 400", got)
	}
	if got := status(url.Values{"filter[belongs_to]": {"PRJ-001", "PRJ-001"}}); got != http.StatusOK {
		t.Errorf("same value twice: status %d, want 200", got)
	}
}

// The store and the Go path agree under a row-gated policy (RR-2BKZC54):
// alice reads a project, and the tasks under it, only through a local role.
func TestListPushdown_RelationFilterByIDRowGated(t *testing.T) {
	assertRelationIDPushdownRowGated(t, sameTitleApp(t))
}

// assertRelationIDPushdownRowGated runs the row-gated parity check on app.
func assertRelationIDPushdownRowGated(t *testing.T, app *App) {
	t.Helper()
	seedRelation(app, entity.NewRelation("alice", "editor-of", "PRJ-001"))
	policy := &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"viewer": {Read: []string{"persoon"}},
			"editor": {Read: []string{"taak", "project"}},
		},
		Assignments:         map[string]string{"alice": "viewer"},
		RoleRelations:       map[string]acl.RoleRelationDef{"editor-of": {Confers: "editor"}},
		InheritRolesThrough: []string{"belongs_to"},
	}
	assertRelationIDParity(t, app, policy, []string{
		"filter%5Bbelongs_to%5D=PRJ-001&sort=title",
		"filter%5Bbelongs_to%5D%5Bne%5D=PRJ-001&sort=title",
		"filter%5Bbelongs_to%5D=PRJ-003&sort=title",
		"filter%5Bverantwoordelijk_voor%5D=PER-001&sort=title",
		"filter%5Bverantwoordelijk_voor%5D%5Bne%5D=PER-001&sort=-title",
	})
}

// assertRelationIDParity checks that each query returns the Go path's page
// and total under policy, whichever path serves it.
func assertRelationIDParity(t *testing.T, app *App, policy *acl.Policy, queries []string) {
	t.Helper()
	d := mustNewACL(t, policy, app.store)
	setTestACL(t, app, d)
	const perPage = 2
	for _, base := range queries {
		for page := 1; page <= 2; page++ {
			t.Run(fmt.Sprintf("%s page=%d", base, page), func(t *testing.T) {
				q := fmt.Sprintf("%s&per_page=%d&page=%d", base, perPage, page)
				resp, rec := listEntitiesAs(aliceCtx(), t, app, d, "taak", "taaks", q)
				if rec.Code != http.StatusOK {
					t.Fatalf("status %d: %s", rec.Code, rec.Body)
				}
				ctx := gateCtxFor(aliceCtx(), t, d)
				wantIDs, wantTotal := goPath(ctx, t, app, "taak", parseQuery(q), page, perPage)
				gotIDs := make([]string, 0, len(resp.Data))
				for _, e := range resp.Data {
					gotIDs = append(gotIDs, e.ID)
				}
				if !equalStringSlices(gotIDs, wantIDs) || resp.Meta.Total != wantTotal {
					t.Errorf("served %v (total %d), go %v (total %d)", gotIDs, resp.Meta.Total, wantIDs, wantTotal)
				}
			})
		}
	}
}

// restrictedReadGate reads nothing whole and refuses traversals it is told to.
type restrictedReadGate struct {
	nopReadGate
	traversalErr error
}

func (restrictedReadGate) ReadQuery(context.Context, string) acl.ReadQueryResult {
	return acl.ReadQueryResult{}
}

func (g restrictedReadGate) GateTraversal(
	ctx context.Context, candidateType string, hop acl.TraversalHop,
) (*store.RelationPredicate, error) {
	if g.traversalErr != nil {
		return nil, g.traversalErr
	}
	return g.nopReadGate.GateTraversal(ctx, candidateType, hop)
}

// lowerRelationFilters keeps the Go path for a faced target the principal
// does not read whole, and for a traversal the gate cannot lower
// (RR-2BKZC54).
func TestLowerRelationFilters_Declines(t *testing.T) {
	key := "filter[belongs_to]"
	q := testGatedQuery(map[string][]string{key: {"PRJ-001"}}, func(string) bool { return true })
	q.relationTargets = map[string]relationTarget{key: {id: "PRJ-001", entityType: "project"}}
	faced := func(app *App) *Schema {
		st := *app.State()
		meta := *st.Meta
		meta.Entities = maps.Clone(meta.Entities)
		def := meta.Entities["project"]
		def.Faces = map[string]metamodel.FaceDef{"public": {}, "internal": {}}
		meta.Entities["project"] = def
		st.Meta = &meta
		return &st
	}
	for _, tc := range []struct {
		name  string
		state func(*App) *Schema
		gate  restrictedReadGate
		want  bool
	}{
		{"faceless target, row-gated", func(a *App) *Schema { return a.State() }, restrictedReadGate{}, true},
		{"faced target, row-gated", faced, restrictedReadGate{}, false},
		{"traversal unsupported", func(a *App) *Schema { return a.State() },
			restrictedReadGate{traversalErr: acl.ErrTraversalUnsupported}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := relationFilterApp(t)
			ctx := withReadGate(context.Background(), tc.gate)
			if _, _, ok := lowerRelationFilters(ctx, tc.state(app), "taak", q); ok != tc.want {
				t.Errorf("ok = %v, want %v", ok, tc.want)
			}
		})
	}
}

// An id relation filter reads the same calls at 10 and 50 matching rows,
// and the same rows over a store of 100 and of 1,000 other tasks
// (RR-KDDXSAF).
func TestQueryBudget_ListRelationFilterByID(t *testing.T) {
	run := func(n, pad int) *storetest.Counting {
		counting := storetest.NewCounting(memstore.New())
		app := relationFilterAppOn(t, appbuildtest.WithStore(counting))
		for i := range n {
			id := fmt.Sprintf("TAAK-1%04d", i)
			seedEntity(app, &entity.Entity{ID: id, Type: "taak", Properties: map[string]any{"title": id}})
			seedRelation(app, entity.NewRelation(id, "belongs_to", "PRJ-001"))
		}
		for i := range pad {
			id := fmt.Sprintf("TAAK-P%04d", i)
			seedEntity(app, &entity.Entity{ID: id, Type: "taak", Properties: map[string]any{"title": id}})
			seedRelation(app, entity.NewRelation(id, "belongs_to", "PRJ-002"))
		}
		d := mustNewACL(t, &acl.Policy{
			Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"*"}}},
			Assignments: map[string]string{"alice": "viewer"},
		}, app.store)
		setTestACL(t, app, d)
		counting.Reset()
		resp, rec := listEntitiesAs(aliceCtx(), t, app, d, "taak", "taaks",
			"filter%5Bbelongs_to%5D=PRJ-001&sort=title&per_page=100")
		if rec.Code != http.StatusOK || len(resp.Data) != n+2 {
			t.Fatalf("list: %d, %d rows, want %d (%s)", rec.Code, len(resp.Data), n+2, rec.Body)
		}
		return counting
	}
	small, large := run(10, 0), run(50, 0)
	if small.Calls()["ListEntityHeaders"] != large.Calls()["ListEntityHeaders"] ||
		small.Cost().Calls != large.Cost().Calls {

		t.Errorf("calls grow with rows: %s | %s", small, large)
	}
	storetest.AssertScaleFree(t, func(pad int) (storetest.Cost, string) {
		counting := run(10, pad)
		return counting.Cost(), counting.String()
	})
}
