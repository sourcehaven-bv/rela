package entitymanager_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// elevatedProvider is the optional capability the elevated handle is obtained
// through. *Manager satisfies it (TKT-D8T148).
type elevatedProvider interface {
	Elevated() autocascade.Mutator
}

// TestManager_Elevated_BypassesACLDenyAndAudits pins the core TKT-D8T148
// contract: the gated Manager denies a write the principal isn't authorized
// for, while the elevated handle (Manager.Elevated()) performs the SAME write —
// recording an acl-bypass audit row that preserves the real principal, and NOT
// a denied-write row.
func TestManager_Elevated_BypassesACLDenyAndAudits(t *testing.T) {
	t.Parallel()
	sink := audit.NewMemory()
	// ReadOnlyACL denies every write.
	mgr, cs := newManagerWithACL(t, acl.ReadOnlyACL{}, sink)
	seedEntity(t, cs, "decision", "From decision")
	seedEntity(t, cs, "requirement", "To requirement")

	ctx := principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry})

	// Gated write is denied.
	_, gErr := mgr.CreateRelation(ctx, "DEC-001", "addresses", "REQ-001", entity.RelationOptions{})
	if gErr == nil {
		t.Fatal("gated CreateRelation: expected ACL denial, got nil")
	}
	var forbidden *acl.ForbiddenError
	if !errors.As(gErr, &forbidden) {
		t.Fatalf("gated denial = %v, want *acl.ForbiddenError", gErr)
	}

	before := len(sink.Records())

	// Elevated write succeeds.
	em, ok := any(mgr).(elevatedProvider)
	if !ok {
		t.Fatal("Manager does not satisfy elevatedProvider — Elevated() missing")
	}
	elevated := em.Elevated()
	if _, eErr := elevated.CreateRelation(ctx, "DEC-001", "addresses", "REQ-001", entity.RelationOptions{}); eErr != nil {
		t.Fatalf("elevated CreateRelation: expected success, got %v", eErr)
	}

	// The elevated write records BOTH the normal successful-write row (every
	// Manager write audits) AND one acl-bypass marker row with the REAL
	// principal — and NEVER a denied-write row.
	newRecs := sink.Records()[before:]
	var bypassRows, deniedRows int
	for _, rec := range newRecs {
		switch rec.Op {
		case audit.OpACLBypass:
			bypassRows++
			if rec.Principal.User != "alice" {
				t.Errorf("acl-bypass Principal.User = %q, want \"alice\" (real principal preserved)", rec.Principal.User)
			}
			if !strings.Contains(rec.Summary, "acl_bypass=true") {
				t.Errorf("acl-bypass Summary = %q, want it to contain acl_bypass=true", rec.Summary)
			}
		case audit.OpDeniedWrite:
			deniedRows++
		}
	}
	if bypassRows != 1 {
		t.Errorf("acl-bypass audit rows = %d, want exactly 1", bypassRows)
	}
	if deniedRows != 0 {
		t.Errorf("denied-write audit rows = %d, want 0 (elevated write must not record a denial)", deniedRows)
	}
}

// TestManager_Elevated_DoesNotMutateGatedManager pins that obtaining an
// elevated handle does not turn the original Manager into a bypassing one:
// after Elevated(), the original mgr still denies. (Elevation is per-handle.)
func TestManager_Elevated_DoesNotMutateGatedManager(t *testing.T) {
	t.Parallel()
	sink := audit.NewMemory()
	mgr, cs := newManagerWithACL(t, acl.ReadOnlyACL{}, sink)
	seedEntity(t, cs, "decision", "From decision")
	seedEntity(t, cs, "requirement", "To requirement")
	ctx := principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry})

	em := any(mgr).(elevatedProvider)
	_ = em.Elevated() // obtaining the handle must not affect mgr

	if _, err := mgr.CreateRelation(ctx, "DEC-001", "addresses", "REQ-001", entity.RelationOptions{}); err == nil {
		t.Fatal("original Manager allowed a write after Elevated() was called — elevation leaked onto the gated handle")
	}
}

// TestManager_Elevated_DoesNotLeakIntoNestedCascade is the load-bearing leak
// test (go-architect review): an entity write through the ELEVATED handle
// triggers a cascade, and the Mutator handed to the cascade's scripts MUST be
// GATED — elevation does not propagate to descendant writes. We capture the
// cascade's Mutator and prove a write through it is still ACL-denied.
func TestManager_Elevated_DoesNotLeakIntoNestedCascade(t *testing.T) {
	t.Parallel()
	scripts := &recordingScripts{}
	cs := &countingStore{Store: memstore.New()}
	auto := automation.Automation{
		Name: "fires-on-create",
		On:   automation.Trigger{Entity: []string{"requirement"}, Created: true},
		Do:   []automation.Action{{Lua: "-- noop"}},
	}
	engine := automation.NewEngine([]automation.Automation{auto})
	runner, err := autocascade.New(autocascade.Deps{Engine: engine})
	if err != nil {
		t.Fatalf("autocascade.New: %v", err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:        cs,
		Meta:         parseMeta(t),
		Templater:    nopTemplater{},
		Audit:        audit.Nop{},
		ACL:          acl.ReadOnlyACL{}, // deny-all, so a gated write is refused
		Automations:  engine,
		Cascade:      runner,
		ScriptRunner: scripts,
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}

	// Create an entity through the ELEVATED handle (the gated path would be
	// denied at the create itself). This fires the cascade.
	elevated := any(mgr).(elevatedProvider).Elevated()
	ctx := principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	e := entity.New("", "requirement")
	e.SetString("title", "Trigger")
	if _, err := elevated.CreateEntity(ctx, e, entity.CreateOptions{}); err != nil {
		t.Fatalf("elevated CreateEntity: %v", err)
	}

	// The cascade ran and handed the script a Mutator.
	if scripts.calls != 1 || scripts.mutator == nil {
		t.Fatalf("cascade script not invoked with a mutator (calls=%d)", scripts.calls)
	}

	// CRITICAL: that Mutator must be GATED — a write through it is denied,
	// proving elevation did not propagate into the nested cascade.
	_, wErr := scripts.mutator.CreateRelation(ctx, "REQ-001", "addresses", "REQ-001", entity.RelationOptions{})
	if wErr == nil {
		t.Fatal("LEAK: the nested cascade's Mutator allowed a write — elevation propagated to descendants")
	}
	var forbidden *acl.ForbiddenError
	if !errors.As(wErr, &forbidden) {
		t.Fatalf("nested-cascade write error = %v, want *acl.ForbiddenError (gated)", wErr)
	}
}
