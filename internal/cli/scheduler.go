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

// Run dispatches `rela scheduler`.
func (c *SchedulerCmd) Run(ctx context.Context, svc *cliServices) error {
	data, err := svc.Config().Load(ctx, scheduler.ConfigFile)
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
	// svc.svc (the appbuild.Services) implements scheduler.WorkspaceProvider
	// structurally. Pass it through the embedded accessor.
	s := scheduler.New(cfg, script.NewEngine(), svc, logger)
	return s.Run(ctx)
}
