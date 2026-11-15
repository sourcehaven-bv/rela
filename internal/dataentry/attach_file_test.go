package dataentry

import (
	"strconv"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// pdfBytes is a Lua string expression for bytes that sniff as a PDF and
// carry a NUL and a non-UTF-8 byte, as a real PDF does.
const pdfBytes = `"%PDF-1.4\n\0\255 invoice F001\n%%EOF"`

// attachFileCall is a Lua script that calls rela.attach_file with opts and
// returns "OK name size sha256 mime" or "ERR message".
func attachFileCall(opts string) string {
	return `local att, err = rela.attach_file{` + opts + `}
if not att then return { message = "ERR " .. err } end
return { message = "OK " .. att.name .. " " .. att.size .. " " .. att.sha256 .. " " .. att.mime }`
}

func TestAttachFile_ActionStoresBytes(t *testing.T) {
	a := newAttachApp(t)
	msg := a.runAction(t, "alice", attachFileCall(
		`entity = "AUD-1", property = "report", content = `+pdfBytes+`, filename = "F001", mime = "application/pdf"`))
	name, size, sum, mimeType := okFields(t, msg)
	if name != "F001.pdf" || mimeType != "application/pdf" {
		t.Errorf("name %q mime %q, want F001.pdf application/pdf", name, mimeType)
	}
	data, ok := a.stored(t, "report")[sum]
	if !ok {
		t.Fatalf("no stored file with the returned sha256 %s", sum)
	}
	if want := "%PDF-1.4\n\x00\xff invoice F001\n%%EOF"; string(data) != want {
		t.Errorf("stored %q, want %q", data, want)
	}
	if strconv.Itoa(len(data)) != size {
		t.Errorf("size %s, stored %d bytes", size, len(data))
	}
	// The API listing reports the same hash, as for an upload.
	rec := getEntityAs(principalCtx("alice"), t, a.app, a.decl, "audit", "audits", "AUD-1", "")
	if !strings.Contains(rec.Body.String(), `"sha256":"`+sum+`"`) {
		t.Errorf("entity response lacks the attachment hash %s: %s", sum, rec.Body)
	}
}

func TestAttachFile_WithoutMIMEStoresLikeAnUpload(t *testing.T) {
	a := newAttachApp(t)
	name, _, sum, mimeType := okFields(t, a.runAction(t, "alice", attachFileCall(
		`entity = "AUD-1", property = "report", content = "plain notes", filename = "notes.txt"`)))
	if name != "notes.txt" || mimeType != "text/plain" {
		t.Errorf("name %q mime %q", name, mimeType)
	}
	if got := string(a.stored(t, "report")[sum]); got != "plain notes" {
		t.Errorf("stored %q", got)
	}
}

func TestAttachFile_DeclaredTypeSniffingCannotFingerprint(t *testing.T) {
	a := newAttachApp(t)
	// CSV sniffs as text/plain, which the property accepts.
	msg := a.runAction(t, "alice", attachFileCall(
		`entity = "AUD-1", property = "report", content = "a,b\n1,2\n", filename = "data", mime = "text/csv"`))
	name, _, sum, _ := okFields(t, msg)
	if got := string(a.stored(t, "report")[sum]); got != "a,b\n1,2\n" {
		t.Errorf("stored %q", got)
	}
	if name != "data.csv" {
		t.Errorf("name %q, want data.csv", name)
	}
}

func TestAttachFile_Refusals(t *testing.T) {
	const pdf = `content = ` + pdfBytes + `, filename = "F001.pdf", mime = "application/pdf"`
	for _, tc := range []struct {
		name, user, opts, want string
	}{
		{"no update grant", "dave", `entity = "AUD-1", property = "report", ` + pdf, "permission denied"},
		{"hidden entity", "mallory", `entity = "AUD-1", property = "report", ` + pdf,
			`attach_file: entity "AUD-1" or its file property "report" not found`},
		{"missing entity", "alice", `entity = "AUD-9", property = "report", ` + pdf,
			`entity "AUD-9" or its file property "report" not found`},
		{"not a file property", "alice", `entity = "AUD-1", property = "title", ` + pdf,
			`file property "title" not found`},
		{"type not allowed", "alice", `entity = "AUD-1", property = "frozen", ` + pdf, "attachment rejected"},
		{"content is not the declared type", "alice",
			`entity = "AUD-1", property = "report", content = "<html>error</html>", filename = "F001.pdf", mime = "application/pdf"`,
			"content is text/html, not the declared application/pdf"},
		{"extension of another type", "alice",
			`entity = "AUD-1", property = "report", content = ` + pdfBytes + `, filename = "F001.txt", mime = "application/pdf"`,
			"does not match mime"},
		{"malformed mime", "alice",
			`entity = "AUD-1", property = "report", content = "x", filename = "x.txt", mime = "not a type"`,
			"is not a media type"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newAttachApp(t)
			msg := a.runAction(t, tc.user, attachFileCall(tc.opts))
			if !strings.HasPrefix(msg, "ERR ") || !strings.Contains(msg, tc.want) {
				t.Fatalf("message = %q, want an error containing %q", msg, tc.want)
			}
			if n := len(a.stored(t, "report")) + len(a.stored(t, "frozen")); n != 0 {
				t.Errorf("a refused call stored %d files", n)
			}
		})
	}
}

// TestAttachFile_BypassDoesNotWiden pins that rela.bypass_acl does not lift
// attach_file past the caller's own grants: dave may not update audits, and
// calling the binding inside an elevated block changes nothing.
func TestAttachFile_BypassDoesNotWiden(t *testing.T) {
	a := newAttachApp(t)
	elevated := dataentryconfig.Action{
		AllowACLBypass:  metamodel.ACLBypassWrite,
		ElevatedUpdates: map[string][]string{"audit": {"title"}},
	}
	msg := a.runActionAs(t, "dave", elevated, `local att, err = rela.bypass_acl(function(admin)
	return rela.attach_file{entity = "AUD-1", property = "report", content = `+pdfBytes+`,
		filename = "F001.pdf", mime = "application/pdf"}
end)
if not att then return { message = "ERR " .. tostring(err) } end
return { message = "OK " .. att.name }`)
	if !strings.HasPrefix(msg, "ERR ") || !strings.Contains(msg, "permission denied") {
		t.Fatalf("message = %q, want a permission denial", msg)
	}
	if n := len(a.stored(t, "report")); n != 0 {
		t.Errorf("an elevated call by a user without the grant stored %d files", n)
	}
}

func TestAttachFile_RejectedFileIsAudited(t *testing.T) {
	a := newAttachApp(t)
	a.runAction(t, "alice", attachFileCall(
		`entity = "AUD-1", property = "frozen", content = `+pdfBytes+`, filename = "F001.pdf"`))
	for _, r := range a.audit.Records() {
		if r.Op == audit.OpDeniedWrite && strings.Contains(r.Summary, "rejected upload") {
			return
		}
	}
	t.Fatalf("no rejected-upload record: %+v", a.audit.Records())
}

func TestSniffContradicts(t *testing.T) {
	for _, tc := range []struct {
		sniffed, declared string
		want              bool
	}{
		{"application/pdf", "application/pdf", false},
		{"text/html", "application/pdf", true},
		{"text/plain", "application/pdf", true},
		{"text/plain", "application/json", false},
		{"text/plain", "text/csv", false},
		{"application/zip", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", false},
		{"application/octet-stream", "application/x-custom", false},
		{"text/html", "application/json", true},
		{"image/png", "image/jpeg", true},
	} {
		if got := sniffContradicts(tc.sniffed, tc.declared); got != tc.want {
			t.Errorf("sniffContradicts(%q, %q) = %v, want %v", tc.sniffed, tc.declared, got, tc.want)
		}
	}
}

func TestMIMEExtensions_PreferredFirst(t *testing.T) {
	for mimeType, want := range map[string]string{
		"text/plain": ".txt", "image/jpeg": ".jpg", "application/pdf": ".pdf",
		"application/x-unknown-type": ".bin",
	} {
		if got := mimeExtensions(mimeType); got[0] != want {
			t.Errorf("mimeExtensions(%q)[0] = %q, want %q", mimeType, got[0], want)
		}
	}
}

func TestAttachFile_SizeLimitApplies(t *testing.T) {
	a := newAttachApp(t)
	a.app.State().Cfg.App.MaxAttachmentBytes = 8
	msg := a.runAction(t, "alice", attachFileCall(
		`entity = "AUD-1", property = "report", content = "more than eight bytes", filename = "x.txt"`))
	if !strings.Contains(msg, "exceeds the 8 byte attachment limit") {
		t.Fatalf("message = %q, want the size refusal", msg)
	}
	if n := len(a.stored(t, "report")); n != 0 {
		t.Errorf("an over-limit call stored %d files", n)
	}
}

// Schedules, flow steps and background automations call the filer outside
// an HTTP request, with a bound identity but no read gate.
func TestAttachFile_OutsideRequestIsGated(t *testing.T) {
	a := newAttachApp(t)
	af, err := newAttachFiler(a.app)
	if err != nil {
		t.Fatal(err)
	}
	req := lua.AttachFileRequest{Entity: "AUD-1", Property: "report", Content: []byte("hi"), Filename: "x.txt"}

	att, err := af.AttachFile(boundCtx(t, a, "alice"), req)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(a.stored(t, "report")[att.SHA256]); got != "hi" {
		t.Errorf("stored %q", got)
	}
	if _, err := af.AttachFile(boundCtx(t, a, "dave"), req); err == nil ||
		!strings.Contains(err.Error(), "permission denied") {

		t.Errorf("dave outside a request: err = %v, want permission denied", err)
	}
	// No identity bound at all: refused, never written as anyone.
	if _, err := af.AttachFile(principalCtx("alice"), req); err == nil ||
		!strings.Contains(err.Error(), "no identity") {

		t.Errorf("unbound ctx: err = %v, want a refusal", err)
	}
}

func TestNewAttachFiler_RejectsMissingCollaborators(t *testing.T) {
	if _, err := newAttachFiler(nil); err == nil {
		t.Error("nil app accepted")
	}
	if _, err := newAttachFiler(&App{}); err == nil {
		t.Error("app without an attachment handler accepted")
	}
}
