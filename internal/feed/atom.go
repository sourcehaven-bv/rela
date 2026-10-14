package feed

import (
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/safexml"
)

func atom(n *safexml.Node) bool { return n.Space == nsAtom }

// parseAtom reads an Atom 1.0 <feed>.
func parseAtom(root *safexml.Node) (Feed, error) {
	f := Feed{
		Format:      FormatAtom,
		Title:       textOf(child(root, "title", atom)),
		Link:        atomLink(root),
		Description: textOf(child(root, "subtitle", atom)),
	}
	for _, c := range root.Children {
		if c.Name == "entry" && atom(c) {
			if len(f.Items) == MaxItems {
				return Feed{}, errTooManyItems()
			}
			f.Items = append(f.Items, atomEntry(c))
		}
	}
	return f, nil
}

func atomEntry(n *safexml.Node) Item {
	it := Item{
		Title:        textOf(child(n, "title", atom)),
		Link:         atomLink(n),
		Summary:      atomText(child(n, "summary", atom)),
		Content:      atomText(child(n, "content", atom)),
		PublishedRaw: textOf(child(n, "published", atom)),
		UpdatedRaw:   textOf(child(n, "updated", atom)),
	}
	it.ID = firstNonEmpty(textOf(child(n, "id", atom)), it.Link)
	it.Published = parseDate(it.PublishedRaw)
	it.Updated = parseDate(it.UpdatedRaw)

	for _, c := range n.Children {
		if !atom(c) {
			continue
		}
		switch c.Name {
		case "author":
			name := firstNonEmpty(textOf(child(c, "name", atom)), textOf(child(c, "email", atom)))
			if name != "" {
				it.Authors = append(it.Authors, name)
			}
		case "category":
			if term := strings.TrimSpace(attr(c, "term")); term != "" {
				it.Categories = append(it.Categories, term)
			}
		}
	}
	return it
}

// atomLink returns the alternate link of a feed or entry: rel="alternate",
// or no rel at all, which Atom defines to mean the same.
func atomLink(n *safexml.Node) string {
	for _, c := range n.Children {
		if c.Name != "link" || !atom(c) {
			continue
		}
		if rel := attr(c, "rel"); rel == "" || rel == "alternate" {
			return strings.TrimSpace(attr(c, "href"))
		}
	}
	return ""
}

// atomText converts an Atom text construct to HTML according to its type.
// Out-of-line content (src=) and non-text media types yield nothing: the
// first is a link the consumer can follow itself, and the second is not
// something this package can render.
func atomText(n *safexml.Node) string {
	if n == nil || attr(n, "src") != "" {
		return ""
	}
	switch typ := strings.ToLower(strings.TrimSpace(attr(n, "type"))); {
	case typ == "html", typ == "text/html":
		return htmlField(n)
	case typ == "xhtml", typ == "application/xhtml+xml":
		return xhtmlBody(n)
	case typ == "", typ == "text", strings.HasPrefix(typ, "text/"):
		return textToHTML(textOf(n))
	default:
		return ""
	}
}

// xhtmlBody rebuilds the markup inside an Atom xhtml construct. Atom wraps it
// in one xhtml <div> that is not part of the content.
func xhtmlBody(n *safexml.Node) string {
	if len(n.Children) == 1 && n.Children[0].Name == "div" {
		return innerMarkup(n.Children[0])
	}
	return innerMarkup(n)
}
