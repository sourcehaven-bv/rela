package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/views"
)

var viewCmd = &cobra.Command{
	Use:   "view <view-name> <entry-id>",
	Short: "Generate context using a view definition",
	Long: `Executes a view definition to generate complete context for an entity.

Views are defined in views.yaml and specify declarative graph traversals,
filters, and derived collections for context generation.

Examples:
  rela view document_publish DOC-001 -o yaml
  rela view document_publish DOC-001 -o json --pretty`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		viewName := args[0]
		entryID := args[1]

		// Load views file
		viewsFile, err := ws.LoadViews()
		if err != nil {
			return fmt.Errorf("failed to load views file: %w", err)
		}

		// Get the view definition
		viewDef, ok := viewsFile.GetView(viewName)
		if !ok {
			return fmt.Errorf("view not found: %s", viewName)
		}

		// Validate the view against the metamodel
		if validationErr := viewDef.Validate(meta, viewName); validationErr != nil {
			return fmt.Errorf("view validation failed: %w", validationErr)
		}

		snap := ws.Snapshot()
		g := snap.Graph()

		// Create view engine
		engine := views.NewEngine(g, meta)

		// Execute the view
		result, err := engine.Execute(viewDef, entryID)
		if err != nil {
			return fmt.Errorf("view execution failed: %w", err)
		}

		// Format the output
		format := outputFormat
		if format == "" || format == "table" {
			format = "yaml" // Default to yaml for views
		}

		output, err := views.Format(result, format, g, meta)
		if err != nil {
			return fmt.Errorf("failed to format output: %w", err)
		}

		fmt.Println(output)

		return nil
	},
}

func init() {
	viewCmd.AddCommand(viewDepsCmd)
	viewCmd.AddCommand(viewAffectedCmd)
	rootCmd.AddCommand(viewCmd)
}
