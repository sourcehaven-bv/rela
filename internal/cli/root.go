package cli

import (
	stderrors "errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

var (
	// Version is set at build time
	Version = "dev"

	// Global flags
	outputFormat string
	verbose      bool
	quiet        bool
	projectPath  string

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
		// Skip project discovery for commands that don't need it
		// Note: We check both command name and that it's a direct child of root
		// to avoid matching subcommands like "template init"
		cmdName := cmd.Name()
		parent := cmd.Parent()
		isRootChild := parent == nil || parent.Name() == "rela"
		if isRootChild && (cmdName == "init" || cmdName == "version" || cmdName == "help" || cmdName == "completion" || cmdName == "tui" || cmdName == "migrate" || cmdName == "mcp" || cmdName == "validate") {
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
		ws, err = workspace.Discover(startDir)
		if err != nil {
			return fmt.Errorf("no project found: run 'rela init' to create one")
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
	if err := rootCmd.Execute(); err != nil {
		// Check for ExitError to use custom exit code
		var exitErr *errors.ExitError
		if stderrors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format (table, json)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Quiet output")
	rootCmd.PersistentFlags().StringVar(&projectPath, "project", "", "Project directory (default: auto-detect from cwd, or RELA_PROJECT env var)")

	// Add version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("rela version %s\n", Version)
		},
	})
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
