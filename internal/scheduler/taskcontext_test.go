package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// downGraph fails every graph read, so resolving any identity fails.
type downGraph struct{ acl.NullGraph }

func (downGraph) OutgoingRelations(context.Context, string, string) ([]string, error) {
	return nil, errors.New("graph down")
}

func newTaskContextDecl(t *testing.T, g acl.Graph) *acl.Declarative {
	t.Helper()
	d, err := acl.NewDeclarative(&acl.Policy{
		Roles: map[string]acl.RoleDef{"r": {Read: []string{"*"}}},
	}, g, acl.NullGraphQueryer{})
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	return d
}

// TestTaskContext_BindsTheTaskIdentity: the scheduler's request edge
// resolves the task principal once and attaches its request scope.
func TestTaskContext_BindsTheTaskIdentity(t *testing.T) {
	d := newTaskContextDecl(t, acl.NullGraph{})
	m := newMockWorkspace(t)
	m.acl = d
	s := &Scheduler{ws: m, bind: m.bind}

	ctx, err := s.taskContext(context.Background(), "nightly", "")
	if err != nil {
		t.Fatalf("taskContext: %v", err)
	}
	if got := principal.From(ctx).User; got != principal.UserScheduler {
		t.Errorf("principal = %q, want %q", got, principal.UserScheduler)
	}
	if _, err := d.RequestFrom(ctx); err != nil {
		t.Errorf("no request scope on the task context: %v", err)
	}
}

// TestTaskContext_ResolveFailureFailsTheRun: when the identity cannot be
// resolved the task does not run under an unbound context. The caller gets
// the error and every evaluation path would have failed closed anyway.
func TestTaskContext_ResolveFailureFailsTheRun(t *testing.T) {
	m := newMockWorkspace(t)
	m.acl = newTaskContextDecl(t, downGraph{})
	s := &Scheduler{ws: m, bind: m.bind}

	if _, err := s.taskContext(context.Background(), "nightly", "alice"); err == nil {
		t.Fatal("taskContext with a failing membership walk: want an error, got nil")
	}
}
