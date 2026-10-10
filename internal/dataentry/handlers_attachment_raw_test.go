package dataentry

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// rawUpload describes one raw-body upload request: the body is the file, and
// the name travels in the query string or a Content-Disposition header.
type rawUpload struct {
	query       string // appended to the URL, e.g. "?filename=x.txt"
	disposition string // Content-Disposition header, if set
	encoding    string // Content-Encoding header, if set
	contentType string // defaults to application/octet-stream
	data        []byte
	chunked     bool // send without a Content-Length
}

// The fixture ticket and file property every raw-upload test writes to.
const (
	rawUploadEntity   = "TKT-001"
	rawUploadProperty = "screenshot"
)

// putRawAttachmentAs sends u to the upload handler with the gate ctx, the way
// `curl -T` or restish would: no multipart envelope.
func putRawAttachmentAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative,
	u rawUpload,
) *httptest.ResponseRecorder {
	t.Helper()
	url := "/api/v1/tickets/" + rawUploadEntity + "/_attachments/" + rawUploadProperty + u.query
	var body io.Reader = bytes.NewReader(u.data)
	if u.chunked {
		body = io.MultiReader(body) // hides the length from NewRequest
	}
	req := httptest.NewRequestWithContext(gateCtxFor(ctx, t, d), http.MethodPut, url, body)
	if u.chunked {
		req.ContentLength = -1
	}
	ct := u.contentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	req.Header.Set("Content-Type", ct)
	if u.disposition != "" {
		req.Header.Set("Content-Disposition", u.disposition)
	}
	if u.encoding != "" {
		req.Header.Set("Content-Encoding", u.encoding)
	}
	rec := httptest.NewRecorder()
	app.attachments.handleV1AttachmentRoute(rec, req, "ticket", "tickets", rawUploadEntity, rawUploadProperty)
	return rec
}

// TestAttachmentUpload_RawBody pins that a raw body is accepted as the file,
// with the name taken from ?filename= or Content-Disposition, and that the
// bytes round-trip exactly.
func TestAttachmentUpload_RawBody(t *testing.T) {
	tests := []struct {
		name     string
		upload   rawUpload
		wantName string
	}{
		{
			name:     "name from query",
			upload:   rawUpload{query: "?filename=shot.txt", data: []byte("RAWDATA")},
			wantName: "shot.txt",
		},
		{
			name:     "name from Content-Disposition",
			upload:   rawUpload{disposition: `attachment; filename="notes.txt"`, data: []byte("RAWDATA")},
			wantName: "notes.txt",
		},
		{
			name: "query wins over Content-Disposition",
			upload: rawUpload{
				query: "?filename=query.txt", disposition: `attachment; filename="header.txt"`,
				data: []byte("RAWDATA"),
			},
			wantName: "query.txt",
		},
		{
			name: "text/plain content type is still a raw body",
			upload: rawUpload{
				query: "?filename=plain.txt", contentType: "text/plain; charset=utf-8", data: []byte("RAWDATA"),
			},
			wantName: "plain.txt",
		},
		{
			name:     "path in the name is reduced to its base name",
			upload:   rawUpload{query: "?filename=..%2F..%2Fetc%2Fpasswd.txt", data: []byte("RAWDATA")},
			wantName: "passwd.txt",
		},
		{
			name:     "no Content-Length",
			upload:   rawUpload{query: "?filename=chunked.txt", data: []byte("RAWDATA"), chunked: true},
			wantName: "chunked.txt",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
			d := writeACL(t, app)
			app.acl = d

			rec := putRawAttachmentAs(aliceCtx(), t, app, d, tc.upload)
			if rec.Code != http.StatusOK {
				t.Fatalf("upload: got %d, want 200; body=%s", rec.Code, rec.Body)
			}
			get := getAttachmentAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "screenshot", tc.wantName)
			if get.Code != http.StatusOK || get.Body.String() != "RAWDATA" {
				t.Fatalf("GET %s: got %d body=%q, want 200 \"RAWDATA\"", tc.wantName, get.Code, get.Body)
			}
		})
	}
}

// TestAttachmentUpload_RawBodyRefusals pins that a raw upload is refused
// exactly like a multipart one: same status, nothing persisted.
func TestAttachmentUpload_RawBodyRefusals(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 32))
	tests := []struct {
		name     string
		maxBytes int64
		upload   rawUpload
		wantCode int
	}{
		{
			name:     "no file name",
			upload:   rawUpload{data: []byte("data")},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "malformed Content-Disposition gives no name",
			upload:   rawUpload{disposition: `attachment; filename="unterminated`, data: []byte("data")},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "declared length over the cap",
			maxBytes: 8,
			upload:   rawUpload{query: "?filename=big.txt", data: []byte("waytoolarge")},
			wantCode: http.StatusRequestEntityTooLarge,
		},
		{
			name:     "undeclared length over the cap",
			maxBytes: 8,
			upload:   rawUpload{query: "?filename=big.txt", data: []byte("waytoolarge"), chunked: true},
			wantCode: http.StatusRequestEntityTooLarge,
		},
		{
			name:     "body exactly at the cap is accepted",
			maxBytes: 8,
			upload:   rawUpload{query: "?filename=fits.txt", data: []byte("12345678")},
			wantCode: http.StatusOK,
		},
		{
			name:     "compressed body",
			upload:   rawUpload{query: "?filename=x.txt", encoding: "gzip", data: []byte("data")},
			wantCode: http.StatusUnsupportedMediaType,
		},
		{
			name:     "identity encoding is accepted",
			upload:   rawUpload{query: "?filename=x.txt", encoding: "identity", data: []byte("data")},
			wantCode: http.StatusOK,
		},
		{
			name:     "MIME allowlist",
			upload:   rawUpload{query: "?filename=evil.png", data: png},
			wantCode: http.StatusUnprocessableEntity,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
			if tc.maxBytes > 0 {
				app.State().Cfg.App.MaxAttachmentBytes = tc.maxBytes
			}
			d := writeACL(t, app)
			app.acl = d

			rec := putRawAttachmentAs(aliceCtx(), t, app, d, tc.upload)
			if rec.Code != tc.wantCode {
				t.Fatalf("got %d, want %d; body=%s", rec.Code, tc.wantCode, rec.Body)
			}
			stamped := mustGet(t, app, "TKT-001").GetString("screenshot") != ""
			if stamped != (tc.wantCode == http.StatusOK) {
				t.Errorf("property stamped = %v after a %d", stamped, rec.Code)
			}
		})
	}
}

// TestAttachmentUpload_RawBodyGatedAndAudited pins that the raw branch sits
// behind the same preflight and audit as multipart: a reader without update
// gets 403 and an ACL denial record, a non-reader gets the uniform 404, and a
// MIME rejection is recorded as a rejected upload.
func TestAttachmentUpload_RawBodyGatedAndAudited(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	sink := audit.NewMemory()
	app.auditSink = sink
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"editor": {Read: []string{"ticket"}, Update: []string{"ticket"}},
			"reader": {Read: []string{"ticket"}},
		},
		Assignments: map[string]string{"alice": "editor", "carol": "reader"},
	}, app.store)
	app.acl = d
	carol := principal.With(context.Background(), principal.Principal{User: "carol", Tool: principal.ToolDataEntry})
	upload := rawUpload{query: "?filename=x.txt", data: []byte("data")}

	if rec := putRawAttachmentAs(bobCtx(), t, app, d, upload); rec.Code != http.StatusNotFound {
		t.Fatalf("bob (no read): got %d, want 404", rec.Code)
	}
	if rec := putRawAttachmentAs(carol, t, app, d, upload); rec.Code != http.StatusForbidden {
		t.Fatalf("carol (read only): got %d, want 403; body=%s", rec.Code, rec.Body)
	}
	png := rawUpload{query: "?filename=evil.png", data: []byte("\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 32))}
	if rec := putRawAttachmentAs(aliceCtx(), t, app, d, png); rec.Code != 422 {
		t.Fatalf("alice PNG: got %d, want 422; body=%s", rec.Code, rec.Body)
	}

	var denied, rejected bool
	for _, r := range sink.Records() {
		if r.Op != audit.OpDeniedWrite {
			continue
		}
		switch {
		case r.Principal.User == "carol":
			denied = true
		case r.Principal.User == "alice" && strings.Contains(r.Summary, "evil.png"):
			rejected = true
		}
	}
	if !denied {
		t.Error("carol's denied raw upload was not audited")
	}
	if !rejected {
		t.Error("alice's MIME-rejected raw upload was not audited")
	}
}

// TestAttachmentUpload_RejectionAuditsNormalizedName pins that a rejected
// upload is audited under the name rela evaluated, not the client's raw
// name: a path is dropped and an oversize name is shortened, so a client
// cannot bloat the append-only log with a megabyte-long query string.
func TestAttachmentUpload_RejectionAuditsNormalizedName(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	sink := audit.NewMemory()
	app.auditSink = sink
	d := writeACL(t, app)
	app.acl = d

	long := strings.Repeat("a", 4096)
	png := []byte("\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 32))
	rec := putRawAttachmentAs(aliceCtx(), t, app, d,
		rawUpload{query: "?filename=dir%2F" + long + ".png", data: png})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d, want 422; body=%s", rec.Code, rec.Body)
	}
	records := sink.Records()
	if len(records) != 1 {
		t.Fatalf("got %d audit records, want 1", len(records))
	}
	summary := records[0].Summary
	if strings.Contains(summary, "dir/") || strings.Contains(summary, long) {
		t.Errorf("audit summary carries the raw name (%d bytes): %.120s", len(summary), summary)
	}
	if !strings.Contains(summary, ".png") {
		t.Errorf("audit summary lost the extension: %.120s", summary)
	}
}
