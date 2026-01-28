package migration

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(&IDPrefixRenameMigration{})
}

// IDPrefixRenameMigration renames deprecated id_patterns to id_prefix/id_prefixes.
// Single-element arrays become id_prefix (scalar), multi-element arrays become id_prefixes.
type IDPrefixRenameMigration struct{}

func (m *IDPrefixRenameMigration) Name() string {
	return "id-prefix-rename"
}

func (m *IDPrefixRenameMigration) Description() string {
	return `Rename id_patterns to id_prefix (single) or id_prefixes (multiple)`
}

func (m *IDPrefixRenameMigration) FileTypes() []FileType {
	return []FileType{FileTypeMetamodel}
}

func (m *IDPrefixRenameMigration) Detect(doc *yaml.Node) bool {
	root := GetDocumentRoot(doc)
	if root == nil {
		return false
	}

	// Look for entities section
	entities := GetMapValue(root, "entities")
	if entities == nil || entities.Kind != yaml.MappingNode {
		return false
	}

	// Check each entity definition for id_patterns
	for i := 1; i < len(entities.Content); i += 2 {
		entityDef := entities.Content[i]
		if entityDef.Kind != yaml.MappingNode {
			continue
		}

		if GetMapKey(entityDef, "id_patterns") != nil {
			return true
		}
	}

	return false
}

func (m *IDPrefixRenameMigration) Apply(doc *yaml.Node) error {
	root := GetDocumentRoot(doc)
	if root == nil {
		return fmt.Errorf("empty document")
	}

	entities := GetMapValue(root, "entities")
	if entities == nil || entities.Kind != yaml.MappingNode {
		return nil // No entities section, nothing to migrate
	}

	// Iterate through entity definitions
	for i := 1; i < len(entities.Content); i += 2 {
		entityDef := entities.Content[i]
		if entityDef.Kind != yaml.MappingNode {
			continue
		}

		patternsValue := GetMapValue(entityDef, "id_patterns")
		if patternsValue == nil {
			continue
		}

		// Determine new key name based on number of elements
		if patternsValue.Kind == yaml.SequenceNode && len(patternsValue.Content) == 1 {
			// Single element: rename to id_prefix and convert to scalar
			RenameMapKey(entityDef, "id_patterns", "id_prefix")
			// Replace sequence node with scalar node
			scalarValue := patternsValue.Content[0].Value
			patternsValue.Kind = yaml.ScalarNode
			patternsValue.Tag = "!!str"
			patternsValue.Value = scalarValue
			patternsValue.Content = nil
			patternsValue.Style = yaml.DoubleQuotedStyle
		} else {
			// Multiple elements or non-sequence: rename to id_prefixes
			RenameMapKey(entityDef, "id_patterns", "id_prefixes")
		}
	}

	return nil
}
