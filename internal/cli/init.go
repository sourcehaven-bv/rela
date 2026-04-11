package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

var initCmd = &cobra.Command{
	Use:         "init",
	Short:       "Initialize a new rela project",
	Long:        `Creates a new rela project in the current directory with a default metamodel.`,
	Annotations: map[string]string{skipProjectDiscovery: "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Determine target directory: flag > env var > cwd
		targetDir := projectPath
		if targetDir == "" {
			targetDir = os.Getenv("RELA_PROJECT")
		}

		result, err := workspace.Initialize(targetDir)
		if err != nil {
			return err
		}

		out.WriteSuccess("Initialized rela project in %s", result.Root)
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
