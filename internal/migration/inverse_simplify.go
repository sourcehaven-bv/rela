package migration

import (
	"errors"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(&InverseSimplifyMigration{})
}

// InverseSimplifyMigration renames "name" to "id" in inverse definitions and
// simplifies to string form when the label adds nothing over the id itself.
type InverseSimplifyMigration struct{}

func (m *InverseSimplifyMigration) Name() string {
	return "inverse-simplify"
}

func (m *InverseSimplifyMigration) Description() string {
	return `Simplify inverse definitions: rename "name" to "id", use string form when possible`
}

func (m *InverseSimplifyMigration) FileTypes() []FileType {
	return []FileType{FileTypeMetamodel}
}

func (m *InverseSimplifyMigration) Detect(doc *yaml.Node) bool {
	root := GetDocumentRoot(doc)
	if root == nil {
		return false
	}

	relations := GetMapValue(root, "relations")
	if relations == nil || relations.Kind != yaml.MappingNode {
		return false
	}

	// Check each relation definition for deprecated "name" field in inverse
	for i := 1; i < len(relations.Content); i += 2 {
		relDef := relations.Content[i]
		if relDef.Kind != yaml.MappingNode {
			continue
		}

		inverseNode := GetMapValue(relDef, "inverse")
		if inverseNode != nil && inverseNode.Kind == yaml.MappingNode {
			// Check if it uses the deprecated "name" field
			if GetMapKey(inverseNode, "name") != nil {
				return true
			}
		}
	}

	return false
}

func (m *InverseSimplifyMigration) Apply(doc *yaml.Node) error {
	root := GetDocumentRoot(doc)
	if root == nil {
		return errors.New("empty document")
	}

	relations := GetMapValue(root, "relations")
	if relations == nil || relations.Kind != yaml.MappingNode {
		return nil
	}

	// Iterate through relation definitions
	for i := 1; i < len(relations.Content); i += 2 {
		relDef := relations.Content[i]
		if relDef.Kind != yaml.MappingNode {
			continue
		}

		inverseNode := GetMapValue(relDef, "inverse")
		if inverseNode == nil || inverseNode.Kind != yaml.MappingNode {
			continue
		}

		nameNode := GetMapValue(inverseNode, "name")
		if nameNode == nil {
			continue
		}

		labelNode := GetMapValue(inverseNode, "label")
		name := nameNode.Value

		// Collapse to the simple string form only when the label adds nothing
		// — i.e. it is absent, or identical to the id itself. A label that
		// merely looks derivable (e.g. "addressed by" for `addressedBy`) is
		// kept: labels are authored, never derived (DEC-6C1NAA), so dropping
		// it would permanently downgrade the display text to the raw id.
		if labelNode == nil || labelNode.Value == name {
			// Convert to simple string form: replace mapping with scalar
			inverseNode.Kind = yaml.ScalarNode
			inverseNode.Tag = ""
			inverseNode.Value = name
			inverseNode.Content = nil
		} else {
			// Custom label - rename "name" to "id"
			RenameMapKey(inverseNode, "name", "id")
		}
	}

	return nil
}
