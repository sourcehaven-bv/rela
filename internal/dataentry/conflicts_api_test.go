package dataentry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

const conflictedTicket = `<<<<<<< HEAD
---
id: TKT-001
type: ticket
title: Ours
---
=======
---
id: TKT-001
type: ticket
title: Theirs
---
>>>>>>> incoming
Body text.
`

const conflictedRelation = `<<<<<<< HEAD
---
from: TKT-001
relation: blocks
to: TKT-002
note: ours
---
=======
---
from: TKT-001
relation: blocks
to: TKT-002
note: theirs
---
>>>>>>> incoming
`

// newConflictTestApp builds an app bound to a real on-disk project root
// — the conflict endpoints read and write project files directly.
func newConflictTestApp(t *testing.T) (app *App, root string) {
	t.Helper()
	app = newTestAppV1(t)
	root = t.TempDir()
	bindRepo(app, root)
	app.paths.EntitiesDir = filepath.Join(root, "entities")
	app.paths.RelationsDir = filepath.Join(root, "relations")
	return app, root
}

// newConflictACLApp is newConflictTestApp with the policy wired into the
// entity manager as well as the handlers, as production wires one ACL into
// both. seed runs before the policy is built. The returned sink records the
// manager's audit rows and the handler's.
func newConflictACLApp(
	t *testing.T, p *acl.Policy, seed func(app *App),
) (app *App, root string, d *acl.Declarative, sink *audit.Memory) {
	t.Helper()
	app, root = newConflictTestApp(t)
	if seed != nil {
		seed(app)
	}
	d = mustNewACL(t, p, app.store)
	sink = audit.NewMemory()
	svc := appbuildtest.New(app.State().Meta, appbuildtest.WithStore(app.store),
		appbuildtest.WithFS(app.fs, app.paths), appbuildtest.WithACL(d), appbuildtest.WithAudit(sink))
	rebindApp(app, app.fs, app.paths, svc)
	setTestACL(t, app, d)
	return app, root, d, sink
}

func writeProjectFile(t *testing.T, root, rel, content string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func postConflictResolve(app *App, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/_conflicts/resolve", strings.NewReader(body))
	rec := httptest.NewRecorder()
	app.write.handleV1ConflictResolve(rec, req)
	return rec
}

func TestV1ConflictDetailPathTraversal(t *testing.T) {
	app, root := newConflictTestApp(t)
	// A file outside the project root that a traversal could read.
	secret := filepath.Join(filepath.Dir(root), "secret.md")
	if err := os.WriteFile(secret, []byte("top secret"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{"relative traversal to existing file", "../secret.md"},
		{"relative traversal to missing file", "../../nope.md"},
		{"absolute path", "/etc/passwd"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/_conflicts/"+tc.path, http.NoBody)
			rec := httptest.NewRecorder()
			app.handleV1ConflictRoutes(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("expected 403, got %d: %s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "top secret") {
				t.Error("response leaked file content from outside the project")
			}
		})
	}
}

func TestV1ConflictDetailContainedPath(t *testing.T) {
	app, root := newConflictTestApp(t)
	writeProjectFile(t, root, "entities/ticket/TKT-001.md", conflictedTicket)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_conflicts/entities/ticket/TKT-001.md", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ConflictRoutes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "TKT-001") {
		t.Errorf("expected detail for TKT-001, got: %s", rec.Body.String())
	}
}

func TestV1ConflictDetailMissingFile(t *testing.T) {
	app, _ := newConflictTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_conflicts/entities/ticket/MISSING.md", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ConflictRoutes(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestV1ConflictResolvePathTraversal(t *testing.T) {
	app, root := newConflictTestApp(t)
	// An existing conflicted file outside the project root: without
	// containment the resolve endpoint would happily rewrite it.
	outside := filepath.Join(filepath.Dir(root), "outside.md")
	if err := os.WriteFile(outside, []byte(conflictedTicket), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rec := postConflictResolve(app, `{"path":"../outside.md","content_choice":"theirs"}`)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	got, err := os.ReadFile(outside)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != conflictedTicket {
		t.Error("file outside the project root was modified")
	}
}

// TestV1ConflictResolveACLDenied pins that git:sync does not replace the
// write check: alice may sync but may not update tickets, so the resolve is
// refused by the write check and audited as a denied write.
func TestV1ConflictResolveACLDenied(t *testing.T) {
	app, root, d, sink := newConflictACLApp(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{"syncer": {
			Read: []string{"ticket"}, Permissions: []string{acl.PermGitSync},
		}},
		Assignments: map[string]string{"alice": "syncer"},
	}, nil)
	path := writeProjectFile(t, root, "entities/ticket/TKT-001.md", conflictedTicket)

	req := asAlice(t, d, httptest.NewRequest(http.MethodPost, "/api/v1/_conflicts/resolve",
		strings.NewReader(`{"path":"entities/ticket/TKT-001.md","content_choice":"ours"}`)))
	rec := httptest.NewRecorder()
	app.write.handleV1ConflictResolve(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "role-grant") {
		t.Errorf("expected structured ACL deny body from the write check, got: %s", rec.Body.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != conflictedTicket {
		t.Error("denied resolve still modified the file")
	}

	var denied bool
	for _, r := range sink.Records() {
		if r.Op == audit.OpDeniedWrite && r.Subject != nil && r.Subject.ID == "TKT-001" {
			denied = true
		}
	}
	if !denied {
		t.Errorf("expected a denied-write audit record for TKT-001, got %+v", sink.Records())
	}
}

func TestV1ConflictResolveEntityWritesAndAudits(t *testing.T) {
	app, root := newConflictTestApp(t)
	path := writeProjectFile(t, root, "entities/ticket/TKT-001.md", conflictedTicket)
	sink := audit.NewMemory()
	app.auditSink = sink

	rec := postConflictResolve(app,
		`{"path":"entities/ticket/TKT-001.md","property_choices":{"title":"theirs"},"content_choice":"theirs"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(got), "<<<<<<<") {
		t.Errorf("resolved file still contains conflict markers:\n%s", got)
	}
	if !strings.Contains(string(got), "Theirs") {
		t.Errorf("expected theirs-side title in resolved file, got:\n%s", got)
	}

	var audited bool
	for _, r := range sink.Records() {
		if r.Op == audit.OpUpdateEntity && r.Subject != nil &&
			r.Subject.Kind == "entity" && r.Subject.ID == "TKT-001" {

			audited = true
		}
	}
	if !audited {
		t.Errorf("expected an update-entity audit record for TKT-001, got %+v", sink.Records())
	}
}

func TestV1ConflictResolveRelationWritesAndAudits(t *testing.T) {
	app, root := newConflictTestApp(t)
	// The read gate needs both endpoints: an edge is readable only when they are.
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "a"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "b"}})
	path := writeProjectFile(t, root, "relations/TKT-001--blocks--TKT-002.md", conflictedRelation)
	sink := audit.NewMemory()
	app.auditSink = sink

	rec := postConflictResolve(app,
		`{"path":"relations/TKT-001--blocks--TKT-002.md","content_choice":"ours"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(got), "<<<<<<<") {
		t.Errorf("resolved file still contains conflict markers:\n%s", got)
	}

	var audited bool
	for _, r := range sink.Records() {
		if r.Op == audit.OpUpdateRelation && r.Subject != nil &&
			r.Subject.Kind == "relation" && r.Subject.RelationType == "blocks" {

			audited = true
		}
	}
	if !audited {
		t.Errorf("expected an update-relation audit record, got %+v", sink.Records())
	}
}

// TestV1Conflicts_GitSyncGate pins that every conflict route needs git:sync,
// under the same rule as the sync itself. A refused list or detail must not
// name the conflicted file, and a refused resolve must not touch it.
func TestV1Conflicts_GitSyncGate(t *testing.T) {
	const rel = "entities/ticket/TKT-001.md"
	routes := []struct {
		name string
		call func(app *App, ctx context.Context) *httptest.ResponseRecorder
	}{
		{"list", func(app *App, ctx context.Context) *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			app.handleV1Conflicts(rec, httptest.NewRequestWithContext(ctx, http.MethodGet,
				"/api/v1/_conflicts", http.NoBody))
			return rec
		}},
		{"detail", func(app *App, ctx context.Context) *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			app.handleV1ConflictRoutes(rec, httptest.NewRequestWithContext(ctx, http.MethodGet,
				"/api/v1/_conflicts/"+rel, http.NoBody))
			return rec
		}},
		// A missing file must get the same 403 as a present one, or the
		// answer would tell a caller without git:sync which files exist.
		{"detail of a missing file", func(app *App, ctx context.Context) *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			app.handleV1ConflictRoutes(rec, httptest.NewRequestWithContext(ctx, http.MethodGet,
				"/api/v1/_conflicts/entities/ticket/NOPE.md", http.NoBody))
			return rec
		}},
		{"resolve of a missing file", func(app *App, ctx context.Context) *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			app.handleV1ConflictRoutes(rec, httptest.NewRequestWithContext(ctx, http.MethodPost,
				"/api/v1/_conflicts/resolve", strings.NewReader(`{"path":"entities/ticket/NOPE.md"}`)))
			return rec
		}},
		{"resolve", func(app *App, ctx context.Context) *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			app.handleV1ConflictRoutes(rec, httptest.NewRequestWithContext(ctx, http.MethodPost,
				"/api/v1/_conflicts/resolve", strings.NewReader(`{"path":"`+rel+`","content_choice":"theirs"}`)))
			return rec
		}},
	}
	modes := []struct {
		name    string
		acl     string // "nop", "read-only", "policy", "broken" or "unwired"
		holds   bool
		allowed bool
	}{
		{name: "no policy", acl: "nop", allowed: true},
		{name: "read-only", acl: "read-only", holds: true},
		{name: "policy without git:sync", acl: "policy"},
		{name: "policy with git:sync", acl: "policy", holds: true, allowed: true},
		{name: "broken policy", acl: "broken", holds: true},
		{name: "unwired", acl: "unwired", holds: true},
	}
	for _, route := range routes {
		for _, mode := range modes {
			t.Run(route.name+"/"+mode.name, func(t *testing.T) {
				app, root := newConflictTestApp(t)
				path := writeProjectFile(t, root, rel, conflictedTicket)
				switch mode.acl {
				case "nop":
					setTestACL(t, app, acl.NopACL{})
				case "read-only":
					setTestACL(t, app, acl.ReadOnlyACL{})
				case "policy":
					setTestACL(t, app, mustNewACL(t, &acl.Policy{}, app.store))
				case "broken":
					setTestACL(t, app, (*acl.Declarative)(nil))
				case "unwired":
					setTestACL(t, app, nil)
				}
				ctx := withReadGate(context.Background(), fakeGate{holdsPermission: mode.holds})

				rec := route.call(app, ctx)

				refusedForSync := rec.Code == http.StatusForbidden &&
					strings.Contains(rec.Body.String(), acl.PermGitSync)
				if refusedForSync == mode.allowed {
					t.Fatalf("allowed=%v, got %d: %s", mode.allowed, rec.Code, rec.Body.String())
				}
				if mode.allowed {
					return
				}
				// The problem body's instance echoes the path the caller sent;
				// only the rest may not name the file.
				var body map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode: %v", err)
				}
				delete(body, "instance")
				if strings.Contains(fmt.Sprint(body), "TKT-001") {
					t.Errorf("refused response names the conflicted file: %s", rec.Body.String())
				}
				got, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("ReadFile: %v", err)
				}
				if string(got) != conflictedTicket {
					t.Error("refused request modified the conflicted file")
				}
			})
		}
	}
}

// conflictReadGateApp builds a conflict app for alice, who may sync and read
// tickets but not features. She may update both, so only the read gate can
// keep the feature's conflict from her. The project holds a conflicted
// ticket, a conflicted feature and a conflicted edge to the feature.
func conflictReadGateApp(t *testing.T) (app *App, root string, d *acl.Declarative, sink *audit.Memory) {
	t.Helper()
	app, root, d, sink = newConflictACLApp(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{"syncer": {
			Read: []string{"ticket"}, Update: []string{"ticket", "feature"},
			Permissions: []string{acl.PermGitSync},
		}},
		Assignments: map[string]string{"alice": "syncer"},
	}, func(app *App) {
		seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "t"}})
		seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "f"}})
	})
	writeProjectFile(t, root, "entities/ticket/TKT-001.md", conflictedTicket)
	writeProjectFile(t, root, "entities/feature/FEAT-001.md", conflictedFeature)
	writeProjectFile(t, root, "relations/TKT-001--implements--FEAT-001.md", conflictedImplements)
	return app, root, d, sink
}

var conflictedFeature = strings.NewReplacer("TKT-001", "FEAT-001", "type: ticket", "type: feature",
	"title: Ours", "title: Secret ours", "title: Theirs", "title: Secret theirs").Replace(conflictedTicket)

var conflictedImplements = strings.NewReplacer("TKT-002", "FEAT-001", "blocks", "implements",
	"note: ours", "note: secret ours", "note: theirs", "note: secret theirs").Replace(conflictedRelation)

func conflictDetailAs(t *testing.T, app *App, d *acl.Declarative, rel string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	app.handleV1ConflictRoutes(rec, asAlice(t, d,
		httptest.NewRequest(http.MethodGet, "/api/v1/_conflicts/"+rel, http.NoBody)))
	return rec
}

// problemWithoutInstance decodes a problem body and drops the instance,
// which echoes the path the caller sent.
func problemWithoutInstance(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v: %s", err, rec.Body)
	}
	delete(body, "instance")
	return body
}

// TestV1Conflicts_ReadGate pins BUG-Q3Z15V: the conflict list leaves out a
// file naming an entity or edge endpoint the caller may not read, and its
// detail is the same 404 as a missing file, with none of its values.
func TestV1Conflicts_ReadGate(t *testing.T) {
	app, _, d, _ := conflictReadGateApp(t)

	rec := httptest.NewRecorder()
	app.handleV1Conflicts(rec, asAlice(t, d, httptest.NewRequest(http.MethodGet, "/api/v1/_conflicts", http.NoBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", rec.Code, rec.Body)
	}
	var list v1.ConflictsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var paths []string
	for _, c := range list.Conflicts {
		paths = append(paths, filepath.ToSlash(c.Path))
	}
	if want := []string{"entities/ticket/TKT-001.md"}; fmt.Sprint(paths) != fmt.Sprint(want) || list.Count != 1 {
		t.Errorf("listed %v (count %d), want only %v", paths, list.Count, want)
	}

	if got := conflictDetailAs(t, app, d, "entities/ticket/TKT-001.md"); got.Code != http.StatusOK {
		t.Errorf("readable detail = %d: %s", got.Code, got.Body)
	}
	missing := conflictDetailAs(t, app, d, "entities/feature/NOPE.md")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing detail = %d: %s", missing.Code, missing.Body)
	}
	for _, rel := range []string{"entities/feature/FEAT-001.md", "relations/TKT-001--implements--FEAT-001.md"} {
		t.Run(rel, func(t *testing.T) {
			got := conflictDetailAs(t, app, d, rel)
			if got.Code != http.StatusNotFound {
				t.Fatalf("hidden detail = %d: %s", got.Code, got.Body)
			}
			if strings.Contains(got.Body.String(), "ecret") {
				t.Errorf("hidden detail leaked a value: %s", got.Body)
			}
			if a, b := problemWithoutInstance(t, got), problemWithoutInstance(t, missing); fmt.Sprint(a) != fmt.Sprint(b) {
				t.Errorf("hidden detail %v differs from a missing file's %v", a, b)
			}
		})
	}
}

// TestV1ConflictResolve_HiddenRowIs404 pins BUG-0E6S7Q: alice may update
// features but not read them, so resolving a feature's conflict is the same
// 404 as a missing file, and the file is left alone.
func TestV1ConflictResolve_HiddenRowIs404(t *testing.T) {
	app, root, d, sink := conflictReadGateApp(t)

	rec := httptest.NewRecorder()
	app.write.handleV1ConflictResolve(rec, asAlice(t, d, httptest.NewRequest(http.MethodPost,
		"/api/v1/_conflicts/resolve",
		strings.NewReader(`{"path":"entities/feature/FEAT-001.md","content_choice":"theirs"}`))))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("resolve = %d, want 404: %s", rec.Code, rec.Body)
	}
	got, err := os.ReadFile(filepath.Join(root, "entities", "feature", "FEAT-001.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != conflictedFeature {
		t.Error("resolve of a hidden row rewrote the file")
	}
	for _, r := range sink.Records() {
		if r.Op == audit.OpUpdateEntity {
			t.Errorf("hidden resolve audited a write: %+v", r)
		}
	}

	// The readable ticket still resolves through the same gate.
	rec = httptest.NewRecorder()
	app.write.handleV1ConflictResolve(rec, asAlice(t, d, httptest.NewRequest(http.MethodPost,
		"/api/v1/_conflicts/resolve",
		strings.NewReader(`{"path":"entities/ticket/TKT-001.md","content_choice":"theirs"}`))))
	if rec.Code != http.StatusOK {
		t.Errorf("readable resolve = %d: %s", rec.Code, rec.Body)
	}
}

// TestV1ConflictResolve_IDMustMatchFile pins that resolve authorizes the row
// the file write replaces: a file whose sides name another id is refused.
func TestV1ConflictResolve_IDMustMatchFile(t *testing.T) {
	app, root := newConflictTestApp(t)
	path := writeProjectFile(t, root, "entities/ticket/TKT-009.md", conflictedTicket)

	rec := postConflictResolve(app, `{"path":"entities/ticket/TKT-009.md","content_choice":"theirs"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("resolve = %d, want 422: %s", rec.Code, rec.Body)
	}
	if got, _ := os.ReadFile(path); string(got) != conflictedTicket {
		t.Error("refused resolve rewrote the file")
	}
}

// TestV1ConflictDetail_RedactsHiddenProperties pins the field half of
// BUG-Q3Z15V: a property `visible:` hides is served from neither side.
func TestV1ConflictDetail_RedactsHiddenProperties(t *testing.T) {
	app, root := newConflictTestApp(t)
	setTestResolver(t, app, fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"title": false}}})
	writeProjectFile(t, root, "entities/ticket/TKT-001.md", conflictedTicket)

	rec := httptest.NewRecorder()
	app.handleV1ConflictRoutes(rec,
		httptest.NewRequest(http.MethodGet, "/api/v1/_conflicts/entities/ticket/TKT-001.md", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("detail = %d: %s", rec.Code, rec.Body)
	}
	var detail v1.ConflictDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, diff := range detail.PropertyDiffs {
		if diff.Property == "title" {
			t.Errorf("hidden property served: %+v", diff)
		}
	}
}

// TestV1Conflicts_HiddenAnswersMatchMissing pins that no answer of the
// detail or the resolve tells a hidden file from a missing one: not a file
// without markers, not a conflicted file whose side names a hidden entity.
func TestV1Conflicts_HiddenAnswersMatchMissing(t *testing.T) {
	app, root, d, _ := conflictReadGateApp(t)
	writeProjectFile(t, root, "entities/feature/FEAT-002.md", "---\nid: FEAT-002\ntype: feature\ntitle: x\n---\n")
	// A ticket file whose theirs side claims the hidden feature's id.
	writeProjectFile(t, root, "entities/ticket/TKT-003.md", strings.Replace(
		strings.ReplaceAll(conflictedTicket, "TKT-001", "TKT-003"),
		"id: TKT-003\ntype: ticket\ntitle: Theirs", "id: FEAT-001\ntype: feature\ntitle: Theirs", 1))

	resolve := func(rel string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		app.write.handleV1ConflictResolve(rec, asAlice(t, d, httptest.NewRequest(http.MethodPost,
			"/api/v1/_conflicts/resolve", strings.NewReader(`{"path":"`+rel+`","content_choice":"theirs"}`))))
		return rec
	}
	detail := func(rel string) *httptest.ResponseRecorder { return conflictDetailAs(t, app, d, rel) }
	const missing = "entities/feature/NOPE.md"
	for _, rel := range []string{"entities/feature/FEAT-002.md", "entities/ticket/TKT-003.md"} {
		for name, call := range map[string]func(string) *httptest.ResponseRecorder{
			"detail": detail, "resolve": resolve,
		} {
			t.Run(name+"/"+rel, func(t *testing.T) {
				got, want := call(rel), call(missing)
				if got.Code != want.Code ||
					fmt.Sprint(problemWithoutInstance(t, got)) != fmt.Sprint(problemWithoutInstance(t, want)) {

					t.Errorf("got %d %s, want the missing file's %d %s", got.Code, got.Body, want.Code, want.Body)
				}
			})
		}
	}
}

// TestV1ConflictResolve_FileIdentity pins that resolve authorizes the row
// or edge the file write replaces: a side naming another type or edge is
// refused, and a file outside the entity and relation directories is not a
// conflict the routes serve.
func TestV1ConflictResolve_FileIdentity(t *testing.T) {
	app, root := newConflictTestApp(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "a"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "b"}})
	writeProjectFile(t, root, "entities/feature/TKT-001.md", conflictedTicket)
	writeProjectFile(t, root, "relations/TKT-001--implements--TKT-002.md", conflictedRelation)
	writeProjectFile(t, root, "docs/TKT-001.md", conflictedTicket)

	for _, tc := range []struct {
		path string
		want int
	}{
		{"entities/feature/TKT-001.md", http.StatusUnprocessableEntity},
		{"relations/TKT-001--implements--TKT-002.md", http.StatusUnprocessableEntity},
		{"docs/TKT-001.md", http.StatusNotFound},
	} {
		t.Run(tc.path, func(t *testing.T) {
			path := filepath.Join(root, filepath.FromSlash(tc.path))
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			rec := postConflictResolve(app, `{"path":"`+tc.path+`","content_choice":"theirs"}`)
			if rec.Code != tc.want {
				t.Fatalf("resolve = %d, want %d: %s", rec.Code, tc.want, rec.Body)
			}
			if after, _ := os.ReadFile(path); !bytes.Equal(after, before) {
				t.Error("refused resolve rewrote the file")
			}
		})
	}
}

// TestV1Conflicts_KindFollowsDirectory pins that the parser and the gate
// agree on whether a file is an entity or a relation. An entity file named
// like a relation answers the missing file's 404, since no entity id holds
// "--". A "--" in a directory name above the file does not make an entity
// file a relation.
func TestV1Conflicts_KindFollowsDirectory(t *testing.T) {
	t.Run("entity file named like a relation", func(t *testing.T) {
		app, root := newConflictTestApp(t)
		setTestResolver(t, app, fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"title": false}}})
		seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "a"}})
		const rel = "entities/ticket/TKT-001--blocks--TKT-002.md"
		writeProjectFile(t, root, rel, conflictedTicket)

		detail := func(rel string) *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			app.handleV1ConflictRoutes(rec,
				httptest.NewRequest(http.MethodGet, "/api/v1/_conflicts/"+rel, http.NoBody))
			return rec
		}
		got, want := detail(rel), detail("entities/ticket/NOPE.md")
		if got.Code != http.StatusNotFound ||
			fmt.Sprint(problemWithoutInstance(t, got)) != fmt.Sprint(problemWithoutInstance(t, want)) {

			t.Errorf("detail = %d %s, want the missing file's %d %s", got.Code, got.Body, want.Code, want.Body)
		}
		if rec := postConflictResolve(app, `{"path":"`+rel+`","content_choice":"theirs"}`); rec.Code != http.StatusNotFound {
			t.Errorf("resolve = %d, want 404: %s", rec.Code, rec.Body)
		}
	})

	t.Run("separator in a directory name", func(t *testing.T) {
		app := newTestAppV1(t)
		root := filepath.Join(t.TempDir(), "TKT-001--blocks--TKT-002")
		bindRepo(app, root)
		app.paths.EntitiesDir = filepath.Join(root, "entities")
		app.paths.RelationsDir = filepath.Join(root, "relations")
		setTestResolver(t, app, fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"title": false}}})
		writeProjectFile(t, root, "entities/ticket/TKT-001.md", conflictedTicket)

		rec := httptest.NewRecorder()
		app.handleV1ConflictRoutes(rec,
			httptest.NewRequest(http.MethodGet, "/api/v1/_conflicts/entities/ticket/TKT-001.md", http.NoBody))
		if rec.Code != http.StatusOK {
			t.Fatalf("detail = %d: %s", rec.Code, rec.Body)
		}
		var got v1.ConflictDetail
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.EntityID != "TKT-001" || got.EntityType != "ticket" {
			t.Errorf("detail judged as %q/%q, want ticket/TKT-001", got.EntityType, got.EntityID)
		}
		for _, diff := range got.PropertyDiffs {
			if diff.Property == "title" {
				t.Errorf("hidden property served: %+v", diff)
			}
		}
	})
}
