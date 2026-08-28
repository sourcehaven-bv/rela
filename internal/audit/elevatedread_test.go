package audit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TKT-ACSBSA: the row that records "a bypass_acl closure read raw data",
// closing the gap where elevated WRITES were audited (OpACLBypass) and
// elevated READS left no trace at all.

// TestElevationRecorder_RecordsShapeNotContent pins what the row carries.
//
// It must identify WHO read raw data under WHICH automation using WHICH
// bindings, and must NOT carry entity data or a subject: the read set is
// unbounded (one admin.list_entities can span the graph), and logging it
// would copy ACL-protected content into the audit log — a wider disclosure
// than the read being recorded.
func TestElevationRecorder_RecordsShapeNotContent(t *testing.T) {
	t.Parallel()
	sink := &audit.Memory{}
	rec := audit.NewElevationRecorder(sink)

	ctx := principal.With(context.Background(), principal.Principal{
		User: "alice", Tool: principal.ToolDataEntry,
	})
	rec.RecordElevatedRead(ctx, []string{"get_entity", "list_entities"})

	records := sink.Records()
	if len(records) != 1 {
		t.Fatalf("got %d audit records, want 1", len(records))
	}
	got := records[0]

	if got.Op != audit.OpACLBypassRead {
		t.Errorf("Op = %q, want %q -- elevated reads must be isolable separately "+
			"from elevated writes, which answer a different question and have a "+
			"different blast radius", got.Op, audit.OpACLBypassRead)
	}
	// The REAL triggering identity, not a system user: "who caused this"
	// stays answerable, like ruid under sudo.
	if got.Principal.User != "alice" {
		t.Errorf("Principal.User = %q, want %q -- the elevated read must be "+
			"attributed to the real triggering identity", got.Principal.User, "alice")
	}
	if got.Subject != nil {
		t.Errorf("Subject = %+v, want nil -- the read set is unbounded, so naming "+
			"a subject would be either wrong or a disclosure", got.Subject)
	}
	for _, want := range []string{"acl_bypass_read=true", "get_entity", "list_entities"} {
		if !strings.Contains(got.Summary, want) {
			t.Errorf("Summary = %q, want it to contain %q", got.Summary, want)
		}
	}
	if got.Time.IsZero() {
		t.Error("Time is zero -- an audit row with no timestamp is not forensic")
	}
}

// TestElevationRecorder_NilSink pins that no sink yields no recorder. The
// nil must be converted to a genuine nil INTERFACE by the wiring site — see
// appbuild.NewElevationAuditor and its test, which covers the typed-nil trap
// this constructor deliberately leaves to the caller.
func TestElevationRecorder_NilSink(t *testing.T) {
	t.Parallel()
	if got := audit.NewElevationRecorder(nil); got != nil {
		t.Errorf("NewElevationRecorder(nil) = %#v, want nil", got)
	}
}
