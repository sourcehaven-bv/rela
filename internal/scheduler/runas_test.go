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

// TestStampTaskAuditContext_EmptyRunAsIsFixedIdentity pins the default
// identity to a FIXED constant rather than the OS user (RR-1USMEZ).
//
// The scheduler used to default to principal.SystemUser() ($USER). That
// made the acl.yaml assignment an operator needed depend on which account
// ran `rela scheduler`, so the grant could not be written down in advance
// — it differed per host, and a migration could not compute it.
//
// $USER is deliberately set to a decoy here: if the default ever reverts
// to the OS user, this fails.
func TestStampTaskAuditContext_EmptyRunAsIsFixedIdentity(t *testing.T) {
	t.Setenv("USER", "operator")

	ctx := stampTaskAuditContext(context.Background(), "nightly", "")

	if got := principal.From(ctx).User; got != principal.UserScheduler {
		t.Errorf("Principal.User = %q, want the fixed %q (not $USER)", got, principal.UserScheduler)
	}
}

// TestStampTaskAuditContext_NoUserEnvStillStamped covers the deployment
// that motivated the fixed identity: a systemd unit typically has no
// $USER, so SystemUser() returned the literal "unknown" — which
// acl.Declarative.ForPrincipal REJECTS as an unstamped principal
// (ErrUnstampedPrincipal). Scheduled tasks then failed outright rather
// than merely being scoped. The fixed identity is never "unknown".
func TestStampTaskAuditContext_NoUserEnvStillStamped(t *testing.T) {
	t.Setenv("USER", "")

	ctx := stampTaskAuditContext(context.Background(), "nightly", "")

	p := principal.From(ctx)
	if p.User != principal.UserScheduler {
		t.Errorf("Principal.User = %q, want %q", p.User, principal.UserScheduler)
	}
	if p.User == "unknown" || p.User == "" {
		t.Error("principal is unstamped; acl.ForPrincipal would reject it")
	}
}
