package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/importer"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

var (
	importFormat        string
	importDryRun        bool
	importUpdate        bool
	importSkipErrors    bool
	importRelationsFile string
)

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import entities and relations from JSON, YAML, or CSV",
	Long: `Import entities and relations from structured files.

Supported formats:
  json  - JSON with 'entities' and 'relations' arrays, or array of entities
  yaml  - YAML with 'entities' and 'relations' arrays, or array of entities
  csv   - CSV with columns for entity fields (id, type, + properties)

The format is auto-detected from file extension, or use --format to specify.

Examples:
  # Import from JSON
  rela import entities.json

  # Import from YAML
  rela import data.yaml

  # Import from CSV
  rela import entities.csv

  # Import with separate relations file (CSV)
  rela import entities.csv --relations relations.csv

  # Dry-run to validate without creating files
  rela import --dry-run data.json

  # Update existing entities instead of failing
  rela import --update data.json

  # Continue on errors
  rela import --skip-errors data.json

JSON/YAML format:
  {
    "entities": [
      {"id": "REQ-001", "type": "requirement", "properties": {"title": "...", "status": "draft"}}
    ],
    "relations": [
      {"from": "DEC-001", "relation": "addresses", "to": "REQ-001"}
    ]
  }

CSV format (entities):
  id,type,title,status
  REQ-001,requirement,Must support 1000 users,draft

CSV format (relations):
  from,relation,to
  DEC-001,addresses,REQ-001`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		// Parse the import file
		parseOpts := importer.ParseOptions{
			Format:        importer.Format(importFormat),
			RelationsFile: importRelationsFile,
		}
		source := importer.NewImportSource(cliFS)

		data, err := importer.ParseFile(filePath, parseOpts, source)
		if err != nil {
			return err
		}

		// Convert to workspace import types
		wsData := convertImportData(data)
		wsOpts := workspace.ImportOptions{
			DryRun:     importDryRun,
			Update:     importUpdate,
			SkipErrors: importSkipErrors,
		}

		if importDryRun {
			out.WriteInfo("Dry run - validating without creating files...")
		}

		result, err := ws.Import(wsData, wsOpts)
		if err != nil {
			return err
		}

		// Report results
		if importDryRun {
			out.WriteInfo("Validation complete:")
			if result.EntitiesCreated > 0 {
				out.WriteInfo("  Would create %d entities", result.EntitiesCreated)
			}
			if result.RelationsCreated > 0 {
				out.WriteInfo("  Would create %d relations", result.RelationsCreated)
			}
			if result.EntitiesSkipped > 0 {
				out.WriteWarning("  Would skip %d entities (errors)", result.EntitiesSkipped)
			}
			if result.RelationsSkipped > 0 {
				out.WriteWarning("  Would skip %d relations (errors)", result.RelationsSkipped)
			}
		} else {
			if result.EntitiesCreated > 0 {
				out.WriteSuccess("Created %d entities", result.EntitiesCreated)
			}
			if result.EntitiesUpdated > 0 {
				out.WriteSuccess("Updated %d entities", result.EntitiesUpdated)
			}
			if result.RelationsCreated > 0 {
				out.WriteSuccess("Created %d relations", result.RelationsCreated)
			}
			if result.EntitiesSkipped > 0 {
				out.WriteWarning("Skipped %d entities (errors)", result.EntitiesSkipped)
			}
			if result.RelationsSkipped > 0 {
				out.WriteWarning("Skipped %d relations (errors or duplicates)", result.RelationsSkipped)
			}
		}

		// Report any errors
		if len(result.Errors) > 0 {
			fmt.Println()
			out.WriteWarning("Errors encountered:")
			for _, e := range result.Errors {
				out.WriteError("  %s", e.Error())
			}
		}

		// Return error if there were failures and not skipping
		if len(result.Errors) > 0 && !importSkipErrors {
			return fmt.Errorf("import completed with %d errors", len(result.Errors))
		}

		return nil
	},
}

// convertImportData converts importer types to workspace types.
func convertImportData(data *importer.ImportData) *workspace.ImportData {
	wsData := &workspace.ImportData{
		Entities:  make([]workspace.ImportEntity, len(data.Entities)),
		Relations: make([]workspace.ImportRelation, len(data.Relations)),
	}

	for i, e := range data.Entities {
		wsData.Entities[i] = workspace.ImportEntity{
			ID:         e.ID,
			Type:       e.Type,
			Properties: e.Properties,
		}
	}

	for i, r := range data.Relations {
		wsData.Relations[i] = workspace.ImportRelation{
			From:       r.From,
			Type:       r.Relation,
			To:         r.To,
			Properties: r.Properties,
		}
	}

	return wsData
}

func init() {
	importCmd.Flags().StringVarP(&importFormat, "format", "f", "", "Input format (json, yaml, csv). Auto-detected from extension if not specified")
	importCmd.Flags().BoolVarP(&importDryRun, "dry-run", "n", false, "Validate without creating files")
	importCmd.Flags().BoolVarP(&importUpdate, "update", "u", false, "Replace existing entities instead of failing on duplicates (full replacement, not merge)")
	importCmd.Flags().BoolVar(&importSkipErrors, "skip-errors", false, "Continue importing on validation errors")
	importCmd.Flags().StringVarP(&importRelationsFile, "relations", "r", "", "Path to relations CSV file (for CSV imports)")

	rootCmd.AddCommand(importCmd)
}
