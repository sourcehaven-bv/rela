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
}

// TotalChanges returns the total number of planned changes.
func (p *CleanupPlan) TotalChanges() int {
	return len(p.MetamodelChanges) + len(p.DataEntryChanges)
}

// IsEmpty returns true if there are no planned changes.
func (p *CleanupPlan) IsEmpty() bool {
	return p.TotalChanges() == 0
}

// PlanCleanup creates a cleanup plan based on the analysis results.
// Types with zero instances are removed along with all their references
// in config files (cascade cleanup).
func PlanCleanup(analysis *Analysis) *CleanupPlan {
	plan := &CleanupPlan{}

	// Plan removal of unused entity types and their references
	for _, usage := range analysis.UnusedEntityTypes {
		if canSafelyRemove(usage) {
			plan.MetamodelChanges = append(plan.MetamodelChanges, Change{
				File:   "metamodel.yaml",
				Action: "remove_entity_type",
				Target: usage.Name,
			})
			// Cascade: remove references in config files
			planEntityTypeCascade(plan, usage)
		}
	}

	// Plan removal of unused relation types and their references
	for _, usage := range analysis.UnusedRelationTypes {
		if canSafelyRemove(usage) {
			plan.MetamodelChanges = append(plan.MetamodelChanges, Change{
				File:   "metamodel.yaml",
				Action: "remove_relation_type",
				Target: usage.Name,
			})
			// Cascade: remove references in config files
			planRelationTypeCascade(plan, usage)
		}
	}

	// Plan removal of unused custom types (they have no instances by definition)
	for _, usage := range analysis.UnusedCustomTypes {
		plan.MetamodelChanges = append(plan.MetamodelChanges, Change{
			File:   "metamodel.yaml",
			Action: "remove_custom_type",
			Target: usage.Name,
		})
	}

	return plan
}

// planEntityTypeCascade adds cascade changes for an entity type's references.
func planEntityTypeCascade(plan *CleanupPlan, usage TypeUsage) {
	for _, ref := range usage.References {
		switch ref.Kind {
		case "form":
			plan.DataEntryChanges = append(plan.DataEntryChanges, Change{
				File:   ref.File,
				Action: "remove_form",
				Target: extractName(ref.Section, "forms."),
			})
		case "list":
			plan.DataEntryChanges = append(plan.DataEntryChanges, Change{
				File:   ref.File,
				Action: "remove_list",
				Target: extractName(ref.Section, "lists."),
			})
		case "kanban":
			plan.DataEntryChanges = append(plan.DataEntryChanges, Change{
				File:   ref.File,
				Action: "remove_kanban",
				Target: extractName(ref.Section, "kanbans."),
			})
		case "view":
			plan.DataEntryChanges = append(plan.DataEntryChanges, Change{
				File:   ref.File,
				Action: "remove_data_entry_view",
				Target: extractName(ref.Section, "views."),
			})
		case "validation":
			plan.MetamodelChanges = append(plan.MetamodelChanges, Change{
				File:   ref.File,
				Action: "remove_validation",
				Target: extractName(ref.Section, "validations."),
			})
		case "automation":
			plan.MetamodelChanges = append(plan.MetamodelChanges, Change{
				File:   ref.File,
				Action: "remove_automation",
				Target: extractName(ref.Section, "automations."),
			})
		case "relation_from", "relation_to":
			// These will be handled when the relation type itself is cleaned up
			// or when we clean up the entity type from relation definitions
		}
	}
}

// planRelationTypeCascade adds cascade changes for a relation type's references.
// Currently a no-op: relation type references in forms/automations require
// surgical removal (just the relation field, not the whole config) which is complex.
// TODO: Implement surgical removal of relation references.
func planRelationTypeCascade(_ *CleanupPlan, _ TypeUsage) {
	// Relation references are more complex to cascade:
	// - form.relations[].relation - remove just that relation entry
	// - view.traverse[].follow - remove just that traverse rule
	// - automation triggers/actions - remove just the relation reference
	// For now, unused relation types are removed but their config references remain
	// (which may cause validation errors until manually cleaned up)
}

// extractName extracts the name from a section path like "forms.my-form" -> "my-form".
func extractName(section, prefix string) string {
	if len(section) > len(prefix) && section[:len(prefix)] == prefix {
		name := section[len(prefix):]
		// Handle nested paths like "forms.my-form.relations" -> "my-form"
		for i, c := range name {
			if c == '.' {
				return name[:i]
			}
		}
		return name
	}
	return section
}

// canSafelyRemove returns true if a type can be safely removed.
// A type can be safely removed if it has no instances. References in config
// files (forms, lists, etc.) will be cascade-removed along with the type.
func canSafelyRemove(usage TypeUsage) bool {
	return usage.Count == 0
}

// ExecuteCleanup applies the cleanup plan to the project files.
// projectRoot is the path to the project root directory.
// If dryRun is true, no files are modified.
func ExecuteCleanup(plan *CleanupPlan, projectRoot string, dryRun bool) error {
	if plan.IsEmpty() {
		return nil
	}

	metamodelPath := filepath.Join(projectRoot, "metamodel.yaml")
	dataEntryPath := filepath.Join(projectRoot, "data-entry.yaml")

	// Apply metamodel changes
	if len(plan.MetamodelChanges) > 0 {
		if err := applyMetamodelChanges(metamodelPath, plan.MetamodelChanges, dryRun); err != nil {
			return fmt.Errorf("failed to update metamodel.yaml: %w", err)
		}
	}

	// Apply data-entry.yaml changes
	if len(plan.DataEntryChanges) > 0 {
		if err := applyDataEntryChanges(dataEntryPath, plan.DataEntryChanges, dryRun); err != nil {
			return fmt.Errorf("failed to update data-entry.yaml: %w", err)
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

// applyDataEntryChanges applies changes to data-entry.yaml using AST manipulation.
func applyDataEntryChanges(path string, changes []Change, dryRun bool) error {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // No data-entry.yaml, nothing to clean
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var doc yaml.Node
	if unmarshalErr := yaml.Unmarshal(data, &doc); unmarshalErr != nil {
		return unmarshalErr
	}

	root := migration.GetDocumentRoot(&doc)
	if root == nil {
		return fmt.Errorf("failed to get document root")
	}

	for _, change := range changes {
		switch change.Action {
		case "remove_form":
			forms := migration.GetMapValue(root, "forms")
			if forms != nil {
				migration.DeleteMapKey(forms, change.Target)
			}
		case "remove_list":
			lists := migration.GetMapValue(root, "lists")
			if lists != nil {
				migration.DeleteMapKey(lists, change.Target)
			}
		case "remove_kanban":
			kanbans := migration.GetMapValue(root, "kanbans")
			if kanbans != nil {
				migration.DeleteMapKey(kanbans, change.Target)
			}
		case "remove_data_entry_view":
			views := migration.GetMapValue(root, "views")
			if views != nil {
				migration.DeleteMapKey(views, change.Target)
			}
		}
	}

	if dryRun {
		return nil
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}

	return os.WriteFile(path, out, 0o644)
}
