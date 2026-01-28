package migration

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(&IDTypeRenameMigration{})
}

// IDTypeRenameMigration renames deprecated id_type values:
// - "sequential" -> "auto"
// - "string" -> "manual"
type IDTypeRenameMigration struct{}

func (m *IDTypeRenameMigration) Name() string {
	return "id-type-rename"
}

func (m *IDTypeRenameMigration) Description() string {
	return `Rename id_type values: "sequential" → "auto", "string" → "manual"`
}

func (m *IDTypeRenameMigration) FileTypes() []FileType {
	return []FileType{FileTypeMetamodel}
}

func (m *IDTypeRenameMigration) Detect(doc *yaml.Node) bool {
	root := GetDocumentRoot(doc)
	if root == nil {
		return false
	}

	// Look for entities section
	entities := GetMapValue(root, "entities")
	if entities == nil || entities.Kind != yaml.MappingNode {
		return false
	}

	// Check each entity definition for deprecated id_type values
	for i := 1; i < len(entities.Content); i += 2 {
		entityDef := entities.Content[i]
		if entityDef.Kind != yaml.MappingNode {
			continue
		}

		idTypeValue := GetMapValue(entityDef, "id_type")
		if idTypeValue != nil && idTypeValue.Kind == yaml.ScalarNode {
			if idTypeValue.Value == "sequential" || idTypeValue.Value == "string" {
				return true
			}
		}
	}

	return false
}

func (m *IDTypeRenameMigration) Apply(doc *yaml.Node) error {
	root := GetDocumentRoot(doc)
	if root == nil {
		return fmt.Errorf("empty document")
	}

	entities := GetMapValue(root, "entities")
	if entities == nil || entities.Kind != yaml.MappingNode {
		return nil // No entities section, nothing to migrate
	}

	count := 0

	// Iterate through entity definitions
	for i := 1; i < len(entities.Content); i += 2 {
		entityDef := entities.Content[i]
		if entityDef.Kind != yaml.MappingNode {
			continue
		}

		idTypeValue := GetMapValue(entityDef, "id_type")
		if idTypeValue != nil && idTypeValue.Kind == yaml.ScalarNode {
			switch idTypeValue.Value {
			case "sequential":
				idTypeValue.Value = "auto"
				count++
			case "string":
				idTypeValue.Value = "manual"
				count++
			}
		}
	}

	// Note: count == 0 is valid if Apply is called on a file that was already migrated
	// The runner checks Detect() before Apply(), so this shouldn't normally happen,
	// but we don't treat it as an error.

	return nil
}

// CountDeprecatedIDTypes returns the number of deprecated id_type values found.
// Useful for detailed reporting.
func (m *IDTypeRenameMigration) CountDeprecatedIDTypes(doc *yaml.Node) (sequential, stringType int) {
	root := GetDocumentRoot(doc)
	if root == nil {
		return 0, 0
	}

	entities := GetMapValue(root, "entities")
	if entities == nil || entities.Kind != yaml.MappingNode {
		return 0, 0
	}

	for i := 1; i < len(entities.Content); i += 2 {
		entityDef := entities.Content[i]
		if entityDef.Kind != yaml.MappingNode {
			continue
		}

		idTypeValue := GetMapValue(entityDef, "id_type")
		if idTypeValue != nil && idTypeValue.Kind == yaml.ScalarNode {
			switch idTypeValue.Value {
			case "sequential":
				sequential++
			case "string":
				stringType++
			}
		}
	}

	return sequential, stringType
}
