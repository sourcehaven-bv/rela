package dataentry

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// multipartBody builds a multipart/form-data body with a single "file"
// field and returns the body plus its Content-Type header.
func multipartBody(t *testing.T, fileName string, data []byte) (body *bytes.Buffer, contentType string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return &buf, mw.FormDataContentType()
}

// putAttachmentAs invokes the upload handler directly with the gate ctx.
// Type/plural are the fixture's "ticket"/"tickets" (newTestAppV1).
//
//nolint:unparam // entityID is conceptually variable; tests use one fixture.
func putAttachmentAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative,
	entityID, property, fileName string, data []byte,
) *httptest.ResponseRecorder {
	t.Helper()
	body, contentType := multipartBody(t, fileName, data)
	url := "/api/v1/tickets/" + entityID + "/_attachments/" + property
	req := httptest.NewRequest(http.MethodPut, url, body)
	req.Header.Set("Content-Type", contentType)
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.handleV1AttachmentRoute(rec, req, "ticket", "tickets", entityID, property)
	return rec
}

func deleteAttachmentAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative,
	entityID, property, fileName string,
) *httptest.ResponseRecorder {
	t.Helper()
	url := "/api/v1/tickets/" + entityID + "/_attachments/" + property + "/" + fileName
	req := httptest.NewRequest(http.MethodDelete, url, http.NoBody)
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.handleV1AttachmentFileRoute(rec, req, "ticket", entityID, property, fileName)
	return rec
}

// writeACL grants alice read+update on ticket and denies bob everything.
// An attachment write authorizes as OpUpdate (it mutates the owning
// entity's property), so the role needs Read + Update.
func writeACL(t *testing.T, app *App) *acl.Declarative {
	t.Helper()
	return mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"editor": {Read: []string{"ticket"}, Update: []string{"ticket"}},
			"none":   {},
		},
		Assignments: map[string]string{"alice": "editor", "bob": "none"},
	}, app.store)
}

func bobCtx() context.Context {
	return principal.With(context.Background(), principal.Principal{User: "bob", Tool: principal.ToolDataEntry})
}

// TestAttachmentUpload_RoundTrips pins AC1: an authorized upload persists
// the bytes (readable back via GET) and stamps the property; the response
// carries the new _attachments entry.
func TestAttachmentUpload_RoundTrips(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	d := writeACL(t, app)
	app.acl = d

	rec := putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "screenshot", "shot.png", []byte("PNGDATA"))
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: got %d, want 200; body=%s", rec.Code, rec.Body)
	}

	// Bytes readable via GET.
	get := getAttachmentAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "screenshot", "shot.png")
	if get.Code != http.StatusOK || get.Body.String() != "PNGDATA" {
		t.Fatalf("GET after upload: got %d body=%q, want 200 \"PNGDATA\"", get.Code, get.Body)
	}

	// Property stamped on the entity.
	e := mustGet(t, app, "TKT-001")
	if got := e.GetString("screenshot"); got != "attachments/TKT-001/screenshot/shot.png" {
		t.Errorf("property = %q, want the stamped attachment path", got)
	}
}

// TestAttachmentUpload_Replaces pins that a second upload overwrites the
// first.
func TestAttachmentUpload_Replaces(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	d := writeACL(t, app)
	app.acl = d

	putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "screenshot", "a.png", []byte("first"))
	putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "screenshot", "b.png", []byte("second"))

	get := getAttachmentAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "screenshot", "b.png")
	if get.Body.String() != "second" {
		t.Errorf("after replace, body = %q, want \"second\"", get.Body)
	}
}

// TestAttachmentDelete_RemovesBytesAndProperty pins AC1 delete: detach
// clears the property and the bytes are gone.
func TestAttachmentDelete_RemovesBytesAndProperty(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	d := writeACL(t, app)
	app.acl = d
	putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "screenshot", "a.png", []byte("data"))

	del := deleteAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "screenshot", "a.png")
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d, want 204; body=%s", del.Code, del.Body)
	}

	get := getAttachmentAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "screenshot", "a.png")
	if get.Code != http.StatusNotFound {
		t.Errorf("GET after delete: got %d, want 404", get.Code)
	}
	if e := mustGet(t, app, "TKT-001"); e.GetString("screenshot") != "" {
		t.Errorf("property not cleared after delete: %q", e.GetString("screenshot"))
	}
}

// TestAttachmentWrite_DeniedBeforeBytes pins AC3: a principal without
// write access is denied (403), and crucially NO bytes are written — the
// authorize runs before AttachFile.
func TestAttachmentWrite_DeniedBeforeBytes(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	d := writeACL(t, app)
	app.acl = d

	// bob can't read ticket → upload 404s at the read gate (no existence
	// leak, no bytes).
	rec := putAttachmentAs(bobCtx(), t, app, d, "TKT-001", "screenshot", "x.png", []byte("data"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("bob (no read) upload: got %d, want 404", rec.Code)
	}

	// alice can read but suppose a write-only deny: use a role that reads
	// ticket but writes nothing.
	d2 := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"reader": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"carol": "reader"},
	}, app.store)
	app.acl = d2
	carol := principal.With(context.Background(), principal.Principal{User: "carol", Tool: principal.ToolDataEntry})
	rec = putAttachmentAs(carol, t, app, d2, "TKT-001", "screenshot", "x.png", []byte("data"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("carol (read, no write) upload: got %d, want 403; body=%s", rec.Code, rec.Body)
	}

	// No bytes were written by either denied attempt.
	app.acl = d
	get := getAttachmentAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "screenshot", "x.png")
	if get.Code != http.StatusNotFound {
		t.Errorf("a denied upload must not write bytes; GET got %d, want 404", get.Code)
	}
}

// TestAttachmentUpload_TooLarge pins AC2: an over-cap upload is rejected
// with 413, on the ingress path, before any bytes persist.
func TestAttachmentUpload_TooLarge(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	// Tighten the cap via config so the test stays small.
	app.State().Cfg.App.MaxAttachmentBytes = 8
	d := writeACL(t, app)
	app.acl = d

	rec := putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "screenshot", "big.bin", []byte("waytoolarge"))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize upload: got %d, want 413; body=%s", rec.Code, rec.Body)
	}

	get := getAttachmentAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "screenshot", "big.bin")
	if get.Code != http.StatusNotFound {
		t.Errorf("oversize upload must not persist; GET got %d, want 404", get.Code)
	}
}

// TestAttachmentUpload_MissingFieldOrBadProperty covers the 400 (no file
// field) and 404 (non-file property) negative cases.
func TestAttachmentUpload_MissingFieldOrBadProperty(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	d := writeACL(t, app)
	app.acl = d

	// Wrong form field name → 400.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("notfile", "x.png")
	_, _ = fw.Write([]byte("data"))
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/tickets/TKT-001/_attachments/screenshot", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.handleV1AttachmentRoute(rec, req, "ticket", "tickets", "TKT-001", "screenshot")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing file field: got %d, want 400; body=%s", rec.Code, rec.Body)
	}

	// Non-file property → 404 (no oracle).
	rec = putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "title", "x.png", []byte("data"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("upload to non-file property: got %d, want 404", rec.Code)
	}
}

// TestMaxAttachmentBytes_ClampsToStoreCap pins RR-81XSCY: a configured
// limit above the store backstop is clamped, so the 413 ceiling can never
// promise more than the store will accept.
func TestMaxAttachmentBytes_ClampsToStoreCap(t *testing.T) {
	app := newTestAppV1(t)

	// Unset → product default.
	if got := maxAttachmentBytes(app.State()); got != DefaultMaxAttachmentBytes {
		t.Errorf("default: got %d, want %d", got, DefaultMaxAttachmentBytes)
	}
	// Lower override honored.
	app.State().Cfg.App.MaxAttachmentBytes = 1024
	if got := maxAttachmentBytes(app.State()); got != 1024 {
		t.Errorf("override: got %d, want 1024", got)
	}
	// Above the store cap → clamped to the store cap.
	app.State().Cfg.App.MaxAttachmentBytes = store.MaxAttachmentBytes * 2
	if got := maxAttachmentBytes(app.State()); got != store.MaxAttachmentBytes {
		t.Errorf("over-cap: got %d, want clamp to %d", got, store.MaxAttachmentBytes)
	}
}

// TestAttachmentDelete_IdempotentWhenNoAttachment pins that deleting a
// property with no attachment still succeeds (204) and clears the
// property (RR-JIRLLU / S5).
func TestAttachmentDelete_IdempotentWhenNoAttachment(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	d := writeACL(t, app)
	app.acl = d

	del := deleteAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "screenshot", "gone.png")
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete with no attachment: got %d, want 204; body=%s", del.Code, del.Body)
	}
	if e := mustGet(t, app, "TKT-001"); e.GetString("screenshot") != "" {
		t.Errorf("property should remain empty: %q", e.GetString("screenshot"))
	}
}

// TestAttachmentUpload_ReplaceOversizeKeepsExisting pins (at the HTTP
// layer) the RR-EISJTE fix: a failed oversize replace must not destroy
// the existing valid attachment.
func TestAttachmentUpload_ReplaceOversizeKeepsExisting(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	d := writeACL(t, app)
	app.acl = d

	// Upload a valid file, then attempt an oversize replace (cap tightened).
	putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "screenshot", "ok.png", []byte("good"))
	app.State().Cfg.App.MaxAttachmentBytes = 4
	rec := putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "screenshot", "huge.bin", []byte("waytoobig"))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize replace: got %d, want 413", rec.Code)
	}

	// The original must still be downloadable.
	get := getAttachmentAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "screenshot", "ok.png")
	if get.Code != http.StatusOK || get.Body.String() != "good" {
		t.Errorf("failed replace destroyed the original: GET got %d body=%q, want 200 \"good\"", get.Code, get.Body)
	}
}

// attachmentsFor returns the _attachments list for a property on a freshly
// serialized entity.
//
//nolint:unparam // entityID is conceptually variable; tests use one fixture.
func attachmentsFor(t *testing.T, app *App, entityID, property string) []v1.Attachment {
	t.Helper()
	result := app.serializer.forWire(context.Background(), mustGet(t, app, entityID), app.reader.outgoingRelations(context.Background(), entityID), app.Meta(), "tickets")
	if result.Attachments == nil {
		return nil
	}
	return (*result.Attachments)[property]
}

// TestAttachmentUpload_MultiAppendsUpToMax pins the max>1 behavior on the
// `docs` property (max:3): files with different names accumulate, and the
// 4th is rejected with 409. The _attachments wire value is always a list.
func TestAttachmentUpload_MultiAppendsUpToMax(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	d := writeACL(t, app)
	app.acl = d

	for _, name := range []string{"a.pdf", "b.pdf", "c.pdf"} {
		rec := putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "docs", name, []byte(name))
		if rec.Code != http.StatusOK {
			t.Fatalf("upload %s: got %d, want 200; body=%s", name, rec.Code, rec.Body)
		}
	}
	if got := attachmentsFor(t, app, "TKT-001", "docs"); len(got) != 3 {
		t.Fatalf("docs should hold 3 files; got %d", len(got))
	}

	// 4th over the cap → 409.
	rec := putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "docs", "d.pdf", []byte("d"))
	if rec.Code != http.StatusConflict {
		t.Errorf("upload past max: got %d, want 409; body=%s", rec.Code, rec.Body)
	}
	if got := attachmentsFor(t, app, "TKT-001", "docs"); len(got) != 3 {
		t.Errorf("rejected upload must not add a file; got %d", len(got))
	}

	// The property is stamped as a LIST of paths.
	e := mustGet(t, app, "TKT-001")
	paths, _ := e.Properties["docs"].([]string)
	if len(paths) != 3 {
		t.Errorf("docs property should be a 3-element list; got %#v", e.Properties["docs"])
	}
}

// TestAttachmentUpload_MultiAutoSuffix pins that uploading a duplicate name
// to a multi-file property auto-suffixes rather than overwriting.
func TestAttachmentUpload_MultiAutoSuffix(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	d := writeACL(t, app)
	app.acl = d

	putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "docs", "report.pdf", []byte("one"))
	putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "docs", "report.pdf", []byte("two"))

	got := attachmentsFor(t, app, "TKT-001", "docs")
	if len(got) != 2 {
		t.Fatalf("duplicate name should auto-suffix into 2 files; got %d: %+v", len(got), got)
	}
	names := map[string]bool{got[0].FileName: true, got[1].FileName: true}
	if !names["report.pdf"] || !names["report (1).pdf"] {
		t.Errorf("expected report.pdf + report (1).pdf; got %v", names)
	}
}

// TestAttachmentDelete_MultiLeavesSiblings pins that deleting one file from
// a multi-file property leaves the others, and re-stamps the list.
func TestAttachmentDelete_MultiLeavesSiblings(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	d := writeACL(t, app)
	app.acl = d

	putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "docs", "a.pdf", []byte("a"))
	putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "docs", "b.pdf", []byte("b"))

	del := deleteAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "docs", "a.pdf")
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete one of many: got %d, want 204", del.Code)
	}

	got := attachmentsFor(t, app, "TKT-001", "docs")
	if len(got) != 1 || got[0].FileName != "b.pdf" {
		t.Errorf("after deleting a.pdf, expected only b.pdf; got %+v", got)
	}
	// b.pdf still downloads.
	g := getAttachmentAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "docs", "b.pdf")
	if g.Code != http.StatusOK || g.Body.String() != "b" {
		t.Errorf("sibling b.pdf must survive; GET got %d body=%q", g.Code, g.Body)
	}
}
