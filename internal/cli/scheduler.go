package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/scheduler"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// coverage-ignore: scheduler command - long-running process
var schedulerCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "Run scheduled Lua tasks",
	Long: `Starts a long-running process that executes Lua scripts on cron schedules.

Schedules are defined in schedules.yaml in the project root:

  tasks:
    - name: daily-report
      script: reports/daily.lua
      schedule: "0 9 * * *"
      timeout: 5m
    - name: validate-orphans
      script: checks/orphans.lua
      schedule: "*/30 * * * *"

Each task references a Lua script in the scripts/ directory. Scripts have access
to the same capabilities as 'rela script' (entity CRUD, graph queries, AI).

On startup, the scheduler checks for missed runs: if a task's scheduled window
passed while the scheduler was not running, it executes immediately.

The scheduler supports graceful shutdown via Ctrl+C / SIGTERM.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runScheduler(cmd)
	},
}

func runScheduler(cmd *cobra.Command) error {
	startDir := projectPath
	if startDir == "" {
		startDir = os.Getenv("RELA_PROJECT")
	}

	engine := script.NewEngine()

	schedWs, err := workspace.Discover(startDir, engine)
	if err != nil {
		return fmt.Errorf("no project found: run 'rela init' to create one")
	}

	data, err := schedWs.ReadProjectFile(scheduler.ConfigFile)
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

	s := scheduler.New(cfg, engine, schedWs, schedWs, logger)
	return s.Run(cmd.Context())
}

func init() {
	rootCmd.AddCommand(schedulerCmd)
}
