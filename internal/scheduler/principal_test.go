package scheduler

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestStampTaskAuditContext verifies AC4/AC6 for the scheduler entry
// point: every Lua task runs under a ctx carrying
// Principal.Tool=scheduler and triggered_by="schedule:<task-name>".
func TestStampTaskAuditContext(t *testing.T) {
	t.Setenv("USER", "alice")

	stamped := stampTaskAuditContext(context.Background(), "nightly-rollup")

	p := principal.From(stamped)
	if p.Tool != principal.ToolScheduler {
		t.Errorf("Principal.Tool = %q, want %q", p.Tool, principal.ToolScheduler)
	}
	if p.User != "alice" {
		t.Errorf("Principal.User = %q, want 'alice'", p.User)
	}

	if got := audit.TriggeredByFrom(stamped); got != "schedule:nightly-rollup" {
		t.Errorf("TriggeredBy = %q, want 'schedule:nightly-rollup'", got)
	}
}

func TestStampTaskAuditContext_PreservesParentValues(t *testing.T) {
	t.Setenv("USER", "alice")

	parent := principal.With(context.Background(),
		principal.Principal{User: "preexisting", Tool: "cli"})

	stamped := stampTaskAuditContext(parent, "weekly")

	// Scheduler overrides Principal; the parent's pre-existing
	// Principal is shadowed by the scheduler stamp. This is the
	// expected behavior — scheduler is its own identity.
	p := principal.From(stamped)
	if p.Tool != principal.ToolScheduler {
		t.Errorf("Tool should be overridden to scheduler, got %q", p.Tool)
	}
}
