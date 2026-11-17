package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/readtime"
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

// TestTaskContext_FreshReadTime: each run is a new operation, so it replaces
// a read time already on ctx (TKT-08ZI5XN). A `rela scheduler` process whose
// root ctx was stamped at start must not judge `$today` by its start day.
func TestTaskContext_FreshReadTime(t *testing.T) {
	m := newMockWorkspace(t)
	m.acl = newTaskContextDecl(t, acl.NullGraph{})
	run := time.Date(2026, 11, 19, 6, 0, 0, 0, time.UTC)
	s := &Scheduler{ws: m, bind: m.bind, now: func() time.Time { return run }}
	started := readtime.With(context.Background(), time.Date(2026, 11, 16, 6, 0, 0, 0, time.UTC))

	for name, mk := range map[string]func() (context.Context, error){
		"taskContext": func() (context.Context, error) { return s.taskContext(started, "nightly", "") },
		"taskContextAs": func() (context.Context, error) {
			return s.taskContextAs(started, "nightly", principal.Principal{User: "alice"})
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, err := mk()
			if err != nil {
				t.Fatal(err)
			}
			if got, _ := readtime.From(ctx); !got.Equal(run) {
				t.Errorf("read time = %v, want %v", got, run)
			}
		})
	}
}
