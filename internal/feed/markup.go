package feed

import (
	"html"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/safexml"
)

// nsXML is the namespace of the reserved xml: prefix (xml:lang, xml:base).
const nsXML = "http://www.w3.org/XML/1998/namespace"

// htmlField returns the HTML of an element whose content is HTML: escaped
// HTML text as most feeds send it, or child elements when a feed embeds
// well-formed markup directly.
func htmlField(n *safexml.Node) string {
	if n == nil {
		return ""
	}
	if len(n.Children) == 0 {
		return strings.TrimSpace(n.Text)
	}
	return innerMarkup(n)
}

// innerMarkup serializes the content of n, without n's own tags, as HTML.
func innerMarkup(n *safexml.Node) string {
	var b strings.Builder
	b.WriteString(html.EscapeString(n.Text))
	for _, c := range n.Children {
		writeMarkup(&b, c)
		b.WriteString(html.EscapeString(c.Tail))
	}
	return strings.TrimSpace(b.String())
}

// voidElements are written without a closing tag when empty: in HTML
// "<br></br>" is two line breaks.
var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true,
	"img": true, "input": true, "link": true, "meta": true, "source": true, "track": true, "wbr": true,
}

// writeMarkup serializes one element and its subtree as HTML. An xml:
// attribute keeps its prefix; other namespaced attributes are dropped, since
// HTML has no way to express them and keeping only the local name could
// collide with a real attribute. The result is untrusted HTML for the
// consumer to sanitize.
func writeMarkup(b *strings.Builder, n *safexml.Node) {
	b.WriteString("<" + n.Name)
	for _, a := range n.Attrs {
		name := a.Name
		switch a.Space {
		case "":
		case nsXML:
			name = "xml:" + a.Name
		default:
			continue
		}
		b.WriteString(" " + name + `="` + html.EscapeString(a.Value) + `"`)
	}
	b.WriteString(">")
	if voidElements[n.Name] && n.Text == "" && len(n.Children) == 0 {
		return
	}
	b.WriteString(html.EscapeString(n.Text))
	for _, c := range n.Children {
		writeMarkup(b, c)
		b.WriteString(html.EscapeString(c.Tail))
	}
	b.WriteString("</" + n.Name + ">")
}

// textToHTML escapes plain text so it can stand in an HTML field.
func textToHTML(s string) string {
	return html.EscapeString(strings.TrimSpace(s))
}
