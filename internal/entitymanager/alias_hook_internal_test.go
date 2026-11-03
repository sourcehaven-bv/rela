package entitymanager

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// ctxNotifier records whether the context each create hook received was live
// and bounded.
type ctxNotifier struct {
	AliasRewriter
	live []bool
}

func (r *ctxNotifier) EntityCreated(ctx context.Context, _ string, _ entity.Face) error {
	_, hasDeadline := ctx.Deadline()
	r.live = append(r.live, ctx.Err() == nil && hasDeadline)
	return nil
}

// TestAliasHook_SurvivesCancelledRequest: the hooks run after the commit, so
// a request cancelled by then must not cancel them. A cancelled create hook
// would leave a deleted entity's channels on the new entity reusing the id.
func TestAliasHook_SurvivesCancelledRequest(t *testing.T) {
	rw := &ctxNotifier{}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	notifyAliasesOfCreate(ctx, rw, "TSK-1", "")
	if len(rw.live) != 1 || !rw.live[0] {
		t.Fatalf("hook context live = %v, want [true]", rw.live)
	}
}
