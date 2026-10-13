// Package safexml decodes untrusted XML into a plain element tree under hard
// resource limits.
//
// It builds on encoding/xml, which already closes the classic XML attack
// classes: it never processes a DTD or resolves an external entity (no XXE),
// and it expands only the predefined entities plus the fixed map in
// Decoder.Entity (no entity-expansion bombs). A <!DOCTYPE> arrives as an
// opaque directive token, which Decode skips; an entity the document declares
// there is still unknown to the decoder, so referencing it is a syntax error.
//
// What encoding/xml leaves to the caller, this package adds:
//
//   - a size cap, checked before decoding starts;
//   - caps on nesting depth, element count and attributes per element, so a
//     small document cannot fan out into a huge tree;
//   - a charset allowlist (UTF-8, US-ASCII, ISO-8859-1 / windows-1252).
//
// The tree follows the ElementTree model: an element's Text is the character
// data before its first child, and each child's Tail is the character data
// after it. Together they keep mixed content in document order, so markup
// such as an Atom xhtml body can be rebuilt from the tree.
package safexml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"maps"
	"strings"
	"unicode/utf8"
)

// Limits bounds the work a single Decode may do.
type Limits struct {
	MaxBytes    int // size of the input
	MaxDepth    int // nesting depth; the root element is depth 1
	MaxElements int // elements in the whole document
	MaxAttrs    int // attributes on one element, namespace declarations included
}

// DefaultLimits suits feeds and API payloads: the byte cap matches the
// response cap of the Lua http module, so anything a script can fetch can be
// decoded.
var DefaultLimits = Limits{
	MaxBytes:    10 * 1024 * 1024,
	MaxDepth:    64,
	MaxElements: 100_000,
	MaxAttrs:    64,
}

// Attr is one attribute. Space is the namespace URI the prefix resolved to;
// for a prefix the document never declared, encoding/xml leaves the prefix
// itself there.
type Attr struct {
	Name  string
	Space string
	Value string
}

// Node is one element. Space follows the same rule as [Attr.Space].
type Node struct {
	Name     string
	Space    string
	Attrs    []Attr // namespace declarations (xmlns, xmlns:p) are not included
	Children []*Node
	Text     string // character data before the first child
	Tail     string // character data after this element, before the next sibling
}

// LimitError reports input refused because it exceeds a [Limits] field or uses
// a charset outside the allowlist. Malformed XML is reported with a plain
// error instead, so a caller can tell hostile or oversize input apart.
type LimitError struct {
	Limit  string // "bytes", "depth", "elements", "attributes" or "charset"
	Detail string
}

func (e *LimitError) Error() string { return "xml limit exceeded: " + e.Detail }

// htmlEntities is a private copy of xml.HTMLEntity: the exported map is a
// mutable global, and the decoder must not change behaviour if another
// package writes to it. It holds only single-character replacements, so it
// cannot recurse or amplify input. Feeds use &nbsp; and friends freely, and
// without the map Strict mode rejects them.
var htmlEntities = maps.Clone(xml.HTMLEntity)

// Decode parses data into its root element.
//
// Every field of lim must be positive; a zero field is an error rather than
// "unlimited", because a forgotten limit is the bug this package prevents.
func Decode(data []byte, lim Limits) (*Node, error) {
	if lim.MaxBytes <= 0 || lim.MaxDepth <= 0 || lim.MaxElements <= 0 || lim.MaxAttrs <= 0 {
		return nil, fmt.Errorf("safexml: every limit must be positive: %+v", lim)
	}
	if len(data) > lim.MaxBytes {
		return nil, &LimitError{Limit: "bytes",
			Detail: fmt.Sprintf("input is %d bytes, limit is %d", len(data), lim.MaxBytes)}
	}

	d := xml.NewDecoder(bytes.NewReader(TrimBOM(data)))
	d.Strict = true
	d.Entity = htmlEntities
	d.CharsetReader = charsetReader
	return decodeTree(d, lim)
}

// TrimBOM drops a leading UTF-8 byte order mark. encoding/xml would hand it
// back as character data, and a format sniffer looking at the first byte
// would see it instead of the document.
func TrimBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
}

// builder carries the decode state for one document.
type builder struct {
	lim      Limits
	root     *Node
	stack    []*Node
	last     *Node // most recently closed child of the top of stack, for Tail
	text     []byte
	elements int
}

func decodeTree(d *xml.Decoder, lim Limits) (*Node, error) {
	b := &builder{lim: lim}
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if le, ok := errors.AsType[*LimitError](err); ok {
				return nil, le
			}
			line, _ := d.InputPos()
			return nil, fmt.Errorf("safexml: line %d: %w", line, err)
		}
		if err := b.token(tok); err != nil {
			return nil, err
		}
	}
	if b.root == nil {
		return nil, errors.New("safexml: no root element")
	}
	return b.root, nil
}

func (b *builder) token(tok xml.Token) error {
	switch t := tok.(type) {
	case xml.StartElement:
		return b.start(t)
	case xml.EndElement:
		b.flushText()
		top := len(b.stack) - 1
		b.last = b.stack[top]
		b.stack = b.stack[:top]
	case xml.CharData:
		if len(b.stack) > 0 {
			b.text = append(b.text, t...)
		}
	case xml.Directive, xml.Comment, xml.ProcInst:
		// Directives are skipped, not processed (see the package doc). The
		// only ProcInst encoding/xml acts on, the XML declaration, has
		// already been handled through CharsetReader.
	}
	return nil
}

func (b *builder) start(t xml.StartElement) error {
	if b.root != nil && len(b.stack) == 0 {
		return errors.New("safexml: content after the root element")
	}
	b.elements++
	if b.elements > b.lim.MaxElements {
		return &LimitError{Limit: "elements", Detail: fmt.Sprintf("more than %d elements", b.lim.MaxElements)}
	}
	if len(b.stack)+1 > b.lim.MaxDepth {
		return &LimitError{Limit: "depth", Detail: fmt.Sprintf("nesting deeper than %d", b.lim.MaxDepth)}
	}
	if len(t.Attr) > b.lim.MaxAttrs {
		return &LimitError{Limit: "attributes", Detail: fmt.Sprintf(
			"element <%s> has %d attributes, limit is %d", t.Name.Local, len(t.Attr), b.lim.MaxAttrs)}
	}
	b.flushText()
	n := &Node{Name: t.Name.Local, Space: t.Name.Space, Attrs: attrs(t.Attr)}
	if len(b.stack) == 0 {
		b.root = n
	} else {
		parent := b.stack[len(b.stack)-1]
		parent.Children = append(parent.Children, n)
	}
	b.stack = append(b.stack, n)
	b.last = nil
	return nil
}

// flushText assigns the character data collected since the last tag: to the
// Tail of the element that just closed, or else to the Text of the open one.
func (b *builder) flushText() {
	if len(b.text) == 0 {
		return
	}
	s := string(b.text)
	b.text = b.text[:0]
	if b.last != nil {
		b.last.Tail += s
		return
	}
	if len(b.stack) > 0 {
		b.stack[len(b.stack)-1].Text += s
	}
}

func attrs(in []xml.Attr) []Attr {
	var out []Attr
	for _, a := range in {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		out = append(out, Attr{Name: a.Name.Local, Space: a.Name.Space, Value: a.Value})
	}
	return out
}

// charsetReader is consulted for every declared encoding other than UTF-8.
// ISO-8859-1 is decoded as windows-1252, as browsers do, because feeds that
// claim latin-1 are routinely windows-1252 in practice and the two agree on
// every printable latin-1 character. Anything else is refused: decoding it
// would need a table this package does not carry, and guessing produces
// silently wrong text.
func charsetReader(label string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "us-ascii", "ascii":
		return input, nil
	case "iso-8859-1", "iso8859-1", "latin1", "latin-1", "l1", "windows-1252", "cp1252":
		return &cp1252Reader{r: input}, nil
	}
	return nil, &LimitError{Limit: "charset", Detail: fmt.Sprintf("unsupported charset %q", label)}
}

// cp1252High maps bytes 0x80-0x9F to their windows-1252 code points. The five
// bytes windows-1252 leaves undefined map to their C1 control code points, as
// the WHATWG encoding standard specifies.
var cp1252High = [32]rune{
	0x20AC, 0x0081, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x008D, 0x017D, 0x008F,
	0x0090, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x009D, 0x017E, 0x0178,
}

// cp1252Reader transcodes windows-1252 to UTF-8. One input byte becomes at
// most three output bytes, so the size cap on the input bounds the output.
type cp1252Reader struct {
	r       io.Reader
	pending []byte
	buf     [512]byte
}

func (c *cp1252Reader) Read(p []byte) (int, error) {
	if len(c.pending) == 0 {
		n, err := c.r.Read(c.buf[:])
		if n == 0 {
			return 0, err
		}
		out := make([]byte, 0, 3*n)
		for _, b := range c.buf[:n] {
			r := rune(b)
			if b >= 0x80 && b <= 0x9F {
				r = cp1252High[b-0x80]
			}
			out = utf8.AppendRune(out, r)
		}
		c.pending = out
	}
	n := copy(p, c.pending)
	c.pending = c.pending[n:]
	return n, nil
}
