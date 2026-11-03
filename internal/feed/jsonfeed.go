package feed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// jsonFeedHeader is the feed-level subset of JSON Feed 1.0 and 1.1 that maps
// onto [Feed]. Items are decoded one at a time by [parseJSON] so the item cap
// applies before memory is spent on them.
type jsonFeedHeader struct {
	Version     string `json:"version"`
	Title       string `json:"title"`
	HomePageURL string `json:"home_page_url"`
	Description string `json:"description"`
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

// parseJSON reads a JSON Feed. It walks the top-level object token by token:
// "items" is streamed element by element under [MaxItems]; every other
// member is decoded into the header.
func parseJSON(data []byte) (Feed, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := expectDelim(dec, '{'); err != nil {
		return Feed{}, err
	}
	var (
		hdr   jsonFeedHeader
		items []Item
	)
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return Feed{}, invalidJSON(err)
		}
		key, _ := tok.(string)
		switch key {
		case "items":
			if items, err = decodeJSONItems(dec); err != nil {
				return Feed{}, err
			}
		case "version", "title", "home_page_url", "description":
			if err := decodeHeaderField(dec, key, &hdr); err != nil {
				return Feed{}, err
			}
		default:
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return Feed{}, invalidJSON(err)
			}
		}
	}
	if err := expectDelim(dec, '}'); err != nil {
		return Feed{}, err
	}
	if !strings.HasPrefix(hdr.Version, jsonFeedVersionPrefix) {
		return Feed{}, fmt.Errorf("feed: JSON document is not a JSON Feed (version %q)", hdr.Version)
	}
	return Feed{
		Format:      FormatJSON,
		Title:       strings.TrimSpace(hdr.Title),
		Link:        strings.TrimSpace(hdr.HomePageURL),
		Description: strings.TrimSpace(hdr.Description),
		Items:       items,
	}, nil
}

func decodeHeaderField(dec *json.Decoder, key string, hdr *jsonFeedHeader) error {
	dst := map[string]*string{
		"version": &hdr.Version, "title": &hdr.Title,
		"home_page_url": &hdr.HomePageURL, "description": &hdr.Description,
	}[key]
	// A non-string value is not this field; leave it empty rather than fail
	// the whole feed over an optional member.
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return invalidJSON(err)
	}
	_ = json.Unmarshal(raw, dst)
	return nil
}

func decodeJSONItems(dec *json.Decoder) ([]Item, error) {
	if err := expectDelim(dec, '['); err != nil {
		return nil, err
	}
	var items []Item
	for dec.More() {
		if len(items) == MaxItems {
			return nil, errTooManyItems()
		}
		var ji jsonItem
		if err := dec.Decode(&ji); err != nil {
			return nil, invalidJSON(err)
		}
		items = append(items, jsonFeedItem(ji))
	}
	return items, expectDelim(dec, ']')
}

func expectDelim(dec *json.Decoder, want json.Delim) error {
	tok, err := dec.Token()
	if err != nil {
		return invalidJSON(err)
	}
	if d, ok := tok.(json.Delim); !ok || d != want {
		return invalidJSON(fmt.Errorf("expected %q, found %v", want, tok))
	}
	return nil
}

func invalidJSON(err error) error {
	return fmt.Errorf("feed: invalid JSON: %w", err)
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
