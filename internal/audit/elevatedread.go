package audit

import (
	"context"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// ElevationRecorder records that an elevated automation closure
// (rela.bypass_acl) performed at least one RAW READ — TKT-ACSBSA.
//
// Why it exists: elevated WRITES are recorded by entitymanager as
// [OpACLBypass]. Elevated reads never reach entitymanager — they go straight
// to the store — so without this, an operator querying the audit log for
// elevation saw every elevated write and was silently blind to every
// elevated read. A reviewer could reasonably conclude no elevated access to
// a sensitive entity had occurred when in fact it had.
//
// It lives in this package rather than at the wiring site because the
// attribution it performs ([principal.From] on the ctx) is audit's concern,
// and the wiring package (appbuild) deliberately does not depend on
// principal. It satisfies lua.ElevationRecorder structurally, so lua keeps
// its consumer-side interface and this package does not depend on lua.
type ElevationRecorder struct{ sink Audit }

// NewElevationRecorder returns a recorder over sink, or nil when sink is
// nil.
//
// CAUTION at wiring sites: this returns a CONCRETE pointer, so a nil result
// assigned into an interface-typed field becomes a TYPED nil (!= nil) and
// defeats the consumer's nil guard. Wiring sites must convert through a
// helper that returns the interface type — see appbuild.NewElevationAuditor.
func NewElevationRecorder(sink Audit) *ElevationRecorder {
	if sink == nil {
		return nil
	}
	return &ElevationRecorder{sink: sink}
}

// RecordElevatedRead emits one audit row per bypass_acl closure that used
// its read elevation. Satisfies lua.ElevationRecorder.
//
// The row deliberately carries NO subject and NO entity data: the read set
// is unbounded (one admin.list_entities can span the graph), and recording
// the content would copy the very data the ACL was protecting into the
// audit log — a wider disclosure than the read being recorded. What it does
// record is the SHAPE of the access — who, under which automation, using
// which bindings — which is what a forensic query needs in order to decide
// whether to look further.
//
// Principal is the REAL triggering identity, not a system user, matching
// the elevated-write row: "who caused this" stays answerable, like ruid
// under sudo.
func (r *ElevationRecorder) RecordElevatedRead(ctx context.Context, bindings []string) {
	r.sink.Record(Record{
		Time:        time.Now().UTC(),
		Op:          OpACLBypassRead,
		Principal:   principal.From(ctx),
		TriggeredBy: TriggeredByFrom(ctx),
		Summary:     "acl_bypass_read=true bindings=" + strings.Join(bindings, ","),
	})
}
