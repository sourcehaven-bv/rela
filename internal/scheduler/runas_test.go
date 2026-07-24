package scheduler

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TKT-ZF2DTV / DEC-O59WM4: a scheduled task's identity is what decides
// what its script may read. `run_as` selects that identity; privileges
// come from acl.yaml assignments, never from task config.

func TestStampTaskAuditContext_RunAsOverridesIdentity(t *testing.T) {
	t.Setenv("USER", "operator")

	ctx := stampTaskAuditContext(context.Background(), "weekly-digest", "system:digest")

	p := principal.From(ctx)
	if p.User != "system:digest" {
		t.Errorf("Principal.User = %q, want the task's run_as identity %q", p.User, "system:digest")
	}
	if p.Tool != principal.ToolScheduler {
		t.Errorf("Principal.Tool = %q, want %q", p.Tool, principal.ToolScheduler)
	}
	// Attribution stays honest: the audit log names the specific job, not a
	// generic scheduler, which is the point of giving a job its own identity.
	if got := audit.TriggeredByFrom(ctx); got != "schedule:weekly-digest" {
		t.Errorf("triggered_by = %q, want %q", got, "schedule:weekly-digest")
	}
}

func TestStampTaskAuditContext_EmptyRunAsKeepsSystemUser(t *testing.T) {
	t.Setenv("USER", "operator")

	ctx := stampTaskAuditContext(context.Background(), "nightly", "")

	// Non-regressing: tasks without run_as keep today's shared scheduler
	// identity, so existing deployments behave exactly as before.
	if got, want := principal.From(ctx).User, principal.SystemUser(); got != want {
		t.Errorf("Principal.User = %q, want the default system user %q", got, want)
	}
}
