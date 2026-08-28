package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/scheduler"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// SchedulerCmd runs scheduled Lua tasks.
//
// coverage-ignore: scheduler command - long-running process
type SchedulerCmd struct{}

// Run dispatches `rela scheduler`. The WorkspaceProvider is supplied at
// the kong wiring site (appbuild.Services implements it structurally).
func (c *SchedulerCmd) Run(ctx context.Context, ws scheduler.WorkspaceProvider) error {
	data, err := ws.Config().Load(ctx, scheduler.ConfigFile)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", scheduler.ConfigFile, err)
	}
	cfg, err := scheduler.ParseConfig(data)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	// NewWithQueue, not New: script execution happens on the job queue, so a
	// scheduler built without one starts cleanly and then fails every task.
	s, err := scheduler.NewWithQueue(cfg, script.NewEngine(), ws, logger)
	if err != nil {
		return err
	}
	return s.Run(ctx)
}
