package cli

import (
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// configureLogging sets the default slog logger based on the global
// --verbose/--quiet flags. Logs are written to stderr so they don't
// pollute structured CLI output on stdout.
func configureLogging() {
	level := slog.LevelInfo
	switch {
	case verbose:
		level = slog.LevelDebug
	case quiet:
		level = slog.LevelWarn
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}

var (
	// Version is set at build time
	Version = "dev"

	// Global flags
	outputFormat string
	verbose      bool
	quiet        bool
	projectPath  string

	// skipProjectDiscovery is a Cobra annotation key. Commands that set this
	// annotation skip workspace initialization in PersistentPreRunE.
	skipProjectDiscovery = "skipProjectDiscovery"

	// Shared state initialized by PersistentPreRunE
	ws         *workspace.Workspace
	projectCtx *project.Context // derived from ws.Paths()
	meta       *metamodel.Metamodel
	out        *output.Writer
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:     "rela",
	Short:   "Traceability CLI for managing entities and relationships",
	Version: Version,
	Long: `rela is a CLI tool for managing entities and their relationships with full traceability.

It allows you to document requirements, decisions, solutions, and components,
and maintain semantic relationships between them.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		configureLogging()
		// Commands that annotate themselves with skipProjectDiscovery
		// handle their own initialization (or don't need a project).
		if cmd.Annotations[skipProjectDiscovery] == "true" {
			out = output.New(output.Format(outputFormat))
			return nil
		}

		// Determine project path: flag > env var > cwd
		startDir := projectPath
		if startDir == "" {
			startDir = os.Getenv("RELA_PROJECT")
		}

		// Discover project and initialize workspace
		var err error
		ws, err = workspace.Discover(startDir, script.NewEngine())
		if err != nil {
			return wrapDiscoverError(err)
		}

		// Convenience aliases for read-only commands
		projectCtx = ws.Paths()
		meta = ws.Meta()

		// Set up output writer
		out = output.New(output.Format(outputFormat))

		return nil
	},
}

// Execute runs the root command
// coverage-ignore: CLI entry point - tested via integration tests
func Execute() {
	os.Exit(run())
}

// run executes the root command with a signal-aware context and returns the
// process exit code. It is split out from Execute so that signal.NotifyContext
// cleanup runs before os.Exit.
// coverage-ignore: CLI entry point - tested via integration tests
func run() int {
	// Set up a signal-aware context so Ctrl+C (SIGINT) and SIGTERM cancel
	// in-flight operations, including embedded Lua execution.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := rootCmd.ExecuteContext(ctx)
	if err == nil {
		return 0
	}
	// Check for ExitError to use custom exit code
	var exitErr *errors.ExitError
	if stderrors.As(err, &exitErr) {
		return exitErr.Code
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format (table, json)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Quiet output")
	rootCmd.PersistentFlags().StringVar(&projectPath, "project", "", "Project directory (default: auto-detect from cwd, or RELA_PROJECT env var)")

	// Add version command
	rootCmd.AddCommand(&cobra.Command{
		Use:         "version",
		Short:       "Print version information",
		Annotations: map[string]string{skipProjectDiscovery: "true"},
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("rela version %s\n", Version)
		},
	})
}

// wrapDiscoverError translates errors from workspace.Discover into user-facing
// messages. Only the "no metamodel.yaml found" case (errors.ErrNoProject) gets
// the "run 'rela init'" hint; all other failures (parse errors, permission
// denied, corrupt cache, pending migration, etc.) are surfaced verbatim so the
// user can see what actually went wrong.
func wrapDiscoverError(err error) error {
	if stderrors.Is(err, errors.ErrNoProject) {
		return fmt.Errorf("no project found: run 'rela init' to create one")
	}
	return err
}

// saveCache saves the graph to the cache file.
func saveCache() error {
	if ws != nil {
		return ws.SaveCache()
	}
	return nil
}

// resolveEntityType delegates to workspace.
func resolveEntityType(typeName string) (string, *metamodel.EntityDef, error) {
	return ws.ResolveEntityType(typeName)
}
