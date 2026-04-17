package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

var (
	updateTitle       string
	updateStatus      string
	updatePriority    string
	updateDescription string
	updateProperties  []string
	updateBody        string
	updateBodyFile    string
)

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an entity",
	Long: `Updates properties of an existing entity.

Use -P/--property for setting arbitrary properties, including custom properties
defined in your metamodel.

The --body flag sets the markdown body content directly, while --body-file reads
the body from a file. Use "-" as the filename to read from stdin.

Examples:
  rela update REQ-001 --status accepted
  rela update DEC-042 --title "New title" --status proposed
  rela update RB-001 -P "review_status=current"
  rela update CTRL-001 -P "iso27001=A.5.15" -P "owner=Security Team"
  rela update REQ-001 --body "## Updated Description\n\nNew content here."
  rela update REQ-001 --body-file description.md
  echo "New content" | rela update REQ-001 --body-file -`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		entityID := args[0]

		entity, ok := ws.GetEntity(entityID)
		if !ok {
			return &entityNotFoundError{ID: entityID}
		}

		// Capture old state for automation property change detection
		oldEntity := entity.Clone()

		// Track if anything changed
		changed := false

		// Parse and apply --property flags first (so explicit flags can override if needed)
		for _, prop := range updateProperties {
			key, value, err := parsePropertyFlag(prop)
			if err != nil {
				return err
			}
			entity.SetString(key, value)
			changed = true
		}

		if updateTitle != "" {
			entity.SetString("title", updateTitle)
			changed = true
		}

		if updateStatus != "" {
			entity.SetString("status", updateStatus)
			changed = true
		}

		if updatePriority != "" {
			entity.SetString("priority", updatePriority)
			changed = true
		}

		if updateDescription != "" {
			entity.SetString("description", updateDescription)
			changed = true
		}

		// Set body content
		bodyContent, err := getUpdateBodyContent(cmd)
		if err != nil {
			return err
		}
		if bodyContent != "" {
			entity.Content = bodyContent
			changed = true
		}

		if !changed {
			return fmt.Errorf("no updates specified")
		}

		result, err := ws.UpdateEntity(model.EntityFromDomain(entity), model.EntityFromDomain(oldEntity))
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

		out.WriteSuccess("Updated %s", entityID)

		return nil
	},
}

// getUpdateBodyContent returns the body content from --body or --body-file flags.
// Returns an error if both flags are specified or if file reading fails.
func getUpdateBodyContent(cmd *cobra.Command) (string, error) {
	if updateBody != "" && updateBodyFile != "" {
		return "", fmt.Errorf("cannot specify both --body and --body-file")
	}

	if updateBody != "" {
		return updateBody, nil
	}

	if updateBodyFile != "" {
		var content []byte
		var err error

		if updateBodyFile == "-" {
			content, err = io.ReadAll(cmd.InOrStdin())
		} else {
			content, err = os.ReadFile(updateBodyFile)
		}

		if err != nil {
			return "", fmt.Errorf("failed to read body file: %w", err)
		}
		return strings.TrimSpace(string(content)), nil
	}

	return "", nil
}

func init() {
	updateCmd.Flags().StringVarP(&updateTitle, "title", "t", "", "New title")
	updateCmd.Flags().StringVarP(&updateStatus, "status", "s", "", "New status")
	updateCmd.Flags().StringVarP(&updatePriority, "priority", "p", "", "New priority")
	updateCmd.Flags().StringVarP(&updateDescription, "description", "d", "", "New description")
	updateCmd.Flags().StringArrayVarP(&updateProperties, "property", "P", nil, "Set a property (format: key=value, can be repeated)")
	updateCmd.Flags().StringVarP(&updateBody, "body", "b", "", "Markdown body content for the entity")
	updateCmd.Flags().StringVarP(&updateBodyFile, "body-file", "B", "", "Read body content from file (use - for stdin)")

	rootCmd.AddCommand(updateCmd)
}
