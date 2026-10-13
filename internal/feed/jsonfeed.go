package feed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// jsonFeed is the subset of JSON Feed 1.0 and 1.1 that maps onto [Feed].
type jsonFeed struct {
	Version     string     `json:"version"`
	Title       string     `json:"title"`
	HomePageURL string     `json:"home_page_url"`
	Description string     `json:"description"`
	Items       []jsonItem `json:"items"`
}

type jsonItem struct {
	// ID is a string in the spec, but 1.0 feeds in the wild use numbers.
	ID            json.RawMessage `json:"id"`
	URL           string          `json:"url"`
	Title         string          `json:"title"`
	ContentHTML   string          `json:"content_html"`
	ContentText   string          `json:"content_text"`
	Summary       string          `json:"summary"`
	DatePublished string          `json:"date_published"`
	DateModified  string          `json:"date_modified"`
	Author        *jsonAuthor     `json:"author"` // 1.0
	Authors       []jsonAuthor    `json:"authors"`
	Tags          []string        `json:"tags"`
}

type jsonAuthor struct {
	Name string `json:"name"`
}

const jsonFeedVersionPrefix = "https://jsonfeed.org/version/1"

func parseJSON(data []byte) (Feed, error) {
	var jf jsonFeed
	if err := json.Unmarshal(data, &jf); err != nil {
		return Feed{}, fmt.Errorf("feed: invalid JSON: %w", err)
	}
	if !strings.HasPrefix(jf.Version, jsonFeedVersionPrefix) {
		return Feed{}, fmt.Errorf("feed: JSON document is not a JSON Feed (version %q)", jf.Version)
	}
	f := Feed{
		Format:      FormatJSON,
		Title:       strings.TrimSpace(jf.Title),
		Link:        strings.TrimSpace(jf.HomePageURL),
		Description: strings.TrimSpace(jf.Description),
	}
	for _, ji := range jf.Items {
		f.Items = append(f.Items, jsonFeedItem(ji))
	}
	return f, nil
}

func jsonFeedItem(ji jsonItem) Item {
	it := Item{
		Title:        strings.TrimSpace(ji.Title),
		Link:         strings.TrimSpace(ji.URL),
		Summary:      textToHTML(ji.Summary),
		Content:      firstNonEmpty(strings.TrimSpace(ji.ContentHTML), textToHTML(ji.ContentText)),
		PublishedRaw: strings.TrimSpace(ji.DatePublished),
		UpdatedRaw:   strings.TrimSpace(ji.DateModified),
		Categories:   nonEmpty(ji.Tags),
	}
	it.ID = firstNonEmpty(jsonID(ji.ID), it.Link)
	it.Published = parseDate(it.PublishedRaw)
	it.Updated = parseDate(it.UpdatedRaw)
	authors := ji.Authors
	if ji.Author != nil {
		authors = append(authors, *ji.Author)
	}
	for _, a := range authors {
		if s := strings.TrimSpace(a.Name); s != "" {
			it.Authors = append(it.Authors, s)
		}
	}
	return it
}

// jsonID renders an item id given as a JSON string or number. Any other
// JSON value is not an identifier and yields "".
func jsonID(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	return ""
}

func nonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
