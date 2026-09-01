package markdown

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
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
	Frontmatter map[string]any
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

	var fm map[string]any
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
func FormatDocument(frontmatter map[string]any, content string) (string, error) {
	return FormatDocumentOrdered(frontmatter, content, nil)
}

// FormatDocumentOrdered formats a document with YAML frontmatter in a specific key order.
// If keyOrder is nil or empty, keys are sorted alphabetically.
// Keys in keyOrder appear first (in that order), followed by any remaining keys alphabetically.
func FormatDocumentOrdered(frontmatter map[string]any, content string, keyOrder []string) (string, error) {
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
func marshalOrdered(data map[string]any, keyOrder []string) ([]byte, error) {
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
// ValueToNode encodes a property value as a YAML node, working around a
// yaml.v3 emitter defect for strings that would otherwise become an
// unreadable block scalar (BUG-B1RA3J / issue #993).
//
// Exported so fsstore shares this one implementation. It was duplicated there
// until BUG-B1RA3J, where fixing one copy and re-running the fuzz target still
// failed — entity writes and relation writes must not disagree about how a
// value is encoded.
func ValueToNode(val any) (*yaml.Node, error) {
	return valueToNode(val)
}

func valueToNode(val any) (*yaml.Node, error) {
	if node, ok := quotedScalarNode(val); ok {
		return node, nil
	}
	var node yaml.Node
	if err := node.Encode(val); err != nil {
		return nil, err
	}
	return &node, nil
}

// quotedScalarNode builds a double-quoted scalar node directly for values
// yaml.v3 cannot round-trip through a block scalar, and reports whether it did
// (BUG-B1RA3J / issue #993).
//
// The upstream defect: for a string starting with a newline, the emitter writes
// an indent indicator that disagrees with the body it then writes.
//
//	yaml.Marshal([]string{"\n0"})  ->  "- |4-\n  0\n"
//
// The indicator says 4, the body is indented 2, and Unmarshal rejects yaml.v3's
// own output. A lone "\n" is worse than an error: it emits "|4+" and reads back
// as "", losing the value silently.
//
// Built BEFORE Encode rather than fixing up its result, because Node.Encode
// round-trips internally and returns the error itself — there is no node to
// post-process.
//
// Quoting rather than REJECTING is deliberate. pgstore serializes properties
// with json.Marshal, where these strings round-trip fine, so refusing them here
// would make the two backends disagree about what a valid entity is: the same
// write would succeed on Postgres and fail on fsstore. A storage-layer
// serialization limit must not become a data-validity rule. The store persists
// what it is given or fails loudly; it does not hold an opinion about which
// strings are worth keeping.
//
// Scoped to values that ACTUALLY break, so ordinary multi-line strings keep
// block style and no reflow churn lands on unrelated files.
func quotedScalarNode(val any) (*yaml.Node, bool) {
	switch v := val.(type) {
	case string:
		if !needsQuoting(v) {
			return nil, false
		}
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: v,
			Style: yaml.DoubleQuotedStyle,
		}, true
	case []string:
		if !slices.ContainsFunc(v, needsQuoting) {
			return nil, false
		}
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, item := range v {
			node := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: item}
			if needsQuoting(item) {
				node.Style = yaml.DoubleQuotedStyle
			}
			seq.Content = append(seq.Content, node)
		}
		return seq, true
	case []any:
		anyBreaks := slices.ContainsFunc(v, func(e any) bool {
			s, ok := e.(string)
			return ok && needsQuoting(s)
		})
		if !anyBreaks {
			return nil, false
		}
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, item := range v {
			child, handled := quotedScalarNode(item)
			if !handled {
				child = &yaml.Node{}
				if err := child.Encode(item); err != nil {
					return nil, false
				}
			}
			seq.Content = append(seq.Content, child)
		}
		return seq, true
	}
	return nil, false
}

// needsQuoting reports whether s would emit a block scalar that yaml.v3 cannot
// read back. Only a LEADING newline trips the emitter; interior and trailing
// newlines are handled correctly, so "a\nb" and "0\n" keep block style.
func needsQuoting(s string) bool {
	return strings.HasPrefix(s, "\n")
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
		case []any:
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
