package dataentryconfig

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
)

// Valid top-level keys in data-entry.yaml
var validTopLevelKeys = map[string]bool{
	"version":    true,
	"app":        true,
	"git":        true,
	"styles":     true,
	"forms":      true,
	"lists":      true,
	"views":      true,
	"kanbans":    true,
	"documents":  true,
	"dashboard":  true,
	"commands":   true,
	"navigation": true,
	"palette":    true,
}

// Known typos with suggestions
var knownTypos = map[string]string{
	"form":        "forms",
	"list":        "lists",
	"view":        "views",
	"kanban":      "kanbans",
	"command":     "commands",
	"style":       "styles",
	"nav":         "navigation",
	"navaigation": "navigation",
}

// Valid filter operators
var validFilterOperators = map[string]bool{
	"=":  true,
	"!=": true,
	"<":  true,
	"<=": true,
	">":  true,
	">=": true,
	"=~": true,
}

// Valid sort directions
var validSortDirections = map[string]bool{
	"":     true, // default (asc)
	"asc":  true,
	"desc": true,
}

// Valid display modes for view sections
var validSectionDisplayModes = map[string]bool{
	"properties": true,
	"content":    true,
	"table":      true,
	"list":       true,
	"cards":      true,
	"breakdown":  true,
}

// Valid display modes for dashboard cards
var validDashboardDisplayModes = map[string]bool{
	"count":     true,
	"table":     true,
	"breakdown": true,
}

// Valid command contexts
var validCommandContexts = map[string]bool{
	"entity": true,
	"list":   true,
	"view":   true,
	"global": true,
}

// Valid relation directions
var validRelationDirections = map[Direction]bool{
	"":                true, // default (outgoing)
	DirectionOutgoing: true,
	DirectionIncoming: true,
}

// Valid relation widgets
var validRelationWidgets = map[string]bool{
	"":                true, // default (auto-detect from cardinality)
	WidgetSelect:      true,
	WidgetMultiSelect: true,
	WidgetCards:       true,
}

// ValidateConfig performs comprehensive validation of a data-entry config.
// It returns a ConfigValidationError containing all validation issues,
// or nil if the config is valid.
func ValidateConfig(data []byte, cfg *Config, meta *metamodel.Metamodel) error {
	var errs []string

	// Phase 1: Structural validation (unknown keys)
	errs = append(errs, checkUnknownKeys(data)...)

	// Phase 2: Semantic validation (cross-references, types, etc.)
	errs = append(errs, validateNavigation(cfg)...)
	errs = append(errs, validateForms(cfg, meta)...)
	errs = append(errs, validateLists(cfg, meta)...)
	errs = append(errs, validateViews(cfg, meta)...)
	errs = append(errs, validateKanbans(cfg, meta)...)
	errs = append(errs, validateDashboard(cfg, meta)...)
	errs = append(errs, validateCommands(cfg, meta)...)
	errs = append(errs, validateDocuments(cfg)...)
	errs = append(errs, validateStyles(cfg, meta)...)
	errs = append(errs, validateCrossReferences(cfg)...)

	if len(errs) > 0 {
		natsort.Strings(errs)
		return &ConfigValidationError{Errors: errs}
	}
	return nil
}

// checkUnknownKeys detects unknown top-level keys in the config YAML.
func checkUnknownKeys(data []byte) []string {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil // struct unmarshal already caught this
	}

	var errs []string
	for key := range raw {
		if validTopLevelKeys[key] {
			continue
		}
		if suggestion, ok := knownTypos[key]; ok {
			errs = append(errs, fmt.Sprintf("unknown key %q (did you mean %q?)", key, suggestion))
		} else {
			keys := sortedMapKeys(validTopLevelKeys)
			errs = append(errs, fmt.Sprintf("unknown key %q (valid keys: %s)", key, strings.Join(keys, ", ")))
		}
	}
	return errs
}

// validateNavigation validates navigation entries.
func validateNavigation(cfg *Config) []string {
	var errs []string

	for _, nav := range cfg.Navigation {
		if nav.IsGroup() {
			for _, child := range nav.Items {
				if child.IsGroup() {
					errs = append(errs, fmt.Sprintf(
						"navigation: group %q contains nested group %q (nested groups are not supported)",
						nav.Group, child.Group))
				}
			}
		}
	}

	// Validate list references in navigation
	for _, nav := range cfg.Navigation {
		errs = append(errs, validateNavEntry(nav, cfg)...)
	}

	return errs
}

func validateNavEntry(nav NavigationEntry, cfg *Config) []string {
	var errs []string

	if nav.List != "" {
		if _, ok := cfg.Lists[nav.List]; !ok {
			errs = append(errs, fmt.Sprintf(
				"navigation: references unknown list %q", nav.List))
		}
	}

	if nav.Kanban != "" {
		if _, ok := cfg.Kanbans[nav.Kanban]; !ok {
			errs = append(errs, fmt.Sprintf(
				"navigation: references unknown kanban %q", nav.Kanban))
		}
	}

	if nav.IsGroup() {
		for _, child := range nav.Items {
			errs = append(errs, validateNavEntry(child, cfg)...)
		}
	}

	return errs
}

// validateForms validates form definitions.
func validateForms(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	for formID, form := range cfg.Forms {
		entDef, ok := meta.GetEntityDef(form.EntityType)
		if !ok {
			errs = append(errs, fmt.Sprintf("form %q: unknown entity type %q", formID, form.EntityType))
			continue
		}

		// Validate fields
		for i, f := range form.Fields {
			if _, ok := entDef.Properties[f.Property]; !ok {
				errs = append(errs, fmt.Sprintf(
					"form %q: field[%d] property %q not in metamodel for entity %q",
					formID, i, f.Property, form.EntityType))
			}

			// Validate transitions (if specified, must be valid enum values)
			if len(f.Transitions) > 0 {
				propDef, hasProp := entDef.Properties[f.Property]
				if hasProp {
					errs = append(errs, validateTransitions(formID, i, f, propDef, meta)...)
				}
			}
		}

		// Validate relations
		for i, r := range form.Relations {
			if _, ok := meta.GetRelationDef(r.Relation); !ok {
				errs = append(errs, fmt.Sprintf(
					"form %q: relation[%d] references unknown relation %q",
					formID, i, r.Relation))
			}

			// Validate direction
			if !validRelationDirections[r.Direction] {
				errs = append(errs, fmt.Sprintf(
					"form %q: relation[%d] has invalid direction %q (valid: outgoing, incoming)",
					formID, i, r.Direction))
			}

			// Validate widget
			if !validRelationWidgets[r.Widget] {
				errs = append(errs, fmt.Sprintf(
					"form %q: relation[%d] has invalid widget %q (valid: select, multi-select, cards)",
					formID, i, r.Widget))
			}

			// Validate create_form reference
			if r.CreateForm != "" {
				if _, ok := cfg.Forms[r.CreateForm]; !ok {
					errs = append(errs, fmt.Sprintf(
						"form %q: relation[%d] references unknown create_form %q",
						formID, i, r.CreateForm))
				}
			}
		}
	}

	return errs
}

// validateTransitions checks that transition values are valid for the property's type.
func validateTransitions(formID string, fieldIdx int, field FormField, propDef metamodel.PropertyDef, meta *metamodel.Metamodel) []string {
	var errs []string

	// Get valid values for this property type
	validValues := GetValidEnumValues(propDef, meta)
	if len(validValues) == 0 {
		return errs // not an enum type, skip validation
	}

	validSet := make(map[string]bool)
	for _, v := range validValues {
		validSet[v] = true
	}

	// Check all transition keys and values
	for fromState, toStates := range field.Transitions {
		if !validSet[fromState] {
			errs = append(errs, fmt.Sprintf(
				"form %q: field[%d] transitions has invalid from-state %q",
				formID, fieldIdx, fromState))
		}
		for _, toState := range toStates {
			if !validSet[toState] {
				errs = append(errs, fmt.Sprintf(
					"form %q: field[%d] transitions has invalid to-state %q",
					formID, fieldIdx, toState))
			}
		}
	}

	return errs
}

// GetValidEnumValues returns the valid values for an enum or custom type property.
func GetValidEnumValues(propDef metamodel.PropertyDef, meta *metamodel.Metamodel) []string {
	if propDef.Type == metamodel.PropertyTypeEnum {
		return propDef.Values
	}
	// Check if it's a custom type
	if customType, ok := meta.Types[propDef.Type]; ok {
		return customType.Values
	}
	return nil
}

// validateLists validates list definitions.
func validateLists(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	for listID, list := range cfg.Lists {
		entDef, ok := meta.GetEntityDef(list.EntityType)
		if !ok {
			errs = append(errs, fmt.Sprintf("list %q: unknown entity type %q", listID, list.EntityType))
			continue
		}

		// Validate columns
		for i, c := range list.Columns {
			if c.Relation != "" {
				if _, ok := meta.GetRelationDef(c.Relation); !ok {
					errs = append(errs, fmt.Sprintf(
						"list %q: column[%d] references unknown relation %q",
						listID, i, c.Relation))
				}
			} else if c.Property != "" {
				if _, ok := entDef.Properties[c.Property]; !ok {
					errs = append(errs, fmt.Sprintf(
						"list %q: column[%d] property %q not in metamodel for entity %q",
						listID, i, c.Property, list.EntityType))
				}
			}
		}

		// Validate sort
		for i, s := range list.Sort {
			if !validSortDirections[s.Direction] {
				errs = append(errs, fmt.Sprintf(
					"list %q: sort[%d] has invalid direction %q (valid: asc, desc)",
					listID, i, s.Direction))
			}
			if s.Property != "" && s.Property != "id" && s.Property != "modified" {
				if _, ok := entDef.Properties[s.Property]; !ok {
					errs = append(errs, fmt.Sprintf(
						"list %q: sort[%d] references unknown property %q",
						listID, i, s.Property))
				}
			}
		}

		// Validate filters
		for i, f := range list.Filters {
			if !validFilterOperators[f.Operator] {
				errs = append(errs, fmt.Sprintf(
					"list %q: filter[%d] has invalid operator %q (valid: %s)",
					listID, i, f.Operator, joinMapKeys(validFilterOperators)))
			}
			if f.Property != "" && f.Property != "id" && f.Property != "type" {
				if _, ok := entDef.Properties[f.Property]; !ok {
					errs = append(errs, fmt.Sprintf(
						"list %q: filter[%d] references unknown property %q",
						listID, i, f.Property))
				}
			}
		}

		// Validate filter_controls
		for i, fc := range list.FilterControls {
			if fc.Property == "" && fc.Relation == "" {
				errs = append(errs, fmt.Sprintf(
					"list %q: filter_controls[%d] must specify either property or relation",
					listID, i))
			}
			if fc.Property != "" {
				if _, ok := entDef.Properties[fc.Property]; !ok {
					errs = append(errs, fmt.Sprintf(
						"list %q: filter_controls[%d] references unknown property %q",
						listID, i, fc.Property))
				}
			}
			if fc.Relation != "" {
				if _, ok := meta.GetRelationDef(fc.Relation); !ok {
					errs = append(errs, fmt.Sprintf(
						"list %q: filter_controls[%d] references unknown relation %q",
						listID, i, fc.Relation))
				}
			}
		}
	}

	return errs
}

// validateViews validates view definitions with their traversal rules and sections.
func validateViews(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	for viewID, view := range cfg.Views {
		// Validate entry type
		entDef, ok := meta.GetEntityDef(view.Entry.Type)
		if !ok {
			errs = append(errs, fmt.Sprintf(
				"view %q: entry type %q not in metamodel",
				viewID, view.Entry.Type))
			continue
		}

		// Build collection map: tracks what collections are available at each point
		// Start with "entry" which refers to the entry entity type
		collections := map[string]string{
			"entry": view.Entry.Type, // collection name -> entity type
		}

		// Validate traverse rules and track collections
		for i, t := range view.Traverse {
			// Validate from
			if t.From != "*" {
				if _, ok := collections[t.From]; !ok {
					validCollections := sortedMapKeys(collections)
					validCollections = append(validCollections, "*")
					errs = append(errs, fmt.Sprintf(
						"view %q: traverse[%d] references unknown collection %q in from (valid: %s)",
						viewID, i, t.From, strings.Join(validCollections, ", ")))
				}
			}

			// Validate follow/follow_incoming relation
			var relType string
			if t.Follow != "" {
				relType = t.Follow
				if _, ok := meta.GetRelationDef(t.Follow); !ok {
					if suggestion := suggestRelation(t.Follow, meta); suggestion != "" {
						errs = append(errs, fmt.Sprintf(
							"view %q: traverse[%d] references unknown relation %q in follow (did you mean %q?)",
							viewID, i, t.Follow, suggestion))
					} else {
						errs = append(errs, fmt.Sprintf(
							"view %q: traverse[%d] references unknown relation %q in follow",
							viewID, i, t.Follow))
					}
				}
			}
			if t.FollowIncoming != "" {
				relType = t.FollowIncoming
				if _, ok := meta.GetRelationDef(t.FollowIncoming); !ok {
					if suggestion := suggestRelation(t.FollowIncoming, meta); suggestion != "" {
						errs = append(errs, fmt.Sprintf(
							"view %q: traverse[%d] references unknown relation %q in follow_incoming (did you mean %q?)",
							viewID, i, t.FollowIncoming, suggestion))
					} else {
						errs = append(errs, fmt.Sprintf(
							"view %q: traverse[%d] references unknown relation %q in follow_incoming",
							viewID, i, t.FollowIncoming))
					}
				}
			}
			if t.Follow == "" && t.FollowIncoming == "" {
				errs = append(errs, fmt.Sprintf(
					"view %q: traverse[%d] must specify either follow or follow_incoming",
					viewID, i))
			}
			if t.Follow != "" && t.FollowIncoming != "" {
				errs = append(errs, fmt.Sprintf(
					"view %q: traverse[%d] cannot specify both follow and follow_incoming",
					viewID, i))
			}

			// Validate collect_as is specified
			if t.CollectAs == "" {
				errs = append(errs, fmt.Sprintf(
					"view %q: traverse[%d] must specify collect_as",
					viewID, i))
			} else {
				// Determine target entity type for this collection
				targetType := determineTargetType(t, relType, meta)
				collections[t.CollectAs] = targetType
			}
		}

		// Validate sections
		for i, s := range view.Sections {
			// Validate source
			sourceType := ""
			if s.Source != "" {
				if entityType, ok := collections[s.Source]; ok {
					sourceType = entityType
				} else {
					validSources := sortedMapKeys(collections)
					errs = append(errs, fmt.Sprintf(
						"view %q: section[%d] references unknown collection %q in source (valid: %s)",
						viewID, i, s.Source, strings.Join(validSources, ", ")))
				}
			}

			// Validate display mode
			if !validSectionDisplayModes[s.Display] {
				errs = append(errs, fmt.Sprintf(
					"view %q: section[%d] has invalid display mode %q (valid: %s)",
					viewID, i, s.Display, joinMapKeys(validSectionDisplayModes)))
			}

			// Validate fields (if source type is known)
			if sourceType != "" {
				if sourceDef, ok := meta.GetEntityDef(sourceType); ok {
					for j, f := range s.Fields {
						if f.Property != "" && f.Property != "title" && f.Property != "id" {
							if _, ok := sourceDef.Properties[f.Property]; !ok {
								errs = append(errs, fmt.Sprintf(
									"view %q: section[%d] field[%d] property %q not in entity %q",
									viewID, i, j, f.Property, sourceType))
							}
						}
					}

					// Validate columns
					for j, c := range s.Columns {
						if c.Property != "" && c.Property != "title" && c.Property != "id" {
							if _, ok := sourceDef.Properties[c.Property]; !ok {
								errs = append(errs, fmt.Sprintf(
									"view %q: section[%d] column[%d] property %q not in entity %q",
									viewID, i, j, c.Property, sourceType))
							}
						}
						if c.Relation != "" {
							if _, ok := meta.GetRelationDef(c.Relation); !ok {
								errs = append(errs, fmt.Sprintf(
									"view %q: section[%d] column[%d] references unknown relation %q",
									viewID, i, j, c.Relation))
							}
						}
					}

					// Validate group_by
					if s.GroupBy != "" {
						if _, ok := sourceDef.Properties[s.GroupBy]; !ok {
							errs = append(errs, fmt.Sprintf(
								"view %q: section[%d] group_by references unknown property %q",
								viewID, i, s.GroupBy))
						}
					}
				}
			} else if s.Source == "entry" {
				// Source is entry, use entry entity def
				for j, f := range s.Fields {
					if f.Property != "" && f.Property != "title" && f.Property != "id" {
						if _, ok := entDef.Properties[f.Property]; !ok {
							errs = append(errs, fmt.Sprintf(
								"view %q: section[%d] field[%d] property %q not in entity %q",
								viewID, i, j, f.Property, view.Entry.Type))
						}
					}
				}
			}
		}
	}

	return errs
}

// determineTargetType determines the entity type that a traverse rule collects.
func determineTargetType(t ViewTraverse, relType string, meta *metamodel.Metamodel) string {
	if relType == "" {
		return ""
	}
	relDef, ok := meta.GetRelationDef(relType)
	if !ok {
		return ""
	}

	// For outgoing (follow), target is relation's To types
	// For incoming (follow_incoming), target is relation's From types
	if t.Follow != "" {
		if len(relDef.To) == 1 {
			return relDef.To[0]
		}

		return "" // multiple possible types, can't determine statically
	}
	if t.FollowIncoming != "" {
		if len(relDef.From) == 1 {
			return relDef.From[0]
		}

		return "" // multiple possible types
	}

	return ""
}

// suggestRelation finds a similar relation name for typo suggestions.
func suggestRelation(name string, meta *metamodel.Metamodel) string {
	nameLower := strings.ToLower(name)
	for relName := range meta.Relations {
		if strings.EqualFold(relName, name) {
			return relName
		}
		// Simple similarity: check if one contains the other
		relNameLower := strings.ToLower(relName)
		if strings.Contains(relNameLower, nameLower) || strings.Contains(nameLower, relNameLower) {
			return relName
		}
	}

	return ""
}

// validateKanbans validates kanban board definitions.
func validateKanbans(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	for kanbanID, kanban := range cfg.Kanbans {
		// Validate entity type
		entDef, ok := meta.GetEntityDef(kanban.EntityType)
		if !ok {
			errs = append(errs, fmt.Sprintf("kanban %q: unknown entity type %q", kanbanID, kanban.EntityType))
			continue
		}

		// Validate column_property exists and is enum type
		if kanban.ColumnProperty == "" {
			errs = append(errs, fmt.Sprintf("kanban %q: column_property is required", kanbanID))
		} else {
			propDef, ok := entDef.Properties[kanban.ColumnProperty]
			if !ok {
				errs = append(errs, fmt.Sprintf(
					"kanban %q: column_property %q not in entity %q",
					kanbanID, kanban.ColumnProperty, kanban.EntityType))
			} else {
				// Check if it's an enum type
				validValues := GetValidEnumValues(propDef, meta)
				if len(validValues) == 0 {
					errs = append(errs, fmt.Sprintf(
						"kanban %q: column_property %q must be an enum type",
						kanbanID, kanban.ColumnProperty))
				} else {
					// Validate column values if specified
					validSet := make(map[string]bool)
					for _, v := range validValues {
						validSet[v] = true
					}
					for i, col := range kanban.Columns {
						if !validSet[col.Value] {
							errs = append(errs, fmt.Sprintf(
								"kanban %q: columns[%d] value %q is not valid for %q (valid: %s)",
								kanbanID, i, col.Value, kanban.ColumnProperty, strings.Join(validValues, ", ")))
						}
					}
				}
			}
		}

		// Validate swimlane_property if specified
		if kanban.SwimlaneProperty != "" {
			propDef, ok := entDef.Properties[kanban.SwimlaneProperty]
			if !ok {
				errs = append(errs, fmt.Sprintf(
					"kanban %q: swimlane_property %q not in entity %q",
					kanbanID, kanban.SwimlaneProperty, kanban.EntityType))
			} else {
				// Check if it's an enum type
				validValues := GetValidEnumValues(propDef, meta)
				if len(validValues) == 0 {
					errs = append(errs, fmt.Sprintf(
						"kanban %q: swimlane_property %q must be an enum type",
						kanbanID, kanban.SwimlaneProperty))
				} else {
					// Validate swimlane values if specified
					validSet := make(map[string]bool)
					for _, v := range validValues {
						validSet[v] = true
					}
					for i, lane := range kanban.Swimlanes {
						if !validSet[lane.Value] {
							errs = append(errs, fmt.Sprintf(
								"kanban %q: swimlanes[%d] value %q is not valid for %q (valid: %s)",
								kanbanID, i, lane.Value, kanban.SwimlaneProperty, strings.Join(validValues, ", ")))
						}
					}
				}
			}
		}

		// Validate card title property
		if kanban.Card.Title != "" {
			if _, ok := entDef.Properties[kanban.Card.Title]; !ok {
				errs = append(errs, fmt.Sprintf(
					"kanban %q: card.title property %q not in entity %q",
					kanbanID, kanban.Card.Title, kanban.EntityType))
			}
		}

		// Validate card fields
		for i, f := range kanban.Card.Fields {
			if f.Property != "" && f.Property != "title" && f.Property != "id" {
				if _, ok := entDef.Properties[f.Property]; !ok {
					errs = append(errs, fmt.Sprintf(
						"kanban %q: card.fields[%d] property %q not in entity %q",
						kanbanID, i, f.Property, kanban.EntityType))
				}
			}
		}

		// Validate filter properties
		for i, f := range kanban.Filters {
			if !validFilterOperators[f.Operator] {
				errs = append(errs, fmt.Sprintf(
					"kanban %q: filters[%d] has invalid operator %q (valid: %s)",
					kanbanID, i, f.Operator, joinMapKeys(validFilterOperators)))
			}
			if f.Property != "" && f.Property != "id" && f.Property != "type" {
				if _, ok := entDef.Properties[f.Property]; !ok {
					errs = append(errs, fmt.Sprintf(
						"kanban %q: filters[%d] references unknown property %q",
						kanbanID, i, f.Property))
				}
			}
		}

		// Validate filter_controls
		for i, fc := range kanban.FilterControls {
			if fc.Property == "" && fc.Relation == "" {
				errs = append(errs, fmt.Sprintf(
					"kanban %q: filter_controls[%d] must specify either property or relation",
					kanbanID, i))
			}
			if fc.Property != "" {
				if _, ok := entDef.Properties[fc.Property]; !ok {
					errs = append(errs, fmt.Sprintf(
						"kanban %q: filter_controls[%d] references unknown property %q",
						kanbanID, i, fc.Property))
				}
			}
			if fc.Relation != "" {
				if _, ok := meta.GetRelationDef(fc.Relation); !ok {
					errs = append(errs, fmt.Sprintf(
						"kanban %q: filter_controls[%d] references unknown relation %q",
						kanbanID, i, fc.Relation))
				}
			}
		}

		// Validate form references
		if kanban.EditForm != "" {
			if _, ok := cfg.Forms[kanban.EditForm]; !ok {
				errs = append(errs, fmt.Sprintf(
					"kanban %q: references unknown form %q in edit_form",
					kanbanID, kanban.EditForm))
			}
		}
		if kanban.CreateForm != "" {
			if _, ok := cfg.Forms[kanban.CreateForm]; !ok {
				errs = append(errs, fmt.Sprintf(
					"kanban %q: references unknown form %q in create_form",
					kanbanID, kanban.CreateForm))
			}
		}
	}

	return errs
}

// validateDashboard validates dashboard configuration.
func validateDashboard(cfg *Config, _ *metamodel.Metamodel) []string {
	var errs []string

	if cfg.Dashboard == nil {
		return errs
	}

	for i, card := range cfg.Dashboard.Cards {
		// Validate display mode
		if !validDashboardDisplayModes[card.Display] {
			errs = append(errs, fmt.Sprintf(
				"dashboard: card[%d] %q has invalid display mode %q (valid: %s)",
				i, card.Title, card.Display, joinMapKeys(validDashboardDisplayModes)))
		}

		// Validate sort directions
		for j, s := range card.Sort {
			if !validSortDirections[s.Direction] {
				errs = append(errs, fmt.Sprintf(
					"dashboard: card[%d] %q sort[%d] has invalid direction %q (valid: asc, desc)",
					i, card.Title, j, s.Direction))
			}
		}

		// For breakdown display, group_by should be specified
		if card.Display == "breakdown" && card.GroupBy == "" {
			errs = append(errs, fmt.Sprintf(
				"dashboard: card[%d] %q uses breakdown display but has no group_by",
				i, card.Title))
		}

		// For table display, columns should be specified
		if card.Display == "table" && len(card.Columns) == 0 {
			errs = append(errs, fmt.Sprintf(
				"dashboard: card[%d] %q uses table display but has no columns",
				i, card.Title))
		}
	}

	return errs
}

// validateCommands validates command definitions.
func validateCommands(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	for cmdID, cmd := range cfg.Commands {
		if cmd.Label == "" {
			errs = append(errs, fmt.Sprintf("command %q: label is required", cmdID))
		}
		if cmd.Script == "" {
			errs = append(errs, fmt.Sprintf("command %q: script is required", cmdID))
		}
		if !validCommandContexts[cmd.Context] {
			errs = append(errs, fmt.Sprintf(
				"command %q: invalid context %q (valid: %s)",
				cmdID, cmd.Context, joinMapKeys(validCommandContexts)))
		}
		if cmd.AvailableOn != nil {
			for _, v := range cmd.AvailableOn.Views {
				if _, ok := cfg.Views[v]; !ok {
					errs = append(errs, fmt.Sprintf(
						"command %q: available_on references unknown view %q", cmdID, v))
				}
			}
			for _, l := range cmd.AvailableOn.Lists {
				if _, ok := cfg.Lists[l]; !ok {
					errs = append(errs, fmt.Sprintf(
						"command %q: available_on references unknown list %q", cmdID, l))
				}
			}
			for _, et := range cmd.AvailableOn.EntityTypes {
				if _, ok := meta.GetEntityDef(et); !ok {
					errs = append(errs, fmt.Sprintf(
						"command %q: available_on references unknown entity type %q", cmdID, et))
				}
			}
		}
	}

	return errs
}

// validateDocuments validates document configurations.
func validateDocuments(cfg *Config) []string {
	var errs []string

	for docID, doc := range cfg.Documents {
		if doc.Command == "" {
			errs = append(errs, fmt.Sprintf("document %q: command is required", docID))
		}
		if doc.EntityType == "" {
			errs = append(errs, fmt.Sprintf("document %q: entity_type is required", docID))
		}
	}

	return errs
}

// validateStyles validates style definitions reference valid metamodel types.
func validateStyles(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	for typeName := range cfg.Styles {
		// Check if it's a custom type in metamodel
		if _, ok := meta.Types[typeName]; ok {
			continue
		}
		// Check if it's used as a property type in any entity
		found := false
		for _, entDef := range meta.Entities {
			for _, propDef := range entDef.Properties {
				if propDef.Type == typeName {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			errs = append(errs, fmt.Sprintf(
				"styles: type %q is not defined in metamodel", typeName))
		}
	}

	return errs
}

// validateCrossReferences validates that all cross-references between config sections are valid.
func validateCrossReferences(cfg *Config) []string {
	var errs []string

	// Validate list references to forms and views
	for listID, list := range cfg.Lists {
		if list.CreateForm != "" {
			if _, ok := cfg.Forms[list.CreateForm]; !ok {
				if suggestion := suggestForm(list.CreateForm, cfg); suggestion != "" {
					errs = append(errs, fmt.Sprintf(
						"list %q: references unknown form %q in create_form (did you mean %q?)",
						listID, list.CreateForm, suggestion))
				} else {
					errs = append(errs, fmt.Sprintf(
						"list %q: references unknown form %q in create_form",
						listID, list.CreateForm))
				}
			}
		}
		if list.EditForm != "" {
			if _, ok := cfg.Forms[list.EditForm]; !ok {
				if suggestion := suggestForm(list.EditForm, cfg); suggestion != "" {
					errs = append(errs, fmt.Sprintf(
						"list %q: references unknown form %q in edit_form (did you mean %q?)",
						listID, list.EditForm, suggestion))
				} else {
					errs = append(errs, fmt.Sprintf(
						"list %q: references unknown form %q in edit_form",
						listID, list.EditForm))
				}
			}
		}
		if list.DetailView != "" {
			if _, ok := cfg.Views[list.DetailView]; !ok {
				if suggestion := suggestView(list.DetailView, cfg); suggestion != "" {
					errs = append(errs, fmt.Sprintf(
						"list %q: references unknown view %q in detail_view (did you mean %q?)",
						listID, list.DetailView, suggestion))
				} else {
					errs = append(errs, fmt.Sprintf(
						"list %q: references unknown view %q in detail_view",
						listID, list.DetailView))
				}
			}
		}
	}

	return errs
}

// suggestForm finds a similar form name for typo suggestions.
func suggestForm(name string, cfg *Config) string {
	nameLower := strings.ToLower(name)
	for formName := range cfg.Forms {
		if strings.EqualFold(formName, name) {
			return formName
		}
		formNameLower := strings.ToLower(formName)
		if strings.Contains(formNameLower, nameLower) || strings.Contains(nameLower, formNameLower) {
			return formName
		}
	}

	return ""
}

// suggestView finds a similar view name for typo suggestions.
func suggestView(name string, cfg *Config) string {
	nameLower := strings.ToLower(name)
	for viewName := range cfg.Views {
		if strings.EqualFold(viewName, name) {
			return viewName
		}
		viewNameLower := strings.ToLower(viewName)
		if strings.Contains(viewNameLower, nameLower) || strings.Contains(nameLower, viewNameLower) {
			return viewName
		}
	}

	return ""
}

// Helper functions

func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	natsort.Strings(keys)
	return keys
}

func joinMapKeys(m map[string]bool) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		if k != "" { // skip empty string (default)
			keys = append(keys, k)
		}
	}
	natsort.Strings(keys)
	return strings.Join(keys, ", ")
}
