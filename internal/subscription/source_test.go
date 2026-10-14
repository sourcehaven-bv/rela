package subscription

import (
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/feed"
	"github.com/Sourcehaven-BV/rela/internal/htmlmd"
)

func TestItemSource_Resolve(t *testing.T) {
	published := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	src := &itemSource{
		name: "news",
		feed: &feed.Feed{Title: "Feed", Link: "javascript:alert(1)"},
		item: feed.Item{
			ID: "g1", Title: "Title", Link: "/p/1",
			Summary:   `<h2>Head</h2><p>Hi <script>alert(1)</script><a href="javascript:x">l</a></p>`,
			Published: published, Authors: []string{"A", "B"}, Categories: []string{"x"},
		},
		base: mustURL(t, "https://example.com/feed.xml"),
		now:  time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC),
		warn: discardWarn,
	}
	for ref, want := range map[string]string{
		"item.id":             "g1",
		"item.title":          "Title",
		"item.link":           "https://example.com/p/1",
		"item.published":      "2026-09-28T10:00:00Z",
		"item.published_date": "2026-09-28",
		"item.updated":        "",
		"item.content":        "",
		"item.authors":        "A, B",
		"item.categories":     "x",
		"feed.title":          "Feed",
		"feed.link":           "",
		"subscription.name":   "news",
		"now":                 "2026-10-14T12:00:00Z",
		"today":               "2026-10-14",
		"item.bogus":          "",
	} {
		if got := src.Resolve(ref); got != want {
			t.Errorf("Resolve(%s) = %q, want %q", ref, got, want)
		}
	}

	summary := src.Resolve("item.summary")
	if strings.Contains(summary, "script") || strings.Contains(summary, "javascript") || strings.Contains(summary, "<") {
		t.Errorf("summary %q still carries markup", summary)
	}
	if !strings.Contains(summary, "## Head") {
		t.Errorf("summary %q, want the heading converted", summary)
	}
	if src.DefaultMatchRef("id") != "item.id" || !src.IsBlock("item.summary") || src.IsBlock("item.title") {
		t.Error("DefaultMatchRef or IsBlock wrong")
	}
}

func TestItemSource_HTMLFailureIsEmpty(t *testing.T) {
	var warned bool
	src := &itemSource{
		item: feed.Item{Content: strings.Repeat("a", htmlmd.MaxInput+1)},
		warn: func(string, ...any) { warned = true },
	}
	if got := src.Resolve("item.content"); got != "" || !warned {
		t.Errorf("Resolve = %d bytes, warned %v; want empty with a warning", len(got), warned)
	}
}

func TestItemSource_Links(t *testing.T) {
	src := &itemSource{}
	for in, want := range map[string]string{
		"":                       "",
		"https://a.example/x":    "https://a.example/x",
		"/relative":              "",
		"data:text/html,x":       "",
		"mailto:a@example.com":   "",
		"http://[::1":            "",
		"\thttp://a.example/y\n": "http://a.example/y",
	} {
		if got := src.link(in); got != want {
			t.Errorf("link(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSeenSet(t *testing.T) {
	s := newSeenSet([]seenEntry{{ID: "a", Content: "1"}, {ID: "b", Content: "1"}, {ID: "a", Content: "2"}})
	if !s.unchanged("a", "2") || s.unchanged("a", "1") || s.unchanged("c", "1") {
		t.Error("unchanged wrong after load")
	}
	s.mark("c", "1")
	s.touch("b")
	got := s.entries(2)
	if len(got) != 2 || got[0].ID != "c" || got[1].ID != "b" {
		t.Errorf("entries(2) = %v, want the two most recently seen", got)
	}
}

func TestNewestOrdersByDate(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	items := []feed.Item{
		{ID: "old", Published: day(1)},
		{ID: "undated"},
		{ID: "new", Published: day(3)},
		{ID: "mid", Updated: day(2)},
	}
	got, dropped := newest(items, 3)
	ids := []string{got[0].ID, got[1].ID, got[2].ID}
	if dropped != 1 || strings.Join(ids, ",") != "old,mid,new" {
		t.Errorf("newest = %v, dropped %d", ids, dropped)
	}
}
