package feed

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/safexml"
)

func parseFixture(t *testing.T, name string) Feed {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse(%s): %v", name, err)
	}
	return f
}

func utc(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

// checkItem compares the fields of got that want sets; a zero field in want
// is not checked, except ID, which is always compared.
func checkItem(t *testing.T, got, want Item) {
	t.Helper()
	if got.ID != want.ID {
		t.Errorf("ID = %q, want %q", got.ID, want.ID)
	}
	str := []struct{ name, got, want string }{
		{"Title", got.Title, want.Title},
		{"Link", got.Link, want.Link},
		{"Summary", got.Summary, want.Summary},
		{"Content", got.Content, want.Content},
		{"PublishedRaw", got.PublishedRaw, want.PublishedRaw},
	}
	for _, s := range str {
		if s.want != "" && s.got != s.want {
			t.Errorf("%s = %q, want %q", s.name, s.got, s.want)
		}
	}
	if !want.Published.IsZero() && !got.Published.Equal(want.Published) {
		t.Errorf("Published = %v, want %v", got.Published, want.Published)
	}
	if !want.Updated.IsZero() && !got.Updated.Equal(want.Updated) {
		t.Errorf("Updated = %v, want %v", got.Updated, want.Updated)
	}
	if want.Authors != nil && !slices.Equal(got.Authors, want.Authors) {
		t.Errorf("Authors = %q, want %q", got.Authors, want.Authors)
	}
	if want.Categories != nil && !slices.Equal(got.Categories, want.Categories) {
		t.Errorf("Categories = %q, want %q", got.Categories, want.Categories)
	}
}

func TestParse_RSS2(t *testing.T) {
	f := parseFixture(t, "rss2.xml")
	if f.Format != FormatRSS || f.Title != "Example & Co" || f.Link != "https://example.org/" || f.Description != "News" {
		t.Errorf("feed = %+v", f)
	}
	if len(f.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(f.Items))
	}
	checkItem(t, f.Items[0], Item{
		ID: "urn:post:1", Title: "First post", Link: "https://example.org/1",
		Summary: "<p>Short</p>", Content: "<p>Full <b>body</b></p>",
		// EST is UTC-5; time.Parse alone would read it as UTC.
		Published: utc("2026-10-13T14:30:00Z"),
		// dc:creator without an xmlns:dc declaration is still found.
		Authors:    []string{"a@example.org (Ann)", "Bob"},
		Categories: []string{"go", "xml"},
	})
	second := f.Items[1]
	checkItem(t, second, Item{ID: "https://example.org/2", PublishedRaw: "not a date"})
	if !second.Published.IsZero() {
		t.Errorf("unparseable date gave %v, want zero", second.Published)
	}
	checkItem(t, f.Items[2], Item{ID: ""})
}

func TestParse_RDF(t *testing.T) {
	f := parseFixture(t, "rdf.xml")
	if f.Format != FormatRDF || f.Title != "RDF feed" || len(f.Items) != 1 {
		t.Fatalf("feed = %+v", f)
	}
	checkItem(t, f.Items[0], Item{
		ID: "https://example.org/a", Title: "A",
		Published:  utc("2026-10-13T08:00:00Z"),
		Categories: []string{"misc"},
	})
}

func TestParse_Atom(t *testing.T) {
	f := parseFixture(t, "atom.xml")
	if f.Format != FormatAtom || f.Title != "Atom feed" || f.Description != "All the news" {
		t.Errorf("feed = %+v", f)
	}
	if f.Link != "https://example.org/" {
		t.Errorf("feed link = %q, want the alternate link, not rel=self", f.Link)
	}
	if len(f.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(f.Items))
	}
	checkItem(t, f.Items[0], Item{
		ID: "tag:example.org,2026:1", Title: "XHTML entry", Link: "https://example.org/e/1",
		Summary:    "1 &lt; 2 &amp; more",
		Content:    `Hi <b>there</b> &amp; <a href="https://x.test/?a=1&amp;b=2">link</a><br>end`,
		Published:  utc("2026-10-13T08:00:00Z"),
		Updated:    utc("2026-10-13T09:00:00.5Z"),
		Authors:    []string{"Carol", "dan@example.org"},
		Categories: []string{"news"},
	})
	checkItem(t, f.Items[1], Item{
		ID: "tag:example.org,2026:2", Content: "<p>escaped html</p>", Updated: utc("2026-10-13T00:00:00Z"),
	})
	if f.Items[2].Content != "" {
		t.Errorf("out-of-line content = %q, want empty", f.Items[2].Content)
	}
}

func TestParse_JSONFeed(t *testing.T) {
	f := parseFixture(t, "jsonfeed.json")
	if f.Format != FormatJSON || f.Title != "JSON feed" || f.Link != "https://example.org/" || len(f.Items) != 3 {
		t.Fatalf("feed = %+v", f)
	}
	checkItem(t, f.Items[0], Item{
		ID: "1", Link: "https://example.org/j1", Content: "<p>html</p>", Summary: "a &lt; b",
		Published: utc("2026-10-13T08:00:00Z"), Authors: []string{"Eve"}, Categories: []string{"x", "y"},
	})
	checkItem(t, f.Items[1], Item{ID: "42", Content: "plain &amp; simple", Authors: []string{"Old Style"}})
	checkItem(t, f.Items[2], Item{ID: "https://example.org/j3"})
}

func TestParse_BOM(t *testing.T) {
	for _, doc := range []string{
		"\xef\xbb\xbf" + `{"version":"https://jsonfeed.org/version/1","items":[]}`,
		"\xef\xbb\xbf" + `<rss><channel><title>t</title></channel></rss>`,
	} {
		if _, err := Parse([]byte(doc)); err != nil {
			t.Errorf("Parse with BOM: %v", err)
		}
	}
}

func TestParse_Errors(t *testing.T) {
	tests := []struct {
		name  string
		doc   string
		msg   string
		limit string
	}{
		{"empty", "", "empty input", ""},
		{"whitespace", " \n\t", "empty input", ""},
		{"unknown root", "<html><body/></html>", "unrecognized root element <html>", ""},
		{"atom root in wrong namespace", "<feed/>", "unrecognized root", ""},
		{"rss without channel", "<rss/>", "no <channel>", ""},
		{"rdf without channel", `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"/>`, "no <channel>", ""},
		{"malformed xml", "<rss><channel>", "safexml", ""},
		{"invalid json", "{nope", "invalid JSON", ""},
		{"json but not a feed", `{"version":"2"}`, "not a JSON Feed", ""},
		{"too deep", "<rss>" + strings.Repeat("<a>", 70) + strings.Repeat("</a>", 70) + "</rss>", "deeper", "depth"},
		{"oversize json", "{" + strings.Repeat(" ", safexml.DefaultLimits.MaxBytes) + "}", "bytes", "bytes"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.doc))
			if err == nil {
				t.Fatal("Parse succeeded, want error")
			}
			if !strings.Contains(err.Error(), tc.msg) {
				t.Errorf("error %q does not mention %q", err, tc.msg)
			}
			got := ""
			if le, ok := errors.AsType[*safexml.LimitError](err); ok {
				got = le.Limit
			}
			if got != tc.limit {
				t.Errorf("limit = %q, want %q", got, tc.limit)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct{ in, want string }{
		{"2026-10-13T10:00:00+02:00", "2026-10-13T08:00:00Z"},
		{"2026-10-13T10:00:00", "2026-10-13T10:00:00Z"},
		{"2026-10-13 10:00:00", "2026-10-13T10:00:00Z"},
		{"Tue, 13 Oct 2026 10:00:00 +0200", "2026-10-13T08:00:00Z"},
		{"Tue, 13 Oct 2026 10:00:00 GMT", "2026-10-13T10:00:00Z"},
		{"Tue, 13 Oct 2026 10:00:00 PDT", "2026-10-13T17:00:00Z"},
		{"Tue, 3 Oct 2026 10:00 +0000", "2026-10-03T10:00:00Z"},
		{"Tue, 13 Oct 26 10:00:00 -0500", "2026-10-13T15:00:00Z"},
		{"13 Oct 2026 10:00:00 +0100", "2026-10-13T09:00:00Z"},
		{"  2026-10-13  ", "2026-10-13T00:00:00Z"},
		{"2026-10-13T10:00:00+0200", "2026-10-13T08:00:00Z"},
		{"Tue, 13 Oct 2026 10:00:00 UT", "2026-10-13T10:00:00Z"},
		{"Tue, 13 Oct 2026 10:00:00 Z", "2026-10-13T10:00:00Z"},
		{"Tue, 13 Oct 2026 10:00:00 est", "2026-10-13T15:00:00Z"},
		{"Tuesday, 13-Oct-26 10:00:00 GMT", "2026-10-13T10:00:00Z"},
		{"Tue, 13 Oct 2026 10:00:00 CEST", ""},
		{"", ""},
		{"yesterday", ""},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := parseDate(tc.in)
			if tc.want == "" {
				if !got.IsZero() {
					t.Errorf("parseDate(%q) = %v, want zero", tc.in, got)
				}
				return
			}
			if !got.Equal(utc(tc.want)) || got.Location() != time.UTC {
				t.Errorf("parseDate(%q) = %v, want %s in UTC", tc.in, got, tc.want)
			}
		})
	}
}

func TestParse_ItemCap(t *testing.T) {
	xmlItems := "<rss><channel>" + strings.Repeat("<item/>", MaxItems+1) + "</channel></rss>"
	jsonItems := `{"version":"https://jsonfeed.org/version/1.1","items":[` +
		strings.TrimSuffix(strings.Repeat("{},", MaxItems+1), ",") + `]}`
	atomItems := `<feed xmlns="http://www.w3.org/2005/Atom">` + strings.Repeat("<entry/>", MaxItems+1) + "</feed>"
	for name, doc := range map[string]string{"rss": xmlItems, "json": jsonItems, "atom": atomItems} {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(doc))
			le, ok := errors.AsType[*safexml.LimitError](err)
			if !ok || le.Limit != "items" {
				t.Fatalf("err = %v, want an items LimitError", err)
			}
		})
	}
	atCap := `{"version":"https://jsonfeed.org/version/1","items":[` +
		strings.TrimSuffix(strings.Repeat("{},", MaxItems), ",") + `]}`
	if f, err := Parse([]byte(atCap)); err != nil || len(f.Items) != MaxItems {
		t.Fatalf("feed at the cap: %d items, err %v", len(f.Items), err)
	}
}

func TestParse_JSONFeedShapes(t *testing.T) {
	doc := `{"extra": {"nested": [1, 2]}, "title": 5, "version": "https://jsonfeed.org/version/1", "items": []}`
	f, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if f.Title != "" || len(f.Items) != 0 {
		t.Errorf("feed = %+v; a non-string title is left empty", f)
	}
	for _, bad := range []string{`{"items": {}}`, `{"items": [1]}`, `{"version": "https://jsonfeed.org/version/1"`, `{"a" 1}`} {
		if _, err := Parse([]byte(bad)); err == nil || !strings.Contains(err.Error(), "invalid JSON") {
			t.Errorf("Parse(%s) err = %v, want invalid JSON", bad, err)
		}
	}
}

func TestParse_EmbeddedMarkup(t *testing.T) {
	rss := `<rss><channel><item>
<description>Hello <b>world</b> &amp; <a href="/x">more</a></description>
</item></channel></rss>`
	f, err := Parse([]byte(rss))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := f.Items[0].Summary, `Hello <b>world</b> &amp; <a href="/x">more</a>`; got != want {
		t.Errorf("RSS description with child elements = %q, want %q", got, want)
	}

	atom := `<feed xmlns="http://www.w3.org/2005/Atom" xmlns:o="urn:o"><entry><content type="xhtml">` +
		`<div xmlns="http://www.w3.org/1999/xhtml"><p xml:lang="nl" lang="nl" o:x="1">p</p><br>x</br><hr/></div>` +
		`</content></entry></feed>`
	f, err = Parse([]byte(atom))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := f.Items[0].Content, `<p xml:lang="nl" lang="nl">p</p><br>x</br><hr>`; got != want {
		t.Errorf("xhtml content = %q, want %q", got, want)
	}
}
