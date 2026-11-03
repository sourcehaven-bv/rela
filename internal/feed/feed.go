// Package feed parses syndication feeds (RSS 2.0 and 0.9x, RSS 1.0, Atom 1.0
// and JSON Feed 1.x) into one normalized shape.
//
// XML input goes through [safexml], so every feed is decoded under the same
// size, depth and element limits and the same charset allowlist; JSON input
// gets the same size cap. The package does no I/O.
//
// Normalization rules a consumer can rely on:
//
//   - Summary and Content are always HTML. Plain-text sources are escaped;
//     an Atom xhtml body is rebuilt as markup. Either is untrusted HTML and
//     must be sanitized or converted before it is rendered.
//   - Title is the text as given, never markup-escaped.
//   - Item.ID is the source's own identifier, falling back to the item link.
//     When neither exists it is empty: an identity is never invented, so a
//     consumer that needs one has to decide what to do.
//   - Dates are in UTC. A date without a zone is read as UTC; a date that
//     does not parse leaves the time zero and keeps the source text in the
//     matching Raw field.
package feed

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/safexml"
)

// Format names the syndication format a feed was parsed from.
type Format string

// The formats Parse recognizes.
const (
	FormatRSS  Format = "rss"  // RSS 2.0 and 0.9x (<rss>)
	FormatRDF  Format = "rdf"  // RSS 1.0 and 0.90 (<rdf:RDF>)
	FormatAtom Format = "atom" // Atom 1.0 (<feed>)
	FormatJSON Format = "json" // JSON Feed 1.0 and 1.1
)

// Feed is a parsed feed.
type Feed struct {
	Format      Format
	Title       string
	Link        string
	Description string
	Items       []Item
}

// Item is one entry of a feed. See the package doc for the normalization
// rules.
type Item struct {
	ID           string
	Title        string
	Link         string
	Summary      string // HTML
	Content      string // HTML
	Published    time.Time
	Updated      time.Time
	PublishedRaw string // source text of Published, also when it did not parse
	UpdatedRaw   string // source text of Updated, also when it did not parse
	Authors      []string
	Categories   []string
}

// MaxItems caps the items in one feed, in every format. The XML element cap
// already bounds an XML feed, but a JSON Feed has no element count, and
// 10 MiB of "{}," is millions of items; the cap keeps both formats to the
// same order of memory. Real feeds carry tens to hundreds of items.
const MaxItems = 10_000

// errTooManyItems is returned once a feed passes [MaxItems].
func errTooManyItems() error {
	return &safexml.LimitError{Limit: "items", Detail: fmt.Sprintf("more than %d items", MaxItems)}
}

// Parse detects the format of data and parses it. Input over
// safexml.DefaultLimits.MaxBytes is refused with a *safexml.LimitError, as is
// XML that exceeds any other limit and a feed with more than [MaxItems] items.
func Parse(data []byte) (Feed, error) {
	if len(data) > safexml.DefaultLimits.MaxBytes {
		return Feed{}, &safexml.LimitError{Limit: "bytes", Detail: fmt.Sprintf(
			"input is %d bytes, limit is %d", len(data), safexml.DefaultLimits.MaxBytes)}
	}
	data = safexml.TrimBOM(data)
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return Feed{}, errors.New("feed: empty input")
	}
	if trimmed[0] == '{' {
		return parseJSON(trimmed)
	}

	root, err := safexml.Decode(data, safexml.DefaultLimits)
	if err != nil {
		return Feed{}, err
	}
	switch {
	case root.Name == "rss" && root.Space == "":
		return parseRSS(root)
	case root.Name == "RDF" && isSpace(root, nsRDF, "rdf"):
		return parseRDF(root)
	case root.Name == "feed" && isSpace(root, nsAtom, ""):
		return parseAtom(root)
	}
	return Feed{}, fmt.Errorf("feed: unrecognized root element <%s> (namespace %q)", root.Name, root.Space)
}

// Namespace URIs the parsers match on.
const (
	nsAtom    = "http://www.w3.org/2005/Atom"
	nsRDF     = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	nsRSS10   = "http://purl.org/rss/1.0/"
	nsRSS090  = "http://my.netscape.com/rdf/simple/0.9/"
	nsDC      = "http://purl.org/dc/elements/1.1/"
	nsContent = "http://purl.org/rss/1.0/modules/content/"
)

// isSpace reports whether n is in namespace uri. A feed that uses a
// conventional prefix without declaring it leaves the bare prefix in Space
// (see [safexml.Attr]); prefix names the one to accept in that case, so a
// sloppy feed's dc:creator is still found.
func isSpace(n *safexml.Node, uri, prefix string) bool {
	return n.Space == uri || (prefix != "" && n.Space == prefix)
}

// child returns the first child of n named name for which match holds.
func child(n *safexml.Node, name string, match func(*safexml.Node) bool) *safexml.Node {
	for _, c := range n.Children {
		if c.Name == name && match(c) {
			return c
		}
	}
	return nil
}

// textOf returns all character data inside n, children included, trimmed.
// A nil node has no text.
func textOf(n *safexml.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	appendText(&b, n)
	return strings.TrimSpace(b.String())
}

func appendText(b *strings.Builder, n *safexml.Node) {
	b.WriteString(n.Text)
	for _, c := range n.Children {
		appendText(b, c)
		b.WriteString(c.Tail)
	}
}

// attr returns the value of the unqualified attribute name on n.
func attr(n *safexml.Node, name string) string {
	for _, a := range n.Attrs {
		if a.Name == name && a.Space == "" {
			return a.Value
		}
	}
	return ""
}

// firstNonEmpty returns the first argument that is not empty.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
