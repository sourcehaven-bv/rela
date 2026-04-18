package migration

import (
	"errors"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(&ValidationWhenThenMigration{})
}

// ValidationWhenThenMigration renames deprecated validation rule fields:
// - "match" -> "when"
// - "require" -> "then"
type ValidationWhenThenMigration struct{}

func (m *ValidationWhenThenMigration) Name() string {
	return "validation-when-then"
}

func (m *ValidationWhenThenMigration) Description() string {
	return `Rename validation fields: "match" → "when", "require" → "then"`
}

func (m *ValidationWhenThenMigration) FileTypes() []FileType {
	return []FileType{FileTypeMetamodel}
}

func (m *ValidationWhenThenMigration) Detect(doc *yaml.Node) bool {
	root := GetDocumentRoot(doc)
	if root == nil {
		return false
	}

	validations := GetMapValue(root, "validations")
	if validations == nil || validations.Kind != yaml.SequenceNode {
		return false
	}

	for _, rule := range validations.Content {
		if rule.Kind != yaml.MappingNode {
			continue
		}
		if GetMapKey(rule, "match") != nil || GetMapKey(rule, "require") != nil {
			return true
		}
	}

	return false
}

func (m *ValidationWhenThenMigration) Apply(doc *yaml.Node) error {
	root := GetDocumentRoot(doc)
	if root == nil {
		return errors.New("empty document")
	}

	validations := GetMapValue(root, "validations")
	if validations == nil || validations.Kind != yaml.SequenceNode {
		return nil // No validations section, nothing to migrate
	}

	for _, rule := range validations.Content {
		if rule.Kind != yaml.MappingNode {
			continue
		}
		RenameMapKey(rule, "match", "when")
		RenameMapKey(rule, "require", "then")
	}

	return nil
}
