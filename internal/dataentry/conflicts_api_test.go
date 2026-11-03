package dataentry

import (
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
	"github.com/Sourcehaven-BV/rela/internal/audit"
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
	return app, root
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
	app, root := newConflictTestApp(t)
	path := writeProjectFile(t, root, "entities/ticket/TKT-001.md", conflictedTicket)
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{"syncer": {
			Read: []string{"ticket"}, Permissions: []string{acl.PermGitSync},
		}},
		Assignments: map[string]string{"alice": "syncer"},
	}, app.store)
	setTestACL(t, app, d)
	sink := audit.NewMemory()
	app.auditSink = sink

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
