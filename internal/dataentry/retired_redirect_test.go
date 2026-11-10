package dataentry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// retiringLoader adds a fixed [store.RetiredIDs] table to the test app's
// store, as pgstore and sqlitestore keep one after a rename or conversion.
type retiringLoader struct {
	store.Store
	retired map[string]string
}

func (l retiringLoader) Successors(_ context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	for _, id := range ids {
		if to, ok := l.retired[id]; ok {
			out[id] = to
		}
	}
	return out, nil
}

// newRetiringApp seeds TKT-001 and FEAT-001 and retires TKT-OLD into TKT-001
// and TKT-GONE into FEAT-001. alice reads tickets only; carol reads both.
func newRetiringApp(t *testing.T) (*App, *acl.Declarative) {
	t.Helper()
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "F1"}})

	retireIDs(t, app, map[string]string{"TKT-OLD": "TKT-001", "TKT-GONE": "FEAT-001"})

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"viewer": {Read: []string{"ticket"}},
			"reader": {Read: []string{"ticket", "feature"}},
		},
		Assignments: map[string]string{"alice": "viewer", "carol": "reader"},
	}, app.store)
	setTestACL(t, app, d)
	return app, d
}

// retireIDs gives app a store that maps each key of retired to its value.
func retireIDs(t *testing.T, app *App, retired map[string]string) {
	t.Helper()
	load := retiringLoader{Store: app.store, retired: retired}
	vr, err := visibility.NewPolicyReader(ctxRowGate{}, appRedactor(app), load, familiesOption(app))
	if err != nil {
		t.Fatalf("NewPolicyReader: %v", err)
	}
	app.viewReader = vr
	app.views.viewReader = vr
}

// TestRetiredRedirect_ViewsInAWorld pins the redirect when a world is
// selected: a world that ranks faces answers a missing entry through the
// world-absent page, not the plain miss.
func TestRetiredRedirect_ViewsInAWorld(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	retireIDs(t, app, map[string]string{"TKT-OLD": "TKT-001"})
	app.setWorlds(stubWorlds{names: map[string]bool{"published": true}, resolveDefault: true})

	rec := viewRecord(t, app, "/api/v1/_views/ticket/TKT-OLD?world=published")
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("got %d, want 307; body=%s", rec.Code, rec.Body)
	}
	if got, want := rec.Header().Get("Location"), "../ticket/TKT-001?world=published"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if code := viewStatus(t, app, "/api/v1/_views/ticket/NOPE-1?world=published"); code != http.StatusUnprocessableEntity {
		t.Errorf("a never-existing id: got %d, want the 422 it has always had", code)
	}
}

func TestRetiredRedirect_Views(t *testing.T) {
	app, d := newRetiringApp(t)

	tests := []struct {
		name     string
		user     string
		id       string
		query    string
		location string
	}{
		{"same type", "alice", "TKT-OLD", "", "../ticket/TKT-001"},
		{"keeps the query", "alice", "TKT-OLD", "world=main", "../ticket/TKT-001?world=main"},
		{"drops the face", "alice", "TKT-OLD@draft", "", "../ticket/TKT-001"},
		{"other type", "carol", "TKT-GONE", "", "../feature/FEAT-001"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := viewsAsQuery(principalCtx(tc.user), t, app, d, "ticket", tc.id, tc.query)
			if rec.Code != http.StatusTemporaryRedirect {
				t.Fatalf("got %d, want 307; body=%s", rec.Code, rec.Body)
			}
			if got := rec.Header().Get("Location"); got != tc.location {
				t.Errorf("Location = %q, want %q", got, tc.location)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store: the answer depends on the principal", got)
			}
		})
	}
}

// TestRetiredRedirect_ViewsHiddenSuccessor pins that a successor the caller
// may not read is no successor: the answer is the one an id that never
// existed gets, with only the id in the detail differing.
func TestRetiredRedirect_ViewsHiddenSuccessor(t *testing.T) {
	app, d := newRetiringApp(t)

	gone := viewsAs(aliceCtx(), t, app, d, "ticket", "TKT-GONE")
	never := viewsAs(aliceCtx(), t, app, d, "ticket", "TKT-NEVER")
	if gone.Code != never.Code {
		t.Fatalf("status: retired into a hidden entity %d, never existed %d", gone.Code, never.Code)
	}
	if gone.Header().Get("Location") != "" {
		t.Errorf("Location set for a hidden successor: %q", gone.Header().Get("Location"))
	}
	g := stripInstance(t, strings.ReplaceAll(gone.Body.String(), "TKT-GONE", "X"))
	n := stripInstance(t, strings.ReplaceAll(never.Body.String(), "TKT-NEVER", "X"))
	if g != n {
		t.Errorf("bodies differ:\n  hidden successor: %s\n  never existed:    %s", g, n)
	}
}

func TestRetiredRedirect_ViewsLiveIDServed(t *testing.T) {
	app, d := newRetiringApp(t)
	rec := viewsAs(aliceCtx(), t, app, d, "ticket", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", rec.Code, rec.Body)
	}
}

func TestRetiredRedirect_EntityGet(t *testing.T) {
	app, d := newRetiringApp(t)

	rec := getEntityAs(principalCtx("carol"), t, app, d, "ticket", "tickets", "TKT-GONE", "world=main")
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("got %d, want 307; body=%s", rec.Code, rec.Body)
	}
	if got, want := rec.Header().Get("Location"), "../features/FEAT-001?world=main"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}

	rec = getEntityAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-OLD", "")
	if got, want := rec.Header().Get("Location"), "../tickets/TKT-001"; rec.Code != http.StatusTemporaryRedirect || got != want {
		t.Errorf("got %d Location %q, want 307 %q", rec.Code, got, want)
	}

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}

	// A hidden successor: the uniform 404, byte for byte.
	gone := getEntityAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-GONE", "")
	never := getEntityAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-NEVER", "")
	if gone.Code != http.StatusNotFound || never.Code != http.StatusNotFound {
		t.Fatalf("got %d and %d, want 404 for both", gone.Code, never.Code)
	}
	if g, n := stripInstance(t, gone.Body.String()), stripInstance(t, never.Body.String()); g != n {
		t.Errorf("bodies differ:\n  hidden successor: %s\n  never existed:    %s", g, n)
	}
}

// TestRetiredRedirect_WritesNotRedirected pins that a write to an old id stays
// not found: following it would change an entity the caller did not name.
func TestRetiredRedirect_WritesNotRedirected(t *testing.T) {
	app, d := newRetiringApp(t)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-OLD", strings.NewReader(`{"title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.handleV1SingleEntity(rec, req, "ticket", "tickets", "TKT-OLD")
	if rec.Code == http.StatusTemporaryRedirect || rec.Header().Get("Location") != "" {
		t.Fatalf("PATCH on a retired id redirected: %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func viewsAsQuery(ctx context.Context, t *testing.T, app *App, d *acl.Declarative,
	entityType, entityID, rawQuery string,
) *httptest.ResponseRecorder {
	t.Helper()
	url := "/api/v1/_views/" + entityType + "/" + entityID
	if rawQuery != "" {
		url += "?" + rawQuery
	}
	req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.views.handleV1Views(rec, req)
	return rec
}

// TestRetiredRedirect_Misses pins the misses that must stay misses: the id
// is live under another type, the face does not exist, or the successor is
// gone.
func TestRetiredRedirect_Misses(t *testing.T) {
	app, d := newRetiringApp(t)
	retireIDs(t, app, map[string]string{"TKT-OLD": "TKT-001", "TKT-GONE": "FEAT-001", "TKT-DANGLING": "TKT-DELETED"})
	carol := principalCtx("carol")

	tests := []struct {
		name string
		typ  string
		plur string
		id   string
	}{
		{"live id of another type", "ticket", "tickets", "FEAT-001"},
		{"live id with an undeclared face", "ticket", "tickets", "TKT-001@bogus"},
		{"successor deleted", "ticket", "tickets", "TKT-DANGLING"},
	}
	for _, tc := range tests {
		t.Run(tc.name+" (entity GET)", func(t *testing.T) {
			rec := getEntityAs(carol, t, app, d, tc.typ, tc.plur, tc.id, "")
			if rec.Code != http.StatusNotFound || rec.Header().Get("Location") != "" {
				t.Errorf("got %d Location %q, want the uniform 404", rec.Code, rec.Header().Get("Location"))
			}
		})
		t.Run(tc.name+" (view)", func(t *testing.T) {
			rec := viewsAs(carol, t, app, d, tc.typ, tc.id)
			if rec.Code == http.StatusTemporaryRedirect || rec.Header().Get("Location") != "" {
				t.Errorf("got %d Location %q, want no redirect", rec.Code, rec.Header().Get("Location"))
			}
		})
	}
}

// failingSuccessors is a [store.RetiredIDs] whose lookup fails.
type failingSuccessors struct{ store.Store }

func (failingSuccessors) Successors(context.Context, []string) (map[string]string, error) {
	return nil, errors.New("retired-id table unavailable")
}

// TestRetiredRedirect_LookupFailureIsAMiss pins that a failed retired-id
// lookup answers as the miss it would have been without the redirect.
func TestRetiredRedirect_LookupFailureIsAMiss(t *testing.T) {
	app, d := newRetiringApp(t)
	vr, err := visibility.NewPolicyReader(ctxRowGate{}, appRedactor(app), failingSuccessors{app.store}, familiesOption(app))
	if err != nil {
		t.Fatal(err)
	}
	app.viewReader = vr
	app.views.viewReader = vr

	if rec := getEntityAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-OLD", ""); rec.Code != http.StatusNotFound {
		t.Errorf("entity GET: got %d, want 404; body=%s", rec.Code, rec.Body)
	}
	never := viewsAs(aliceCtx(), t, app, d, "ticket", "TKT-NEVER")
	old := viewsAs(aliceCtx(), t, app, d, "ticket", "TKT-OLD")
	if old.Code != never.Code {
		t.Errorf("view: got %d, want %d as for a never-existing id", old.Code, never.Code)
	}
}

// TestRetiredRedirect_ViewsInAWorldHiddenSuccessor pins the 422 a world
// answers for a missing entry: a successor the caller cannot read leaves it
// byte for byte, apart from the id.
func TestRetiredRedirect_ViewsInAWorldHiddenSuccessor(t *testing.T) {
	app, d := newRetiringApp(t)
	app.setWorlds(stubWorlds{names: map[string]bool{"published": true}, resolveDefault: true})

	gone := viewsAsQuery(aliceCtx(), t, app, d, "ticket", "TKT-GONE", "world=published")
	never := viewsAsQuery(aliceCtx(), t, app, d, "ticket", "TKT-NEVER", "world=published")
	if gone.Code != never.Code || gone.Header().Get("Location") != "" {
		t.Fatalf("got %d (Location %q), want %d", gone.Code, gone.Header().Get("Location"), never.Code)
	}
	g := stripInstance(t, strings.ReplaceAll(gone.Body.String(), "TKT-GONE", "X"))
	n := stripInstance(t, strings.ReplaceAll(never.Body.String(), "TKT-NEVER", "X"))
	if g != n {
		t.Errorf("bodies differ:\n  hidden successor: %s\n  never existed:    %s", g, n)
	}
}
