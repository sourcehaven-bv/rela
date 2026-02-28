package migration

import (
	"gopkg.in/yaml.v3"
)

func init() {
	Register(&LinkToStringMigration{})
}

// LinkToStringMigration converts link: true to link: detail in data-entry.yaml.
// This enables the link field to support different link targets:
//   - "detail" - link to entity detail view (default, replaces true)
//   - "document/<name>" - link to document preview
type LinkToStringMigration struct{}

func (m *LinkToStringMigration) Name() string {
	return "link-to-string"
}

func (m *LinkToStringMigration) Description() string {
	return "Convert link: true to link: detail in data-entry.yaml"
}

func (m *LinkToStringMigration) FileTypes() []FileType {
	return []FileType{FileTypeDataEntry}
}

func (m *LinkToStringMigration) Detect(doc *yaml.Node) bool {
	root := GetDocumentRoot(doc)
	if root == nil {
		return false
	}

	// Check lists section
	lists := GetMapValue(root, "lists")
	if lists != nil && lists.Kind == yaml.MappingNode {
		if m.detectInLists(lists) {
			return true
		}
	}

	// Check views section
	views := GetMapValue(root, "views")
	if views != nil && views.Kind == yaml.MappingNode {
		if m.detectInViews(views) {
			return true
		}
	}

	// Check dashboard section
	dashboard := GetMapValue(root, "dashboard")
	if dashboard != nil && dashboard.Kind == yaml.MappingNode {
		if m.detectInDashboard(dashboard) {
			return true
		}
	}

	return false
}

func (m *LinkToStringMigration) detectInLists(lists *yaml.Node) bool {
	for i := 1; i < len(lists.Content); i += 2 {
		listDef := lists.Content[i]
		if listDef.Kind != yaml.MappingNode {
			continue
		}

		columns := GetMapValue(listDef, "columns")
		if columns != nil && columns.Kind == yaml.SequenceNode {
			for _, col := range columns.Content {
				if col.Kind == yaml.MappingNode && m.hasBoolLink(col) {
					return true
				}
			}
		}
	}
	return false
}

func (m *LinkToStringMigration) detectInViews(views *yaml.Node) bool {
	for i := 1; i < len(views.Content); i += 2 {
		viewDef := views.Content[i]
		if viewDef.Kind != yaml.MappingNode {
			continue
		}

		sections := GetMapValue(viewDef, "sections")
		if sections == nil || sections.Kind != yaml.SequenceNode {
			continue
		}

		for _, sec := range sections.Content {
			if sec.Kind != yaml.MappingNode {
				continue
			}

			// Check section-level link
			if m.hasBoolLink(sec) {
				return true
			}

			// Check columns within section
			if m.detectInColumns(sec) {
				return true
			}
		}
	}
	return false
}

func (m *LinkToStringMigration) detectInDashboard(dashboard *yaml.Node) bool {
	cards := GetMapValue(dashboard, "cards")
	if cards == nil || cards.Kind != yaml.SequenceNode {
		return false
	}

	for _, card := range cards.Content {
		if card.Kind != yaml.MappingNode {
			continue
		}
		if m.detectInColumns(card) {
			return true
		}
	}
	return false
}

func (m *LinkToStringMigration) detectInColumns(node *yaml.Node) bool {
	columns := GetMapValue(node, "columns")
	if columns == nil || columns.Kind != yaml.SequenceNode {
		return false
	}
	for _, col := range columns.Content {
		if col.Kind == yaml.MappingNode && m.hasBoolLink(col) {
			return true
		}
	}
	return false
}

// hasBoolLink checks if a node has a link key with boolean true value.
func (m *LinkToStringMigration) hasBoolLink(node *yaml.Node) bool {
	linkVal := GetMapValue(node, "link")
	if linkVal == nil || linkVal.Kind != yaml.ScalarNode {
		return false
	}
	// YAML boolean true can be "true", "True", "TRUE", "yes", "Yes", "YES", "on", "On", "ON"
	return linkVal.Tag == "!!bool" && linkVal.Value == "true"
}

func (m *LinkToStringMigration) Apply(doc *yaml.Node) error {
	root := GetDocumentRoot(doc)
	if root == nil {
		return nil
	}

	lists := GetMapValue(root, "lists")
	if lists != nil && lists.Kind == yaml.MappingNode {
		m.convertInLists(lists)
	}

	views := GetMapValue(root, "views")
	if views != nil && views.Kind == yaml.MappingNode {
		m.convertInViews(views)
	}

	dashboard := GetMapValue(root, "dashboard")
	if dashboard != nil && dashboard.Kind == yaml.MappingNode {
		m.convertInDashboard(dashboard)
	}

	return nil
}

func (m *LinkToStringMigration) convertInLists(lists *yaml.Node) {
	for i := 1; i < len(lists.Content); i += 2 {
		listDef := lists.Content[i]
		if listDef.Kind != yaml.MappingNode {
			continue
		}

		columns := GetMapValue(listDef, "columns")
		if columns == nil || columns.Kind != yaml.SequenceNode {
			continue
		}
		for _, col := range columns.Content {
			if col.Kind == yaml.MappingNode {
				m.convertBoolLink(col)
			}
		}
	}
}

func (m *LinkToStringMigration) convertInViews(views *yaml.Node) {
	for i := 1; i < len(views.Content); i += 2 {
		viewDef := views.Content[i]
		if viewDef.Kind != yaml.MappingNode {
			continue
		}

		sections := GetMapValue(viewDef, "sections")
		if sections == nil || sections.Kind != yaml.SequenceNode {
			continue
		}

		for _, sec := range sections.Content {
			if sec.Kind != yaml.MappingNode {
				continue
			}

			// Convert section-level link
			m.convertBoolLink(sec)

			// Convert columns within section
			m.convertInColumns(sec)
		}
	}
}

func (m *LinkToStringMigration) convertInDashboard(dashboard *yaml.Node) {
	cards := GetMapValue(dashboard, "cards")
	if cards == nil || cards.Kind != yaml.SequenceNode {
		return
	}

	for _, card := range cards.Content {
		if card.Kind != yaml.MappingNode {
			continue
		}
		m.convertInColumns(card)
	}
}

func (m *LinkToStringMigration) convertInColumns(node *yaml.Node) {
	columns := GetMapValue(node, "columns")
	if columns == nil || columns.Kind != yaml.SequenceNode {
		return
	}
	for _, col := range columns.Content {
		if col.Kind == yaml.MappingNode {
			m.convertBoolLink(col)
		}
	}
}

// convertBoolLink converts link: true to link: detail.
func (m *LinkToStringMigration) convertBoolLink(node *yaml.Node) {
	linkVal := GetMapValue(node, "link")
	if linkVal == nil || linkVal.Kind != yaml.ScalarNode {
		return
	}
	if linkVal.Tag == "!!bool" && linkVal.Value == "true" {
		linkVal.Tag = "!!str"
		linkVal.Value = "detail"
	}
}
