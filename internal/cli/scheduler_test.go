package cli

import (
	"context"
	"strings"
	"testing"
)

// Finding 8: without project services there is nothing to check for a sync
// backend, so the command refuses before the scheduler reads its config.
func TestSchedulerCmd_RefusesWithoutServices(t *testing.T) {
	err := (&SchedulerCmd{}).Run(context.Background(), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "no project services") {
		t.Fatalf("err = %v, want the missing-services refusal", err)
	}
}
