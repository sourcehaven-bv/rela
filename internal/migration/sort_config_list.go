package migration

import (
	"errors"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(&SortConfigListMigration{})
}

// SortConfigListMigration converts the sort config from a single mapping object
// to a list (sequence) format in data-entry.yaml files.
//
// Before:
//
//	sort:
//	  property: priority
//	  direction: desc
//
// After:
//
//	sort:
//	  - property: priority
//	    direction: desc
type SortConfigListMigration struct{}

func (m *SortConfigListMigration) Name() string {
	return "sort-config-to-list"
}

func (m *SortConfigListMigration) Description() string {
	return `Convert sort config from single object to list format`
}

func (m *SortConfigListMigration) FileTypes() []FileType {
	return []FileType{FileTypeDataEntry}
}

func (m *SortConfigListMigration) Detect(doc *yaml.Node) bool {
	entries := FindMapEntriesByKey(doc, "sort")
	for _, entry := range entries {
		if entry[1].Kind == yaml.MappingNode {
			return true
		}
	}
	return false
}

func (m *SortConfigListMigration) Apply(doc *yaml.Node) error {
	root := GetDocumentRoot(doc)
	if root == nil {
		return errors.New("empty document")
	}

	WalkMappings(doc, func(node *yaml.Node) bool {
		for i := 0; i < len(node.Content)-1; i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]
			if keyNode.Value == "sort" && valNode.Kind == yaml.MappingNode {
				// Wrap the mapping in a sequence
				seqNode := &yaml.Node{
					Kind:    yaml.SequenceNode,
					Tag:     "!!seq",
					Content: []*yaml.Node{valNode},
				}
				node.Content[i+1] = seqNode
			}
		}
		return true
	})
	return nil
}
