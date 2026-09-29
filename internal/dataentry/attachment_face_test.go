package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Per-face attachments (BUG-CTUW2N). Bytes are keyed per entity and shared
// by its faces; a face lists and serves only the names its own file value
// references, and only the attachments API may change that value.

// facedAttachmentApp is facedTicketApp with TKT-1 seeded at `draft` and
// `published`, and the policy under test installed as the app's ACL.
func facedAttachmentApp(t *testing.T, policy func(st store.Store) *acl.Declarative) (*App, *acl.Declarative) {
	t.Helper()
	app := facedTicketApp(t)
	seedDeclaredFaceTicket(context.Background(), t, app)
	d := policy(app.store)
	app.acl = d
	return app, d
}

// allTicketFaces names every declared ticket face. A bare "ticket" grant
// covers the default face only, and the faced ticket has no default row.
var allTicketFaces = []string{"ticket@draft", "ticket@published", "ticket@review"}

// faceEditors grants every principal in users read and update on every
// ticket face.
func faceEditors(t *testing.T, users ...string) func(store.Store) *acl.Declarative {
	t.Helper()
	return func(st store.Store) *acl.Declarative {
		t.Helper()
		assign := map[string]string{}
		for _, u := range users {
			assign[u] = "editor"
		}
		return mustNewACL(t, &acl.Policy{
			Roles:       map[string]acl.RoleDef{"editor": {Read: allTicketFaces, Update: allTicketFaces}},
			Assignments: assign,
		}, st)
	}
}

func downloadAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative, addr, name string) *httptest.ResponseRecorder {
	t.Helper()
	return getAttachmentAs(ctx, t, app, d, "ticket", "tickets", addr, "screenshot", name)
}

// attachmentsOf returns the `_attachments` file names of addr's GET response.
func attachmentsOf(ctx context.Context, t *testing.T, app *App, d *acl.Declarative, addr string) []string {
	t.Helper()
	rec := getEntityAs(ctx, t, app, d, "ticket", "tickets", addr, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d (%s)", addr, rec.Code, rec.Body)
	}
	var body struct {
		Attachments map[string][]struct {
			FileName string `json:"filename"`
			Href     string `json:"href"`
		} `json:"_attachments"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", addr, err)
	}
	var names []string
	for _, a := range body.Attachments["screenshot"] {
		if !strings.Contains(a.Href, "/tickets/"+addr+"/_attachments/") {
			t.Errorf("href %q does not address the face %s", a.Href, addr)
		}
		names = append(names, a.FileName)
	}
	return names
}

func storedBytes(t *testing.T, app *App, name string) (string, bool) {
	t.Helper()
	rc, err := app.store.ReadAttachment(context.Background(), "TKT-1", "screenshot", name)
	if errors.Is(err, store.ErrNotFound) {
		return "", false
	}
	if err != nil {
		t.Fatalf("ReadAttachment %s: %v", name, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data), true
}

func TestFacedAttachment_UploadStaysOnItsFace(t *testing.T) {
	app, d := facedAttachmentApp(t, faceEditors(t, "bob"))
	bob := principalCtx("bob")

	if rec := putAttachmentAs(bob, t, app, d, "TKT-1@draft", "screenshot", "a.txt", []byte("draft bytes")); rec.Code != http.StatusOK {
		t.Fatalf("upload on draft = %d (%s)", rec.Code, rec.Body)
	}
	if rec := downloadAs(bob, t, app, d, "TKT-1@draft", "a.txt"); rec.Code != http.StatusOK || rec.Body.String() != "draft bytes" {
		t.Fatalf("download on draft = %d %q", rec.Code, rec.Body)
	}
	if rec := downloadAs(bob, t, app, d, "TKT-1@published", "a.txt"); rec.Code != http.StatusNotFound {
		t.Errorf("draft's upload served through published: %d %q", rec.Code, rec.Body)
	}
	if got := attachmentsOf(bob, t, app, d, "TKT-1@published"); len(got) != 0 {
		t.Errorf("published lists %v, want nothing", got)
	}
	if got := attachmentsOf(bob, t, app, d, "TKT-1@draft"); len(got) != 1 || got[0] != "a.txt" {
		t.Errorf("draft lists %v, want [a.txt]", got)
	}

	// The same name uploaded on published must not replace draft's bytes.
	rec := putAttachmentAs(bob, t, app, d, "TKT-1@published", "screenshot", "a.txt", []byte("published bytes"))
	if rec.Code != http.StatusOK {
		t.Fatalf("upload on published = %d (%s)", rec.Code, rec.Body)
	}
	if got := downloadAs(bob, t, app, d, "TKT-1@draft", "a.txt"); got.Body.String() != "draft bytes" {
		t.Errorf("a published upload clobbered draft's file: %q", got.Body)
	}
	pub := attachmentsOf(bob, t, app, d, "TKT-1@published")
	if len(pub) != 1 || pub[0] == "a.txt" {
		t.Fatalf("published lists %v, want one suffixed name", pub)
	}
	if got := downloadAs(bob, t, app, d, "TKT-1@published", pub[0]); got.Body.String() != "published bytes" {
		t.Errorf("published download = %q", got.Body)
	}
	if got := downloadAs(bob, t, app, d, "TKT-1@draft", pub[0]); got.Code != http.StatusNotFound {
		t.Errorf("published's upload served through draft: %d", got.Code)
	}
}

// TestFacedAttachment_DeleteCountsReferences: a delete on one face keeps
// bytes another face references, and removes them at the last reference.
func TestFacedAttachment_DeleteCountsReferences(t *testing.T) {
	app, d := facedAttachmentApp(t, faceEditors(t, "bob"))
	bob := principalCtx("bob")

	if rec := putAttachmentAs(bob, t, app, d, "TKT-1@draft", "screenshot", "a.txt", []byte("shared")); rec.Code != http.StatusOK {
		t.Fatalf("upload = %d (%s)", rec.Code, rec.Body)
	}
	// What a copy with `fields: all` does: published references the same
	// bytes through the trusted stamp.
	if _, err := app.attachmentOwner.StampAttachments(bob, entity.Ref{ID: "TKT-1", Face: "published"},
		"screenshot", "attachments/TKT-1/screenshot/a.txt"); err != nil {
		t.Fatalf("stamp published: %v", err)
	}

	if rec := deleteAttachmentAs(bob, t, app, d, "TKT-1@draft", "screenshot", "a.txt"); rec.Code != http.StatusNoContent {
		t.Fatalf("delete on draft = %d (%s)", rec.Code, rec.Body)
	}
	if _, ok := storedBytes(t, app, "a.txt"); !ok {
		t.Fatal("delete on draft removed bytes published still references")
	}
	if rec := downloadAs(bob, t, app, d, "TKT-1@published", "a.txt"); rec.Code != http.StatusOK {
		t.Errorf("published download after draft delete = %d", rec.Code)
	}
	if rec := downloadAs(bob, t, app, d, "TKT-1@draft", "a.txt"); rec.Code != http.StatusNotFound {
		t.Errorf("draft still serves a detached file: %d", rec.Code)
	}

	if rec := deleteAttachmentAs(bob, t, app, d, "TKT-1@published", "screenshot", "a.txt"); rec.Code != http.StatusNoContent {
		t.Fatalf("delete on published = %d (%s)", rec.Code, rec.Body)
	}
	if _, ok := storedBytes(t, app, "a.txt"); ok {
		t.Error("bytes survived the last reference's delete")
	}
}

// TestFacedAttachment_FaceDeleteDropsUnreferencedBytes: deleting one face
// drops the bytes only it referenced.
func TestFacedAttachment_FaceDeleteDropsUnreferencedBytes(t *testing.T) {
	app, d := facedAttachmentApp(t, faceEditors(t, "bob"))
	bob := principalCtx("bob")
	for _, up := range []struct{ addr, name string }{{"TKT-1@draft", "d.txt"}, {"TKT-1@published", "p.txt"}} {
		if rec := putAttachmentAs(bob, t, app, d, up.addr, "screenshot", up.name, []byte(up.name)); rec.Code != http.StatusOK {
			t.Fatalf("upload %s = %d (%s)", up.addr, rec.Code, rec.Body)
		}
	}
	if _, err := app.write.manager.DeleteEntityFace(bob, "TKT-1", "draft"); err != nil {
		t.Fatalf("delete draft face: %v", err)
	}
	if _, ok := storedBytes(t, app, "d.txt"); ok {
		t.Error("the deleted face's bytes survived")
	}
	if _, ok := storedBytes(t, app, "p.txt"); !ok {
		t.Error("the remaining face's bytes were removed")
	}
}

// TestFacedAttachment_ACLOnTypeAtFace: upload, download and delete are
// judged on the addressed face's grants.
func TestFacedAttachment_ACLOnTypeAtFace(t *testing.T) {
	app, d := facedAttachmentApp(t, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{
				"editor":           {Read: allTicketFaces, Update: allTicketFaces},
				"published-editor": {Read: allTicketFaces, Update: []string{"ticket@published"}},
				"published-reader": {Read: []string{"ticket@published"}, Update: []string{"ticket@published"}},
			},
			Assignments: map[string]string{"bob": "editor", "alice": "published-editor", "carol": "published-reader"},
		}, st)
	})
	bob, alice, carol := principalCtx("bob"), principalCtx("alice"), principalCtx("carol")
	if rec := putAttachmentAs(bob, t, app, d, "TKT-1@draft", "screenshot", "a.txt", []byte("draft")); rec.Code != http.StatusOK {
		t.Fatalf("setup upload = %d (%s)", rec.Code, rec.Body)
	}

	for _, tc := range []struct {
		name string
		do   func() *httptest.ResponseRecorder
		want int
	}{
		{"update-denied face refuses an upload", func() *httptest.ResponseRecorder {
			return putAttachmentAs(alice, t, app, d, "TKT-1@draft", "screenshot", "b.txt", []byte("x"))
		}, http.StatusForbidden},
		{"update-denied face refuses a delete", func() *httptest.ResponseRecorder {
			return deleteAttachmentAs(alice, t, app, d, "TKT-1@draft", "screenshot", "a.txt")
		}, http.StatusForbidden},
		{"readable face serves its file", func() *httptest.ResponseRecorder {
			return downloadAs(alice, t, app, d, "TKT-1@draft", "a.txt")
		}, http.StatusOK},
		{"update-granted face accepts an upload", func() *httptest.ResponseRecorder {
			return putAttachmentAs(alice, t, app, d, "TKT-1@published", "screenshot", "c.txt", []byte("x"))
		}, http.StatusOK},
		{"hidden face download is a 404", func() *httptest.ResponseRecorder {
			return downloadAs(carol, t, app, d, "TKT-1@draft", "a.txt")
		}, http.StatusNotFound},
		{"hidden face upload is a 404", func() *httptest.ResponseRecorder {
			return putAttachmentAs(carol, t, app, d, "TKT-1@draft", "screenshot", "e.txt", []byte("x"))
		}, http.StatusNotFound},
		{"hidden face delete is a 404", func() *httptest.ResponseRecorder {
			return deleteAttachmentAs(carol, t, app, d, "TKT-1@draft", "screenshot", "a.txt")
		}, http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if rec := tc.do(); rec.Code != tc.want {
				t.Errorf("= %d, want %d (%s)", rec.Code, tc.want, rec.Body)
			}
		})
	}
	if got, _ := storedBytes(t, app, "a.txt"); got != "draft" {
		t.Errorf("draft's bytes changed under denied writes: %q", got)
	}
}

// TestFileProperty_OrdinaryWriteIs422 closes RR-EV48RR: a generic write may
// not change a file value, because the value is what grants a face its
// bytes. A writer on published who cannot read draft cannot point
// published at draft's file, and the download through published is a 404.
func TestFileProperty_OrdinaryWriteIs422(t *testing.T) {
	app, d := facedAttachmentApp(t, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{
				"editor":           {Read: allTicketFaces, Update: allTicketFaces},
				"published-editor": {Read: []string{"ticket@published"}, Update: []string{"ticket@published"}},
			},
			Assignments: map[string]string{"bob": "editor", "alice": "published-editor"},
		}, st)
	})
	bob, alice := principalCtx("bob"), principalCtx("alice")
	if rec := putAttachmentAs(bob, t, app, d, "TKT-1@draft", "screenshot", "secret.txt", []byte("SECRET")); rec.Code != http.StatusOK {
		t.Fatalf("setup upload = %d (%s)", rec.Code, rec.Body)
	}

	for _, body := range []string{
		`{"properties":{"screenshot":"attachments/TKT-1/screenshot/secret.txt"}}`,
		`{"properties":{"screenshot":"secret.txt"}}`,
		`{"properties":{"screenshot":["elsewhere/secret.txt"]}}`,
	} {
		rec := patchEntityAs(alice, t, app, d, "ticket", "tickets", "TKT-1@published", body, nil)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("PATCH %s = %d, want 422 (%s)", body, rec.Code, rec.Body)
		}
	}
	if rec := downloadAs(alice, t, app, d, "TKT-1@published", "secret.txt"); rec.Code != http.StatusNotFound ||
		strings.Contains(rec.Body.String(), "SECRET") {

		t.Errorf("published served draft's file: %d %q", rec.Code, rec.Body)
	}

	// Clearing a file value is a change too; the attachments API does it.
	if rec := patchEntityAs(bob, t, app, d, "ticket", "tickets", "TKT-1@draft",
		`{"properties":{"screenshot":null}}`, nil); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("clearing a file value = %d, want 422 (%s)", rec.Code, rec.Body)
	}
	// A save that echoes the stored value, and one that leaves it out, pass.
	for _, body := range []string{
		`{"properties":{"title":"x","screenshot":"attachments/TKT-1/screenshot/secret.txt"}}`,
		`{"properties":{"title":"y"}}`,
	} {
		if rec := patchEntityAs(bob, t, app, d, "ticket", "tickets", "TKT-1@draft", body, nil); rec.Code != http.StatusOK {
			t.Errorf("PATCH %s = %d, want 200 (%s)", body, rec.Code, rec.Body)
		}
	}
}

// TestHistoryRestore_KeepsLiveFileValues: restoring an old version keeps the
// live file value; an old value may name bytes another face now owns.
func TestHistoryRestore_KeepsLiveFileValues(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "now"}})
	seedAttachment(t, app, "TKT-001", "new.png", []byte("new"))
	app.versions = historyStore{versions: map[string][]store.VersionSnapshot{
		"TKT-001": {snapshot("ticket", "old body", map[string]any{
			"title": "then", "screenshot": "attachments/TKT-001/screenshot/old.png",
		})},
	}}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_history/ticket/TKT-001/1/restore", http.NoBody)
	rec := httptest.NewRecorder()
	restoreHistoryVersion(app, rec, req, app.versions, "ticket", "TKT-001", "1")
	if rec.Code != http.StatusOK {
		t.Fatalf("restore = %d (%s)", rec.Code, rec.Body)
	}
	got, err := app.store.GetEntity(context.Background(), "TKT-001")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got.GetString("title") != "then" {
		t.Errorf("title = %q, want the restored value", got.GetString("title"))
	}
	if v := got.GetString("screenshot"); v != "attachments/TKT-001/screenshot/new.png" {
		t.Errorf("screenshot = %q, want the live value kept", v)
	}
}

// TestExportEntity_AddressesItsFace: `_export` of `ID@face` exports that
// face, and a face the caller may not read is the uniform 404.
func TestExportEntity_AddressesItsFace(t *testing.T) {
	requireCmdexecSandbox(t)
	app, d := facedDocumentApp(t)
	export := func(user, addr string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/policys/"+addr+"/_export?transform=copy", http.NoBody).
			WithContext(gateCtxFor(principalCtx(user), t, d))
		app.export.handleV1ExportEntity(rec, req, "policy", addr)
		return rec
	}

	rec := export("alice", "POL-1@published")
	if rec.Code != http.StatusOK {
		t.Fatalf("published-only reader export = %d (%s)", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "PUBLISHED TEXT") || strings.Contains(rec.Body.String(), "DRAFT TEXT") {
		t.Errorf("exported the wrong face: %s", rec.Body)
	}
	assertUniform404(t, export("alice", "POL-1@draft"), export("alice", "POL-9@draft"))

	rec = export("bob", "POL-1@draft")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "DRAFT TEXT") {
		t.Errorf("draft export = %d (%s)", rec.Code, rec.Body)
	}

	// The per-type override renders the addressed face too.
	withRenderOverride(t, app, "policy", func(entryID string) string { return "ENTRY " + entryID })
	rec = export("alice", "POL-1@published")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ENTRY POL-1@published") {
		t.Errorf("override export = %d (%s)", rec.Code, rec.Body)
	}
}
