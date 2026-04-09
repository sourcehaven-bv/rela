package cli

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/rename"
)

var (
	renameForce    bool
	renamePlural   string
	renameIDDryRun bool
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
	_, statErr := ws.FS().Stat(oldTemplatePath)

	return &renameEntityInfo{
		resolvedOld:     resolvedOld,
		newType:         newType,
		oldPlural:       oldPlural,
		newPlural:       newPlural,
		oldDir:          projectCtx.EntityTypeDirWithPlural(oldPlural),
		newDir:          projectCtx.EntityTypeDirWithPlural(newPlural),
		entityCount:     len(ws.EntitiesByType(resolvedOld)),
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
	count, err := ws.RenameEntityType(info.resolvedOld, info.newType, info.newPlural)
	if err != nil {
		return err
	}
	if count > 0 {
		out.WriteMessage("  Updated %d entity file(s)", count)
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

var renameIDCmd = &cobra.Command{
	Use:   "id <old-id> <new-id>",
	Short: "Rename an entity's ID",
	Long: `Renames an entity's ID and updates all relations that reference it.

This updates:
  - The entity file (renamed and id field updated)
  - All relation files where this entity is the 'from' or 'to' endpoint

Examples:
  rela rename id REQ-001 REQ-100
  rela rename id REQ-001 REQ-100 --dry-run`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRenameID(args[0], args[1])
	},
}

func runRenameID(oldID, newID string) error {
	// Get entity to find type
	entity, ok := ws.GetEntity(oldID)
	if !ok {
		return fmt.Errorf("entity not found: %s", oldID)
	}

	result, err := ws.Rename(entity.Type, oldID, newID, rename.Options{DryRun: renameIDDryRun})
	if err != nil {
		return err
	}

	if renameIDDryRun {
		out.WriteMessage("Dry run - no changes made")
		out.WriteMessage("")
	}

	out.WriteMessage("Rename: %s → %s", result.OldID, result.NewID)
	out.WriteMessage("Entity file: %s", result.EntityFile)

	if len(result.RelationsUpdated) > 0 {
		out.WriteMessage("\nRelations updated (%d):", len(result.RelationsUpdated))
		for _, rel := range result.RelationsUpdated {
			out.WriteMessage("  %s --%s--> %s", rel.From, rel.Type, rel.To)
		}
	} else {
		out.WriteMessage("\nNo relations updated")
	}

	if !renameIDDryRun {
		if saveErr := ws.SaveCache(); saveErr != nil {
			out.WriteWarning("Failed to save cache: %v", saveErr)
		}
		if len(result.OldFilesDeleted) > 0 {
			out.WriteMessage("\nOld files deleted (%d)", len(result.OldFilesDeleted))
		}
	}

	return nil
}

func init() {
	renameEntityCmd.Flags().BoolVarP(&renameForce, "force", "f", false, "Skip confirmation prompt")
	renameEntityCmd.Flags().StringVar(&renamePlural, "plural", "", "Override plural form for directory name")

	renameIDCmd.Flags().BoolVar(&renameIDDryRun, "dry-run", false, "Preview changes without applying")

	renameCmd.AddCommand(renameEntityCmd)
	renameCmd.AddCommand(renameIDCmd)
	rootCmd.AddCommand(renameCmd)
}
