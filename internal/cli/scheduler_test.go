package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/scheduler"
)

// Finding 8: without project services there is nothing to check for a sync
// backend, so the command refuses before the scheduler reads its config.
func TestSchedulerCmd_RefusesWithoutServices(t *testing.T) {
	err := (&SchedulerCmd{}).Run(context.Background(), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "no project services") {
		t.Fatalf("err = %v, want the missing-services refusal", err)
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
