package scheduler_test

import (
	"context"
	"errors"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// RR-1USMEZ: the scheduler's default identity must be a FIXED, grantable
// principal. These tests assert the property that makes the acl.yaml grant
// possible at all — that an operator can write one assignment line, ahead
// of time, without knowing which OS account will run the scheduler.
//
// They go through the real acl.Declarative + visibility stack over a
// memstore, so they fail if either the identity or the gate regresses.

// schedulerReader wires the ACL-bound read handle a scheduled task gets,
// under the given policy.
func schedulerReader(t *testing.T, policyYAML string) *visibility.ScriptReader {
	t.Helper()
	ctx := context.Background()
	st := memstore.New()
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "Nightly"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var policy acl.Policy
	if err := yaml.Unmarshal([]byte(policyYAML), &policy); err != nil {
		t.Fatalf("unmarshal policy: %v", err)
	}
	d, err := acl.NewDeclarative(&policy, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	gate, err := visibility.NewDeclarativeGate(d)
	if err != nil {
		t.Fatalf("NewDeclarativeGate: %v", err)
	}
	reader, err := visibility.NewPolicyReader(gate, visibility.NopRedactor{}, st)
	if err != nil {
		t.Fatalf("NewPolicyReader: %v", err)
	}
	sr, err := visibility.NewScriptReader(reader, st, gate)
	if err != nil {
		t.Fatalf("NewScriptReader: %v", err)
	}
	return sr
}

// schedulerCtx is the context a task with no `run_as` runs under.
func schedulerCtx() context.Context {
	return principal.With(context.Background(), principal.Principal{
		User: principal.UserScheduler,
		Tool: principal.ToolScheduler,
	})
}

// TestSchedulerIdentity_GrantedPrincipalReads is the payoff: an operator
// grants the documented constant, and scheduled tasks read again. This is
// the assignment a migration can safely write, because the key is fixed.
func TestSchedulerIdentity_GrantedPrincipalReads(t *testing.T) {
	const granted = `
roles:
  scheduler-system:
    read: ["*"]
assignments:
  system:scheduler: scheduler-system
`
	sr := schedulerReader(t, granted)

	got, err := sr.GetEntity(schedulerCtx(), "TKT-1")
	if err != nil {
		t.Fatalf("granted scheduler read failed: %v", err)
	}
	if got.ID != "TKT-1" {
		t.Errorf("read %q, want TKT-1", got.ID)
	}
}

// TestSchedulerIdentity_UngrantedPrincipalReadsNothing pins the other half:
// the identity grants nothing by itself (DEC-O59WM4). A policy that never
// assigns the scheduler a role leaves its tasks reading nothing — the
// regression this arc exists to make visible and fixable.
func TestSchedulerIdentity_UngrantedPrincipalReadsNothing(t *testing.T) {
	const ungranted = `
roles:
  viewer:
    read: [ticket]
assignments:
  alice: viewer
`
	sr := schedulerReader(t, ungranted)

	_, err := sr.GetEntity(schedulerCtx(), "TKT-1")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ungranted scheduler read err = %v, want ErrNotFound", err)
	}
}

// TestSchedulerIdentity_IsAcceptedByACL guards the systemd failure mode.
// The old default was principal.SystemUser(), which returns the literal
// "unknown" when $USER is unset — and acl rejects that as an unstamped
// principal, so tasks failed outright rather than being scoped. The fixed
// identity must always be a principal acl will open a Request for.
func TestSchedulerIdentity_IsAcceptedByACL(t *testing.T) {
	t.Setenv("USER", "") // systemd-style: no $USER

	var policy acl.Policy
	if err := yaml.Unmarshal([]byte("roles:\n  r:\n    read: [ticket]\n"), &policy); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	st := memstore.New()
	d, err := acl.NewDeclarative(&policy, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}

	if _, err := d.ForPrincipal(principal.Principal{
		User: principal.UserScheduler, Tool: principal.ToolScheduler,
	}); err != nil {
		t.Errorf("acl rejected the scheduler identity: %v", err)
	}

	// Contrast: the value the OLD default produced under systemd.
	if _, err := d.ForPrincipal(principal.Principal{
		User: principal.SystemUser(), Tool: principal.ToolScheduler,
	}); err == nil {
		t.Error("expected the old $USER-derived 'unknown' principal to be rejected; " +
			"if this now passes, the unstamped guard changed and this test's premise is stale")
	}
}
