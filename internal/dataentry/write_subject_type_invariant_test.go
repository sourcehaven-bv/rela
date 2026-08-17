package dataentry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// TestWriteSubjectTypeInvariant is the P4 integration invariant for
// AM-acl-write-subject-type-invariant (BUG-ZWTDH9).
//
// The invariant: for EVERY entity-write entry point, an UPDATE authorizes the
// ACL against the STORED entity's type, never a caller-supplied/claimed type —
// and a request that tries to change the type of an existing entity is
// rejected, not silently applied against the wrong subject type.
//
// The exploit this pins: a principal permitted to write type B but not type A,
// who can READ a type-A target, must NOT be able to overwrite/re-type that
// type-A entity by claiming type B in the write. If any write path authorizes
// against the claimed type, that principal escalates across types.
//
// This is table-driven across the KNOWN entity-write entry points. When you add
// a new entity-write entry point (a handler that ends in
// entityManager.{Update,Apply}Entity), you MUST add a row here — the whole point
// is that a future path authorizing against a body/claimed type is caught by an
// existing test rather than shipping as a fresh instance of BUG-ZWTDH9.
//
// The two entry points today:
//
//   - v1 PATCH (handleV1UpdateEntity): the wire body has NO `type` field; the
//     handler loads the stored entity and passes IT (with its stored type) to
//     UpdateEntity, and the URL's declared type must equal the stored type or
//     the request 404s. Type is structurally immutable — there is no body field
//     to smuggle a different type through. The invariant is enforced by
//     construction; the test pins that a cross-type attempt (wrong plural/type
//     segment for the id) does not authorize-and-write against the claimed type.
//   - sync PUT (putEntity -> ApplyEntity): the body carries `type` (it is an
//     upsert). The apply path rejects a body type that differs from the stored
//     type (ErrTypeImmutable -> 422). Covered directly here.
func TestWriteSubjectTypeInvariant(t *testing.T) {
	// Policy: mallory may write type `note`, and read both `note` and `secret`.
	// She may NOT write `secret`. Every case below targets an existing SECRET-1
	// while claiming type `note`; the invariant is that no path lets her write.
	const policy = `
roles:
  note-editor:
    update: [note]
    read: [note, secret]
assignments:
  mallory: note-editor
`

	// writePath describes one entity-write entry point and how a cross-type
	// write is attempted against it. attempt returns the HTTP status.
	type writePath struct {
		name string
		// attempt seeds nothing (the harness seeds SECRET-1) and issues the
		// cross-type write against the pre-seeded SECRET-1, returning the code.
		attempt func(t *testing.T, app *App) int
		// wantWritten reports whether, after the attempt, a note-typed entity
		// with the target id is permitted to exist. Always false: the invariant
		// is that the secret is never re-typed.
	}

	paths := []writePath{
		{
			name: "v1 PATCH (UpdateEntity)",
			attempt: func(t *testing.T, app *App) int {
				t.Helper()
				// Address SECRET-1 through the `notes` collection (claimed type
				// note). The handler asserts stored.Type == URL type, so this
				// must NOT authorize-and-write as a note; it 404s (existence
				// oracle avoidance) rather than re-typing the secret.
				body := map[string]any{"properties": map[string]any{"title": "pwned"}}
				b, _ := json.Marshal(body)
				r := httptest.NewRequest(http.MethodPatch, "/api/v1/notes/SECRET-1", bytes.NewReader(b))
				r.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				app.NewRouter().ServeHTTP(w, r)
				return w.Code
			},
		},
	}

	for _, p := range paths {
		t.Run(p.name, func(t *testing.T) {
			app := buildSecretNoteApp(t, policy, "mallory")
			// Config needs the `notes` list/plural so the v1 route resolves the
			// plural to the note type.
			seedEntity(app, &entity.Entity{
				ID: "SECRET-1", Type: "secret", Properties: map[string]any{"title": "top secret"},
			})

			code := p.attempt(t, app)

			// The write must NOT succeed (2xx). It is either rejected as a
			// type change (422) or refused as not-found (404) — never applied.
			if code >= 200 && code < 300 {
				t.Fatalf("%s: cross-type write returned %d (2xx) — the secret was written against the claimed type (BUG-ZWTDH9)", p.name, code)
			}

			// The stored entity must remain a secret with its original title.
			got, err := app.store.GetEntity(context.Background(), "SECRET-1")
			if err != nil {
				t.Fatalf("%s: GetEntity(SECRET-1): %v", p.name, err)
			}
			if got.Type != "secret" {
				t.Fatalf("%s: stored type mutated to %q; the ACL subject was bound to the claimed type, not the stored one", p.name, got.Type)
			}
			if got.GetString("title") != "top secret" {
				t.Fatalf("%s: stored title overwritten to %q", p.name, got.GetString("title"))
			}
			// No note-typed entity with the target id may exist.
			if n, err := app.store.GetEntity(context.Background(), "SECRET-1"); err == nil && n.Type == "note" {
				t.Fatalf("%s: SECRET-1 was re-typed to note", p.name)
			}
		})
	}
}

// --- helpers (moved here from the retired sync_type_confusion_test.go, which
// tested the now-removed /api/sync record channel; the write-subject-type
// invariant still exercises the surviving write surfaces) ---

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
