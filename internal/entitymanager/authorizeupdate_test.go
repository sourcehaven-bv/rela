package entitymanager_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestManager_AuthorizeUpdate pins that the early update check is the
// manager's own: a denial is the write's *acl.ForbiddenError with its one
// `denied-write` row, an allow writes nothing, and the method never writes
// the entity.
func TestManager_AuthorizeUpdate(t *testing.T) {
	t.Parallel()
	ctx := principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	ref := entity.Ref{ID: "DEC-001", Face: entity.ImplicitFace}

	t.Run("denied", func(t *testing.T) {
		t.Parallel()
		sink := audit.NewMemory()
		mgr, _ := newManagerWithACL(t, acl.ReadOnlyACL{}, sink)
		err := mgr.AuthorizeUpdate(ctx, "decision", ref)
		var forbidden *acl.ForbiddenError
		if !errors.As(err, &forbidden) {
			t.Fatalf("AuthorizeUpdate = %v, want *acl.ForbiddenError", err)
		}
		recs := sink.Records()
		if len(recs) != 1 || recs[0].Op != audit.OpDeniedWrite {
			t.Fatalf("audit = %+v, want one denied-write row", recs)
		}
		if s := recs[0].Subject; s == nil || s.Kind != "entity" || s.ID != "DEC-001" || s.Type != "decision" ||
			!strings.Contains(recs[0].Summary, "attempted op=update") {

			t.Errorf("denied-write row = %+v, want the manager's update shape for DEC-001", recs[0])
		}
	})

	t.Run("allowed", func(t *testing.T) {
		t.Parallel()
		sink := audit.NewMemory()
		mgr, _ := newManagerWithACL(t, acl.NopACL{}, sink)
		if err := mgr.AuthorizeUpdate(ctx, "decision", ref); err != nil {
			t.Fatalf("AuthorizeUpdate = %v, want nil", err)
		}
		if recs := sink.Records(); len(recs) != 0 {
			t.Errorf("audit = %+v, want no rows for an allowed check", recs)
		}
	})
}
