package metamodel

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// RenameEntityType performs an AST-level rename of an entity type in a metamodel YAML file
// using the given filesystem. It preserves comments, formatting, and key ordering.
//
// Updates:
//   - The entity key under `entities:`
//   - All references in `relations:` `from:` and `to:` arrays
//   - All references in `validations:` `entity_type:` fields
func RenameEntityType(path, oldType, newType string, fs storage.FS) error {
	data, err := fs.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read metamodel: %w", err)
	}

	var doc yaml.Node
	if unmarshalErr := yaml.Unmarshal(data, &doc); unmarshalErr != nil {
		return fmt.Errorf("failed to parse metamodel: %w", unmarshalErr)
	}

	root := getDocRoot(&doc)
	if root == nil {
		return errors.New("empty metamodel document")
	}

	// Rename entity key under entities:
	entities := getMapValue(root, "entities")
	if entities == nil {
		return errors.New("no entities section found in metamodel")
	}

	if !renameMapKey(entities, oldType, newType) {
		return fmt.Errorf("entity type %q not found in metamodel", oldType)
	}

	// Check new type doesn't conflict with existing types (other than the one we just renamed)
	// We already renamed it, so check if there are duplicates
	count := 0
	for i := 0; i < len(entities.Content)-1; i += 2 {
		if entities.Content[i].Value == newType {
			count++
		}
	}
	if count > 1 {
		// Undo the rename
		renameMapKey(entities, newType, oldType)
		return fmt.Errorf("entity type %q already exists in metamodel", newType)
	}

	// Update relation from/to references
	relations := getMapValue(root, "relations")
	if relations != nil && relations.Kind == yaml.MappingNode {
		for i := 1; i < len(relations.Content); i += 2 {
			relDef := relations.Content[i]
			if relDef.Kind != yaml.MappingNode {
				continue
			}
			replaceInSequence(getMapValue(relDef, "from"), oldType, newType)
			replaceInSequence(getMapValue(relDef, "to"), oldType, newType)
		}
	}

	// Update validation entity_type references
	validations := getMapValue(root, "validations")
	if validations != nil && validations.Kind == yaml.SequenceNode {
		for _, item := range validations.Content {
			if item.Kind != yaml.MappingNode {
				continue
			}
			replaceMapScalar(item, "entity_type", oldType, newType)
		}
	}

	// Write back
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("failed to marshal metamodel: %w", err)
	}

	info, err := fs.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat metamodel: %w", err)
	}

	if err := fs.WriteFile(path, out, info.Mode()); err != nil {
		return fmt.Errorf("failed to write metamodel: %w", err)
	}

	return nil
}

// getDocRoot returns the root mapping from a document node.
func getDocRoot(doc *yaml.Node) *yaml.Node {
	if doc == nil {
		return nil
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	return doc
}

// getMapValue finds a value in a mapping node by key name.
func getMapValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// renameMapKey renames a key in a mapping node. Returns true if found.
func renameMapKey(node *yaml.Node, oldKey, newKey string) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == oldKey {
			node.Content[i].Value = newKey
			return true
		}
	}
	return false
}

// replaceInSequence replaces scalar values in a sequence node.
func replaceInSequence(seq *yaml.Node, oldVal, newVal string) {
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return
	}
	for _, item := range seq.Content {
		if item.Kind == yaml.ScalarNode && item.Value == oldVal {
			item.Value = newVal
		}
	}
}

// replaceMapScalar replaces a scalar value for a given key in a mapping node.
func replaceMapScalar(node *yaml.Node, key, oldVal, newVal string) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			valNode := node.Content[i+1]
			if valNode.Kind == yaml.ScalarNode && valNode.Value == oldVal {
				valNode.Value = newVal
			}
			return
		}
	}
}
