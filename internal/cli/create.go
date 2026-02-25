package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
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

		// Build properties map
		props := make(map[string]interface{})
		for _, prop := range createProperties {
			key, value, parseErr := parsePropertyFlag(prop)
			if parseErr != nil {
				return parseErr
			}
			props[key] = value
		}

		// Set the primary property using -t/--title flag
		if strings.TrimSpace(createTitle) != "" {
			primaryProp := entityDef.GetPrimaryProperty()
			if primaryProp == "" {
				primaryProp = "title"
			}
			props[primaryProp] = createTitle
		}

		// Set explicit status/priority flags
		if createStatus != "" {
			props["status"] = createStatus
		}
		if createPriority != "" {
			props["priority"] = createPriority
		}

		// Read body content
		bodyContent, err := getBodyContent(cmd)
		if err != nil {
			return err
		}

		entity, result, err := ws.CreateEntity(resolvedType, workspace.CreateOptions{
			ID:         createID,
			Properties: props,
			Content:    bodyContent,
		})
		if err != nil {
			return err
		}

		// Show automation feedback
		for _, warning := range result.AutomationWarnings {
			out.WriteWarning("Automation: %s", warning)
		}
		for _, errMsg := range result.AutomationErrors {
			out.WriteWarning("Automation error: %s", errMsg)
		}
		for _, rel := range result.RelationsCreated {
			out.WriteInfo("Automation created relation: %s --%s--> %s", rel.From, rel.Type, rel.To)
		}

		out.WriteSuccess("Created %s %s", resolvedType, entity.ID)
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
