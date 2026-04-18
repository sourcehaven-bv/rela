package migration

import (
	"errors"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(&InverseSimplifyMigration{})
}

// InverseSimplifyMigration renames "name" to "id" in inverse definitions and
// simplifies to string form when the label matches the auto-derived label.
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

		// Check if label matches auto-derived label
		autoLabel := camelCaseToSpaced(name)
		if labelNode == nil || labelNode.Value == autoLabel {
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

// camelCaseToSpaced converts camelCase/PascalCase to space-separated lowercase.
// Examples: "addressedBy" → "addressed by", "implementedBy" → "implemented by"
func camelCaseToSpaced(s string) string {
	if s == "" {
		return ""
	}

	const asciiCaseOffset = 'a' - 'A'   // 32, but as a named constant
	result := make([]byte, 0, len(s)+4) // Extra space for inserted spaces

	for i := range len(s) {
		c := s[i]
		isUpper := c >= 'A' && c <= 'Z'

		switch {
		case i > 0 && isUpper:
			// Insert space before uppercase letters (except at start) and convert to lowercase
			result = append(result, ' ', c+asciiCaseOffset)
		case isUpper:
			// First character - just convert to lowercase
			result = append(result, c+asciiCaseOffset)
		default:
			result = append(result, c)
		}
	}
	return string(result)
}
