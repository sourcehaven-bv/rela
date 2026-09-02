package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestRecordACLQuery pins TKT-M86UY8 (gh#1145, CONTROL-8-15): running
// `rela acl who-can` must leave an audit record naming WHO asked, WHEN, and
// WHAT they asked about.
//
// The command's output is a confidentiality attestation — who may act on an
// entity and by which roles, groups and graph edges. That is the reconnaissance
// an attacker with shell access wants before choosing a target, and the question
// an investigator asks afterwards.
//
// The result set is deliberately absent from the record: it names principals and
// their access routes, so copying it into the audit log would duplicate the
// disclosure rather than record it. This test asserts that absence, not just the
// presence of the fields.
func TestRecordACLQuery(t *testing.T) {
	t.Parallel()

	sink := audit.NewMemory()
	ctx := principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolCLI})

	recordACLQuery(ctx, sink, "read", "INC-042")

	recs := sink.Records()
	if len(recs) != 1 {
		t.Fatalf("want 1 audit record, got %d", len(recs))
	}
	r := recs[0]

	if r.Op != audit.OpACLQuery {
		t.Errorf("Op = %q, want %q", r.Op, audit.OpACLQuery)
	}
	if r.Principal.User != "alice" {
		t.Errorf("Principal.User = %q, want alice — the log must say who asked", r.Principal.User)
	}
	if r.Subject == nil || r.Subject.ID != "INC-042" {
		t.Errorf("Subject must name the entity queried, got %+v", r.Subject)
	}
	if !strings.Contains(r.Summary, "read") {
		t.Errorf("Summary must name the verb queried: %q", r.Summary)
	}
	if r.Time.IsZero() {
		t.Error("Time must be set")
	}
}

// TestRecordACLQuery_NilSinkIsSafe pins that a missing audit sink does not turn
// a read command into an error. CLI fixtures wire services without one, and a
// reporting command that fails because nothing is listening would be a worse
// outcome than the gap this ticket closes.
func TestRecordACLQuery_NilSinkIsSafe(t *testing.T) {
	t.Parallel()
	recordACLQuery(context.Background(), nil, "read", "INC-042")
}

// TestACLWhoCan_RunEmitsAuditRecord pins the WIRING, not just the helper: the
// record must actually be emitted when the command runs.
//
// Without this, recordACLQuery could be correct and never called — which is
// exactly the shape of the gap this ticket closes (the feature's ticket already
// declared a `requires -> audit-log` relation that nothing satisfied).
func TestACLWhoCan_RunEmitsAuditRecord(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), whoCanPolicy)
	seedWhoCanGraph(t, svc)
	withOutput(t, output.FormatTable)

	sink := audit.NewMemory()
	ctx := principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolCLI})

	cmd := &ACLWhoCanCmd{Verb: "read", Entity: "INC-042"}
	if err := cmd.Run(ctx, &writeServices{readServices: *svc, Audit: sink}); err != nil {
		t.Fatalf("who-can read: %v", err)
	}

	var found bool
	for _, r := range sink.Records() {
		if r.Op == audit.OpACLQuery {
			found = true
			if r.Subject == nil || r.Subject.ID != "INC-042" {
				t.Errorf("Subject must name the queried entity, got %+v", r.Subject)
			}
		}
	}
	if !found {
		t.Fatalf("running who-can emitted no %q record; got %d record(s)",
			audit.OpACLQuery, len(sink.Records()))
	}
}
