package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/reconclient"
	"github.com/Sourcehaven-BV/rela/internal/reconclient/httpremote"
	"github.com/Sourcehaven-BV/rela/internal/store/elementnaive"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// The end-to-end tests of the reconcile client (TKT-0Y566OJ): the client
// talks HTTP to the real handler over one memstore, and reads its local
// side through visibility.Elements over another.

const peerToken = "peer-token"

// servePeer serves the reconcile fixture's handler over HTTP as user,
// after checking the bearer token, as the gate in front of it would.
func servePeer(t *testing.T, f *reconcileFixture, user string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+peerToken {
			writeV1Error(w, r, http.StatusUnauthorized, "unauthenticated", "Authentication required", "")
			return
		}
		// The peer's identity replaces the request's, as the auth gate does.
		r = r.WithContext(gateCtxFor(principalCtx(user), t, f.d)) //nolint:contextcheck // see above
		f.h.handleV1Reconcile(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// localElements returns an element read over app's store: for the whole
// operator view when d is nil, otherwise gated by d.
func localElements(t *testing.T, app *App, d *acl.Declarative) *visibility.Elements {
	t.Helper()
	ranges, err := elementnaive.New(app.store)
	if err != nil {
		t.Fatal(err)
	}
	deps := visibility.ElementDeps{
		Ranges: ranges, Rows: app.store, Reads: visibility.AllowAllReadQueries{},
		Policy: visibility.NopRedactor{}, Rollups: noRollups{},
	}
	if d != nil {
		resolver, rErr := affordances.New(app.Meta(), storeRelationLookup{st: app.store}, d)
		if rErr != nil {
			t.Fatal(rErr)
		}
		if deps.Reads, err = visibility.NewDeclarativeGate(d, defaultWorldScope(app.worlds)); err != nil {
			t.Fatal(err)
		}
		if deps.Policy, err = visibility.NewPolicyRedactor(resolver); err != nil {
			t.Fatal(err)
		}
	}
	el, err := visibility.NewElements(deps)
	if err != nil {
		t.Fatal(err)
	}
	return el
}

func newDiffer(t *testing.T, srv *httptest.Server, local reconclient.Local, token string) *reconclient.Differ {
	t.Helper()
	auth, err := httpremote.Bearer(token)
	if err != nil {
		t.Fatal(err)
	}
	remote, err := httpremote.New(httpremote.Config{BaseURL: srv.URL, Auth: auth})
	if err != nil {
		t.Fatal(err)
	}
	d, err := reconclient.New(local, remote, reconclient.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

var e2eScope = reconclient.Scope{Entities: []string{"feature", "ticket"}, Relations: []string{"blocks", "implements"}}

// seedFixtureCopy gives app the rows the reconcile fixture seeds, and its
// relations.
func seedFixtureCopy(app *App) {
	for _, e := range []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "one", "status": "open"}},
		{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "two", "status": "open"}},
		{ID: "TKT-003", Type: "ticket", Properties: map[string]any{"title": "three"}},
		{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "f1"}},
		{ID: "FEAT-002", Type: "feature", Properties: map[string]any{"title": "f2"}},
	} {
		seedEntity(app, e)
	}
	seedRelation(app, &entity.Relation{From: "TKT-001", Type: "implements", To: "FEAT-001"})
}

func seedFixtureRelations(f *reconcileFixture) {
	seedRelation(f.app, &entity.Relation{From: "TKT-001", Type: "implements", To: "FEAT-001"})
}

func diffAddrs(res reconclient.Result) (onlyL, onlyR, chg []string) {
	for _, e := range res.OnlyLocal {
		onlyL = append(onlyL, e.Address)
	}
	for _, e := range res.OnlyRemote {
		onlyR = append(onlyR, e.Address)
	}
	for _, c := range res.Differing {
		chg = append(chg, c.Address)
	}
	return onlyL, onlyR, chg
}

func TestReconcileClient_EndToEnd(t *testing.T) {
	cases := []struct {
		name      string
		user      string
		edit      func(local *App)
		wantL     []string
		wantR     []string
		wantChg   []string
		wantTrips int
	}{
		{name: "identical", user: "root", wantTrips: 1},
		{
			name: "overlapping", user: "root",
			edit: func(local *App) {
				ctx := context.Background()
				if _, err := local.store.DeleteFamily(ctx, "TKT-002", true); err != nil {
					t.Fatal(err)
				}
				seedEntity(local, &entity.Entity{ID: "TKT-009", Type: "ticket", Properties: map[string]any{"title": "nine"}})
				seedRelation(local, &entity.Relation{From: "TKT-003", Type: "blocks", To: "TKT-001"})
			},
			wantL: []string{"ticket/TKT-009", "TKT-003--blocks--TKT-001"},
			wantR: []string{"ticket/TKT-002"},
		},
		{
			name: "differing hash", user: "root",
			edit: func(local *App) {
				e, err := local.store.GetEntity(context.Background(), entity.Ref{ID: "FEAT-002", Face: ""})
				if err != nil {
					t.Fatal(err)
				}
				e.Content = "a body only the local side has"
				if err := local.store.UpdateEntity(context.Background(), e); err != nil {
					t.Fatal(err)
				}
			},
			wantChg: []string{"feature/FEAT-002"},
		},
		{
			// alice sees only a ticket's title on the remote. The rows with
			// a status she cannot see differ; the one without matches.
			name: "remote view with a hidden field", user: "alice",
			wantChg: []string{"ticket/TKT-001", "ticket/TKT-002"},
		},
		{
			// bob reads only tickets: features and the relation to one are
			// absent from his view, so they are only local.
			name: "remote view with fewer rows", user: "bob",
			wantL: []string{"feature/FEAT-001", "feature/FEAT-002", "TKT-001--implements--FEAT-001"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newReconcileFixture(t, reconcileGateConfig{})
			seedFixtureRelations(f)
			local := newTestAppV1(t)
			seedFixtureCopy(local)
			if tc.edit != nil {
				tc.edit(local)
			}
			srv, hits := servePeer(t, f, tc.user)
			res, err := newDiffer(t, srv, localElements(t, local, nil), peerToken).Diff(t.Context(), e2eScope)
			if err != nil {
				t.Fatal(err)
			}
			l, r, c := diffAddrs(res)
			if !slices.Equal(l, tc.wantL) || !slices.Equal(r, tc.wantR) || !slices.Equal(c, tc.wantChg) {
				t.Errorf("only local %v, only remote %v, differing %v; want %v, %v, %v",
					l, r, c, tc.wantL, tc.wantR, tc.wantChg)
			}
			if tc.wantTrips > 0 && int(hits.Load()) != tc.wantTrips {
				t.Errorf("%d round trips, want %d", hits.Load(), tc.wantTrips)
			}
		})
	}
}

// TestReconcileClient_SameViewOnBothSides: the local side read through the
// same policy as the same principal sees what the remote sees, hidden
// field and all, so nothing differs.
func TestReconcileClient_SameViewOnBothSides(t *testing.T) {
	f := newReconcileFixture(t, reconcileGateConfig{})
	seedFixtureRelations(f)
	local := newTestAppV1(t)
	seedFixtureCopy(local)
	var p acl.Policy
	if err := yaml.Unmarshal([]byte(reconcilePolicy), &p); err != nil {
		t.Fatal(err)
	}
	localACL := mustNewACL(t, &p, local.store)
	el := localElements(t, local, localACL)
	srv, hits := servePeer(t, f, "alice")
	d := newDiffer(t, srv, el, peerToken)

	// Without a bound request the local read fails closed, before any
	// request leaves.
	if _, err := d.Diff(t.Context(), e2eScope); err == nil {
		t.Fatal("a diff without a bound identity succeeded")
	}
	if hits.Load() != 0 {
		t.Errorf("%d requests sent before the local side failed", hits.Load())
	}

	ctx := gateCtxFor(principalCtx("alice"), t, localACL)
	res, err := d.Diff(ctx, e2eScope)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Same() || hits.Load() != 1 {
		t.Errorf("same=%v after %d requests: %+v", res.Same(), hits.Load(), res)
	}
}

// TestReconcileClient_Paging follows the real handler's pages through the
// HTTP remote. The differ asks for full 2000-row pages, so it would need
// thousands of seeded rows here; its paging rules are tested against a fake
// remote in internal/reconclient. This test pins the wire contract both rely
// on: a page with More set carries a Last the next request continues after.
func TestReconcileClient_Paging(t *testing.T) {
	f := newReconcileFixture(t, reconcileGateConfig{})
	srv, hits := servePeer(t, f, "root")
	auth, err := httpremote.Bearer(peerToken)
	if err != nil {
		t.Fatal(err)
	}
	remote, err := httpremote.New(httpremote.Config{BaseURL: srv.URL, Auth: auth})
	if err != nil {
		t.Fatal(err)
	}
	ask := func(rr v1.ReconcileRange) v1.ReconcileResult {
		t.Helper()
		resp, err := remote.Reconcile(t.Context(), v1.ReconcileRequest{Ranges: []v1.ReconcileRange{rr}})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Ranges) != 1 {
			t.Fatalf("%d results for one range", len(resp.Ranges))
		}
		return resp.Ranges[0]
	}

	var want []string
	for _, e := range ask(listRange(100, "ticket", "feature")).Elements {
		want = append(want, e.Address)
	}
	hits.Store(0)

	var got []string
	rr := listRange(2, "ticket", "feature")
	for range len(want) + 1 {
		res := ask(rr)
		for _, e := range res.Elements {
			got = append(got, e.Address)
		}
		if !res.More {
			break
		}
		last := res.Last
		rr.After = &last
	}
	if !slices.Equal(got, want) {
		t.Errorf("paged %v, want %v", got, want)
	}
	if pages := int32((len(want) + 1) / 2); hits.Load() != pages {
		t.Errorf("%d requests for %d rows in pages of 2, want %d", hits.Load(), len(want), pages)
	}
}

func TestReconcileClient_WrongToken(t *testing.T) {
	f := newReconcileFixture(t, reconcileGateConfig{})
	local := newTestAppV1(t)
	srv, _ := servePeer(t, f, "root")
	_, err := newDiffer(t, srv, localElements(t, local, nil), "wrong").Diff(t.Context(), e2eScope)
	if err == nil || !strings.Contains(err.Error(), "authentication required") {
		t.Errorf("err = %v", err)
	}
}
