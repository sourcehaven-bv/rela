package markdown

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrConflictedFile is returned when a file has unresolved git conflict markers.
var ErrConflictedFile = errors.New("file has unresolved git conflicts")

const frontmatterDelimiter = "---"

// Git conflict marker for detecting conflicts.
var conflictMarkerStart = []byte("<<<<<<<")

// Document represents a parsed markdown document with YAML frontmatter
type Document struct {
	Frontmatter map[string]interface{}
	Content     string
}

// HasConflictMarkers checks if content contains git conflict markers.
func HasConflictMarkers(content []byte) bool {
	return bytes.Contains(content, conflictMarkerStart)
}

// HasConflictMarkersString checks if content contains git conflict markers.
func HasConflictMarkersString(content string) bool {
	return strings.Contains(content, string(conflictMarkerStart))
}

// ParseDocument parses a markdown document with YAML frontmatter.
// Returns ErrConflictedFile if the content contains git conflict markers.
func ParseDocument(content string) (*Document, error) {
	if HasConflictMarkersString(content) {
		return nil, ErrConflictedFile
	}

	frontmatter, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	var fm map[string]interface{}
	if frontmatter != "" {
		if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
			return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
		}
	}

	return &Document{
		Frontmatter: fm,
		Content:     body,
	}, nil
}

// splitFrontmatter separates YAML frontmatter from markdown content
func splitFrontmatter(content string) (frontmatter, body string, err error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var lines []string
	inFrontmatter := false
	frontmatterEnded := false
	frontmatterLines := []string{}

	for scanner.Scan() {
		line := scanner.Text()

		if !inFrontmatter && !frontmatterEnded && strings.TrimSpace(line) == frontmatterDelimiter {
			inFrontmatter = true
			continue
		}

		if inFrontmatter && strings.TrimSpace(line) == frontmatterDelimiter {
			inFrontmatter = false
			frontmatterEnded = true
			continue
		}

		if inFrontmatter {
			frontmatterLines = append(frontmatterLines, line)
		} else if frontmatterEnded || !inFrontmatter {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", "", err
	}

	frontmatter = strings.Join(frontmatterLines, "\n")
	body = strings.TrimPrefix(strings.Join(lines, "\n"), "\n")

	return frontmatter, body, nil
}

// FormatDocument formats a document back to markdown with YAML frontmatter.
// Keys are output in alphabetical order (yaml.Marshal default behavior).
func FormatDocument(frontmatter map[string]interface{}, content string) (string, error) {
	return FormatDocumentOrdered(frontmatter, content, nil)
}

// FormatDocumentOrdered formats a document with YAML frontmatter in a specific key order.
// If keyOrder is nil or empty, keys are sorted alphabetically.
// Keys in keyOrder appear first (in that order), followed by any remaining keys alphabetically.
func FormatDocumentOrdered(frontmatter map[string]interface{}, content string, keyOrder []string) (string, error) {
	var sb strings.Builder

	if len(frontmatter) > 0 {
		sb.WriteString(frontmatterDelimiter)
		sb.WriteString("\n")

		var yamlBytes []byte
		var err error

		if len(keyOrder) > 0 {
			yamlBytes, err = marshalOrdered(frontmatter, keyOrder)
		} else {
			yamlBytes, err = yaml.Marshal(frontmatter)
		}
		if err != nil {
			return "", err
		}
		sb.Write(yamlBytes)
		sb.WriteString(frontmatterDelimiter)
		sb.WriteString("\n")
	}

	if content != "" {
		sb.WriteString("\n")
		sb.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

// marshalOrdered marshals a map to YAML with keys in the specified order.
// Keys in keyOrder appear first, followed by remaining keys alphabetically.
func marshalOrdered(data map[string]interface{}, keyOrder []string) ([]byte, error) {
	// Build yaml.Node with ordered keys
	node := &yaml.Node{
		Kind: yaml.MappingNode,
	}

	// Track which keys we've added
	added := make(map[string]bool)

	// Add keys in specified order first
	for _, key := range keyOrder {
		val, ok := data[key]
		if !ok {
			continue
		}
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		)
		valNode, err := valueToNode(val)
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, valNode)
		added[key] = true
	}

	// Collect remaining keys and sort them
	var remaining []string
	for key := range data {
		if !added[key] {
			remaining = append(remaining, key)
		}
	}
	sort.Strings(remaining)

	// Add remaining keys alphabetically
	for _, key := range remaining {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		)
		valNode, err := valueToNode(data[key])
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, valNode)
	}

	return yaml.Marshal(node)
}

// valueToNode converts a Go value to a yaml.Node.
func valueToNode(val interface{}) (*yaml.Node, error) {
	var node yaml.Node
	if err := node.Encode(val); err != nil {
		return nil, err
	}
	return &node, nil
}

// GetString extracts a string value from frontmatter
func (d *Document) GetString(key string) string {
	if d.Frontmatter == nil {
		return ""
	}
	if v, ok := d.Frontmatter[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetStringSlice extracts a string slice from frontmatter
func (d *Document) GetStringSlice(key string) []string {
	if d.Frontmatter == nil {
		return nil
	}
	if v, ok := d.Frontmatter[key]; ok {
		switch val := v.(type) {
		case []interface{}:
			result := make([]string, 0, len(val))
			for _, item := range val {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		case []string:
			return val
		}
	}
	return nil
}
