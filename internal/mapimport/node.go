package mapimport

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/safexml"
)

type nodeKind int

const (
	kindScalar nodeKind = iota
	kindObject
	kindArray
)

// Node is one value of a decoded document: an object with ordered fields, an
// array, or a scalar (string, json.Number, bool or nil). Every node knows its
// parent, so a path can step up with `^`.
//
// XML maps onto the same shape: an element is an object whose attributes are
// fields named `@name`, whose child elements are arrays under their local
// name, and whose `#text` field holds all its inner text with whitespace
// collapsed. The text is computed when a path reads it, not stored, so a
// deep document does not hold a copy of its text at every level.
type Node struct {
	kind   nodeKind
	keys   []string
	fields map[string]*Node
	items  []*Node
	scalar any
	// xml is the source element of an XML element node, nil otherwise.
	xml *safexml.Node

	parent *Node
	name   string // field name in the parent object
	index  int    // position in the parent array
	line   int    // source line, when the decoder knows it
}

func (n *Node) get(name string) *Node {
	if n.kind != kindObject {
		return nil
	}
	if name == xmlTextField && n.xml != nil {
		t := innerText(n.xml)
		if t == "" {
			return nil
		}
		v := newScalar(t)
		v.parent, v.name = n, name
		return v
	}
	return n.fields[name]
}

// xmlTextField is the field that reads an XML element's inner text.
const xmlTextField = "#text"

// set adds a field to an object node. A repeated name keeps the last value
// in the position of the first, as encoding/json does.
func (n *Node) set(name string, v *Node) {
	if _, dup := n.fields[name]; !dup {
		n.keys = append(n.keys, name)
	}
	v.parent, v.name = n, name
	n.fields[name] = v
}

func (n *Node) add(v *Node) {
	v.parent, v.index = n, len(n.items)
	n.items = append(n.items, v)
}

func newObject() *Node { return &Node{kind: kindObject, fields: map[string]*Node{}} }
func newArray() *Node  { return &Node{kind: kindArray} }
func newScalar(v any) *Node {
	return &Node{kind: kindScalar, scalar: v}
}

// Location renders where n sits in its document, such as
// `$.catalog.groups[0].controls[2]`, with the source line when known.
func (n *Node) Location() string {
	var parts []string
	for c := n; c.parent != nil; c = c.parent {
		if c.parent.kind == kindArray {
			parts = append(parts, "["+strconv.Itoa(c.index)+"]")
		} else {
			parts = append(parts, "."+c.name)
		}
	}
	var b strings.Builder
	b.WriteString("$")
	for i := len(parts) - 1; i >= 0; i-- {
		b.WriteString(parts[i])
	}
	if n.line > 0 {
		fmt.Fprintf(&b, " (line %d)", n.line)
	}
	return b.String()
}

// text returns the string form of a scalar, or the inner text of an XML
// element. An element without child elements has a text even when it is
// empty, so `<title/>` reads as absent. ok is false for any other object
// and for an array.
func (n *Node) text() (s string, ok bool) {
	switch n.kind {
	case kindScalar:
		switch v := n.scalar.(type) {
		case nil:
			return "", true
		case string:
			return v, true
		case json.Number:
			return v.String(), true
		case bool:
			return strconv.FormatBool(v), true
		default:
			return fmt.Sprint(v), true
		}
	case kindObject:
		if n.xml != nil {
			t := innerText(n.xml)
			return t, t != "" || len(n.xml.Children) == 0
		}
	case kindArray:
	}
	return "", false
}

// plain converts n to the plain Go value a Lua hook receives: a
// map[string]any, an []any, or a scalar. Numbers stay in their source
// spelling as strings.
func (n *Node) plain() any {
	switch n.kind {
	case kindObject:
		out := make(map[string]any, len(n.keys)+1)
		for _, k := range n.keys {
			out[k] = n.fields[k].plain()
		}
		if n.xml != nil {
			if t := innerText(n.xml); t != "" {
				out[xmlTextField] = t
			}
		}
		return out
	case kindArray:
		out := make([]any, 0, len(n.items))
		for _, it := range n.items {
			out = append(out, it.plain())
		}
		return out
	default:
		if num, isNum := n.scalar.(json.Number); isNum {
			return num.String()
		}
		return n.scalar
	}
}
