package configedit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// identityKeys are the keys that name an item in a configuration list, in the
// order they are tried. A list item is matched to its old self by the first
// of these it carries, so reordering a list moves each item's YAML node, and
// its comments, along with it.
var identityKeys = []string{"name", "id", "property", "relation", "group", "label", "entity", "list", "form"}

// Apply applies an edited tree to a YAML file and returns the new contents.
//
// Nodes whose value did not change are reused as they are, so their comments
// and quoting survive. A mapping takes its key order from the tree; a list
// matches items by identity (see identityKeys) and otherwise by position.
// The result is then laid back over the original text (see keepLayout), so
// blank lines and every untouched line stay byte-identical.
func Apply(original []byte, tree any) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(original, &doc); err != nil {
		return nil, err
	}
	var root *yaml.Node
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		root = doc.Content[0]
	}
	merged, err := mergeValue(root, tree)
	if err != nil {
		return nil, err
	}
	editedDoc := doc
	editedDoc.Kind = yaml.DocumentNode
	editedDoc.Content = []*yaml.Node{merged}
	edited, err := encode(&editedDoc)
	if err != nil {
		return nil, err
	}
	// What is written must read back as exactly the tree that was checked:
	// the allowlist judged the tree, and the server will run the file.
	if !readsBackAs(edited, tree) {
		return nil, fmt.Errorf("%w: the edit does not read back as written", ErrBadDraft)
	}
	if root == nil {
		return []byte(edited), nil
	}
	base, err := encode(&doc)
	if err != nil {
		return nil, err
	}
	out := keepLayout(string(original), base, edited)
	if out == edited {
		return []byte(out), nil
	}
	if !readsBackAs(out, tree) {
		// The line merge lines up text, not structure, and can pair lines at
		// the wrong depth when the file's layout differs from the encoder's.
		// Fall back to the encoding, which keeps comments and loses only
		// layout.
		return []byte(edited), nil
	}
	return []byte(out), nil
}

// readsBackAs reports whether text parses to tree.
func readsBackAs(text string, tree any) bool {
	back, err := parseTree(text)
	return err == nil && equalTree(back, tree)
}

// equalTree reports whether two trees hold the same content. List origins
// are ignored, and numbers compare by value, since 1.0 is written as 1.
func equalTree(a, b any) bool {
	switch x := a.(type) {
	case *Map:
		y, ok := b.(*Map)
		if !ok || len(x.Keys) != len(y.Keys) {
			return false
		}
		for i, k := range x.Keys {
			if y.Keys[i] != k || !equalTree(x.Values[i], y.Values[i]) {
				return false
			}
		}
		return true
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !equalTree(x[i], y[i]) {
				return false
			}
		}
		return true
	case json.Number:
		y, ok := b.(json.Number)
		if !ok {
			return false
		}
		fx, errX := x.Float64()
		fy, errY := y.Float64()
		return x == y || (errX == nil && errY == nil && fx == fy)
	}
	return reflect.DeepEqual(a, b)
}

func parseTree(text string) (any, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, err
	}
	return FromYAML(&doc)
}

// encode writes a document with a two-space indent, the indent rela's files
// are written in; the YAML library's default of four would rewrite every line.
func encode(doc *yaml.Node) (string, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func mergeValue(old *yaml.Node, v any) (*yaml.Node, error) {
	switch t := v.(type) {
	case *Map:
		if old != nil && old.Kind == yaml.MappingNode {
			return mergeMap(old, t)
		}
		return newNode(v)
	case []any:
		if old != nil && old.Kind == yaml.SequenceNode {
			return mergeSeq(old, t)
		}
		return newNode(v)
	default:
		if old != nil && old.Kind == yaml.ScalarNode && sameScalar(old, v) {
			return old, nil
		}
		n, err := newNode(v)
		if err != nil {
			return nil, err
		}
		if old != nil && old.Kind == yaml.ScalarNode {
			// The value changed but the comments around it did not.
			n.HeadComment, n.LineComment, n.FootComment = old.HeadComment, old.LineComment, old.FootComment
			if n.Tag == "!!str" && old.ShortTag() == "!!str" && old.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) != 0 {
				n.Style = old.Style
			}
		}
		return n, nil
	}
}

func mergeMap(old *yaml.Node, m *Map) (*yaml.Node, error) {
	oldPairs := make(map[string][2]*yaml.Node, len(old.Content)/2)
	for i := 0; i+1 < len(old.Content); i += 2 {
		oldPairs[old.Content[i].Value] = [2]*yaml.Node{old.Content[i], old.Content[i+1]}
	}
	content := make([]*yaml.Node, 0, 2*len(m.Keys))
	for i, key := range m.Keys {
		var keyNode, valNode *yaml.Node
		if pair, ok := oldPairs[key]; ok {
			keyNode, valNode = pair[0], pair[1]
		} else {
			keyNode = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
		}
		merged, err := mergeValue(valNode, m.Values[i])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		content = append(content, keyNode, merged)
	}
	out := *old
	out.Content = content
	return &out, nil
}

func mergeSeq(old *yaml.Node, items []any) (*yaml.Node, error) {
	matches := matchItems(old.Content, items)
	content := make([]*yaml.Node, 0, len(items))
	for i, item := range items {
		var prev *yaml.Node
		if matches[i] >= 0 {
			prev = old.Content[matches[i]]
		}
		merged, err := mergeValue(prev, item)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		content = append(content, merged)
	}
	out := *old
	out.Content = content
	return &out, nil
}

// matchItems returns, for each edited list item, the index of the old node
// it continues, or -1 for a new item. Each old node is matched at most once.
func matchItems(oldItems []*yaml.Node, items []any) []int {
	used := make([]bool, len(oldItems))
	byIdentity := map[string]int{}
	for i, n := range oldItems {
		if id, ok := nodeIdentity(n); ok {
			if _, dup := byIdentity[id]; !dup {
				byIdentity[id] = i
			}
		}
	}

	matches := make([]int, len(items))
	for i := range matches {
		matches[i] = -1
	}
	// The index an item had when the file was read is the surest match.
	for i, item := range items {
		if m, ok := item.(*Map); ok && m.HasOrigin && m.Origin < len(oldItems) && !used[m.Origin] {
			matches[i], used[m.Origin] = m.Origin, true
		}
	}
	for i, item := range items {
		if matches[i] >= 0 {
			continue
		}
		if id, ok := treeIdentity(item); ok {
			if j, found := byIdentity[id]; found && !used[j] {
				matches[i], used[j] = j, true
			}
		}
	}
	// Items without an identity fall back to their position, as long as the
	// old item there has no identity of its own and is still free.
	for i, item := range items {
		if matches[i] >= 0 || i >= len(oldItems) || used[i] {
			continue
		}
		if _, ok := treeIdentity(item); ok {
			continue
		}
		if _, ok := nodeIdentity(oldItems[i]); ok {
			continue
		}
		matches[i], used[i] = i, true
	}
	return matches
}

func nodeIdentity(n *yaml.Node) (string, bool) {
	if n.Kind == yaml.ScalarNode {
		return "=" + n.Value, true
	}
	if n.Kind != yaml.MappingNode {
		return "", false
	}
	for _, key := range identityKeys {
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == key && n.Content[i+1].Kind == yaml.ScalarNode {
				return key + "=" + n.Content[i+1].Value, true
			}
		}
	}
	return "", false
}

func treeIdentity(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return "=" + t, true
	case json.Number:
		return "=" + t.String(), true
	case *Map:
		for _, key := range identityKeys {
			if val, ok := t.Get(key); ok {
				switch s := val.(type) {
				case string:
					return key + "=" + s, true
				case json.Number:
					return key + "=" + s.String(), true
				}
			}
		}
	}
	return "", false
}

// sameScalar reports whether a scalar node already holds the tree value, so
// the node, with its quoting and comments, can be kept.
func sameScalar(n *yaml.Node, v any) bool {
	old := scalarValue(n)
	switch a := old.(type) {
	case json.Number:
		b, ok := v.(json.Number)
		if !ok {
			return false
		}
		fa, errA := strconv.ParseFloat(a.String(), 64)
		fb, errB := strconv.ParseFloat(b.String(), 64)
		return errA == nil && errB == nil && fa == fb
	default:
		return old == v
	}
}

// newNode builds YAML for a value that has no old node to reuse.
func newNode(v any) (*yaml.Node, error) {
	switch t := v.(type) {
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(t)}, nil
	case json.Number:
		tag := "!!int"
		if _, err := t.Int64(); err != nil {
			tag = "!!float"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: t.String()}, nil
	case string:
		n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: t}
		if strings.ContainsRune(t, '\n') {
			n.Style = yaml.LiteralStyle
		}
		return n, nil
	case []any:
		n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for i, item := range t {
			c, err := newNode(item)
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			n.Content = append(n.Content, c)
		}
		return n, nil
	case *Map:
		n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for i, key := range t.Keys {
			c, err := newNode(t.Values[i])
			if err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, c)
		}
		return n, nil
	}
	return nil, fmt.Errorf("unsupported value of type %T", v)
}
