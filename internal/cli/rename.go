package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RenameCmd is the parent of rename subcommands.
type RenameCmd struct {
	Entity RenameEntityCmd `cmd:"" help:"Rename an entity type."`
	ID     RenameIDCmd     `cmd:"" name:"id" help:"Rename an entity's ID."`
}

// RenameEntityCmd renames an entity type.
type RenameEntityCmd struct {
	OldType string `arg:"" help:"Existing entity type name."`
	NewType string `arg:"" help:"New entity type name."`
	Force   bool   `short:"f" help:"Skip confirmation prompt."`
	Plural  string `help:"Override plural form for directory name."`
}

// Run dispatches `rela rename entity <old> <new>`.
func (c *RenameEntityCmd) Run(svc *cliServices) error {
	return runRenameEntity(svc, c.OldType, c.NewType, c.Force, c.Plural)
}

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
func runRenameEntity(svc *cliServices, oldType, newType string, force bool, plural string) error {
	info, err := resolveRenameEntity(svc, oldType, newType, plural)
	if err != nil {
		return err
	}
	showRenamePreview(info)

	if !force {
		confirmed, confirmErr := confirmRename()
		if confirmErr != nil {
			return confirmErr
		}
		if !confirmed {
			out.WriteMessage("Cancelled")
			return nil
		}
	}
	return applyRenameEntity(svc, info)
}

func resolveRenameEntity(svc *cliServices, oldType, newType, renamePlural string) (*renameEntityInfo, error) {
	meta := svc.Meta()
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
	oldPlural := oldDef.GetPlural(resolvedOld)
	newPlural := renamePlural
	if newPlural == "" {
		newPlural = newType + "s"
	}
	paths := svc.Paths()
	oldTemplatePath := paths.EntityTemplatePath(resolvedOld)
	_, statErr := svc.FS().Stat(oldTemplatePath)
	entityCount, _ := svc.Store().CountEntities(context.Background(), store.EntityQuery{Type: resolvedOld})

	return &renameEntityInfo{
		resolvedOld:     resolvedOld,
		newType:         newType,
		oldPlural:       oldPlural,
		newPlural:       newPlural,
		oldDir:          paths.EntityTypeDirWithPlural(oldPlural),
		newDir:          paths.EntityTypeDirWithPlural(newPlural),
		entityCount:     entityCount,
		relCount:        countAffectedRelations(meta, resolvedOld),
		oldTemplatePath: oldTemplatePath,
		hasTemplate:     statErr == nil,
	}, nil
}

func countAffectedRelations(meta *metamodel.Metamodel, entityType string) int {
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

func applyRenameEntity(svc *cliServices, info *renameEntityInfo) error {
	count, err := svc.RenameEntityType(info.resolvedOld, info.newType, info.newPlural)
	if err != nil {
		return err
	}
	if count > 0 {
		out.WriteMessage("  Updated %d entity file(s)", count)
	}
	out.WriteSuccess("Renamed entity type: %s → %s", info.resolvedOld, info.newType)
	return nil
}

func validateTypeName(name string) error {
	if name == "" {
		return errors.New("type name cannot be empty")
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

// RenameIDCmd renames an entity's ID.
type RenameIDCmd struct {
	OldID  string `arg:"" name:"old-id" help:"Existing entity ID."`
	NewID  string `arg:"" name:"new-id" help:"New entity ID."`
	DryRun bool   `name:"dry-run" help:"Preview changes without applying."`
}

// Run dispatches `rela rename id <old> <new>`.
func (c *RenameIDCmd) Run(svc *cliServices) error {
	result, err := svc.EntityManager().RenameEntity(
		context.Background(), c.OldID, c.NewID, entity.RenameOptions{DryRun: c.DryRun})
	if err != nil {
		return err
	}
	verb := "Renamed"
	if c.DryRun {
		verb = "Dry run — would rename"
	}
	out.WriteMessage("%s: %s → %s (%d relations updated)", verb, result.OldID, result.NewID, result.RelationsUpdated)
	return nil
}
