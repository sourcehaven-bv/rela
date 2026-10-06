// Package configedit edits a project's configuration files (schema.yaml and
// data-entry.yaml) on behalf of the in-app Configure space (TKT-F5NGMG).
//
// The browser edits the two files as plain trees and sends the whole edited
// tree back. This package turns a YAML file into such a tree, checks that an
// edited tree only touches what the Configure space may change, and merges it
// back into the file's YAML syntax tree so comments and untouched lines stay
// as the author wrote them.
package configedit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Map is a YAML mapping with its key order kept. Key order is meaningful in
// rela's configuration: the order of an entity type's properties is the order
// forms and the detail page start from.
//
// On the wire a Map is the object {"$m": [[key, value], ...]}. A plain JSON
// object cannot carry the order, because a browser sorts integer-like keys
// ("1", "2") ahead of the others whatever order they arrived in.
type Map struct {
	Keys   []string
	Values []any

	// Origin is the index this mapping had in its list when the file was
	// read, sent as "$i" beside "$m". Items in rela's lists often have no
	// stable name (a navigation entry is known by its label, which is
	// editable), so the merge matches an edited item to its old self by this
	// index. A new item has none.
	Origin    int
	HasOrigin bool
}

// Get returns the value under key and whether it is present.
func (m *Map) Get(key string) (any, bool) {
	for i, k := range m.Keys {
		if k == key {
			return m.Values[i], true
		}
	}
	return nil, false
}

// Set replaces the value under key, or appends the key when it is absent.
func (m *Map) Set(key string, value any) {
	for i, k := range m.Keys {
		if k == key {
			m.Values[i] = value
			return
		}
	}
	m.Keys = append(m.Keys, key)
	m.Values = append(m.Values, value)
}

const (
	mapMarker    = "$m"
	originMarker = "$i"
)

// MarshalJSON writes the ordered wire form.
func (m *Map) MarshalJSON() ([]byte, error) {
	pairs := make([][2]any, len(m.Keys))
	for i, k := range m.Keys {
		pairs[i] = [2]any{k, m.Values[i]}
	}
	obj := map[string]any{mapMarker: pairs}
	if m.HasOrigin {
		obj[originMarker] = m.Origin
	}
	return json.Marshal(obj)
}

// A tree value is one of *Map, []any, string, json.Number, bool or nil.

// FromYAML converts a parsed YAML document into a tree.
//
// Scalars keep their YAML type: a quoted "1.0" stays a string, an unquoted 3
// becomes a number. Tags the tree has no type for (timestamps, binary) arrive
// as their literal text, which is what the author typed.
func FromYAML(doc *yaml.Node) (any, error) {
	if doc == nil || (doc.Kind == yaml.DocumentNode && len(doc.Content) == 0) {
		return nil, nil //nolint:nilnil // nil is the tree value of an empty document (YAML null), not a missing result
	}
	switch doc.Kind {
	case yaml.DocumentNode:
		return FromYAML(doc.Content[0])
	case yaml.MappingNode:
		m := &Map{}
		for i := 0; i+1 < len(doc.Content); i += 2 {
			key := doc.Content[i]
			if key.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("line %d: a mapping key must be a plain value", key.Line)
			}
			if key.Value == "<<" {
				return nil, fmt.Errorf("line %d: merge keys (<<) are not supported", key.Line)
			}
			v, err := FromYAML(doc.Content[i+1])
			if err != nil {
				return nil, err
			}
			m.Keys = append(m.Keys, key.Value)
			m.Values = append(m.Values, v)
		}
		return m, nil
	case yaml.SequenceNode:
		items := make([]any, 0, len(doc.Content))
		for i, c := range doc.Content {
			v, err := FromYAML(c)
			if err != nil {
				return nil, err
			}
			if m, ok := v.(*Map); ok {
				m.Origin, m.HasOrigin = i, true
			}
			items = append(items, v)
		}
		return items, nil
	case yaml.ScalarNode:
		return scalarValue(doc), nil
	case yaml.AliasNode:
		return nil, fmt.Errorf("line %d: anchors and aliases are not supported", doc.Line)
	}
	return nil, fmt.Errorf("line %d: unsupported YAML node", doc.Line)
}

// scalarValue is the tree value of a scalar node.
func scalarValue(n *yaml.Node) any {
	switch n.ShortTag() {
	case "!!null":
		return nil
	case "!!bool":
		var b bool
		if err := n.Decode(&b); err == nil {
			return b
		}
	case "!!int":
		var i int64
		if err := n.Decode(&i); err == nil {
			return json.Number(strconv.FormatInt(i, 10))
		}
	case "!!float":
		var f float64
		// JSON has no infinity or NaN; those keep the text the author wrote.
		if err := n.Decode(&f); err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) {
			return json.Number(strconv.FormatFloat(f, 'g', -1, 64))
		}
	}
	return n.Value
}

// DecodeJSON reads a tree from its wire form.
func DecodeJSON(r io.Reader) (any, error) {
	dec := &decoder{Decoder: json.NewDecoder(r)}
	dec.UseNumber()
	v, err := decodeValue(dec, 0)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected data after the tree")
	}
	return v, nil
}

// UnmarshalTree reads a tree from a JSON value already split out of a larger
// request body.
func UnmarshalTree(raw json.RawMessage) (any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil //nolint:nilnil // an absent value reads as null, the same tree value JSON null decodes to
	}
	return DecodeJSON(bytes.NewReader(raw))
}

// Limits on a decoded tree. rela's largest configuration files hold a few
// thousand nodes and nest a dozen deep; these bound the work a request can
// cause well above that.
const (
	maxDepth = 64
	maxNodes = 200_000
)

// decoder counts nodes across one decode.
type decoder struct {
	*json.Decoder
	nodes int
}

func decodeValue(dec *decoder, depth int) (any, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("the tree nests deeper than %d levels", maxDepth)
	}
	dec.nodes++
	if dec.nodes > maxNodes {
		return nil, fmt.Errorf("the tree has more than %d values", maxNodes)
	}
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '[':
			items := []any{}
			for dec.More() {
				v, err := decodeValue(dec, depth+1)
				if err != nil {
					return nil, err
				}
				items = append(items, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return items, nil
		case '{':
			return decodeMap(dec, depth)
		}
		return nil, fmt.Errorf("unexpected %q", t)
	case string, json.Number, bool, nil:
		return t, nil
	}
	return nil, fmt.Errorf("unexpected token %v", tok)
}

// decodeMap reads the {"$m": [[key, value], ...], "$i": n} object after its
// opening brace.
func decodeMap(dec *decoder, depth int) (any, error) {
	m := &Map{}
	sawPairs := false
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch tok {
		case mapMarker:
			if sawPairs {
				return nil, fmt.Errorf("duplicate %q", mapMarker)
			}
			sawPairs = true
			if err := decodePairs(dec, m, depth); err != nil {
				return nil, err
			}
		case originMarker:
			if m.HasOrigin {
				return nil, fmt.Errorf("duplicate %q", originMarker)
			}
			tok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			n, ok := tok.(json.Number)
			if !ok {
				return nil, fmt.Errorf("%q must be a number", originMarker)
			}
			i, err := n.Int64()
			if err != nil || i < 0 || i > maxOrigin {
				return nil, fmt.Errorf("%q is out of range", originMarker)
			}
			m.Origin, m.HasOrigin = int(i), true
		default:
			return nil, fmt.Errorf("a mapping must be written as {%q: [[key, value], ...]}", mapMarker)
		}
	}
	if !sawPairs {
		return nil, fmt.Errorf("a mapping must be written as {%q: [[key, value], ...]}", mapMarker)
	}
	if _, err := dec.Token(); err != nil { // closing }
		return nil, err
	}
	return m, nil
}

// maxOrigin bounds "$i"; no configuration list comes near it.
const maxOrigin = 1 << 20

func decodePairs(dec *decoder, m *Map, depth int) error {
	if tok, err := dec.Token(); err != nil || tok != json.Delim('[') {
		return errors.New("a mapping's pairs must be an array")
	}
	seen := map[string]bool{}
	for dec.More() {
		if tok, err := dec.Token(); err != nil || tok != json.Delim('[') {
			return errors.New("a mapping pair must be a [key, value] array")
		}
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := tok.(string)
		if !ok {
			return errors.New("a mapping key must be a string")
		}
		if seen[key] {
			return fmt.Errorf("duplicate key %q", key)
		}
		if key == "<<" {
			// Written out, this key is a YAML merge key, which would graft
			// its value onto the mapping one level up from where it is
			// checked.
			return errors.New("merge keys (<<) are not supported")
		}
		seen[key] = true
		v, err := decodeValue(dec, depth+1)
		if err != nil {
			return err
		}
		if tok, err := dec.Token(); err != nil || tok != json.Delim(']') {
			return errors.New("a mapping pair must hold exactly a key and a value")
		}
		m.Keys = append(m.Keys, key)
		m.Values = append(m.Values, v)
	}
	_, err := dec.Token() // closing ]
	return err
}
