package migration

import (
	"gopkg.in/yaml.v3"
)

// NodeKind helpers for yaml.Node navigation.

// GetMapValue finds a value in a mapping node by key name.
// Returns nil if the key doesn't exist or the node isn't a mapping.
func GetMapValue(node *yaml.Node, key string) *yaml.Node {
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

// GetMapKey finds a key node in a mapping node by key name.
// Returns nil if the key doesn't exist or the node isn't a mapping.
func GetMapKey(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i]
		}
	}
	return nil
}

// SetMapValue sets a value in a mapping node by key name.
// If the key exists, the value is updated. If not, the key-value pair is added.
func SetMapValue(node *yaml.Node, key, value string) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1].Value = value
			return
		}
	}
	// Key doesn't exist, add it
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	valueNode := &yaml.Node{Kind: yaml.ScalarNode, Value: value}
	node.Content = append(node.Content, keyNode, valueNode)
}

// RenameMapKey renames a key in a mapping node.
// Returns true if the key was found and renamed.
func RenameMapKey(node *yaml.Node, oldKey, newKey string) bool {
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

// WalkMappings calls fn for each mapping node in the tree (depth-first).
// If fn returns false, traversal stops.
func WalkMappings(node *yaml.Node, fn func(node *yaml.Node) bool) bool {
	if node == nil {
		return true
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if !WalkMappings(child, fn) {
				return false
			}
		}
	case yaml.MappingNode:
		if !fn(node) {
			return false
		}
		for _, child := range node.Content {
			if !WalkMappings(child, fn) {
				return false
			}
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			if !WalkMappings(child, fn) {
				return false
			}
		}
	case yaml.ScalarNode, yaml.AliasNode:
		// Scalar and alias nodes have no children to walk
	}
	return true
}

// FindScalarValues finds all scalar nodes with the given value anywhere in the tree.
// Useful for detecting deprecated values.
func FindScalarValues(node *yaml.Node, value string) []*yaml.Node {
	var results []*yaml.Node
	walkAll(node, func(n *yaml.Node) {
		if n.Kind == yaml.ScalarNode && n.Value == value {
			results = append(results, n)
		}
	})
	return results
}

// FindMapEntriesByKey finds all mapping entries where the key matches.
// Returns pairs of (key node, value node).
func FindMapEntriesByKey(node *yaml.Node, key string) [][2]*yaml.Node {
	var results [][2]*yaml.Node
	WalkMappings(node, func(mapping *yaml.Node) bool {
		for i := 0; i < len(mapping.Content)-1; i += 2 {
			if mapping.Content[i].Value == key {
				results = append(results, [2]*yaml.Node{mapping.Content[i], mapping.Content[i+1]})
			}
		}
		return true
	})
	return results
}

// ReplaceScalarValue replaces all occurrences of oldValue with newValue.
// Only affects scalar nodes. Returns count of replacements made.
func ReplaceScalarValue(node *yaml.Node, oldValue, newValue string) int {
	count := 0
	walkAll(node, func(n *yaml.Node) {
		if n.Kind == yaml.ScalarNode && n.Value == oldValue {
			n.Value = newValue
			count++
		}
	})
	return count
}

// ReplaceMapValueByKey replaces values of entries with matching key.
// Only replaces if the current value matches oldValue.
// Returns count of replacements made.
func ReplaceMapValueByKey(node *yaml.Node, key, oldValue, newValue string) int {
	count := 0
	entries := FindMapEntriesByKey(node, key)
	for _, entry := range entries {
		valueNode := entry[1]
		if valueNode.Kind == yaml.ScalarNode && valueNode.Value == oldValue {
			valueNode.Value = newValue
			count++
		}
	}
	return count
}

// walkAll visits every node in the tree.
func walkAll(node *yaml.Node, fn func(*yaml.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for _, child := range node.Content {
		walkAll(child, fn)
	}
}

// GetDocumentRoot returns the root mapping node from a document node.
// YAML documents typically have a single document node containing the actual content.
func GetDocumentRoot(doc *yaml.Node) *yaml.Node {
	if doc == nil {
		return nil
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	return doc
}

// DeleteMapKey removes a key-value pair from a mapping node by key name.
// Returns true if the key was found and deleted.
func DeleteMapKey(node *yaml.Node, key string) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			// Remove both key and value (2 elements)
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			return true
		}
	}
	return false
}
