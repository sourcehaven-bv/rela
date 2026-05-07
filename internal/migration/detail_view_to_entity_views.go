// Package migration: detail_view_to_entity_views
//
// Lifts list-level `detail_view` into a new top-level `entity_views` section.
// The migration is idempotent: Detect() reports true only when Apply() will
// actually move at least one value. Conflicting groups (multiple lists for
// the same entity_type with different detail_view values) are skipped — the
// conflict surfaces via `rela validate` (out of scope here) rather than
// blocking startup forever.
//
// Subset/filter lists that previously had no detail_view inherit the
// canonical detail_view for their type after migration. This is intentional:
// "where do you view an X" should have one answer, and a subset view of
// type X should land users on the same detail page as the canonical list.

package migration

import (
	"errors"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(&DetailViewToEntityViewsMigration{})
}

// DetailViewToEntityViewsMigration moves lists.<id>.detail_view to
// entity_views.<entity_type>.detail_view.
type DetailViewToEntityViewsMigration struct{}

func (m *DetailViewToEntityViewsMigration) Name() string {
	return "detail-view-to-entity-views"
}

func (m *DetailViewToEntityViewsMigration) Description() string {
	return "Move list-level detail_view to top-level entity_views.<type>.detail_view"
}

func (m *DetailViewToEntityViewsMigration) FileTypes() []FileType {
	return []FileType{FileTypeDataEntry}
}

// Detect returns true only when Apply() would change something. This keeps
// the migration idempotent: after running, conflicting groups remain
// untouched and Detect() returns false on the next pass so the server can
// start.
func (m *DetailViewToEntityViewsMigration) Detect(doc *yaml.Node) bool {
	root := GetDocumentRoot(doc)
	if root == nil {
		return false
	}
	migratable := m.collectMigratableGroups(root)
	return len(migratable) > 0
}

func (m *DetailViewToEntityViewsMigration) Apply(doc *yaml.Node) error {
	root := GetDocumentRoot(doc)
	if root == nil {
		return errors.New("empty document")
	}

	migratable := m.collectMigratableGroups(root)
	if len(migratable) == 0 {
		return nil
	}

	// Ensure top-level `entity_views:` mapping exists.
	entityViews := GetMapValue(root, "entity_views")
	if entityViews == nil {
		entityViews = &yaml.Node{Kind: yaml.MappingNode}
		// Insert after `lists:` if present, else after `forms:`, else append.
		anchor := "lists"
		if GetMapValue(root, anchor) == nil {
			anchor = "forms"
		}
		InsertMapKeyAfter(root, anchor, "entity_views", entityViews)
	} else if entityViews.Kind != yaml.MappingNode {
		// Hand-written but malformed — refuse to merge.
		return errors.New("entity_views: exists but is not a mapping")
	}

	// Sorted iteration so output is deterministic.
	types := make([]string, 0, len(migratable))
	for t := range migratable {
		types = append(types, t)
	}
	sort.Strings(types)

	for _, entityType := range types {
		view := migratable[entityType]
		// Merge into existing entity_views entry if present.
		existing := GetMapValue(entityViews, entityType)
		switch {
		case existing == nil:
			entry := &yaml.Node{Kind: yaml.MappingNode}
			SetMapValue(entry, "detail_view", view)
			entityViews.Content = append(entityViews.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: entityType},
				entry,
			)
		case existing.Kind == yaml.MappingNode:
			SetMapValue(existing, "detail_view", view)
		default:
			// Existing entry is malformed (scalar, sequence, etc.). Refuse
			// to silently strip the list-level detail_view we'd otherwise
			// delete below, since we can't safely write the new value.
			return fmt.Errorf("entity_views.%s: exists but is not a mapping", entityType)
		}
	}

	// Strip detail_view from every list whose entity_type was migrated.
	lists := GetMapValue(root, "lists")
	if lists != nil && lists.Kind == yaml.MappingNode {
		for i := 1; i < len(lists.Content); i += 2 {
			listDef := lists.Content[i]
			if listDef.Kind != yaml.MappingNode {
				continue
			}
			entityType := getScalarValue(listDef, "entity_type")
			if _, ok := migratable[entityType]; !ok {
				continue
			}
			DeleteMapKey(listDef, "detail_view")
		}
	}

	return nil
}

// collectMigratableGroups walks lists, groups by entity_type, and returns
// the migrate-able groups (single distinct detail_view value, taking any
// pre-existing entity_views entry as one of the votes). Conflicting
// groups (multiple lists for the same type with different detail_views)
// are silently skipped — surfacing them belongs in `rela validate`.
func (m *DetailViewToEntityViewsMigration) collectMigratableGroups(root *yaml.Node) map[string]string {
	migratable := map[string]string{}

	lists := GetMapValue(root, "lists")
	if lists == nil || lists.Kind != yaml.MappingNode {
		return migratable
	}

	// Pre-existing entity_views: their detail_view counts as the canonical
	// vote for that entity_type — list-level values must agree with it or
	// the group is conflicting.
	existing := map[string]string{}
	if entityViews := GetMapValue(root, "entity_views"); entityViews != nil && entityViews.Kind == yaml.MappingNode {
		for i := 0; i < len(entityViews.Content)-1; i += 2 {
			typeName := entityViews.Content[i].Value
			entry := entityViews.Content[i+1]
			if entry.Kind != yaml.MappingNode {
				continue
			}
			if v := getScalarValue(entry, "detail_view"); v != "" {
				existing[typeName] = v
			}
		}
	}

	// Per-type vote: collect distinct detail_view values from lists.
	votes := map[string]map[string]bool{}
	hasListLevel := map[string]bool{}
	for i := 1; i < len(lists.Content); i += 2 {
		listDef := lists.Content[i]
		if listDef.Kind != yaml.MappingNode {
			continue
		}
		entityType := getScalarValue(listDef, "entity_type")
		detailView := getScalarValue(listDef, "detail_view")
		if entityType == "" || detailView == "" {
			continue
		}
		hasListLevel[entityType] = true
		if votes[entityType] == nil {
			votes[entityType] = map[string]bool{}
		}
		votes[entityType][detailView] = true
	}

	// Only entity_types with list-level votes are candidates — we don't
	// rewrite anything if there's no list-level value to move.
	for entityType, distinct := range votes {
		if !hasListLevel[entityType] {
			continue
		}
		// Include the existing entity_views value as a vote.
		if v, ok := existing[entityType]; ok {
			distinct[v] = true
		}
		// Single distinct value → migrate-able. Otherwise a conflict; skip.
		if len(distinct) == 1 {
			for v := range distinct {
				migratable[entityType] = v
			}
		}
	}
	return migratable
}
