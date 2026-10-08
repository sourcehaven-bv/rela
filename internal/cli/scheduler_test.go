package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/scheduler"
)

// otherWorkspace is a WorkspaceProvider that is not *appbuild.Services.
type otherWorkspace struct{ scheduler.WorkspaceProvider }

// Finding 8: a workspace the sync-backend check cannot inspect is refused
// before the scheduler reads its config.
func TestSchedulerCmd_RefusesUncheckableWorkspace(t *testing.T) {
	err := (&SchedulerCmd{}).Run(context.Background(), otherWorkspace{})
	if err == nil || !strings.Contains(err.Error(), "sync backend") {
		t.Fatalf("err = %v, want the sync-backend refusal", err)
	}
}
