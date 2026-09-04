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
		valNode, err := ValueToNode(val)
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
		valNode, err := ValueToNode(data[key])
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, valNode)
	}

	return yaml.Marshal(node)
}

// ValueToNode encodes a property value as a YAML node, working around a
// yaml.v3 emitter defect for strings that would otherwise become an
// unreadable block scalar (BUG-B1RA3J / issue #993).
//
// Exported so fsstore shares this one implementation. It was duplicated there
// until BUG-B1RA3J, where fixing one copy and re-running the fuzz target still
// failed — entity writes and relation writes must not disagree about how a
// value is encoded.
//
// The upstream defect: for a string starting with a newline, the emitter writes
// an indent indicator that disagrees with the body it then writes.
//
//	yaml.Marshal([]string{"\n0"})  ->  "- |4-\n  0\n"
//
// The indicator says 4, the body is indented 2, and Unmarshal rejects yaml.v3's
// own output. A lone "\n" is worse than an error: it emits "|4+" and reads back
// as "", losing the value silently. See [needsQuoting] for the full list of
// shapes that break.
//
// Values are built BEFORE Encode rather than fixing up its result, because
// Node.Encode round-trips internally and returns the error itself — there is
// no node to post-process.
//
// Quoting rather than REJECTING is deliberate. pgstore serializes properties
// with json.Marshal, where these strings round-trip fine, so refusing them here
// would make the two backends disagree about what a valid entity is: the same
// write would succeed on Postgres and fail on fsstore. A storage-layer
// serialization limit must not become a data-validity rule. The store persists
// what it is given or fails loudly; it does not hold an opinion about which
// strings are worth keeping.
//
// Scoped to values that ACTUALLY break: a value whose subtree contains no
// breaking string goes through Encode untouched, so ordinary multi-line strings
// keep block style and no reflow churn lands on unrelated files.
func ValueToNode(val any) (*yaml.Node, error) {
	if !containsBreakingString(val, false) {
		return encodeNode(val)
	}
	return buildNode(val, false)
}

// encodeNode is the plain yaml.v3 path, used for every value that does not
// contain a string [needsQuoting] would flag.
func encodeNode(val any) (*yaml.Node, error) {
	var node yaml.Node
	if err := node.Encode(val); err != nil {
		return nil, err
	}
	return &node, nil
}

// containsBreakingString reports whether val, or any string nested in it
// through the container shapes a property value can take, would trip the
// emitter. Anything else (numbers, bools, nil, times) cannot. underSeq is
// whether a sequence sits anywhere above val; see [needsQuoting].
func containsBreakingString(val any, underSeq bool) bool {
	switch v := val.(type) {
	case string:
		return needsQuoting(v, underSeq)
	case []string:
		return slices.ContainsFunc(v, func(s string) bool { return needsQuoting(s, true) })
	case []any:
		return slices.ContainsFunc(v, func(e any) bool { return containsBreakingString(e, true) })
	case map[string]string:
		for _, s := range v {
			if needsQuoting(s, underSeq) {
				return true
			}
		}
	case map[string]any:
		for _, e := range v {
			if containsBreakingString(e, underSeq) {
				return true
			}
		}
	}
	return false
}

// buildNode assembles the node tree by hand for a value that contains a
// breaking string, quoting exactly those strings. Leaves that do not break
// still go through Encode so their tags and styles match what yaml.v3 would
// have chosen (a string that looks like a number stays quoted, for instance).
// Container shapes not listed here fall through to Encode, which either
// succeeds or reports its own error; nothing is swallowed.
func buildNode(val any, underSeq bool) (*yaml.Node, error) {
	switch v := val.(type) {
	case string:
		if needsQuoting(v, underSeq) {
			return &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: v,
				Style: yaml.DoubleQuotedStyle,
			}, nil
		}
	case []string:
		items := make([]any, len(v))
		for i, s := range v {
			items[i] = s
		}
		return buildSequence(items)
	case []any:
		return buildSequence(v)
	case map[string]string:
		entries := make(map[string]any, len(v))
		for k, s := range v {
			entries[k] = s
		}
		return buildMapping(entries, underSeq)
	case map[string]any:
		return buildMapping(v, underSeq)
	}
	return encodeNode(val)
}

func buildSequence(items []any) (*yaml.Node, error) {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, item := range items {
		child, err := buildNode(item, true)
		if err != nil {
			return nil, err
		}
		seq.Content = append(seq.Content, child)
	}
	return seq, nil
}

// buildMapping emits keys in the order Encode would have used. yaml.v3 sorts
// map keys with a numeric-aware comparison ("a2" before "a10"), not
// bytewise, and that ordering is what every existing file on disk has; the
// keys are learned by encoding a value-free copy of the map rather than by
// reimplementing the comparator.
func buildMapping(entries map[string]any, underSeq bool) (*yaml.Node, error) {
	probe := make(map[string]any, len(entries))
	for k := range entries {
		probe[k] = nil
	}
	ordered, err := encodeNode(probe)
	if err != nil {
		return nil, err
	}
	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for i := 0; i < len(ordered.Content); i += 2 {
		keyNode := ordered.Content[i]
		child, err := buildNode(entries[keyNode.Value], underSeq)
		if err != nil {
			return nil, err
		}
		mapping.Content = append(mapping.Content, keyNode, child)
	}
	return mapping, nil
}

// needsQuoting reports whether s would emit a block scalar yaml.v3 cannot read
// back. Three shapes trip the emitter, characterized by fuzzing yaml.v3
// directly in every nesting a property value can have (top level, map value,
// sequence item, and their combinations); 45s of fuzzing with these excluded
// found no fourth:
//
//   - a LEADING newline. The indent indicator disagrees with the body, so
//     "\n0" reads back as "0" and "\n" as "" — silently.
//   - a multi-line string whose FIRST line starts with a tab. The tab lands in
//     the indentation column of the block scalar and the parser rejects it.
//   - a multi-line string whose first line starts with a SPACE, but only when a
//     sequence sits anywhere above it (underSeq). As a plain map value
//     " x\ny" emits a correct "|4-" indicator; one level under a "- " the
//     indicator and body disagree again. Quoting it in map context too would
//     reflow every existing file carrying such a value for no gain, which is
//     why the caller passes the context in rather than this being a pure
//     function of s.
//
// Interior and trailing newlines, and tabs or spaces on later lines, are all
// emitted correctly, so "a\nb", "0\n" and "a\n\tb" keep block style. A string
// whose first line is only whitespace, or a tab-led string with no newline, is
// already double-quoted by the emitter itself, so flagging it changes nothing.
func needsQuoting(s string, underSeq bool) bool {
	if strings.HasPrefix(s, "\n") {
		return true
	}
	if !strings.Contains(s, "\n") {
		return false
	}
	return strings.HasPrefix(s, "\t") || (underSeq && strings.HasPrefix(s, " "))
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
