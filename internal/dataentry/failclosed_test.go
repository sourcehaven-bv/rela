package dataentry

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// rela#1198 / TKT-EIWW4M: when an ACL policy IS configured but the gate
// cannot be constructed, the script read seam must REFUSE, not fall back to
// the raw ungated store.
//
// Why this is worth pinning even though the fault is currently unreachable
// through App.luaWriteDeps (the redactor there is a non-nil struct literal
// and a.store is nil-rejected in NewApp): the fail-open branch was dead code
// that would silently reactivate the moment any of the three constructors
// grew a new error condition, or a.store/the redactor became nilable. The
// exposure is latent, and a latent ungated read on the IdP-webhook path —
// unattended, machine-to-machine, nobody reading the log — is exactly what
// DenyReader exists to prevent. Pinning the CHOICE, not just the current
// reachability, is the point.
//
// These tests drive scriptReader/scriptTracer directly with a nil redactor,
// which is a real fault path in NewPolicyReader/NewVisibleTracer (both
// reject a nil redact). No production code is weakened to make them run.

// configuredACLApp returns an App with a real Declarative policy installed.
// A policy must be present or the NopACL early-return short-circuits before
// any fault branch is reachable.
func configuredACLApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "Readable"}})
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	return app
}

// TestScriptReader_FailsClosedOnGateFault pins that a construction fault
// yields DenyReader rather than the raw store. If this regresses, an
// unattended webhook script silently reads the entire graph.
func TestScriptReader_FailsClosedOnGateFault(t *testing.T) {
	app := configuredACLApp(t)

	// nil redactor => NewPolicyReader errors => the fault branch runs.
	reader := app.scriptReader(nil)

	if reader == nil {
		t.Fatal("scriptReader returned nil")
	}
	if _, isRaw := reader.(store.Store); isRaw {
		t.Fatalf("FAIL-OPEN: scriptReader degraded to the raw store (%T) on a "+
			"gate construction fault — an unattended script would read the "+
			"whole graph ungated (rela#1198)", reader)
	}
	if _, ok := reader.(visibility.DenyReader); !ok {
		t.Fatalf("expected visibility.DenyReader on a construction fault, got %T", reader)
	}

	// And it must actually refuse, with the diagnosable error rather than a
	// not-found that reads as "no such entity".
	got, err := reader.GetEntity(context.Background(), "TKT-001")
	if err == nil {
		t.Fatalf("DenyReader.GetEntity returned no error (entity=%v)", got)
	}
	if !errors.Is(err, visibility.ErrReaderUnavailable) {
		t.Errorf("want ErrReaderUnavailable, got %v", err)
	}
	if got != nil {
		t.Errorf("DenyReader leaked an entity: %v", got)
	}
}

// TestScriptTracer_FailsClosedOnGateFault is the traversal counterpart.
// Traversal is a read: an ungated trace walks the whole graph.
func TestScriptTracer_FailsClosedOnGateFault(t *testing.T) {
	app := configuredACLApp(t)

	tr := app.scriptTracer(nil)

	if tr == nil {
		t.Fatal("scriptTracer returned nil")
	}
	if tr == app.tracer {
		t.Fatal("FAIL-OPEN: scriptTracer degraded to the bare tracer on a " +
			"construction fault — hidden nodes would leak into traversals (rela#1198)")
	}
	if _, ok := tr.(visibility.DenyTracer); !ok {
		t.Fatalf("expected visibility.DenyTracer on a construction fault, got %T", tr)
	}

	ctx := context.Background()
	if res := tr.TraceFrom(ctx, "TKT-001", 2); res != nil {
		t.Errorf("DenyTracer.TraceFrom leaked a result: %v", res)
	}
	if res := tr.TraceTo(ctx, "TKT-001", 2); res != nil {
		t.Errorf("DenyTracer.TraceTo leaked a result: %v", res)
	}
	if p := tr.FindPath(ctx, "TKT-001", "TKT-001"); p != nil {
		t.Errorf("DenyTracer.FindPath leaked a path: %v", p)
	}

	// FindOrphans is the only traversal that CAN report failure (no script
	// can reach it, but the wiring must still not silently claim an empty
	// graph to a Go caller).
	if _, err := tr.FindOrphans(ctx); !errors.Is(err, visibility.ErrReaderUnavailable) {
		t.Errorf("FindOrphans: want ErrReaderUnavailable, got %v", err)
	}
}

// TestScriptReadSeam_PolicylessProjectStaysUnrestricted pins the other side
// of the contract. Absence of a policy is a documented intentional state
// ("no access control desired"), NOT a fault — it must keep returning the
// raw store, byte-identical to pre-ACL behavior. Failing closed here would
// break every policy-less deployment's scripts.
func TestScriptReadSeam_PolicylessProjectStaysUnrestricted(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "Readable"}})
	app.acl = acl.NopACL{}

	reader := app.scriptReader(nil)
	if _, isDeny := reader.(visibility.DenyReader); isDeny {
		t.Error("policy-less project got DenyReader — absence of acl.yaml is " +
			"an intentional state, not a construction fault")
	}
	if any(reader) != any(app.store) {
		t.Errorf("policy-less scriptReader should be the raw store, got %T", reader)
	}

	tr := app.scriptTracer(nil)
	if _, isDeny := tr.(visibility.DenyTracer); isDeny {
		t.Error("policy-less project got DenyTracer")
	}
	if tr != app.tracer {
		t.Errorf("policy-less scriptTracer should be the bare tracer, got %T", tr)
	}
}
