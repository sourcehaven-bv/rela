package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/natsort"
)

var (
	templateForce     bool
	templateEntities  bool
	templateRelations bool
	templateVariant   string
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage entity and relation templates",
	Long: `Manage templates for creating entities and relations.

Templates provide default frontmatter values and markdown body content
when creating new entities or relations.

Templates are stored in:
  templates/entities/<type>.md    - Entity templates
  templates/relations/<type>.md   - Relation templates`,
}

var templateInitCmd = &cobra.Command{
	Use:   "init [type...]",
	Short: "Generate template files from metamodel",
	Long: `Generate template files for entity and relation types.

Without arguments, generates templates for all entity and relation types.
With arguments, generates templates only for the specified types.

Use --entities or --relations to filter by kind.
Use --variant to create a named variant template (e.g., requirement--epic.md).
Use --force to overwrite existing templates.

Examples:
  rela template init                         # Generate all templates
  rela template init requirement             # Generate requirement template
  rela template init addresses               # Generate addresses relation template
  rela template init --entities              # Generate all entity templates
  rela template init --relations             # Generate all relation templates
  rela template init --force                 # Overwrite existing templates
  rela template init requirement --variant epic  # Generate requirement--epic.md variant`,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := cliReadFromContext(cmd.Context())
		meta := svc.Meta()

		// Collect types to generate
		var entityTypes, relationTypes []string

		if len(args) > 0 {
			// User specified types - determine if each is entity or relation
			for _, typeName := range args {
				if _, ok := meta.GetEntityDef(typeName); ok {
					entityTypes = append(entityTypes, meta.ResolveAlias(typeName))
				} else if _, ok := meta.GetRelationDef(typeName); ok {
					relationTypes = append(relationTypes, typeName)
				} else {
					return fmt.Errorf("unknown type: %s (not an entity or relation type)", typeName)
				}
			}
		} else {
			// No args - generate all (filtered by flags)
			if !templateRelations {
				// Include entities (default or --entities flag)
				entityTypes = meta.EntityTypes()
				natsort.Strings(entityTypes)
			}
			if !templateEntities {
				// Include relations (default or --relations flag)
				relationTypes = meta.RelationTypes()
				natsort.Strings(relationTypes)
			}
		}

		// Apply flags to filter
		if templateEntities && !templateRelations {
			relationTypes = nil
		}
		if templateRelations && !templateEntities {
			entityTypes = nil
		}

		var createdCount, skippedCount int

		tmpl := svc.Templater()
		ctx := context.Background()

		// Generate entity templates
		for _, entityType := range entityTypes {
			created, err := tmpl.GenerateEntity(ctx, meta, entityType, templateVariant, templateForce)
			if err != nil {
				return fmt.Errorf("failed to generate template for %s: %w", entityType, err)
			}
			filename := entityType + ".md"
			if templateVariant != "" {
				filename = entityType + "--" + templateVariant + ".md"
			}
			if created {
				out.WriteSuccess("Created template: templates/entities/%s", filename)
				createdCount++
			} else {
				if !quiet {
					out.WriteInfo("Skipped (exists): templates/entities/%s", filename)
				}
				skippedCount++
			}
		}

		// Generate relation templates
		for _, relationType := range relationTypes {
			created, err := tmpl.GenerateRelation(ctx, meta, relationType, templateForce)
			if err != nil {
				return fmt.Errorf("failed to generate template for %s: %w", relationType, err)
			}
			if created {
				out.WriteSuccess("Created template: templates/relations/%s.md", relationType)
				createdCount++
			} else {
				if !quiet {
					out.WriteInfo("Skipped (exists): templates/relations/%s.md", relationType)
				}
				skippedCount++
			}
		}

		// Summary
		if createdCount > 0 || skippedCount > 0 {
			if !quiet {
				out.WriteInfo("Generated %d template(s), skipped %d existing", createdCount, skippedCount)
			}
		} else {
			out.WriteInfo("No templates to generate")
		}

		return nil
	},
}

func init() {
	templateInitCmd.Flags().BoolVar(&templateForce, "force", false, "Overwrite existing templates")
	templateInitCmd.Flags().BoolVar(&templateEntities, "entities", false, "Only generate entity templates")
	templateInitCmd.Flags().BoolVar(&templateRelations, "relations", false, "Only generate relation templates")
	templateInitCmd.Flags().StringVar(&templateVariant, "variant", "", "Create a named variant template (e.g., --variant epic)")

	templateCmd.AddCommand(templateInitCmd)
	rootCmd.AddCommand(templateCmd)
}
