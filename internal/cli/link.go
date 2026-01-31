package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

var linkCmd = &cobra.Command{
	Use:   "link <from> <relation> <to>",
	Short: "Create a relation between entities",
	Long: `Creates a directed relation between two entities.

Examples:
  rela link DEC-001 addresses REQ-001
  rela link SOL-001 implements DEC-001
  rela link COMP-001 realizes SOL-001`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		fromID := args[0]
		relationType := args[1]
		toID := args[2]

		// Check that both entities exist
		fromEntity, ok := g.GetNode(fromID)
		if !ok {
			return fmt.Errorf("source entity not found: %s", fromID)
		}

		toEntity, ok := g.GetNode(toID)
		if !ok {
			return fmt.Errorf("target entity not found: %s", toID)
		}

		// Validate relation against metamodel
		if err := meta.ValidateRelation(relationType, fromEntity.Type, toEntity.Type); err != nil {
			return err
		}

		// Check if relation already exists
		if _, exists := g.GetEdge(fromID, relationType, toID); exists {
			return fmt.Errorf("relation already exists: %s --%s--> %s", fromID, relationType, toID)
		}

		// Create relation
		relation := model.NewRelation(fromID, relationType, toID)

		// Load and apply template defaults (if template exists)
		template, err := repo.LoadRelationTemplate(relationType)
		if err != nil {
			return fmt.Errorf("failed to load template: %w", err)
		}
		if template != nil {
			markdown.ApplyRelationTemplate(relation, template)
		}

		// Write to file (repo computes path and sets relation.FilePath)
		if err := repo.WriteRelation(relation); err != nil {
			return fmt.Errorf("failed to write relation: %w", err)
		}

		// Add to graph
		g.AddEdge(relation)

		// Save cache
		if err := saveCache(); err != nil {
			out.WriteWarning("Failed to save cache: %v", err)
		}

		out.WriteSuccess("Created link: %s --%s--> %s", fromID, relationType, toID)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(linkCmd)
}
