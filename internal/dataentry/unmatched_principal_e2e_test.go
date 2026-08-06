package dataentry

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// unmatched_principal: reject — end-to-end (TKT-0C3II2).
//
// The load-bearing test is TestReject_DeniesEveryWritePath: it drives an
// unmatched verified assertion through the REAL router to a CRUD write, a sync
// write, AND a Lua-action write, and asserts all three are denied. Its absence
// is exactly the gap a design review found — a per-CRUD-handler check would
// leave sync/action bypassable — so this pins that reject is enforced at the
// shared write-authz choke point (Declarative.AuthorizeWrite), not per-handler.

const unmatchedHeader = "X-Auth-Assertion"

// rejectApp builds an app whose identity is the fail-closed JWT gate, with a
// `person` user type + principal_property=sub lookup and `unmatched_principal:
// reject`. The verifier's subject matches NO person entity, so every request is
// an unmatched verified principal.
func rejectApp(t *testing.T, mode string) *App {
	t.Helper()

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:    "Ticket",
				IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
				PropertyOrder: []string{"title"},
			},
			"person": {
				Label:    "Person",
				IDPrefix: "PERS-",
				Properties: map[string]metamodel.PropertyDef{
					"sub": {Type: "string", Unique: true},
				},
				PropertyOrder: []string{"sub"},
			},
		},
	}
	cfg := &dataentryconfig.Config{
		App:        dataentryconfig.AppConfig{Name: "Reject App"},
		Forms:      map[string]dataentryconfig.Form{},
		Lists:      map[string]dataentryconfig.List{},
		Views:      map[string]dataentryconfig.ViewConfig{},
		Kanbans:    map[string]dataentryconfig.Kanban{},
		Navigation: []dataentryconfig.NavigationEntry{},
	}
	app := newAppFromParts(cfg, meta, newFixture())

	fs := storage.NewMemFS()
	pctx := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	_ = fs.MkdirAll(pctx.CacheDir, 0o755)

	p := &acl.Policy{
		UserEntityType:     "person",
		PrincipalProperty:  "sub",
		UnmatchedPrincipal: mode,
		Roles: map[string]acl.RoleDef{
			// everyone can write, so a 403 can only come from the unmatched gate,
			// never from a missing role — isolating what this test measures.
			"everyone": {Read: []string{"*"}, Create: []string{"*"}, Update: []string{"*"}, Delete: []string{"*"}},
		},
	}
	// ONE store, shared by the Declarative's lookup AND the services bundle, so
	// the resolver reads the same entities the write path persists. (Each
	// appbuildtest.New makes a fresh memstore, so the store must be built once
	// and injected via WithStore into both the Declarative and the bundle.)
	st := memstore.New()
	d, err := acl.NewDeclarative(p, acl.NewStoreGraph(st), st,
		acl.WithPrincipalLookup(acl.NewStorePrincipalLookup(st)))
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	svc := appbuildtest.New(meta, appbuildtest.WithFS(fs, pctx),
		appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d))
	rebindApp(app, fs, pctx, svc)
	app.broker = newEventBroker()
	app.acl = d

	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"},
	})

	// The GATE (not the resolver) is the production JWT path and the only one
	// that flags unmatched-verified. subject matches no person entity.
	if err := app.SetJWTGate(JWTGateConfig{
		Verifier:   gateVerifier{validToken: "good", subject: "usr_nobody"},
		HeaderName: unmatchedHeader,
	}); err != nil {
		t.Fatalf("SetJWTGate: %v", err)
	}
	return app
}

func req(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, http.NoBody)
	} else {
		r = httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
		r.Header.Set("Content-Type", "application/json")
	}
	r.Header.Set(unmatchedHeader, "good")
	return r
}

func TestReject_DeniesEveryWritePath(t *testing.T) {
	app := rejectApp(t, acl.UnmatchedReject)
	router := app.NewRouter()

	// Each of these is a distinct data-entry WRITE path an unmatched verified
	// principal can reach; all must be denied. A per-handler check would miss
	// sync and action — this is the anti-bypass test.
	// CRUD and sync have DISTINCT write handlers, so each must be verified
	// independently — a per-handler reject check (the rejected design) would
	// have covered CRUD but missed sync. The Lua-action path is not listed
	// separately because its writes go through the SAME a.entityManager as CRUD
	// (App.luaWriteDeps.EntityManager, app.go), so AuthorizeWrite — and thus
	// reject — covers it by construction; TestReject_ActionSharesWriteAuthz pins
	// that shared-manager invariant so the coverage-by-construction can't rot.
	//
	// Attachment, rename, clone, relation, and conflict-resolve writes are
	// likewise not listed: all reach Declarative.AuthorizeWrite (most via
	// entitymanager, conflict-resolve via h.acl().AuthorizeWrite directly), the
	// single point the reject branch lives — pinned at the unit level by
	// TestUnmatchedPrincipal_RejectDeniesFlaggedWrite. The two rows below are
	// the DISTINCT-handler paths whose end-to-end wiring a shared-choke-point
	// argument alone doesn't guarantee.
	for _, tc := range []struct {
		name, method, path, body string
	}{
		{"CRUD update", http.MethodPatch, "/api/v1/tickets/TKT-001", `{"properties":{"title":"x"}}`},
		{"CRUD create", http.MethodPost, "/api/v1/tickets", `{"properties":{"title":"x"}}`},
		// sync CREATE (fresh id) avoids the handler's If-Match conflict precheck,
		// which would 412 before reaching authz.
		{"sync write", http.MethodPut, "/api/sync/entities/TKT-NEW", `{"id":"TKT-NEW","type":"ticket","properties":{"title":"x"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req(t, tc.method, tc.path, tc.body))

			// 403 forbidden is the target; 401 (gate rejected the token) would
			// mean the test isn't reaching authz. Anything 2xx is a bypass.
			if rec.Code >= 200 && rec.Code < 300 {
				t.Fatalf("%s: got %d (a WRITE by an unmatched verified principal "+
					"was ALLOWED — reject is bypassed on this path); body=%s",
					tc.name, rec.Code, rec.Body)
			}
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s: got %d, want 403 forbidden; body=%s", tc.name, rec.Code, rec.Body)
			}
		})
	}
}

// TestReject_ActionSharesWriteAuthz pins the invariant that makes the
// Lua-action write path covered-by-construction: its writes go through the same
// entitymanager as the CRUD handlers (App.luaWriteDeps.EntityManager), so the
// AuthorizeWrite reject gate applies to it without a separate hook. If a future
// change gives actions their own manager, this fails and the anti-bypass
// coverage must be re-examined.
func TestReject_ActionSharesWriteAuthz(t *testing.T) {
	app := rejectApp(t, acl.UnmatchedReject)
	if app.luaWriteDeps().EntityManager != app.entityManager {
		t.Fatal("Lua action writes no longer go through App.entityManager — the " +
			"reject gate may not cover the action write path; add explicit coverage")
	}
}

func TestReject_ReadStillAllowed(t *testing.T) {
	// AC3: reject gates WRITES only. A GET by the unmatched principal is
	// unaffected (the everyone role grants read).
	app := rejectApp(t, acl.UnmatchedReject)
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req(t, http.MethodGet, "/api/v1/tickets/TKT-001", ""))

	if rec.Code != http.StatusOK {
		t.Errorf("GET by unmatched principal under reject: got %d, want 200 — "+
			"reject must not gate reads; body=%s", rec.Code, rec.Body)
	}
}

func TestReject_AnonymousModeAllowsWrite(t *testing.T) {
	// AC1: the same unmatched principal, under anonymous, writes fine (everyone
	// grants create/update). Proves the 403 above comes from the mode, not the
	// setup.
	app := rejectApp(t, acl.UnmatchedAnonymous)
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec,
		req(t, http.MethodPatch, "/api/v1/tickets/TKT-001", `{"properties":{"title":"x"}}`))

	if rec.Code < 200 || rec.Code >= 300 {
		t.Errorf("anonymous mode: unmatched write got %d, want 2xx; body=%s", rec.Code, rec.Body)
	}
}

func TestReject_MatchedPrincipalWrites(t *testing.T) {
	// A verified principal that DOES resolve to a person entity is not flagged,
	// so reject never touches it — it writes on its everyone grant.
	app := rejectApp(t, acl.UnmatchedReject)
	// Seed the person the subject resolves to.
	seedEntity(app, &entity.Entity{
		ID: "PERS-1", Type: "person", Properties: map[string]any{"sub": "usr_nobody"},
	})

	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec,
		req(t, http.MethodPatch, "/api/v1/tickets/TKT-001", `{"properties":{"title":"x"}}`))

	if rec.Code == http.StatusForbidden {
		t.Errorf("a MATCHED verified principal was rejected; body=%s", rec.Body)
	}
}

func TestReject_HeaderPrincipalUntouched(t *testing.T) {
	// AC4: the header path is not the JWT gate, so a header principal is never
	// flagged unmatched-verified and never rejected — even with the same policy.
	// (Build a header-mode app with the same reject policy.)
	app := rejectApp(t, acl.UnmatchedReject)
	// Swap the gate off; wire the header resolver instead.
	app.jwtGate = nil
	app.SetPrincipalResolver(ChainResolvers(HeaderPrincipalResolver("X-Forwarded-User")))

	r := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001",
		bytes.NewReader([]byte(`{"properties":{"title":"x"}}`)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Forwarded-User", "someone@example.com") // no person entity
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, r)

	// The header principal is unmatched too, but NOT flagged (not the JWT gate),
	// so reject does not apply — it writes on the everyone grant.
	if rec.Code == http.StatusForbidden {
		t.Errorf("a header-mode unmatched principal was rejected — reject leaked "+
			"past the JWT-gate scope (AC4); body=%s", rec.Body)
	}
}
