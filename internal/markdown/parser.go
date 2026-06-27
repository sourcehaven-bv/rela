package markdown

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/frontmatter"
)

// ErrConflictedFile is returned when a file has unresolved git conflict markers.
var ErrConflictedFile = errors.New("file has unresolved git conflicts")

const frontmatterDelimiter = frontmatter.Delimiter

// conflictMarkerStart is the canonical opening-marker pattern Git
// writes when a merge produces a conflict (`<<<<<<< <ref>`).
// Detection MUST be line-anchored: the marker is meaningful only at
// column 0. Matching it as a substring anywhere triggers a
// false-positive on legitimate content (BUG-WN6D) — e.g. a markdown
// codespan or quoted prose mentioning the marker — which silently
// excludes the file from rela's validator and search index.
var conflictMarkerStart = []byte("<<<<<<<")

// Document represents a parsed markdown document with YAML frontmatter
type Document struct {
	Frontmatter map[string]interface{}
	Content     string
}

// HasConflictMarkers reports whether content begins a conflict block
// — i.e. contains the opening marker (`<<<<<<<`) at column 0 of any
// line. A substring match anywhere else (inline code, prose) is
// NOT a conflict; see [BUG-WN6D] for the regression that motivated
// the line-anchoring.
func HasConflictMarkers(content []byte) bool {
	return hasLineAnchoredMarker(content, conflictMarkerStart)
}

// HasConflictMarkersString is the string-typed companion of
// [HasConflictMarkers]; same semantics.
func HasConflictMarkersString(content string) bool {
	return hasLineAnchoredMarker([]byte(content), conflictMarkerStart)
}

// hasLineAnchoredMarker reports whether content contains marker at
// the start of any line (i.e. either at offset 0 or immediately
// after a `\n`). Pure-Go scan; cheaper than compiling a regex for
// the same predicate.
func hasLineAnchoredMarker(content, marker []byte) bool {
	// Fast path: cheap substring check first — most content won't
	// contain the marker at all. The line-anchor check only runs
	// when the substring exists.
	idx := bytes.Index(content, marker)
	for idx >= 0 {
		if idx == 0 || content[idx-1] == '\n' {
			return true
		}
		// Search the remainder.
		offset := idx + len(marker)
		rest := bytes.Index(content[offset:], marker)
		if rest < 0 {
			return false
		}
		idx = offset + rest
	}
	return false
}

// ParseDocument parses a markdown document with YAML frontmatter.
// Returns ErrConflictedFile if the content contains git conflict markers.
func ParseDocument(content string) (*Document, error) {
	if HasConflictMarkersString(content) {
		return nil, ErrConflictedFile
	}

	fmBlock, body := frontmatter.Split(content)

	var fm map[string]interface{}
	if fmBlock != "" {
		if err := yaml.Unmarshal([]byte(fmBlock), &fm); err != nil {
			return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
		}
	}

	return &Document{
		Frontmatter: fm,
		Content:     body,
	}, nil
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
