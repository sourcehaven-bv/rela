package subscription

import (
	"net/url"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/feed"
	"github.com/Sourcehaven-BV/rela/internal/htmlmd"
)

// itemSource resolves a mapping's references for one feed item. It implements
// ingest.Source.
type itemSource struct {
	name     string
	feed     *feed.Feed
	item     feed.Item
	base     *url.URL // the final feed URL, for relative links
	now      time.Time
	markdown map[string]string // converted HTML, per reference
	warn     func(msg string, args ...any)
}

// Resolve returns the raw value of ref; ingest flattens it. The values are
// still untrusted: HTML is converted to sanitized markdown here, and a link
// survives only as an absolute http or https URL.
func (s *itemSource) Resolve(ref string) string {
	switch ref {
	case "item.id":
		return s.item.ID
	case "item.title":
		return s.item.Title
	case "item.link":
		return s.link(s.item.Link)
	case "item.summary":
		return s.html(ref, s.item.Summary)
	case "item.content":
		return s.html(ref, s.item.Content)
	case "item.published":
		return timestamp(s.item.Published)
	case "item.published_date":
		if s.item.Published.IsZero() {
			return ""
		}
		return s.item.Published.Format(time.DateOnly)
	case "item.updated":
		return timestamp(s.item.Updated)
	case "item.authors":
		return strings.Join(s.item.Authors, ", ")
	case "item.categories":
		return strings.Join(s.item.Categories, ", ")
	case "feed.title":
		return s.feed.Title
	case "feed.link":
		return s.link(s.feed.Link)
	case "subscription.name":
		return s.name
	case "now":
		return s.now.Format(time.RFC3339)
	case "today":
		return s.now.Format(time.DateOnly)
	}
	return ""
}

// DefaultMatchRef reads the item field named like the match property.
func (s *itemSource) DefaultMatchRef(prop string) string {
	return "item." + prop
}

// IsBlock reports the HTML fields, which resolve to sanitized markdown.
func (s *itemSource) IsBlock(ref string) bool {
	return blockRefs[ref]
}

// html converts untrusted item HTML to markdown once per reference. A
// conversion failure (input over htmlmd.MaxInput, or markup that does not
// settle) yields an empty value rather than failing the item: the same input
// would fail every later poll too.
func (s *itemSource) html(ref, in string) string {
	if md, ok := s.markdown[ref]; ok {
		return md
	}
	md, _, err := htmlmd.FromHTML(in)
	if err != nil {
		s.warn("item HTML not imported", "subscription", s.name, "field", ref, "error", err)
		md = ""
	}
	if s.markdown == nil {
		s.markdown = make(map[string]string, len(blockRefs))
	}
	s.markdown[ref] = md
	return md
}

// link resolves raw against the feed URL and keeps it only when the result
// is an absolute http or https URL, so a javascript: or data: link never
// reaches a property that is rendered as a link.
func (s *itemSource) link(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if s.base != nil {
		u = s.base.ResolveReference(u)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.String()
}

func timestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
