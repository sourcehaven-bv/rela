package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/scheduler"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// coverage-ignore: scheduler command - long-running process
var schedulerCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "Run scheduled Lua tasks",
	Long: `Starts a long-running process that executes Lua scripts on recurring schedules.

Schedules are defined in schedules.yaml in the project root:

  tasks:
    - name: daily-report
      script: reports/daily.lua
      every: day
    - name: weekly-review
      script: checks/weekly.lua
      every: week
    - name: quick-check
      script: checks/orphans.lua
      every: 30m

Schedule values:
  day          Run once per day (after midnight local time)
  monday       Run once per week on Mondays (any weekday name works)
  friday       Run once per week on Fridays
  week         Alias for monday
  30m, 2h      Run at a fixed interval

Tasks execute sequentially in config order. Each task references a Lua script
in the scripts/ directory with the same capabilities as 'rela script'.

On startup, tasks that missed their window are executed immediately.
Graceful shutdown via Ctrl+C / SIGTERM.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runScheduler(cmd)
	},
}

func runScheduler(cmd *cobra.Command) error {
	svc := cliWriteFromContext(cmd.Context())

	data, err := svc.Config().Load(context.Background(), scheduler.ConfigFile)
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

	// cliWrite satisfies scheduler.WorkspaceProvider structurally
	// (Paths / Config / State / LuaWriteDeps).
	s := scheduler.New(cfg, script.NewEngine(), svc, logger)
	return s.Run(cmd.Context())
}

func init() {
	rootCmd.AddCommand(schedulerCmd)
}
