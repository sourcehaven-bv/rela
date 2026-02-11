package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

var (
	createTitle      string
	createStatus     string
	createPriority   string
	createID         string
	createProperties []string
	createBody       string
	createBodyFile   string
)

var createCmd = &cobra.Command{
	Use:   "create <type>",
	Short: "Create a new entity",
	Long: `Creates a new entity of the specified type.

The -t/--title flag sets the primary required property for the entity type.
For most types this is "title", but for some types (like stakeholder) it may
be "name" or another property. Use -P/--property for setting arbitrary properties.

The --body flag sets the markdown body content directly, while --body-file reads
the body from a file. Use "-" as the filename to read from stdin.

Examples:
  rela create requirement --title "System must handle 1000 users"
  rela create decision --title "Use PostgreSQL for persistence"
  rela create req -t "Short alias works too"
  rela create stakeholder -t "John Smith"
  rela create control -t "Access Control" -P "iso27001=A.5.15" -P "owner=Security Team"
  rela create requirement -t "Auth feature" --body "## Description\n\nUser authentication."
  rela create requirement -t "From file" --body-file description.md
  echo "Content from stdin" | rela create requirement -t "Piped" --body-file -`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		typeName := args[0]

		// Resolve type (handle aliases)
		resolvedType, entityDef, err := resolveEntityType(typeName)
		if err != nil {
			return err
		}

		// Generate or validate ID
		var entityID string
		if createID != "" {
			// User provided ID
			if validErr := model.ValidateID(createID); validErr != nil {
				return validErr
			}
			// Check if ID already exists
			if _, exists := g.GetNode(createID); exists {
				return fmt.Errorf("entity with ID %s already exists", createID)
			}
			entityID = createID
		} else {
			// Auto-generate ID (only for auto/short ID types)
			if entityDef.IsManualID() {
				return fmt.Errorf("entity type %s uses manual IDs; --id is required", resolvedType)
			}
			prefixes := entityDef.GetIDPrefixes()
			if len(prefixes) == 0 {
				return fmt.Errorf("no ID prefixes defined for type %s", resolvedType)
			}
			prefix := prefixes[0]
			existingIDs := g.AllIDs()

			if entityDef.IsShortID() {
				entityID = model.GenerateShortID(existingIDs, prefix, g.NodeCount())
			} else {
				// Auto (sequential) IDs
				entityID = model.GenerateNextID(existingIDs, prefix)
			}
		}

		// Create entity
		entity := model.NewEntity(entityID, resolvedType)

		// Load and apply template defaults first (if template exists)
		template, err := repo.LoadEntityTemplate(resolvedType)
		if err != nil {
			return fmt.Errorf("failed to load template: %w", err)
		}
		if template != nil {
			markdown.ApplyEntityTemplate(entity, template)
		}

		// Parse and apply --property flags (overrides template defaults)
		for _, prop := range createProperties {
			key, value, parseErr := parsePropertyFlag(prop)
			if parseErr != nil {
				return parseErr
			}
			entity.SetString(key, value)
		}

		// Set the primary property using -t/--title flag
		// The -t flag sets whichever property is the "primary" required string property
		// for this entity type (typically "title", but could be "name" for stakeholder, etc.)
		if strings.TrimSpace(createTitle) != "" {
			primaryProp := entityDef.GetPrimaryProperty()
			if primaryProp == "" {
				// Fallback to "title" if no primary property found
				primaryProp = "title"
			}
			entity.SetString(primaryProp, createTitle)
		}

		// Set status: CLI flag > template > metamodel default
		if createStatus != "" {
			// CLI flag takes precedence
			entity.SetString("status", createStatus)
		} else if entity.GetString("status") == "" {
			// No CLI flag and no template value, use metamodel default
			entity.SetString("status", entityDef.GetDefaultStatus(meta))
		}
		// If template set a status and no CLI flag, keep the template value

		// Set priority if provided
		if createPriority != "" {
			entity.SetString("priority", createPriority)
		}

		// Set body content
		bodyContent, err := getBodyContent(cmd)
		if err != nil {
			return err
		}
		if bodyContent != "" {
			entity.Content = bodyContent
		}

		// Validate entity
		errs := meta.ValidateEntity(entity)
		if len(errs) > 0 {
			var errMsgs []string
			for _, e := range errs {
				errMsgs = append(errMsgs, e.Error())
			}
			return fmt.Errorf("validation errors:\n  %s", strings.Join(errMsgs, "\n  "))
		}

		// Write to file (repo computes path and sets entity.FilePath)
		if err := repo.WriteEntity(entity, meta); err != nil {
			return fmt.Errorf("failed to write entity: %w", err)
		}

		// Add to graph
		g.AddNode(entity)

		// Save cache
		if err := saveCache(); err != nil {
			out.WriteWarning("Failed to save cache: %v", err)
		}

		out.WriteSuccess("Created %s %s", resolvedType, entityID)
		if outputFormat == "json" {
			_ = out.WriteEntities([]*model.Entity{entity})
		}

		return nil
	},
}

// getBodyContent returns the body content from --body or --body-file flags.
// Returns an error if both flags are specified or if file reading fails.
func getBodyContent(cmd *cobra.Command) (string, error) {
	if createBody != "" && createBodyFile != "" {
		return "", fmt.Errorf("cannot specify both --body and --body-file")
	}

	if createBody != "" {
		return createBody, nil
	}

	if createBodyFile != "" {
		var content []byte
		var err error

		if createBodyFile == "-" {
			content, err = io.ReadAll(cmd.InOrStdin())
		} else {
			content, err = cliFS.ReadFile(createBodyFile)
		}

		if err != nil {
			return "", fmt.Errorf("failed to read body file: %w", err)
		}
		return strings.TrimSpace(string(content)), nil
	}

	return "", nil
}

// parsePropertyFlag parses a "key=value" property flag.
// Returns an error if the format is invalid.
func parsePropertyFlag(prop string) (key, value string, err error) {
	idx := strings.Index(prop, "=")
	if idx == -1 {
		return "", "", fmt.Errorf("invalid property format %q: expected key=value", prop)
	}
	key = strings.TrimSpace(prop[:idx])
	value = strings.TrimSpace(prop[idx+1:])
	if key == "" {
		return "", "", fmt.Errorf("invalid property format %q: key cannot be empty", prop)
	}
	return key, value, nil
}

func init() {
	createCmd.Flags().StringVarP(&createTitle, "title", "t", "", "Primary property value (title, name, etc. depending on entity type)")
	createCmd.Flags().StringVarP(&createStatus, "status", "s", "", "Entity status (defaults to entity type's default)")
	createCmd.Flags().StringVarP(&createPriority, "priority", "p", "", "Entity priority")
	createCmd.Flags().StringVar(&createID, "id", "", "Custom entity ID (auto-generated if not provided)")
	createCmd.Flags().StringArrayVarP(&createProperties, "property", "P", nil, "Set a property (format: key=value, can be repeated)")
	createCmd.Flags().StringVarP(&createBody, "body", "b", "", "Markdown body content for the entity")
	createCmd.Flags().StringVarP(&createBodyFile, "body-file", "B", "", "Read body content from file (use - for stdin)")

	rootCmd.AddCommand(createCmd)
}
