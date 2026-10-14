package feed

import (
	"errors"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/safexml"
)

// core matches an RSS core element: unqualified in RSS 2.0, in the RSS 1.0 or
// 0.90 namespace inside an RDF document.
func core(n *safexml.Node) bool {
	return n.Space == "" || n.Space == nsRSS10 || n.Space == nsRSS090
}

func dc(n *safexml.Node) bool      { return isSpace(n, nsDC, "dc") }
func content(n *safexml.Node) bool { return isSpace(n, nsContent, "content") }

// parseRSS reads <rss><channel>...<item>...</item></channel></rss>.
func parseRSS(root *safexml.Node) (Feed, error) {
	ch := child(root, "channel", core)
	if ch == nil {
		return Feed{}, errors.New("feed: <rss> has no <channel>")
	}
	f := channelFeed(FormatRSS, ch)
	for _, c := range ch.Children {
		if c.Name == "item" && core(c) {
			if len(f.Items) == MaxItems {
				return Feed{}, errTooManyItems()
			}
			f.Items = append(f.Items, rssItem(c, ""))
		}
	}
	return f, nil
}

// parseRDF reads RSS 1.0 and 0.90, where items are siblings of the channel
// rather than its children.
func parseRDF(root *safexml.Node) (Feed, error) {
	ch := child(root, "channel", core)
	if ch == nil {
		return Feed{}, errors.New("feed: <rdf:RDF> has no <channel>")
	}
	f := channelFeed(FormatRDF, ch)
	for _, c := range root.Children {
		if c.Name == "item" && core(c) {
			if len(f.Items) == MaxItems {
				return Feed{}, errTooManyItems()
			}
			f.Items = append(f.Items, rssItem(c, rdfAbout(c)))
		}
	}
	return f, nil
}

func channelFeed(format Format, ch *safexml.Node) Feed {
	return Feed{
		Format:      format,
		Title:       textOf(child(ch, "title", core)),
		Link:        textOf(child(ch, "link", core)),
		Description: textOf(child(ch, "description", core)),
	}
}

// rssItem maps one <item>. about is the RDF identifier, used as the ID when
// the item carries no <guid>.
func rssItem(n *safexml.Node, about string) Item {
	it := Item{
		Title:   textOf(child(n, "title", core)),
		Link:    textOf(child(n, "link", core)),
		Summary: htmlField(child(n, "description", core)),
		Content: htmlField(child(n, "encoded", content)),
	}
	it.ID = firstNonEmpty(textOf(child(n, "guid", core)), about, it.Link)
	it.PublishedRaw = firstNonEmpty(textOf(child(n, "pubDate", core)), textOf(child(n, "date", dc)))
	it.Published = parseDate(it.PublishedRaw)

	for _, c := range n.Children {
		switch {
		case c.Name == "author" && core(c), c.Name == "creator" && dc(c):
			if s := textOf(c); s != "" {
				it.Authors = append(it.Authors, s)
			}
		case c.Name == "category" && core(c), c.Name == "subject" && dc(c):
			if s := textOf(c); s != "" {
				it.Categories = append(it.Categories, s)
			}
		}
	}
	return it
}

func rdfAbout(n *safexml.Node) string {
	for _, a := range n.Attrs {
		if a.Name == "about" && (a.Space == nsRDF || a.Space == "rdf") {
			return strings.TrimSpace(a.Value)
		}
	}
	return ""
}
