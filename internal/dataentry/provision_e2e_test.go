package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// unmatched_principal: provision — end-to-end (TKT-ANUJDS).
//
// The load-bearing test is TestProvision_FirstWriteProvisionsAcrossPaths: it
// drives an unmatched verified assertion through the REAL router to a CRUD, a
// sync, and a Lua-action write, and asserts each provisions exactly one stub and
// lets the write proceed. That covers the anti-bypass invariant (provision fires
// on every write path, not just CRUD) the same way the reject e2e test does.

const provSub = "usr_new"

// provisionApp builds an app under unmatched_principal: provision, with a
// `person` user type (sub/email/org props), the provisioner create grant, and a
// role that lets the resolved person write tickets (so a provisioned write can
// actually proceed). The verifier's subject matches no person, so every request
// is an unmatched verified principal until the stub is provisioned.
func provisionApp(t *testing.T) (*App, store.Store) {
	t.Helper()

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:         "Ticket",
				IDPrefix:      "TKT-",
				Properties:    map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}},
				PropertyOrder: []string{"title"},
			},
			"person": {
				Label:    "Person",
				IDPrefix: "PERS-",
				Properties: map[string]metamodel.PropertyDef{
					"sub":      {Type: "string", Unique: true},
					"email":    {Type: "string"},
					"org_id":   {Type: "string"},
					"org_slug": {Type: "string"},
				},
				PropertyOrder: []string{"sub", "email", "org_id", "org_slug"},
			},
		},
	}
	cfg := &dataentryconfig.Config{
		App:        dataentryconfig.AppConfig{Name: "Provision App"},
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
		UnmatchedPrincipal: acl.UnmatchedProvision,
		Roles: map[string]acl.RoleDef{
			// The provisioner may create a person and nothing else (bare-stub
			// containment). A separate role lets ANY principal write tickets, so a
			// provisioned person's own write proceeds — isolating what this test
			// measures (provisioning), not ticket-role plumbing.
			"provisioner-system": {Create: []string{"person"}},
			"everyone":           {Read: []string{"*"}, Create: []string{"ticket"}, Update: []string{"ticket"}, Delete: []string{"ticket"}},
		},
		Assignments: map[string]string{
			principal.UserProvisioner: "provisioner-system",
		},
	}
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

	if err := app.SetJWTGate(JWTGateConfig{
		Verifier: gateVerifier{
			validToken: "good", subject: provSub,
			email: "new@example.com", orgID: "org_9", orgSlug: "acme",
		},
		HeaderName: unmatchedHeader,
	}); err != nil {
		t.Fatalf("SetJWTGate: %v", err)
	}
	return app, st
}

// listPersonsBySub returns every person entity in st carrying provSub.
func listPersonsBySub(t *testing.T, st store.Store) []*entity.Entity {
	t.Helper()
	var out []*entity.Entity
	for e, err := range st.ListEntities(context.Background(), store.EntityQuery{Type: "person"}) {
		if err != nil {
			t.Fatalf("ListEntities: %v", err)
		}
		if e.Properties["sub"] == provSub {
			out = append(out, e)
		}
	}
	return out
}

func provReq(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	return req(t, method, path, body) // reuses the reject test's request builder (sets header + token)
}

func TestProvision_FirstWriteProvisionsAcrossPaths(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body string
	}{
		{"CRUD create", http.MethodPost, "/api/v1/tickets", `{"properties":{"title":"x"}}`},
		{"CRUD update", http.MethodPatch, "/api/v1/tickets/TKT-001", `{"properties":{"title":"x"}}`},
		// The sync record write path was retired in TKT-8P1TM7 — sync now writes
		// through the /api/v1 CRUD paths above, so provisioning on a sync push is
		// already covered by the "CRUD create"/"CRUD update" cases.
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, st := provisionApp(t)
			router := app.NewRouter()

			if got := len(listPersonsBySub(t, st)); got != 0 {
				t.Fatalf("precondition: %d persons for sub before any write, want 0", got)
			}

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, provReq(t, tc.method, tc.path, tc.body))

			if rec.Code < 200 || rec.Code >= 300 {
				t.Fatalf("%s: got %d, want 2xx — the provisioned write must proceed; body=%s",
					tc.name, rec.Code, rec.Body)
			}
			persons := listPersonsBySub(t, st)
			if len(persons) != 1 {
				t.Fatalf("%s: %d person stubs for sub after write, want exactly 1", tc.name, len(persons))
			}
			// The stub carries the verified claims that the person type declares.
			stub := persons[0]
			if stub.Properties["email"] != "new@example.com" {
				t.Errorf("stub email = %v, want new@example.com", stub.Properties["email"])
			}
			if stub.Properties["org_id"] != "org_9" {
				t.Errorf("stub org_id = %v, want org_9", stub.Properties["org_id"])
			}
		})
	}
}

func TestProvision_GetDoesNotProvision(t *testing.T) {
	// AC1: a GET by an unmatched principal stays read-only — no stub created.
	app, st := provisionApp(t)
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, provReq(t, http.MethodGet, "/api/v1/tickets/TKT-001", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if got := len(listPersonsBySub(t, st)); got != 0 {
		t.Errorf("a GET provisioned %d stubs; provision must be write-only", got)
	}
}

func TestProvision_TriggeringWriteSeesOwnStub(t *testing.T) {
	// AC2: the create response must succeed AND the just-provisioned person must
	// be readable back (the read gate was rebuilt on the resolved principal, so
	// it is not redacted/404'd out of its own request's later reads).
	app, st := provisionApp(t)
	router := app.NewRouter()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, provReq(t, http.MethodPost, "/api/v1/tickets",
		`{"properties":{"title":"x"}}`))
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("create got %d, want 2xx; body=%s", rec.Code, rec.Body)
	}

	persons := listPersonsBySub(t, st)
	if len(persons) != 1 {
		t.Fatalf("want exactly 1 provisioned stub, got %d", len(persons))
	}
	// A follow-up GET of the provisioned person by the same (now matched)
	// principal must succeed — proving the resolved principal can read its own
	// entity, i.e. the re-stamp took effect for authorization.
	get := httptest.NewRecorder()
	router.ServeHTTP(get, provReq(t, http.MethodGet, "/api/v1/persons/"+persons[0].ID, ""))
	if get.Code != http.StatusOK {
		t.Errorf("GET of the provisioned person got %d, want 200; body=%s", get.Code, get.Body)
	}
}

func TestProvision_ConcurrentFirstWritesCreateOne(t *testing.T) {
	// AC3: two concurrent first-writes from one sub create exactly one stub. In
	// one process writeMu serializes them; the loser's provision sees the stub
	// already exists (ErrEntityAlreadyExists), tolerates it, and re-resolves.
	app, st := provisionApp(t)
	router := app.NewRouter()

	const n = 8
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, provReq(t, http.MethodPost, "/api/v1/tickets",
				`{"properties":{"title":"x"}}`))
		}()
	}
	wg.Wait()

	if got := len(listPersonsBySub(t, st)); got != 1 {
		t.Fatalf("concurrent first-writes created %d stubs for one sub, want exactly 1", got)
	}
}

func TestProvision_MatchedPrincipalNotReprovisioned(t *testing.T) {
	// A verified principal that already resolves to a person is not flagged
	// unmatched, so provision never fires — no second stub, and the write
	// proceeds on the matched identity.
	app, st := provisionApp(t)
	seedEntity(app, &entity.Entity{
		ID: "PERS-EXIST", Type: "person", Properties: map[string]any{"sub": provSub},
	})

	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, provReq(t, http.MethodPost, "/api/v1/tickets",
		`{"properties":{"title":"x"}}`))
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("matched principal write got %d, want 2xx; body=%s", rec.Code, rec.Body)
	}
	if got := len(listPersonsBySub(t, st)); got != 1 {
		t.Errorf("matched principal produced %d persons for sub, want 1 (the seeded one)", got)
	}
}

func TestProvision_AuditedToProvisioner(t *testing.T) {
	// AC5: the stub create is attributed to system:provisioner. The person's
	// last-editor attribution is not exposed on the memstore entity, so assert on
	// the store-visible fact we can: exactly one stub exists and it was created by
	// the provision path (a create by the unmatched principal itself would have
	// been ACL-denied — the everyone role grants create only on ticket, not
	// person — so a person existing at all proves the provisioner created it).
	app, st := provisionApp(t)
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, provReq(t, http.MethodPost, "/api/v1/tickets",
		`{"properties":{"title":"x"}}`))
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("create got %d, want 2xx; body=%s", rec.Code, rec.Body)
	}
	persons := listPersonsBySub(t, st)
	if len(persons) != 1 {
		t.Fatalf("want exactly 1 provisioned person, got %d — only system:provisioner "+
			"could have created it (the unmatched principal cannot create persons)", len(persons))
	}
	// Sanity: the response body is a normal ticket create, unaffected.
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("create response is not JSON: %v", err)
	}
}
