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
	err := (&SchedulerCmd{}).Run(context.Background(), otherWorkspace{}, nil)
	if err == nil || !strings.Contains(err.Error(), "sync backend") {
		t.Fatalf("err = %v, want the sync-backend refusal", err)
	}
}

func TestRefuseCaptureTasks(t *testing.T) {
	t.Parallel()
	plain := &scheduler.Config{Tasks: []scheduler.TaskConfig{{Name: "report", Script: "r.lua"}}}
	if err := refuseCaptureTasks(plain); err != nil {
		t.Fatalf("plain tasks refused: %v", err)
	}
	withCapture := &scheduler.Config{Tasks: []scheduler.TaskConfig{
		{Name: "report", Script: "r.lua"},
		{Name: "coverage", Capture: &scheduler.CaptureConfig{Card: "C"}},
	}}
	err := refuseCaptureTasks(withCapture)
	if err == nil || !strings.Contains(err.Error(), "coverage") || !strings.Contains(err.Error(), "rela-server") {
		t.Fatalf("got %v, want a refusal naming the task and rela-server", err)
	}
}
