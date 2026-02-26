package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/views"
)

var (
	viewDepsRoots string
	viewDepsFiles bool
)

var viewDepsCmd = &cobra.Command{
	Use:   "deps <view-name>",
	Short: "List entities used by a view",
	Long: `Lists all entity IDs (or file paths) that a view touches when executed.

By default, the view is executed for every entity matching the view's entry type.
Use --roots to restrict to specific root entities.

This is useful for CI scripts that need to determine which documents are affected
by changes detected via git diff.

Examples:
  rela view deps document_publish
  rela view deps document_publish --roots DOC-001,DOC-002
  rela view deps document_publish --files`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		viewName := args[0]

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

		// Determine root entity IDs
		var rootIDs []string
		if viewDepsRoots != "" {
			rootIDs = strings.Split(viewDepsRoots, ",")
		} else {
			// Use all entities of the view's entry type
			for _, entity := range ws.EntitiesByType(viewDef.Entry.Type) {
				rootIDs = append(rootIDs, entity.ID)
			}
		}

		// Execute and collect deps
		engine := views.NewEngine(ws.Graph(), meta)
		ids, err := engine.CollectDeps(viewDef, rootIDs)
		if err != nil {
			return fmt.Errorf("failed to collect dependencies: %w", err)
		}

		// Output
		for _, id := range ids {
			if viewDepsFiles {
				entity, ok := ws.GetEntity(id)
				if !ok || entity.FilePath == "" {
					fmt.Fprintf(os.Stderr, "warning: entity %s has no file path\n", id)
					continue
				}
				fmt.Println(entity.FilePath)
			} else {
				fmt.Println(id)
			}
		}

		return nil
	},
}

func init() {
	viewDepsCmd.Flags().StringVar(&viewDepsRoots, "roots", "", "Comma-separated root entity IDs (default: all entities of entry type)")
	viewDepsCmd.Flags().BoolVar(&viewDepsFiles, "files", false, "Output file paths instead of entity IDs")
}
