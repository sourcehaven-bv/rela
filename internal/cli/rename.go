package cli

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

var (
	renameForce  bool
	renamePlural string
)

var renameCmd = &cobra.Command{
	Use:   "rename",
	Short: "Rename entities or relations",
	Long:  `Rename entity types or relation types across the project.`,
}

var renameEntityCmd = &cobra.Command{
	Use:   "entity <old-type> <new-type>",
	Short: "Rename an entity type",
	Long: `Renames an entity type across the entire project.

This updates:
  - The entity key in metamodel.yaml
  - All relation from/to references in metamodel.yaml
  - All validation entity_type references in metamodel.yaml
  - The entity directory (e.g., entities/issues/ → entities/tickets/)
  - The type field in all entity markdown files
  - Entity templates (if they exist)

Examples:
  rela rename entity issue ticket
  rela rename entity issue ticket --plural tickets
  rela rename entity requirement feature --force`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRenameEntity(args[0], args[1])
	},
}

// renameEntityInfo holds the resolved information needed for a rename operation.
type renameEntityInfo struct {
	resolvedOld     string
	newType         string
	oldPlural       string
	newPlural       string
	oldDir          string
	newDir          string
	entityCount     int
	relCount        int
	oldTemplatePath string
	hasTemplate     bool
}

// coverage-ignore: interactive CLI - tested via integration tests
func runRenameEntity(oldType, newType string) error {
	info, err := resolveRenameEntity(oldType, newType)
	if err != nil {
		return err
	}

	showRenamePreview(info)

	if !renameForce {
		confirmed, confirmErr := confirmRename()
		if confirmErr != nil {
			return confirmErr
		}
		if !confirmed {
			out.WriteMessage("Cancelled")
			return nil
		}
	}

	return applyRenameEntity(info)
}

func resolveRenameEntity(oldType, newType string) (*renameEntityInfo, error) {
	resolvedOld := meta.ResolveAlias(oldType)
	oldDef, ok := meta.GetEntityDef(resolvedOld)
	if !ok {
		return nil, fmt.Errorf("entity type %q not found in metamodel", oldType)
	}

	if err := validateTypeName(newType); err != nil {
		return nil, err
	}

	if _, exists := meta.GetEntityDef(newType); exists {
		return nil, fmt.Errorf("entity type %q already exists in metamodel", newType)
	}

	oldPlural := oldDef.GetDirPlural(resolvedOld)
	newPlural := renamePlural
	if newPlural == "" {
		newPlural = newType + "s"
	}

	oldTemplatePath := projectCtx.EntityTemplatePath(resolvedOld)
	_, statErr := cliFS.Stat(oldTemplatePath)

	return &renameEntityInfo{
		resolvedOld:     resolvedOld,
		newType:         newType,
		oldPlural:       oldPlural,
		newPlural:       newPlural,
		oldDir:          projectCtx.EntityTypeDirWithPlural(oldPlural),
		newDir:          projectCtx.EntityTypeDirWithPlural(newPlural),
		entityCount:     len(g.NodesByType(resolvedOld)),
		relCount:        countAffectedRelations(resolvedOld),
		oldTemplatePath: oldTemplatePath,
		hasTemplate:     statErr == nil,
	}, nil
}

func countAffectedRelations(entityType string) int {
	count := 0
	for _, relName := range meta.RelationTypes() {
		relDef, _ := meta.GetRelationDef(relName)
		if relDef == nil {
			continue
		}
		if sliceContains(relDef.From, entityType) {
			count++
		}
		if sliceContains(relDef.To, entityType) {
			count++
		}
	}
	return count
}

func showRenamePreview(info *renameEntityInfo) {
	out.WriteMessage("Rename entity type: %s → %s", info.resolvedOld, info.newType)
	out.WriteMessage("  metamodel.yaml:  entity key + %d relation reference(s)", info.relCount)
	if info.entityCount > 0 {
		out.WriteMessage("  directory:       %s → %s", info.oldPlural, info.newPlural)
		out.WriteMessage("  entity files:    %d file(s) will be updated", info.entityCount)
	}
	if info.hasTemplate {
		out.WriteMessage("  template:        %s.md → %s.md", info.resolvedOld, info.newType)
	}
}

// coverage-ignore: interactive prompt
func confirmRename() (bool, error) {
	fmt.Print("\nProceed? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read input: %w", err)
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

func applyRenameEntity(info *renameEntityInfo) error {
	renameErr := metamodel.RenameEntityTypeFS(projectCtx.MetamodelPath, info.resolvedOld, info.newType, cliFS)
	if renameErr != nil {
		return fmt.Errorf("failed to update metamodel: %w", renameErr)
	}

	if _, err := cliFS.Stat(info.oldDir); err == nil {
		if renameErr := cliFS.Rename(info.oldDir, info.newDir); renameErr != nil {
			return fmt.Errorf("failed to rename directory: %w", renameErr)
		}
	}

	if _, err := cliFS.Stat(info.newDir); err == nil {
		count, updateErr := markdown.NewFileIO(cliFS).UpdateEntityTypesInDir(info.newDir, info.newType, meta)
		if updateErr != nil {
			return fmt.Errorf("failed to update entity files: %w", updateErr)
		}
		if count > 0 {
			out.WriteMessage("  Updated %d entity file(s)", count)
		}
	}

	if info.hasTemplate {
		newTemplatePath := projectCtx.EntityTemplatePath(info.newType)
		if mkdirErr := cliFS.MkdirAll(projectCtx.EntityTemplatesDir, 0755); mkdirErr != nil {
			out.WriteWarning("Failed to create templates directory: %v", mkdirErr)
		} else if renameErr := cliFS.Rename(info.oldTemplatePath, newTemplatePath); renameErr != nil {
			out.WriteWarning("Failed to rename template: %v", renameErr)
		}
	}

	if err := cliFS.Remove(projectCtx.CachePath); err != nil && !os.IsNotExist(err) {
		out.WriteWarning("Failed to remove cache: %v", err)
	}

	out.WriteSuccess("Renamed entity type: %s → %s", info.resolvedOld, info.newType)
	return nil
}

// validateTypeName checks that a type name is valid for use in the metamodel.
func validateTypeName(name string) error {
	if name == "" {
		return fmt.Errorf("type name cannot be empty")
	}

	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("invalid characters in type name: %s", name)
	}

	valid := regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	if !valid.MatchString(name) {
		return fmt.Errorf(
			"invalid type name %q: must start with a lowercase letter and contain only lowercase letters, digits, hyphens, and underscores",
			name,
		)
	}

	return nil
}

func init() {
	renameEntityCmd.Flags().BoolVarP(&renameForce, "force", "f", false, "Skip confirmation prompt")
	renameEntityCmd.Flags().StringVar(&renamePlural, "plural", "", "Override plural form for directory name")

	renameCmd.AddCommand(renameEntityCmd)
	rootCmd.AddCommand(renameCmd)
}
