package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/transclusion"
)

var (
	renderOutput        string
	renderNoFrontmatter bool
	renderMaxDepth      int
	renderKeepComments  bool
)

var renderCmd = &cobra.Command{
	Use:   "render <entity-id>",
	Short: "Render an entity with resolved transclusions",
	Long: `Render an entity's content with all transclusions (![[EntityID]]) resolved.

Transclusions allow you to embed content from other entities:
  ![[REQ-001]]           - Include full entity content
  ![[REQ-001#Rationale]] - Include only a specific section

Examples:
  # Render an entity to stdout
  rela render REQ-001

  # Save to a file
  rela render REQ-001 -o rendered.md

  # Include YAML frontmatter
  rela render REQ-001 --frontmatter

  # Limit transclusion depth
  rela render REQ-001 --max-depth 3`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		entityID := args[0]

		// Validate entity exists
		entity, ok := g.GetNode(entityID)
		if !ok {
			// Try to find similar IDs
			suggestions := findSimilarEntities(entityID)
			if len(suggestions) > 0 {
				return fmt.Errorf("entity not found: %s\n\nDid you mean?\n  %s",
					entityID, strings.Join(suggestions, "\n  "))
			}
			return fmt.Errorf("entity not found: %s", entityID)
		}

		// Create resolver
		resolver := transclusion.NewResolver(g)
		if renderMaxDepth > 0 {
			resolver = resolver.WithMaxDepth(renderMaxDepth)
		}

		// Render with options
		opts := transclusion.RenderOptions{
			IncludeFrontmatter: !renderNoFrontmatter,
			MaxDepth:           renderMaxDepth,
			StripComments:      !renderKeepComments,
		}

		rendered, err := resolver.RenderEntity(entity.ID, opts)
		if err != nil {
			return fmt.Errorf("failed to render entity: %w", err)
		}

		// Output
		if renderOutput != "" {
			if err := os.WriteFile(renderOutput, []byte(rendered), 0644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}
			fmt.Printf("Rendered to %s\n", renderOutput)
		} else {
			fmt.Print(rendered)
		}

		return nil
	},
}

const maxSuggestions = 5

// findSimilarEntities finds entities with similar IDs.
func findSimilarEntities(id string) []string {
	var suggestions []string
	idLower := strings.ToLower(id)

	for _, entity := range g.AllNodes() {
		entityIDLower := strings.ToLower(entity.ID)
		// Check for prefix match or contains
		if strings.HasPrefix(entityIDLower, idLower) || strings.Contains(entityIDLower, idLower) {
			suggestions = append(suggestions, entity.ID)
			if len(suggestions) >= maxSuggestions {
				break
			}
		}
	}

	return suggestions
}

func init() {
	renderCmd.Flags().StringVarP(&renderOutput, "output", "o", "", "Output file (default: stdout)")
	renderCmd.Flags().BoolVar(&renderNoFrontmatter, "no-frontmatter", false, "Exclude YAML frontmatter")
	renderCmd.Flags().IntVar(&renderMaxDepth, "max-depth", transclusion.DefaultMaxDepth, "Maximum transclusion depth")
	renderCmd.Flags().BoolVar(&renderKeepComments, "keep-comments", false, "Keep HTML comments in output")

	rootCmd.AddCommand(renderCmd)
}
