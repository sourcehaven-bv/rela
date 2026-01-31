package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new rela project",
	Long:  `Creates a new rela project in the current directory with a default metamodel.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := cliFS.Getwd()
		if err != nil {
			return err
		}

		metamodelPath := filepath.Join(cwd, project.MetamodelFile)

		// Check if already initialized
		if _, err := cliFS.Stat(metamodelPath); err == nil {
			return fmt.Errorf("project already initialized (metamodel.yaml exists)")
		}

		// Create project context
		ctx := &project.Context{
			Root:          cwd,
			MetamodelPath: metamodelPath,
			CacheDir:      filepath.Join(cwd, project.CacheDir),
			CachePath:     filepath.Join(cwd, project.CacheDir, project.CacheFile),
			EntitiesDir:   filepath.Join(cwd, project.EntitiesDir),
			RelationsDir:  filepath.Join(cwd, project.RelationsDir),
		}

		// Create directories
		if err := ctx.Initialize(cliFS); err != nil {
			return fmt.Errorf("failed to create directories: %w", err)
		}

		// Write default metamodel
		if err := cliFS.WriteFile(metamodelPath, []byte(metamodel.DefaultMetamodelYAML()), 0644); err != nil {
			return fmt.Errorf("failed to write metamodel: %w", err)
		}

		// Add .rela to .gitignore if it exists
		gitignorePath := filepath.Join(cwd, ".gitignore")
		if _, err := cliFS.Stat(gitignorePath); err == nil {
			// Read existing content
			content, err := cliFS.ReadFile(gitignorePath)
			if err == nil {
				// Check if .rela is already in .gitignore
				if !contains(string(content), ".rela") {
					updated := append(content, []byte("\n# rela cache\n.rela/\n")...)
					_ = cliFS.WriteFile(gitignorePath, updated, 0644)
				}
			}
		}

		out.WriteSuccess("Initialized rela project in %s", cwd)
		out.WriteMessage("  Created metamodel.yaml")
		out.WriteMessage("  Created entities/ directory")
		out.WriteMessage("  Created relations/ directory")
		out.WriteMessage("  Created .rela/ directory (gitignored)")
		out.WriteMessage("")
		out.WriteMessage("Next steps:")
		out.WriteMessage("  rela create requirement    # Create a new requirement")
		out.WriteMessage("  rela list requirements     # List all requirements")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s != "" && containsString(s, substr))
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
