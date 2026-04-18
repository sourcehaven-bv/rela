package migration

import (
	"errors"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(&CardinalityRenameMigration{})
}

// CardinalityRenameMigration renames cardinality constraint keys in relation definitions:
// - "source_min" -> "min_outgoing"
// - "source_max" -> "max_outgoing"
// - "target_min" -> "min_incoming"
// - "target_max" -> "max_incoming"
type CardinalityRenameMigration struct{}

func (m *CardinalityRenameMigration) Name() string {
	return "cardinality-rename"
}

func (m *CardinalityRenameMigration) Description() string {
	return `Rename cardinality keys: source_min → min_outgoing, source_max → max_outgoing, ` +
		`target_min → min_incoming, target_max → max_incoming`
}

func (m *CardinalityRenameMigration) FileTypes() []FileType {
	return []FileType{FileTypeMetamodel}
}

var cardinalityRenames = [][2]string{
	{"source_min", "min_outgoing"},
	{"source_max", "max_outgoing"},
	{"target_min", "min_incoming"},
	{"target_max", "max_incoming"},
}

func (m *CardinalityRenameMigration) Detect(doc *yaml.Node) bool {
	root := GetDocumentRoot(doc)
	if root == nil {
		return false
	}

	relations := GetMapValue(root, "relations")
	if relations == nil || relations.Kind != yaml.MappingNode {
		return false
	}

	for i := 1; i < len(relations.Content); i += 2 {
		relDef := relations.Content[i]
		if relDef.Kind != yaml.MappingNode {
			continue
		}

		for _, rename := range cardinalityRenames {
			if GetMapValue(relDef, rename[0]) != nil {
				return true
			}
		}
	}

	return false
}

func (m *CardinalityRenameMigration) Apply(doc *yaml.Node) error {
	root := GetDocumentRoot(doc)
	if root == nil {
		return errors.New("empty document")
	}

	relations := GetMapValue(root, "relations")
	if relations == nil || relations.Kind != yaml.MappingNode {
		return nil
	}

	for i := 1; i < len(relations.Content); i += 2 {
		relDef := relations.Content[i]
		if relDef.Kind != yaml.MappingNode {
			continue
		}

		for _, rename := range cardinalityRenames {
			RenameMapKey(relDef, rename[0], rename[1])
		}
	}

	return nil
}
