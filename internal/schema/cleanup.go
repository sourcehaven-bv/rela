package schema

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/migration"
)

// Change represents a planned modification to a file.
type Change struct {
	File   string `json:"file"`   // File path relative to project root
	Action string `json:"action"` // "remove_entity_type", "remove_relation_type", "remove_custom_type", "remove_form", etc.
	Target string `json:"target"` // What's being removed (type name, form name, etc.)
}

// CleanupPlan contains the planned changes for cleanup.
type CleanupPlan struct {
	MetamodelChanges []Change `json:"metamodel_changes"`
	DataEntryChanges []Change `json:"data_entry_changes"`
	ViewsChanges     []Change `json:"views_changes"`
}

// TotalChanges returns the total number of planned changes.
func (p *CleanupPlan) TotalChanges() int {
	return len(p.MetamodelChanges) + len(p.DataEntryChanges) + len(p.ViewsChanges)
}

// IsEmpty returns true if there are no planned changes.
func (p *CleanupPlan) IsEmpty() bool {
	return p.TotalChanges() == 0
}

// PlanCleanup creates a cleanup plan based on the analysis results.
// Only types with zero instances AND zero references (except in the files being cleaned)
// can be safely removed.
func PlanCleanup(analysis *Analysis) *CleanupPlan {
	plan := &CleanupPlan{}

	// Plan removal of unused entity types (only those with no references)
	for _, usage := range analysis.UnusedEntityTypes {
		if canSafelyRemove(usage) {
			plan.MetamodelChanges = append(plan.MetamodelChanges, Change{
				File:   "metamodel.yaml",
				Action: "remove_entity_type",
				Target: usage.Name,
			})
		}
	}

	// Plan removal of unused relation types (only those with no references)
	for _, usage := range analysis.UnusedRelationTypes {
		if canSafelyRemove(usage) {
			plan.MetamodelChanges = append(plan.MetamodelChanges, Change{
				File:   "metamodel.yaml",
				Action: "remove_relation_type",
				Target: usage.Name,
			})
		}
	}

	// Plan removal of unused custom types (they have no instances by definition)
	for _, usage := range analysis.UnusedCustomTypes {
		// UnusedCustomTypes already have no references
		plan.MetamodelChanges = append(plan.MetamodelChanges, Change{
			File:   "metamodel.yaml",
			Action: "remove_custom_type",
			Target: usage.Name,
		})
	}

	return plan
}

// canSafelyRemove returns true if a type can be safely removed.
// A type can be safely removed if it has no instances and no references
// in configuration files (data-entry.yaml, views.yaml, validations, automations).
func canSafelyRemove(usage TypeUsage) bool {
	// Must have zero instances
	if usage.Count > 0 {
		return false
	}

	// Check if any references would prevent removal
	for _, ref := range usage.References {
		// References in relation from/to are okay - they'll be cleaned up when
		// we remove the entity type. But other references prevent removal.
		switch ref.Kind {
		case "relation_from", "relation_to":
			// These are okay - the relation definition references the entity type
			// but if the entity type has no instances, it's safe to remove
			continue
		default:
			// Any other reference (form, list, view, validation, automation) prevents removal
			return false
		}
	}

	return true
}

// ExecuteCleanup applies the cleanup plan to the project files.
// projectRoot is the path to the project root directory.
// If dryRun is true, no files are modified.
func ExecuteCleanup(plan *CleanupPlan, projectRoot string, dryRun bool) error {
	if plan.IsEmpty() {
		return nil
	}

	// Group changes by file
	metamodelPath := filepath.Join(projectRoot, "metamodel.yaml")

	// Apply metamodel changes
	if len(plan.MetamodelChanges) > 0 {
		if err := applyMetamodelChanges(metamodelPath, plan.MetamodelChanges, dryRun); err != nil {
			return fmt.Errorf("failed to update metamodel.yaml: %w", err)
		}
	}

	return nil
}

// applyMetamodelChanges applies changes to metamodel.yaml using AST manipulation.
func applyMetamodelChanges(path string, changes []Change, dryRun bool) error {
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Parse as YAML AST
	var doc yaml.Node
	if unmarshalErr := yaml.Unmarshal(data, &doc); unmarshalErr != nil {
		return unmarshalErr
	}

	root := migration.GetDocumentRoot(&doc)
	if root == nil {
		return fmt.Errorf("failed to get document root")
	}

	// Apply each change
	for _, change := range changes {
		switch change.Action {
		case "remove_entity_type":
			entities := migration.GetMapValue(root, "entities")
			if entities != nil {
				migration.DeleteMapKey(entities, change.Target)
			}
		case "remove_relation_type":
			relations := migration.GetMapValue(root, "relations")
			if relations != nil {
				migration.DeleteMapKey(relations, change.Target)
			}
		case "remove_custom_type":
			types := migration.GetMapValue(root, "types")
			if types != nil {
				migration.DeleteMapKey(types, change.Target)
			}
		}
	}

	if dryRun {
		return nil
	}

	// Write back
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}

	return os.WriteFile(path, out, 0o644)
}
