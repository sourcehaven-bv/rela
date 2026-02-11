package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

var (
	// Version is set at build time
	Version = "dev"

	// Global flags
	outputFormat string
	verbose      bool
	quiet        bool
	projectPath  string

	// Shared state
	projectCtx       *project.Context
	meta             *metamodel.Metamodel
	g                *graph.Graph
	out              *output.Writer
	cliFS            storage.FS = storage.NewSafeFS(storage.NewOsFS())
	repo             repository.Store
	automationEngine *automation.Engine
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

		// Discover project
		var err error
		projectCtx, err = project.Discover(startDir, cliFS)
		if err != nil {
			return fmt.Errorf("no project found: run 'rela init' to create one")
		}

		// Initialize repository
		repo = repository.New(cliFS, projectCtx)

		// Load metamodel
		meta, err = repo.LoadMetamodel()
		if err != nil {
			return fmt.Errorf("failed to load metamodel: %w", err)
		}

		// Initialize automation engine
		automationEngine = automation.NewEngineFromMetamodel(meta.Automations)

		// Initialize graph
		g = graph.New()

		// Try to load from cache first
		if repo.CacheExists() {
			if err := repo.LoadCache(g); err != nil {
				// Cache load failed, sync from files
				if _, err := repo.Sync(meta, g); err != nil {
					return fmt.Errorf("failed to sync: %w", err)
				}
			}
		} else {
			// No cache, sync from files
			if _, err := repo.Sync(meta, g); err != nil {
				return fmt.Errorf("failed to sync: %w", err)
			}
		}

		// Set up output writer
		out = output.New(output.Format(outputFormat))

		return nil
	},
}

// Execute runs the root command
// coverage-ignore: CLI entry point - tested via integration tests
func Execute() {
	if err := rootCmd.Execute(); err != nil {
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

// saveCache saves the graph to the cache file
func saveCache() error {
	if repo != nil && g != nil {
		return repo.SaveCache(g)
	}
	return nil
}

// resolveEntityType resolves an entity type name (handling aliases and plurals)
func resolveEntityType(typeName string) (string, *metamodel.EntityDef, error) {
	// First try to resolve directly (handles exact matches and aliases)
	resolved := meta.ResolveAlias(typeName)
	if def, ok := meta.GetEntityDef(resolved); ok {
		return resolved, def, nil
	}

	// If that failed, try stripping plural suffixes and resolve again
	// Try common plural endings in order of specificity
	pluralSuffixes := []string{"ies", "es", "s"}
	singularReplacements := []string{"y", "", ""}

	for i, suffix := range pluralSuffixes {
		if strings.HasSuffix(typeName, suffix) {
			singular := strings.TrimSuffix(typeName, suffix) + singularReplacements[i]
			resolved = meta.ResolveAlias(singular)
			if def, ok := meta.GetEntityDef(resolved); ok {
				return resolved, def, nil
			}
		}
	}

	return "", nil, fmt.Errorf("unknown entity type: %s", typeName)
}
