// Package schema provides analysis and cleanup utilities for metamodel schemas.
// It can identify unused entity types, relation types, and custom types,
// and clean them up from metamodel.yaml, data-entry.yaml, and views.yaml.
package schema

import (
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/views"
)

// Analysis contains the results of analyzing metamodel schema usage.
type Analysis struct {
	// UnusedEntityTypes are entity types with zero instances
	UnusedEntityTypes []TypeUsage `json:"unused_entity_types"`
	// UnusedRelationTypes are relation types with zero instances
	UnusedRelationTypes []TypeUsage `json:"unused_relation_types"`
	// UnusedCustomTypes are custom types (enums) not referenced by any property
	UnusedCustomTypes []TypeUsage `json:"unused_custom_types"`
	// LowUsageEntityTypes are entity types with instances <= threshold (but > 0)
	LowUsageEntityTypes []TypeUsage `json:"low_usage_entity_types,omitempty"`
	// LowUsageRelationTypes are relation types with instances <= threshold (but > 0)
	LowUsageRelationTypes []TypeUsage `json:"low_usage_relation_types,omitempty"`
}

// TypeUsage describes usage of a type in the schema.
type TypeUsage struct {
	Name       string      `json:"name"`
	Count      int         `json:"count"`                // Instance count (0 for custom types)
	References []Reference `json:"references,omitempty"` // Where this type is referenced
}

// Reference describes where a type is referenced.
type Reference struct {
	File    string `json:"file"`    // "metamodel.yaml", "data-entry.yaml", "views.yaml"
	Section string `json:"section"` // e.g., "forms.ticket", "entities.ticket.properties.status"
	Kind    string `json:"kind"`    // "form", "list", "view", "kanban", "validation", "automation", "property", "relation_from", "relation_to"
}

// Analyze examines metamodel usage and returns analysis results.
// threshold controls what counts as "low usage" - types with 0 < count <= threshold
// are reported in LowUsage* fields. Use threshold=0 to only report unused types.
func Analyze(
	meta *metamodel.Metamodel,
	g *graph.Graph,
	dataEntry *dataentryconfig.Config,
	viewsFile *views.File,
	threshold int,
) *Analysis {
	result := &Analysis{}

	// Analyze entity types
	for _, entityType := range meta.EntityTypes() {
		count := len(g.NodesByType(entityType))
		refs := findEntityTypeReferences(entityType, meta, dataEntry, viewsFile)

		usage := TypeUsage{
			Name:       entityType,
			Count:      count,
			References: refs,
		}

		if count == 0 {
			result.UnusedEntityTypes = append(result.UnusedEntityTypes, usage)
		} else if threshold > 0 && count <= threshold {
			result.LowUsageEntityTypes = append(result.LowUsageEntityTypes, usage)
		}
	}

	// Analyze relation types
	for _, relationType := range meta.RelationTypes() {
		count := len(g.RelationsOfType(relationType))
		refs := findRelationTypeReferences(relationType, meta, dataEntry, viewsFile)

		usage := TypeUsage{
			Name:       relationType,
			Count:      count,
			References: refs,
		}

		if count == 0 {
			result.UnusedRelationTypes = append(result.UnusedRelationTypes, usage)
		} else if threshold > 0 && count <= threshold {
			result.LowUsageRelationTypes = append(result.LowUsageRelationTypes, usage)
		}
	}

	// Analyze custom types (enums)
	for typeName := range meta.Types {
		refs := findCustomTypeReferences(typeName, meta)

		usage := TypeUsage{
			Name:       typeName,
			Count:      0, // Custom types don't have instances
			References: refs,
		}

		if len(refs) == 0 {
			result.UnusedCustomTypes = append(result.UnusedCustomTypes, usage)
		}
	}

	// Sort results by name for consistent output
	sortTypeUsages(result.UnusedEntityTypes)
	sortTypeUsages(result.UnusedRelationTypes)
	sortTypeUsages(result.UnusedCustomTypes)
	sortTypeUsages(result.LowUsageEntityTypes)
	sortTypeUsages(result.LowUsageRelationTypes)

	return result
}

// TotalUnused returns the total count of unused types.
func (a *Analysis) TotalUnused() int {
	return len(a.UnusedEntityTypes) + len(a.UnusedRelationTypes) + len(a.UnusedCustomTypes)
}

// TotalLowUsage returns the total count of low-usage types.
func (a *Analysis) TotalLowUsage() int {
	return len(a.LowUsageEntityTypes) + len(a.LowUsageRelationTypes)
}

// HasIssues returns true if there are any unused or low-usage types.
func (a *Analysis) HasIssues() bool {
	return a.TotalUnused() > 0 || a.TotalLowUsage() > 0
}

// findEntityTypeReferences finds all references to an entity type.
func findEntityTypeReferences(
	entityType string,
	meta *metamodel.Metamodel,
	dataEntry *dataentryconfig.Config,
	viewsFile *views.File,
) []Reference {
	var refs []Reference

	// Check metamodel relations (from/to)
	for relName, relDef := range meta.Relations {
		for _, from := range relDef.From {
			if from == entityType {
				refs = append(refs, Reference{
					File:    "metamodel.yaml",
					Section: "relations." + relName + ".from",
					Kind:    "relation_from",
				})
				break
			}
		}
		for _, to := range relDef.To {
			if to == entityType {
				refs = append(refs, Reference{
					File:    "metamodel.yaml",
					Section: "relations." + relName + ".to",
					Kind:    "relation_to",
				})
				break
			}
		}
	}

	// Check metamodel validations
	for _, v := range meta.Validations {
		if v.EntityType == entityType {
			refs = append(refs, Reference{
				File:    "metamodel.yaml",
				Section: "validations." + v.Name,
				Kind:    "validation",
			})
		}
	}

	// Check metamodel automations
	for _, a := range meta.Automations {
		for _, et := range a.On.Entity {
			if et == entityType {
				refs = append(refs, Reference{
					File:    "metamodel.yaml",
					Section: "automations." + a.Name,
					Kind:    "automation",
				})
				break
			}
		}
	}

	// Check data-entry.yaml
	if dataEntry != nil {
		refs = append(refs, findEntityTypeInDataEntry(entityType, dataEntry)...)
	}

	// Check views.yaml
	if viewsFile != nil {
		refs = append(refs, findEntityTypeInViews(entityType, viewsFile)...)
	}

	return refs
}

// findEntityTypeInDataEntry finds references to an entity type in data-entry.yaml.
func findEntityTypeInDataEntry(entityType string, cfg *dataentryconfig.Config) []Reference {
	var refs []Reference

	// Forms
	for name, form := range cfg.Forms {
		if form.EntityType == entityType {
			refs = append(refs, Reference{
				File:    "data-entry.yaml",
				Section: "forms." + name,
				Kind:    "form",
			})
		}
	}

	// Lists
	for name, list := range cfg.Lists {
		if list.EntityType == entityType {
			refs = append(refs, Reference{
				File:    "data-entry.yaml",
				Section: "lists." + name,
				Kind:    "list",
			})
		}
	}

	// Views
	for name, view := range cfg.Views {
		if view.Entry.Type == entityType {
			refs = append(refs, Reference{
				File:    "data-entry.yaml",
				Section: "views." + name,
				Kind:    "view",
			})
		}
	}

	// Kanbans
	for name, kanban := range cfg.Kanbans {
		if kanban.EntityType == entityType {
			refs = append(refs, Reference{
				File:    "data-entry.yaml",
				Section: "kanbans." + name,
				Kind:    "kanban",
			})
		}
	}

	return refs
}

// findEntityTypeInViews finds references to an entity type in views.yaml.
func findEntityTypeInViews(entityType string, viewsFile *views.File) []Reference {
	var refs []Reference

	for name, view := range viewsFile.Views {
		if view.Entry.Type == entityType {
			refs = append(refs, Reference{
				File:    "views.yaml",
				Section: "views." + name,
				Kind:    "view",
			})
		}
	}

	return refs
}

// findRelationTypeReferences finds all references to a relation type.
func findRelationTypeReferences(
	relationType string,
	meta *metamodel.Metamodel,
	dataEntry *dataentryconfig.Config,
	viewsFile *views.File,
) []Reference {
	var refs []Reference

	// Check metamodel automations for create_relation actions
	for _, a := range meta.Automations {
		for _, action := range a.Do {
			if action.CreateRelation != nil && action.CreateRelation.Relation == relationType {
				refs = append(refs, Reference{
					File:    "metamodel.yaml",
					Section: "automations." + a.Name + ".do.create_relation",
					Kind:    "automation",
				})
			}
		}
		// Check relation_created/relation_removed triggers
		if a.On.RelationCreated == relationType || a.On.RelationRemoved == relationType {
			refs = append(refs, Reference{
				File:    "metamodel.yaml",
				Section: "automations." + a.Name + ".on",
				Kind:    "automation",
			})
		}
	}

	// Check data-entry.yaml
	if dataEntry != nil {
		refs = append(refs, findRelationTypeInDataEntry(relationType, dataEntry)...)
	}

	// Check views.yaml
	if viewsFile != nil {
		refs = append(refs, findRelationTypeInViews(relationType, viewsFile)...)
	}

	return refs
}

// findRelationTypeInDataEntry finds references to a relation type in data-entry.yaml.
func findRelationTypeInDataEntry(relationType string, cfg *dataentryconfig.Config) []Reference {
	var refs []Reference
	refs = append(refs, findRelationInForms(relationType, cfg.Forms)...)
	refs = append(refs, findRelationInLists(relationType, cfg.Lists)...)
	refs = append(refs, findRelationInDataEntryViews(relationType, cfg.Views)...)
	return refs
}

// findRelationInForms finds relation references in form configurations.
func findRelationInForms(relationType string, forms map[string]dataentryconfig.Form) []Reference {
	var refs []Reference
	for formName, form := range forms {
		for _, rel := range form.Relations {
			if rel.Relation == relationType {
				refs = append(refs, Reference{
					File:    "data-entry.yaml",
					Section: "forms." + formName + ".relations",
					Kind:    "form",
				})
			}
		}
		if form.SidePanel != nil {
			for _, t := range form.SidePanel.Traverse {
				if t.Follow == relationType || t.FollowIncoming == relationType {
					refs = append(refs, Reference{
						File:    "data-entry.yaml",
						Section: "forms." + formName + ".side_panel.traverse",
						Kind:    "form",
					})
				}
			}
		}
	}
	return refs
}

// findRelationInLists finds relation references in list configurations.
func findRelationInLists(relationType string, lists map[string]dataentryconfig.List) []Reference {
	var refs []Reference
	for listName, list := range lists {
		for _, col := range list.Columns {
			if col.Relation == relationType {
				refs = append(refs, Reference{
					File:    "data-entry.yaml",
					Section: "lists." + listName + ".columns",
					Kind:    "list",
				})
			}
		}
		for _, fc := range list.FilterControls {
			if fc.Relation == relationType {
				refs = append(refs, Reference{
					File:    "data-entry.yaml",
					Section: "lists." + listName + ".filter_controls",
					Kind:    "list",
				})
			}
		}
	}
	return refs
}

// findRelationInDataEntryViews finds relation references in data-entry view configurations.
func findRelationInDataEntryViews(relationType string, cfgViews map[string]dataentryconfig.ViewConfig) []Reference {
	var refs []Reference
	for viewName, view := range cfgViews {
		for _, t := range view.Traverse {
			if t.Follow == relationType || t.FollowIncoming == relationType {
				refs = append(refs, Reference{
					File:    "data-entry.yaml",
					Section: "views." + viewName + ".traverse",
					Kind:    "view",
				})
			}
		}
	}
	return refs
}

// findRelationTypeInViews finds references to a relation type in views.yaml.
func findRelationTypeInViews(relationType string, viewsFile *views.File) []Reference {
	var refs []Reference

	for viewName, view := range viewsFile.Views {
		// Traverse rules
		for _, t := range view.Traverse {
			if t.Follow == relationType || t.FollowIncoming == relationType {
				refs = append(refs, Reference{
					File:    "views.yaml",
					Section: "views." + viewName + ".traverse",
					Kind:    "view",
				})
			}
		}

		// Derived embeds
		for derivedName, derived := range view.Derived {
			for _, embed := range derived.Embed {
				if embed.Relation == relationType {
					refs = append(refs, Reference{
						File:    "views.yaml",
						Section: "views." + viewName + ".derived." + derivedName + ".embed",
						Kind:    "view",
					})
				}
			}
		}

		// Relation exports
		for _, export := range view.RelationExports {
			for _, t := range export.Types {
				if t == relationType {
					refs = append(refs, Reference{
						File:    "views.yaml",
						Section: "views." + viewName + ".relation_exports",
						Kind:    "view",
					})
				}
			}
		}
	}

	return refs
}

// findCustomTypeReferences finds all references to a custom type (enum).
func findCustomTypeReferences(typeName string, meta *metamodel.Metamodel) []Reference {
	var refs []Reference

	// Check all entity properties
	for entityName, entityDef := range meta.Entities {
		for propName, propDef := range entityDef.Properties {
			if propDef.Type == typeName {
				refs = append(refs, Reference{
					File:    "metamodel.yaml",
					Section: "entities." + entityName + ".properties." + propName,
					Kind:    "property",
				})
			}
		}
	}

	return refs
}

// sortTypeUsages sorts a slice of TypeUsage by name.
func sortTypeUsages(usages []TypeUsage) {
	sort.Slice(usages, func(i, j int) bool {
		return usages[i].Name < usages[j].Name
	})
}
