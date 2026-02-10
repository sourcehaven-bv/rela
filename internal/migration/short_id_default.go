package migration

import (
	"gopkg.in/yaml.v3"
)

func init() {
	Register(&ShortIDDefaultMigration{})
}

// ShortIDDefaultMigration adds explicit id_type: sequential to entities that don't have
// an id_type defined, and renames deprecated values:
// - id_type: auto → id_type: sequential
// - id_type: string → id_type: manual
// This preserves the previous default behavior (sequential IDs) when the new default
// is short random IDs.
type ShortIDDefaultMigration struct{}

func (m *ShortIDDefaultMigration) Name() string {
	return "short-id-default"
}

func (m *ShortIDDefaultMigration) Description() string {
	return "Add explicit id_type: sequential to entities without id_type, rename deprecated values"
}

func (m *ShortIDDefaultMigration) FileTypes() []FileType {
	return []FileType{FileTypeMetamodel}
}

func (m *ShortIDDefaultMigration) Detect(doc *yaml.Node) bool {
	root := GetDocumentRoot(doc)
	if root == nil {
		return false
	}

	entities := GetMapValue(root, "entities")
	if entities == nil || entities.Kind != yaml.MappingNode {
		return false
	}

	// Check each entity definition for missing id_type or deprecated values
	for i := 1; i < len(entities.Content); i += 2 {
		entityDef := entities.Content[i]
		if entityDef.Kind != yaml.MappingNode {
			continue
		}

		idTypeValue := GetMapValue(entityDef, "id_type")
		if idTypeValue == nil {
			// Entity has no id_type, needs migration
			return true
		}
		if idTypeValue.Value == "auto" || idTypeValue.Value == "string" {
			// Entity has deprecated id_type value, needs migration
			return true
		}
	}

	return false
}

func (m *ShortIDDefaultMigration) Apply(doc *yaml.Node) error {
	root := GetDocumentRoot(doc)
	if root == nil {
		return nil
	}

	entities := GetMapValue(root, "entities")
	if entities == nil || entities.Kind != yaml.MappingNode {
		return nil
	}

	// Iterate through entity definitions
	for i := 1; i < len(entities.Content); i += 2 {
		entityDef := entities.Content[i]
		if entityDef.Kind != yaml.MappingNode {
			continue
		}

		idTypeValue := GetMapValue(entityDef, "id_type")
		if idTypeValue == nil {
			// Add id_type: sequential to preserve previous default behavior
			// Insert after id_prefix/id_prefixes if present, otherwise at the start
			insertPos := findIDTypeInsertPosition(entityDef)
			insertKeyValue(entityDef, insertPos, "id_type", "sequential")
		} else {
			// Rename deprecated values
			switch idTypeValue.Value {
			case "auto":
				idTypeValue.Value = "sequential"
			case "string":
				idTypeValue.Value = "manual"
			}
		}
	}

	return nil
}

// findIDTypeInsertPosition finds the best position to insert id_type.
// Prefers inserting after id_prefix or id_prefixes, otherwise after label.
func findIDTypeInsertPosition(entityDef *yaml.Node) int {
	// Preferred insertion points (in order of preference)
	insertAfter := []string{"id_prefixes", "id_prefix", "aliases", "label_plural", "label"}

	for _, key := range insertAfter {
		for i := 0; i < len(entityDef.Content)-1; i += 2 {
			if entityDef.Content[i].Value == key {
				// Insert after this key-value pair
				return i + 2
			}
		}
	}

	// Default: insert at position 0 (before first key)
	return 0
}

// insertKeyValue inserts a key-value pair at the specified position in a mapping.
func insertKeyValue(mapping *yaml.Node, pos int, key, value string) {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	valueNode := &yaml.Node{Kind: yaml.ScalarNode, Value: value}

	// Ensure pos is valid
	if pos < 0 {
		pos = 0
	}
	if pos > len(mapping.Content) {
		pos = len(mapping.Content)
	}

	// Insert at position
	newContent := make([]*yaml.Node, 0, len(mapping.Content)+2)
	newContent = append(newContent, mapping.Content[:pos]...)
	newContent = append(newContent, keyNode, valueNode)
	newContent = append(newContent, mapping.Content[pos:]...)
	mapping.Content = newContent
}
