package entitymanager_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// countingACL counts AuthorizeWrite calls and answers allow or deny.
type countingACL struct {
	calls atomic.Int64
	deny  bool
}

func (c *countingACL) AuthorizeWrite(ctx context.Context, req acl.WriteRequest) acl.Decision {
	c.calls.Add(1)
	if c.deny {
		return acl.ReadOnlyACL{}.AuthorizeWrite(ctx, req)
	}
	return acl.NopACL{}.AuthorizeWrite(ctx, req)
}

// An update authorized early through AuthorizeUpdate asks the ACL once:
// the grant's write does not ask again. A grant that does not match the
// row or the principal falls back to the write's own check, and a plain
// PatchEntity always asks.
func TestUpdateGrant_AuthorizesOnce(t *testing.T) {
	t.Parallel()
	alice := principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	bob := principal.With(context.Background(),
		principal.Principal{User: "bob", Tool: principal.ToolDataEntry})
	patch := entity.Patch{Properties: map[string]any{"title": "changed"}}

	setup := func(t *testing.T, c *countingACL, sink audit.Audit) (*entitymanager.Manager, *countingACL) {
		t.Helper()
		mgr, cs := newManagerWithACL(t, c, sink)
		seedEntity(t, cs, "decision", "one")
		seedEntity(t, cs, "decision", "two")
		return mgr, c
	}
	row := func(id string) *entity.Entity { return &entity.Entity{ID: id, Type: "decision"} }

	cases := []struct {
		name string
		run  func(mgr *entitymanager.Manager) error
		want int64
	}{
		{"grant", func(mgr *entitymanager.Manager) error {
			g, err := mgr.AuthorizeUpdate(alice, row("DEC-001"))
			if err != nil {
				return err
			}
			_, err = g.PatchEntity(alice, "DEC-001", patch)
			return err
		}, 1},
		{"plain patch", func(mgr *entitymanager.Manager) error {
			_, err := mgr.PatchEntity(alice, "DEC-001", patch)
			return err
		}, 1},
		{"grant for another row", func(mgr *entitymanager.Manager) error {
			g, err := mgr.AuthorizeUpdate(alice, row("DEC-001"))
			if err != nil {
				return err
			}
			_, err = g.PatchEntity(alice, "DEC-002", patch)
			return err
		}, 2},
		{"grant used by another principal", func(mgr *entitymanager.Manager) error {
			g, err := mgr.AuthorizeUpdate(alice, row("DEC-001"))
			if err != nil {
				return err
			}
			_, err = g.PatchEntity(bob, "DEC-001", patch)
			return err
		}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mgr, c := setup(t, &countingACL{}, audit.Nop{})
			c.calls.Store(0)
			if err := tc.run(mgr); err != nil {
				t.Fatalf("update: %v", err)
			}
			if got := c.calls.Load(); got != tc.want {
				t.Errorf("AuthorizeWrite calls = %d, want %d", got, tc.want)
			}
		})
	}

	t.Run("zero grant", func(t *testing.T) {
		t.Parallel()
		if _, err := (entitymanager.UpdateGrant{}).PatchEntity(alice, "DEC-001", patch); err == nil {
			t.Error("a zero grant wrote")
		}
	})

	t.Run("denied", func(t *testing.T) {
		t.Parallel()
		c := &countingACL{deny: true}
		sink := audit.NewMemory()
		mgr, _ := setup(t, c, sink)
		_, err := mgr.AuthorizeUpdate(alice, row("DEC-001"))
		var forbidden *acl.ForbiddenError
		if !errors.As(err, &forbidden) {
			t.Fatalf("AuthorizeUpdate = %v, want *acl.ForbiddenError", err)
		}
		if got := c.calls.Load(); got != 1 {
			t.Errorf("AuthorizeWrite calls = %d, want 1", got)
		}
		if recs := sink.Records(); len(recs) != 1 || recs[0].Op != audit.OpDeniedWrite {
			t.Errorf("audit = %+v, want one denied-write row", recs)
		}
	})
}
