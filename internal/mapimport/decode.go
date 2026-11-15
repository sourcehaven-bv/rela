package mapimport

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/safexml"
)

// Format is an input format.
type Format string

// Input formats.
const (
	FormatJSON Format = "json"
	FormatYAML Format = "yaml"
	FormatXML  Format = "xml"
	FormatCSV  Format = "csv"
)

// Input caps. A mapping may lower or raise max_bytes up to MaxMaxBytes.
const (
	DefaultMaxBytes = 10 << 20
	MaxMaxBytes     = 50 << 20
	// DefaultMaxNodes bounds the decoded tree. Each node costs a few hundred
	// bytes with its parent link, so the cap keeps a hostile document from
	// multiplying its size in memory.
	DefaultMaxNodes = 1_000_000
	// maxDepth bounds nesting for every format.
	maxDepth = 128
	// maxXMLAttrs bounds the attributes of one XML element.
	maxXMLAttrs = 256
)

// LimitError reports input refused for its size or shape rather than for its
// syntax.
type LimitError struct{ Detail string }

func (e *LimitError) Error() string { return "input limit exceeded: " + e.Detail }

// DetectFormat returns the format for a file extension, or "" when unknown.
func DetectFormat(path string) Format {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return FormatJSON
	case ".yaml", ".yml":
		return FormatYAML
	case ".xml":
		return FormatXML
	case ".csv":
		return FormatCSV
	}
	return ""
}

// Limits bounds one decode. Zero fields take the defaults.
type Limits struct {
	MaxBytes int
	MaxNodes int
}

func (l Limits) withDefaults() Limits {
	if l.MaxBytes <= 0 {
		l.MaxBytes = DefaultMaxBytes
	}
	if l.MaxNodes <= 0 {
		l.MaxNodes = DefaultMaxNodes
	}
	return l
}

// Decode parses data in format f into a node tree. For XML the root node is
// an object holding the document element under its name, so a path reads
// `catalog.group` in XML as it reads `catalog.groups` in JSON. For CSV the
// root is an array of row objects keyed by the header.
func Decode(data []byte, f Format, lim Limits) (*Node, error) {
	lim = lim.withDefaults()
	if len(data) > lim.MaxBytes {
		return nil, &LimitError{Detail: fmt.Sprintf("input is %d bytes, limit is %d", len(data), lim.MaxBytes)}
	}
	data = safexml.TrimBOM(data)
	b := &budget{left: lim.MaxNodes}
	switch f {
	case FormatJSON:
		return decodeJSON(data, b)
	case FormatYAML:
		return decodeYAML(data, b)
	case FormatXML:
		return decodeXML(data, lim, b)
	case FormatCSV:
		return decodeCSV(data, b)
	}
	return nil, fmt.Errorf("unsupported format %q (use json, yaml, xml or csv)", f)
}

// budget counts nodes against the cap.
type budget struct{ left int }

func (b *budget) take() error {
	b.left--
	if b.left < 0 {
		return &LimitError{Detail: "too many values in the input"}
	}
	return nil
}

func depthError() error {
	return &LimitError{Detail: fmt.Sprintf("nesting deeper than %d", maxDepth)}
}

func decodeJSON(data []byte, b *budget) (*Node, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	root, err := jsonValue(d, b, 1)
	if err != nil {
		return nil, jsonError(d, err)
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("json: offset %d: content after the top-level value", d.InputOffset())
	}
	return root, nil
}

func jsonError(d *json.Decoder, err error) error {
	if le, ok := errors.AsType[*LimitError](err); ok {
		return le
	}
	return fmt.Errorf("json: offset %d: %w", d.InputOffset(), err)
}

func jsonValue(d *json.Decoder, b *budget, depth int) (*Node, error) {
	if depth > maxDepth {
		return nil, depthError()
	}
	if err := b.take(); err != nil {
		return nil, err
	}
	tok, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			obj := newObject()
			for d.More() {
				kt, err := d.Token()
				if err != nil {
					return nil, err
				}
				key, _ := kt.(string)
				v, err := jsonValue(d, b, depth+1)
				if err != nil {
					return nil, err
				}
				obj.set(key, v)
			}
			_, err := d.Token() // '}'
			return obj, err
		case '[':
			arr := newArray()
			for d.More() {
				v, err := jsonValue(d, b, depth+1)
				if err != nil {
					return nil, err
				}
				arr.add(v)
			}
			_, err := d.Token() // ']'
			return arr, err
		}
		return nil, fmt.Errorf("unexpected %v", t)
	default:
		return newScalar(t), nil
	}
}

func decodeYAML(data []byte, b *budget) (*Node, error) {
	// yaml.v3 builds its whole tree before the budget runs. Values are
	// separated by line breaks, commas and brackets, so the tree has at most
	// a few nodes per separator. Counting them first keeps a long input from
	// allocating millions of nodes. Aliases are counted during conversion.
	if marks := bytes.Count(data, []byte("\n")) + bytes.Count(data, []byte(",")) +
		bytes.Count(data, []byte("[")) + bytes.Count(data, []byte("{")); marks > b.left {
		return nil, &LimitError{Detail: "too many lines or values in the YAML input"}
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("yaml: %w", err)
	}
	if doc.Kind == 0 {
		return nil, errors.New("yaml: empty document")
	}
	return yamlValue(&doc, b, 1)
}

func yamlValue(y *yaml.Node, b *budget, depth int) (*Node, error) {
	if depth > maxDepth {
		return nil, depthError()
	}
	if err := b.take(); err != nil {
		return nil, err
	}
	var n *Node
	switch y.Kind {
	case yaml.DocumentNode:
		if len(y.Content) == 0 {
			return newScalar(nil), nil
		}
		return yamlValue(y.Content[0], b, depth)
	case yaml.AliasNode:
		// Every expansion of an alias is counted against the budget, so an
		// alias bomb stops at the node cap.
		return yamlValue(y.Alias, b, depth+1)
	case yaml.MappingNode:
		n = newObject()
		for i := 0; i+1 < len(y.Content); i += 2 {
			k := y.Content[i]
			if k.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("yaml: line %d: mapping keys must be scalars", k.Line)
			}
			v, err := yamlValue(y.Content[i+1], b, depth+1)
			if err != nil {
				return nil, err
			}
			n.set(k.Value, v)
		}
	case yaml.SequenceNode:
		n = newArray()
		for _, c := range y.Content {
			v, err := yamlValue(c, b, depth+1)
			if err != nil {
				return nil, err
			}
			n.add(v)
		}
	default:
		n = newScalar(yamlScalar(y))
	}
	n.line = y.Line
	return n, nil
}

func yamlScalar(y *yaml.Node) any {
	switch y.ShortTag() {
	case "!!null":
		return nil
	case "!!bool":
		var v bool
		if y.Decode(&v) == nil {
			return v
		}
	case "!!int", "!!float":
		return json.Number(y.Value)
	}
	return y.Value
}

func decodeXML(data []byte, lim Limits, b *budget) (*Node, error) {
	root, err := safexml.Decode(data, safexml.Limits{
		MaxBytes: lim.MaxBytes, MaxDepth: maxDepth, MaxElements: lim.MaxNodes, MaxAttrs: maxXMLAttrs,
	})
	if err != nil {
		if le, ok := errors.AsType[*safexml.LimitError](err); ok {
			return nil, &LimitError{Detail: le.Detail}
		}
		return nil, fmt.Errorf("xml: %w", err)
	}
	doc := newObject()
	el, err := xmlElement(root, b)
	if err != nil {
		return nil, err
	}
	arr := newArray()
	arr.add(el)
	doc.set(root.Name, arr)
	return doc, nil
}

func xmlElement(x *safexml.Node, b *budget) (*Node, error) {
	if err := b.take(); err != nil {
		return nil, err
	}
	n := newObject()
	for _, a := range x.Attrs {
		if err := b.take(); err != nil {
			return nil, err
		}
		n.set("@"+a.Name, newScalar(a.Value))
	}
	for _, c := range x.Children {
		child, err := xmlElement(c, b)
		if err != nil {
			return nil, err
		}
		arr := n.get(c.Name)
		if arr == nil {
			arr = newArray()
			n.set(c.Name, arr)
		}
		arr.add(child)
	}
	n.xml = x
	return n, nil
}

// innerText is all character data inside x in document order, with runs of
// whitespace collapsed to one space, so mixed content reads as prose.
func innerText(x *safexml.Node) string {
	var b strings.Builder
	var walk func(*safexml.Node)
	walk = func(n *safexml.Node) {
		b.WriteString(n.Text)
		for _, c := range n.Children {
			walk(c)
			b.WriteString(c.Tail)
		}
	}
	walk(x)
	return strings.Join(strings.Fields(b.String()), " ")
}

func decodeCSV(data []byte, b *budget) (*Node, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.ReuseRecord = false
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("csv: header: %w", err)
	}
	seen := map[string]bool{}
	for i, h := range header {
		h = strings.TrimSpace(h)
		if h == "" || seen[h] {
			return nil, fmt.Errorf("csv: header column %d is empty or repeated", i+1)
		}
		seen[h] = true
		header[i] = h
	}
	root := newArray()
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csv: %w", err)
		}
		if err := b.take(); err != nil {
			return nil, err
		}
		line, _ := r.FieldPos(0)
		row := newObject()
		row.line = line
		for i, v := range rec {
			if err := b.take(); err != nil {
				return nil, err
			}
			row.set(header[i], newScalar(v))
		}
		root.add(row)
	}
	return root, nil
}
