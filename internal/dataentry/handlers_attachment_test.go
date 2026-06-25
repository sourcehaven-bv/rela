package dataentry

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// getAttachmentAs invokes the per-file attachment download handler
// directly, attaching the read gate the way the production middleware
// would. fileName selects which file on the property to download.
func getAttachmentAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative,
	typeName, plural, entityID, property, fileName string,
) *httptest.ResponseRecorder {
	t.Helper()
	url := "/api/v1/" + plural + "/" + entityID + "/_attachments/" + property + "/" + fileName
	req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.handleV1GetAttachment(rec, req, typeName, entityID, property, fileName)
	return rec
}

// seedAttachment writes attachment bytes for (entityID, "screenshot")
// via the store, mirroring what `rela attach` does at the storage layer.
// `screenshot` is the only file property the test fixture declares (see
// newTestAppV1). entityID is kept explicit (not hardcoded) so seed calls
// read alongside their matching getAttachmentAs(... entityID ...).
//
//nolint:unparam // entityID happens to always be the fixture's TKT-001.
func seedAttachment(t *testing.T, app *App, entityID, fileName string, data []byte) {
	t.Helper()
	const property = "screenshot"
	if err := app.store.AttachFile(context.Background(), entityID, property, fileName, bytes.NewReader(data)); err != nil {
		t.Fatalf("AttachFile(%s, %s): %v", entityID, property, err)
	}
}

func nopACL(t *testing.T, app *App) *acl.Declarative {
	t.Helper()
	// A policy that grants alice read on ticket but nothing on feature,
	// reused across the allow/deny cases.
	return mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
}

// TestAttachment_AllowedReturnsBytes pins AC1: a reader of the entity
// gets the attachment bytes with the right headers.
func TestAttachment_AllowedReturnsBytes(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedAttachment(t, app, "TKT-001", "shot.png", []byte("\x89PNGfakebytes"))

	d := nopACL(t, app)
	app.acl = d

	rec := getAttachmentAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "screenshot", "shot.png")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if got := rec.Body.String(); got != "\x89PNGfakebytes" {
		t.Errorf("body = %q, want the seeded bytes", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("missing X-Content-Type-Options: nosniff")
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "sandbox") {
		t.Errorf("Content-Security-Policy = %q, want a sandbox directive", csp)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="shot.png"`) {
		t.Errorf("Content-Disposition = %q, want filename=\"shot.png\"", cd)
	}
}

// TestAttachment_DeniedIsNotFound pins AC2: a caller who cannot read the
// entity gets 404 (never 403), byte-identical to a nonexistent
// attachment, and no ETag.
func TestAttachment_DeniedIsNotFound(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "F1"}})
	// feature has no file property in the fixture, but the gate fires
	// before that ever matters — the point is the deny path.
	d := nopACL(t, app)
	app.acl = d

	// alice has no read on feature → denied.
	rec := getAttachmentAs(aliceCtx(), t, app, d, "feature", "features", "FEAT-001", "logo", "x.png")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("denied: got %d, want 404", rec.Code)
	}
	if rec.Header().Get("ETag") != "" {
		t.Errorf("deny path must not emit an ETag")
	}
	if !strings.Contains(rec.Body.String(), "/errors/not_found") {
		t.Errorf("deny body missing not_found code: %s", rec.Body)
	}

	// Parity: a readable type with a nonexistent attachment yields the
	// same 404 shape.
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	recNX := getAttachmentAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "screenshot", "shot.png")
	if recNX.Code != http.StatusNotFound {
		t.Fatalf("nonexistent attachment: got %d, want 404", recNX.Code)
	}
	if stripInstance(t, rec.Body.String()) != stripInstance(t, recNX.Body.String()) {
		t.Errorf("denied vs nonexistent body differ:\n denied: %s\n nonexistent: %s", rec.Body, recNX.Body)
	}
}

// TestAttachment_UnknownOrNonFileProperty pins that a property that
// isn't a declared file property 404s without leaking which case it was.
func TestAttachment_UnknownOrNonFileProperty(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	d := nopACL(t, app)
	app.acl = d

	for _, prop := range []string{"title" /* string, not file */, "nope" /* undeclared */} {
		rec := getAttachmentAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", prop, "x.png")
		if rec.Code != http.StatusNotFound {
			t.Errorf("property %q: got %d, want 404", prop, rec.Code)
		}
	}
}

// TestAttachment_MethodNotAllowed pins that an unsupported method (not
// GET/PUT/POST/DELETE) 405s. The route now dispatches GET→download,
// PUT/POST→upload, DELETE→detach; anything else is method-not-allowed.
func TestAttachment_MethodNotAllowed(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	d := nopACL(t, app)
	app.acl = d

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001/_attachments/screenshot", http.NoBody)
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.handleV1AttachmentRoute(rec, req, "ticket", "tickets", "TKT-001", "screenshot")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want 405", rec.Code)
	}
}

// TestAttachment_ResolvesByCurrentIDAfterRename pins AC4: after an entity
// is renamed, the attachment is reachable by the new id and gone from the
// old id — i.e. access resolves by the canonical current id, not the path
// string frozen in frontmatter.
func TestAttachment_ResolvesByCurrentIDAfterRename(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedAttachment(t, app, "TKT-001", "shot.png", []byte("bytes"))

	if _, err := app.store.RenameEntity(context.Background(), "TKT-001", "TKT-999"); err != nil {
		t.Fatalf("RenameEntity: %v", err)
	}

	d := nopACL(t, app)
	app.acl = d

	// New id serves the bytes.
	rec := getAttachmentAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-999", "screenshot", "shot.png")
	if rec.Code != http.StatusOK || rec.Body.String() != "bytes" {
		t.Fatalf("after rename, GET new id: got %d body=%q, want 200 \"bytes\"", rec.Code, rec.Body)
	}

	// Old id no longer resolves.
	recOld := getAttachmentAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "screenshot", "shot.png")
	if recOld.Code != http.StatusNotFound {
		t.Errorf("after rename, GET old id: got %d, want 404", recOld.Code)
	}
}

// TestAttachment_MetadataOnEntityGET pins AC: the per-entity GET payload
// carries `_attachments` for properties that have a file, with a
// well-formed download href, and omits properties that don't.
func TestAttachment_MetadataOnEntityGET(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedAttachment(t, app, "TKT-001", "shot.png", []byte("bytes"))

	result := app.serializer.forWire(context.Background(), mustGet(t, app, "TKT-001"), app.outgoingRelations(context.Background(), "TKT-001"), app.Meta(), "tickets")
	if result.Attachments == nil {
		t.Fatalf("_attachments map must be present on per-entity GET")
	}
	list, ok := (*result.Attachments)["screenshot"]
	if !ok || len(list) != 1 {
		t.Fatalf("_attachments[screenshot] must be a 1-element list; got %+v", *result.Attachments)
	}
	att := list[0]
	if att.ID != "shot.png" || att.FileName != "shot.png" || att.Size != int64(len("bytes")) {
		t.Errorf("attachment meta = %+v, want id/filename=shot.png size=5", att)
	}
	if att.Href != "/api/v1/tickets/TKT-001/_attachments/screenshot/shot.png" {
		t.Errorf("href = %q, want the per-file download path", att.Href)
	}
	if !strings.HasPrefix(att.ContentType, "image/png") {
		t.Errorf("contentType = %q, want image/png", att.ContentType)
	}

	// A list-row serialization must NOT carry the map (closed-world: it
	// rides on per-entity responses only).
	row := app.serializer.forWireRelated(context.Background(), mustGet(t, app, "TKT-001"), nil, app.Meta(), "tickets")
	if row.Attachments != nil {
		t.Errorf("_attachments must be nil on list-row serialization; got %+v", *row.Attachments)
	}
}

// mustGet loads an entity or fails the test.
func mustGet(t *testing.T, app *App, id string) *entity.Entity {
	t.Helper()
	e, ok := app.getEntity(context.Background(), id)
	if !ok {
		t.Fatalf("getEntity(%s) not found", id)
	}
	return e
}

// TestAttachment_DeceptiveExtensionServesTypeFromName pins the core XSS
// defense: a file whose BYTES are HTML but whose NAME ends in .png is
// served as image/png with nosniff — the browser must not sniff it back
// to text/html and execute it. This is the whole reason the endpoint sets
// these headers; assert it directly.
func TestAttachment_DeceptiveExtensionServesTypeFromName(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedAttachment(t, app, "TKT-001", "evil.png", []byte("<script>alert(1)</script>"))

	d := nopACL(t, app)
	app.acl = d

	rec := getAttachmentAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "screenshot", "evil.png")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Errorf("Content-Type = %q, want image/png (from name, not sniffed from HTML bytes)", ct)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("missing nosniff — browser could sniff the HTML bytes and execute them")
	}
}

// TestAttachment_SvgIsSandboxed pins that an SVG (which can carry inline
// script) is served with the sandbox CSP and inline disposition. The
// sandbox (no allow-scripts) is what makes `inline` safe for SVG/HTML
// content in the app origin; if this header is ever dropped the endpoint
// becomes a stored-XSS vector.
func TestAttachment_SvgIsSandboxed(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedAttachment(t, app, "TKT-001", "pic.svg",
		[]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))

	d := nopACL(t, app)
	app.acl = d

	rec := getAttachmentAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "screenshot", "pic.svg")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "sandbox") || !strings.Contains(csp, "default-src 'none'") {
		t.Errorf("CSP = %q, want sandbox + default-src 'none'", csp)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "inline;") {
		t.Errorf("Content-Disposition = %q, want inline", cd)
	}
}

// TestAttachment_DenyBodyMatchesGate pins that the handler's 404 body is
// byte-identical (modulo instance URL) to the gate's own deny body —
// guards the shared entityNotFoundTitle invariant against either side
// drifting (RR-601TJD). We compare a hidden-entity attachment 404 against
// a hidden-entity GET 404 produced by gateReadOrNotFound.
func TestAttachment_DenyBodyMatchesGate(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "F1"}})
	d := nopACL(t, app)
	app.acl = d

	attRec := getAttachmentAs(aliceCtx(), t, app, d, "feature", "features", "FEAT-001", "logo", "x.png")
	gateRec := getEntityAs(aliceCtx(), t, app, d, "feature", "features", "FEAT-001", "")
	if attRec.Code != http.StatusNotFound || gateRec.Code != http.StatusNotFound {
		t.Fatalf("both must 404; att=%d gate=%d", attRec.Code, gateRec.Code)
	}
	if stripInstance(t, attRec.Body.String()) != stripInstance(t, gateRec.Body.String()) {
		t.Errorf("attachment deny body must match gate deny body:\n att:  %s\n gate: %s",
			attRec.Body, gateRec.Body)
	}
}

// TestAttachment_MetadataOnMutationResponse pins that `_attachments`
// rides per-entity mutation responses too (not just GET) — the
// closed-world map ships on every serializeEntityForWire output, matching
// `_fields`/`_relations` (RR-FKJYMC). We assert via the serializer the
// PATCH/POST handlers use.
func TestAttachment_MetadataOnMutationResponse(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedAttachment(t, app, "TKT-001", "shot.png", []byte("bytes"))

	// serializeEntityForWire is the shared per-entity serializer for GET,
	// PATCH, POST create, and clone. If it carries _attachments, every one
	// of those responses does.
	result := app.serializer.forWire(context.Background(), mustGet(t, app, "TKT-001"), app.outgoingRelations(context.Background(), "TKT-001"), app.Meta(), "tickets")
	if result.Attachments == nil {
		t.Fatalf("_attachments must be present on every per-entity response, including mutations")
	}
	if _, ok := (*result.Attachments)["screenshot"]; !ok {
		t.Errorf("_attachments missing screenshot; got %+v", *result.Attachments)
	}
}

// TestAttachment_HiddenPropertyNotLeaked pins RR-ROF51F: a `file` property
// hidden from the viewer by field-visibility policy must not appear in
// `_attachments`, and its per-file download must 404 — otherwise the
// hidden-field boundary leaks the file's bytes via a guessable URL.
func TestAttachment_HiddenPropertyNotLeaked(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedAttachment(t, app, "TKT-001", "secret.png", []byte("classified"))
	d := nopACL(t, app)
	app.acl = d
	// Hide the `screenshot` file property for everyone.
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"screenshot": false}}}

	// _attachments omits the hidden property entirely.
	result := app.serializer.forWire(context.Background(), mustGet(t, app, "TKT-001"), app.outgoingRelations(context.Background(), "TKT-001"), app.Meta(), "tickets")
	if result.Attachments != nil {
		if _, present := (*result.Attachments)["screenshot"]; present {
			t.Errorf("hidden property leaked into _attachments: %+v", *result.Attachments)
		}
	}

	// The per-file download 404s (not 200) for the hidden property.
	rec := getAttachmentAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "screenshot", "secret.png")
	if rec.Code != http.StatusNotFound {
		t.Errorf("download of a hidden property's file: got %d, want 404 (must not leak bytes)", rec.Code)
	}
}
