package migration

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(&ViewsByEntityTypeMigration{})
}

// ViewsByEntityTypeMigration normalises data-entry.yaml so each entity
// type has at most one view, addressed directly by entity type. It:
//
//   - re-keys the `views:` map from a free-form view ID to the view's
//     `entry.type`,
//   - strips `detail_view:` from list configs,
//   - strips `detail_view:` from each `entity_views:` entry, and drops an
//     entry, or the whole block, that is left empty. The other keys of an
//     entry (`duplicate`, `mentioned_by_panel`, `mentioned_by_hide_id`) are
//     live config and stay.
//
// Errors when more than one existing view targets the same entity type
// — the project owner must consolidate manually. The validator enforces
// the same one-per-type invariant at config load.
type ViewsByEntityTypeMigration struct{}

func (m *ViewsByEntityTypeMigration) Name() string {
	return "views-by-entity-type"
}

func (m *ViewsByEntityTypeMigration) Description() string {
	return "Re-key views: by entity type, remove detail_view"
}

func (m *ViewsByEntityTypeMigration) FileTypes() []FileType {
	return []FileType{FileTypeDataEntry}
}

func (m *ViewsByEntityTypeMigration) Detect(doc *yaml.Node) bool {
	root := GetDocumentRoot(doc)
	if root == nil {
		return false
	}

	if m.detectViewsToRekey(root) {
		return true
	}
	if m.detectDetailViewInLists(root) {
		return true
	}
	return m.detectDetailViewInEntityViews(root)
}

// detectDetailViewInEntityViews reports whether an `entity_views:` entry
// still names a `detail_view:`.
func (m *ViewsByEntityTypeMigration) detectDetailViewInEntityViews(root *yaml.Node) bool {
	ev := GetMapValue(root, "entity_views")
	if ev == nil || ev.Kind != yaml.MappingNode {
		return false
	}
	for i := 1; i < len(ev.Content); i += 2 {
		if GetMapValue(ev.Content[i], "detail_view") != nil {
			return true
		}
	}
	return false
}

// detectViewsToRekey returns true when at least one view's map key differs
// from its entry.type — that's the marker that the rekey hasn't run yet.
func (m *ViewsByEntityTypeMigration) detectViewsToRekey(root *yaml.Node) bool {
	views := GetMapValue(root, "views")
	if views == nil || views.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i < len(views.Content); i += 2 {
		keyNode := views.Content[i]
		viewDef := views.Content[i+1]
		if viewDef.Kind != yaml.MappingNode {
			continue
		}
		entityType := getEntryType(viewDef)
		if entityType != "" && keyNode.Value != entityType {
			return true
		}
	}
	return false
}

func (m *ViewsByEntityTypeMigration) detectDetailViewInLists(root *yaml.Node) bool {
	lists := GetMapValue(root, "lists")
	if lists == nil || lists.Kind != yaml.MappingNode {
		return false
	}
	for i := 1; i < len(lists.Content); i += 2 {
		listDef := lists.Content[i]
		if listDef.Kind != yaml.MappingNode {
			continue
		}
		if GetMapValue(listDef, "detail_view") != nil {
			return true
		}
	}
	return false
}

func (m *ViewsByEntityTypeMigration) Apply(doc *yaml.Node) error {
	root := GetDocumentRoot(doc)
	if root == nil {
		return nil
	}

	if err := m.rekeyViews(root); err != nil {
		return err
	}
	m.stripDetailViewFromLists(root)
	m.stripDetailViewFromEntityViews(root)
	return nil
}

// stripDetailViewFromEntityViews removes `detail_view:` from every
// `entity_views:` entry. An entry left empty is removed, and so is the block
// when no entry is left.
func (m *ViewsByEntityTypeMigration) stripDetailViewFromEntityViews(root *yaml.Node) {
	ev := GetMapValue(root, "entity_views")
	if ev == nil || ev.Kind != yaml.MappingNode {
		return
	}
	kept := ev.Content[:0]
	for i := 0; i+1 < len(ev.Content); i += 2 {
		def := ev.Content[i+1]
		if def.Kind == yaml.MappingNode {
			DeleteMapKey(def, "detail_view")
			if len(def.Content) == 0 {
				continue
			}
		}
		kept = append(kept, ev.Content[i], def)
	}
	ev.Content = kept
	if len(ev.Content) == 0 {
		DeleteMapKey(root, "entity_views")
	}
}

// rekeyViews rewrites each entry of the `views:` map so its key is the
// entity type the view targets. Refuses to proceed when multiple views
// target the same type — collapsing them silently would lose project
// intent.
func (m *ViewsByEntityTypeMigration) rekeyViews(root *yaml.Node) error {
	views := GetMapValue(root, "views")
	if views == nil || views.Kind != yaml.MappingNode {
		return nil
	}

	// First pass — group existing keys by their entry.type, find conflicts.
	byType := map[string][]string{}
	for i := 0; i < len(views.Content); i += 2 {
		keyNode := views.Content[i]
		viewDef := views.Content[i+1]
		if viewDef.Kind != yaml.MappingNode {
			continue
		}
		entityType := getEntryType(viewDef)
		if entityType == "" {
			// Without entry.type the view cannot be re-keyed; skip and let
			// the validator catch it. Apply continues so other views can be
			// migrated.
			continue
		}
		byType[entityType] = append(byType[entityType], keyNode.Value)
	}

	var conflicts []string
	for entityType, viewIDs := range byType {
		if len(viewIDs) <= 1 {
			continue
		}
		sort.Strings(viewIDs)
		conflicts = append(conflicts,
			fmt.Sprintf("entity type %q is targeted by views: %s",
				entityType, strings.Join(viewIDs, ", ")))
	}
	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		return fmt.Errorf(
			"%s: cannot collapse views to one-per-entity-type — please consolidate manually:\n  - %s",
			m.Name(), strings.Join(conflicts, "\n  - "))
	}

	// Second pass — rewrite keys. Mutating a yaml.Node mapping's keys
	// in-place by editing the key node's Value preserves comments,
	// surrounding formatting, and document order.
	for i := 0; i < len(views.Content); i += 2 {
		keyNode := views.Content[i]
		viewDef := views.Content[i+1]
		if viewDef.Kind != yaml.MappingNode {
			continue
		}
		entityType := getEntryType(viewDef)
		if entityType == "" {
			continue
		}
		if keyNode.Value != entityType {
			keyNode.Value = entityType
		}
	}
	return nil
}

func (m *ViewsByEntityTypeMigration) stripDetailViewFromLists(root *yaml.Node) {
	lists := GetMapValue(root, "lists")
	if lists == nil || lists.Kind != yaml.MappingNode {
		return
	}
	for i := 1; i < len(lists.Content); i += 2 {
		listDef := lists.Content[i]
		if listDef.Kind != yaml.MappingNode {
			continue
		}
		DeleteMapKey(listDef, "detail_view")
	}
}

// getEntryType extracts entry.type from a view definition. Returns empty
// when entry or entry.type is missing.
func getEntryType(viewDef *yaml.Node) string {
	entry := GetMapValue(viewDef, "entry")
	if entry == nil || entry.Kind != yaml.MappingNode {
		return ""
	}
	t := GetMapValue(entry, "type")
	if t == nil || t.Kind != yaml.ScalarNode {
		return ""
	}
	return t.Value
}
