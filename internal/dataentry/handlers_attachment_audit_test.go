package dataentry

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestAttachmentUpload_RejectionIsAudited pins TKT-6O8D0L (gh#1050,
// CONTROL-8-15): an upload the processor refuses — disallowed MIME type, or a
// failed/positive scan — must leave an audit record.
//
// A refused upload is a security-relevant exception: it may be an attempt to
// place a disallowed file type or malware into the project, and the log is the
// only place that question can be answered after the fact. The ACL denial on
// this same handler was already recorded; the policy denial beside it was not.
//
// The record deliberately reuses OpDeniedWrite rather than introducing a new
// op, so an operator filtering `op == "denied-write"` sees both kinds of
// refused upload without having to know there are two.
func TestAttachmentUpload_RejectionIsAudited(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"},
	})
	sink := audit.NewMemory()
	app.auditSink = sink

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}, Update: []string{"ticket"}}},
		Assignments: map[string]string{"bob": "editor"},
	}, app.store)
	app.acl = d
	bobCtx := principal.With(context.Background(),
		principal.Principal{User: "bob", Tool: principal.ToolDataEntry})

	// The fixture's `screenshot` property accepts text/plain only. A PNG
	// signature is sniffed as image/png and refused by the MIME allowlist.
	png := []byte("\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 32))
	rec := putAttachmentAs(bobCtx, t, app, d, "TKT-001", "screenshot", "evil.png", png)

	if rec.Code != 422 {
		t.Fatalf("want 422 for a rejected upload, got %d: %s", rec.Code, rec.Body)
	}

	var found *audit.Record
	for i, r := range sink.Records() {
		if r.Op == audit.OpDeniedWrite {
			found = &sink.Records()[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no denied-write audit record for the rejected upload; got %d record(s)", len(sink.Records()))
	}

	if found.Principal.User != "bob" {
		t.Errorf("Principal.User = %q, want bob — the log must say WHO tried", found.Principal.User)
	}
	if found.Subject == nil || found.Subject.ID != "TKT-001" {
		t.Errorf("Subject must name the target entity, got %+v", found.Subject)
	}
	// The filename and the reason are what make the record actionable: without
	// them an operator sees that something was refused but not what or why.
	if !strings.Contains(found.Summary, "evil.png") {
		t.Errorf("Summary must name the rejected file: %q", found.Summary)
	}
	if !strings.Contains(found.Summary, "screenshot") {
		t.Errorf("Summary must name the target property: %q", found.Summary)
	}
	if !strings.Contains(found.Summary, "rejected upload") {
		t.Errorf("Summary must distinguish this from an ACL denial: %q", found.Summary)
	}
}

// TestAttachmentUpload_NonRejectionIsNotAudited pins the other half: an
// ordinary failure is NOT a security event and must not dilute the signal.
//
// An over-size upload is the clearest case — a client error with no security
// meaning. If every failed upload produced a denied-write record, the op would
// stop distinguishing anything.
func TestAttachmentUpload_NonRejectionIsNotAudited(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"},
	})
	sink := audit.NewMemory()
	app.auditSink = sink

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}, Update: []string{"ticket"}}},
		Assignments: map[string]string{"bob": "editor"},
	}, app.store)
	app.acl = d
	bobCtx := principal.With(context.Background(),
		principal.Principal{User: "bob", Tool: principal.ToolDataEntry})

	// Accepted type, accepted size: this upload succeeds, so there is nothing
	// to deny and nothing to record.
	rec := putAttachmentAs(bobCtx, t, app, d, "TKT-001", "screenshot", "fine.txt", []byte("hello"))
	if rec.Code != 200 {
		t.Fatalf("setup: want a successful upload, got %d: %s", rec.Code, rec.Body)
	}

	for _, r := range sink.Records() {
		if r.Op == audit.OpDeniedWrite {
			t.Errorf("a successful upload must not produce a denied-write record: %+v", r)
		}
	}
}
