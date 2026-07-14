package dataentry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// secretNoteMeta is a two-type, manual-id metamodel for the BUG-ZWTDH9
// cross-type write-escalation regression. Manual id_type + no id_prefix means
// the ID-prefix structural guard does NOT fire, so the ACL/apply layer is the
// only defense against re-typing an existing entity via the request body.
func secretNoteMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"secret": {
				Label:  "Secret",
				IDType: "manual",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string"},
				},
			},
			"note": {
				Label:  "Note",
				IDType: "manual",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string"},
				},
			},
		},
	}
}

// buildSecretNoteApp wires an App over secretNoteMeta with a Declarative ACL
// that both the read gate and the write path (entityManager) share, plus a
// principal resolver that stamps the given user with Tool=sync. Mirrors
// production wiring closely enough to exercise the real /api/sync/ router path.
func buildSecretNoteApp(t *testing.T, policyYAML, user string) *App {
	t.Helper()
	meta := secretNoteMeta()
	cfg := &Config{App: AppConfig{Name: "sec"}}
	app := newAppFromParts(cfg, meta, &fixture{})

	fs := storage.NewMemFS()
	ctx := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)

	policy, err := acl.LoadPolicyBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}

	svc := appbuildtest.New(meta, appbuildtest.WithFS(fs, ctx))
	d, err := acl.NewDeclarative(policy, acl.NewStoreGraph(svc.Store()), svc.Store())
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	// Re-derive the services bundle with the Declarative ACL so the write
	// path (entityManager) and the read gate share the same instance.
	svc = appbuildtest.New(meta, appbuildtest.WithFS(fs, ctx), appbuildtest.WithDeclarative(d))
	rebindApp(app, fs, ctx, svc)
	app.broker = newEventBroker()
	app.acl = d
	app.SetPrincipalResolver(func(*http.Request) principal.Principal {
		return principal.Principal{User: user, Tool: principal.ToolSync}
	})
	return app
}

// syncPutJSON issues a sync PUT through the full production router (so the
// principal-stamp + ACL-attach middleware run), with the given JSON body and
// no If-Match (first-touch of an existing record is exercised via seeding
// through the store so If-Match="" only succeeds for the create branch — here
// we deliberately seed via the store and the record already exists, so the
// handler's precondition path is exercised too).
func syncPutJSON(t *testing.T, app *App, path string, body any, ifMatch string) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	r := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	if ifMatch != "" {
		r.Header.Set("If-Match", ifMatch)
	}
	w := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(w, r)
	return w
}

// TestSyncPut_RejectsTypeChangeOnUpdate is the HTTP-layer BUG-ZWTDH9
// regression. mallory may update notes and read secrets, but not update
// secrets. She PUTs {"type":"note",...} to an existing secret's sync URL.
// Before the fix, the handler authorized OpUpdate against the body type (note)
// and overwrote + re-typed the secret (returning 200). After the fix it is
// rejected 422 with code "type_immutable", and the stored secret is untouched.
func TestSyncPut_RejectsTypeChangeOnUpdate(t *testing.T) {
	app := buildSecretNoteApp(t, `
roles:
  note-editor:
    update: [note]
    read: [note, secret]
assignments:
  mallory: note-editor
`, "mallory")

	// Seed an existing secret directly in the store (bypasses authz).
	seedEntity(app, &entity.Entity{
		ID: "SECRET-1", Type: "secret", Properties: map[string]any{"title": "top secret"},
	})

	// Compute the current hash for a valid If-Match (so the request reaches
	// the apply path, not a 412 precondition failure).
	cur, exists := app.sync.currentEntityHash(context.Background(), "SECRET-1")
	if !exists {
		t.Fatal("seeded secret not found")
	}

	body := syncEntityBody{ID: "SECRET-1", Type: "note", Properties: map[string]any{"title": "pwned"}}
	w := syncPutJSON(t, app, "/api/sync/entities/SECRET-1", body, cur)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("cross-type sync PUT returned %d, want 422 (BUG-ZWTDH9): %s", w.Code, w.Body.String())
	}
	var errResp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if !strings.HasSuffix(errResp.Type, "type_immutable") {
		t.Errorf("error type = %q, want suffix type_immutable; body=%s", errResp.Type, w.Body.String())
	}

	// The stored entity must be UNCHANGED: still a secret, original title.
	got, err := app.store.GetEntity(context.Background(), "SECRET-1")
	if err != nil {
		t.Fatalf("GetEntity(SECRET-1): %v", err)
	}
	if got.Type != "secret" {
		t.Fatalf("stored type was mutated to %q; must remain \"secret\"", got.Type)
	}
	if got.GetString("title") != "top secret" {
		t.Fatalf("stored title was overwritten to %q; must remain \"top secret\"", got.GetString("title"))
	}

	// No note-typed entity was written as a side effect.
	for e, err := range app.store.ListEntities(context.Background(), store.EntityQuery{Type: "note"}) {
		if err == nil && e.Type == "note" {
			t.Fatalf("a note-typed entity was written despite rejection: %s", e.ID)
		}
	}
}

// TestSyncPut_SameTypeUpdateStillWorks pins that the fix leaves a legitimate
// same-type sync update working: mallory updates a note she may edit -> 200,
// and the write lands.
func TestSyncPut_SameTypeUpdateStillWorks(t *testing.T) {
	app := buildSecretNoteApp(t, `
roles:
  note-editor:
    update: [note]
    read: [note, secret]
assignments:
  mallory: note-editor
`, "mallory")

	seedEntity(app, &entity.Entity{
		ID: "NOTE-1", Type: "note", Properties: map[string]any{"title": "v1"},
	})
	cur, exists := app.sync.currentEntityHash(context.Background(), "NOTE-1")
	if !exists {
		t.Fatal("seeded note not found")
	}

	body := syncEntityBody{ID: "NOTE-1", Type: "note", Properties: map[string]any{"title": "v2"}}
	w := syncPutJSON(t, app, "/api/sync/entities/NOTE-1", body, cur)
	if w.Code != http.StatusOK {
		t.Fatalf("legitimate same-type sync PUT returned %d, want 200: %s", w.Code, w.Body.String())
	}
	got, err := app.store.GetEntity(context.Background(), "NOTE-1")
	if err != nil {
		t.Fatalf("GetEntity(NOTE-1): %v", err)
	}
	if got.GetString("title") != "v2" {
		t.Fatalf("same-type update did not land: title = %q", got.GetString("title"))
	}
}
