package subscription

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func configMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"article": {Properties: map[string]metamodel.PropertyDef{
			"guid": {Type: "string"}, "title": {Type: "string"}, "link": {Type: "string"},
		}},
	}}
}

const validSub = `
subscriptions:
  news:
    url: https://example.com/feed.xml
    every: 1h
    find: {type: article, match: [guid], values: {guid: "{{item.id}}"}}
    create_if_missing:
      properties: {title: "{{item.title}}"}
      content: "{{item.content}}"
`

func TestParse_Valid(t *testing.T) {
	cfg, err := Parse([]byte(validSub), configMeta())
	if err != nil {
		t.Fatal(err)
	}
	sub := cfg.Subscriptions["news"]
	if sub.Find.Type != "article" || sub.EffectiveMaxItems() != DefaultMaxItems {
		t.Errorf("parsed %+v", sub)
	}
	if names := cfg.Names(); len(names) != 1 || names[0] != "news" {
		t.Errorf("Names() = %v", names)
	}
}

func TestParse_Empty(t *testing.T) {
	cfg, err := Parse(nil, configMeta())
	if err != nil || len(cfg.Subscriptions) != 0 {
		t.Errorf("Parse(empty) = %+v, %v", cfg, err)
	}
}

func TestParse_Errors(t *testing.T) {
	for _, tc := range []struct {
		name, replace, with, want string
	}{
		{"unknown key", "every: 1h", "every: 1h\n    bogus: 1", "field bogus not found"},
		{"bad name", "  news:", "  News:", "invalid name"},
		{"no url", "url: https://example.com/feed.xml", "url: ''", "url is required"},
		{"bad scheme", "https://example.com", "ftp://example.com", "http or https"},
		{"no host", "https://example.com/feed.xml", "https:///feed.xml", "needs a host"},
		{"fragment", "feed.xml", "feed.xml#top", "must not carry a fragment"},
		{"userinfo", "https://example.com", "https://u:p@example.com", "must not carry credentials"},
		{"no every", "every: 1h", "every: ''", "every is required"},
		{"max_items", "every: 1h", "every: 1h\n    max_items: 5000", "max_items 5000"},
		{"unknown type", "type: article", "type: post", `"post" is not a known entity type`},
		{"unknown ref", "{{item.title}}", "{{item.guid}}", "unknown reference {{item.guid}}"},
		{"default match not an item field", `, values: {guid: "{{item.id}}"}`, "", "item.guid is not an item field"},
		{"no match", "match: [guid], values: {guid: \"{{item.id}}\"}", "match: []", "needs find.match"},
		{
			"markdown in append", "content: \"{{item.content}}\"",
			"content: x\n    then: [{append_section: {section: Log, content: \"{{item.summary}}\"}}]",
			"may not use {{item.summary}}",
		},
		{
			"block with append", "content: \"{{item.content}}\"",
			"content: \"{{item.content}}\"\n    then: [{append_section: {section: Log, content: x}}]",
			"feed headings could forge the section",
		},
		{
			"auth over http", "https://example.com/feed.xml\n    every: 1h",
			"http://example.com/feed.xml\n    every: 1h\n    auth: {type: bearer, secret: s}",
			"auth requires an https url",
		},
		{"auth type", "every: 1h", "every: 1h\n    auth: {type: oauth, secret: s}", `auth.type "oauth"`},
		{"auth no secret", "every: 1h", "every: 1h\n    auth: {type: bearer}", "names no secret"},
		{"auth token key", "every: 1h", "every: 1h\n    auth: {type: bearer, secret: token_key}", "signs rela's own"},
		{"basic without user", "every: 1h", "every: 1h\n    auth: {type: basic, secret: s}", "auth.username"},
		{"user on bearer", "every: 1h", "every: 1h\n    auth: {type: bearer, secret: s, username: u}", "auth.username"},
		{"bad header", "every: 1h", "every: 1h\n    auth: {type: header, secret: s, header: 'a b'}", "not a valid HTTP"},
		{"header on bearer", "every: 1h", "every: 1h\n    auth: {type: bearer, secret: s, header: X-K}", "only allowed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := strings.Replace(validSub, tc.replace, tc.with, 1)
			if data == validSub {
				t.Fatalf("replacement %q not found", tc.replace)
			}
			_, err := Parse([]byte(data), configMeta())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Parse error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestParse_ValidAuth(t *testing.T) {
	for _, auth := range []string{
		"{type: bearer, secret: s}",
		"{type: basic, secret: s, username: u}",
		"{type: header, secret: s, header: X-Api-Key}",
	} {
		data := strings.Replace(validSub, "every: 1h", "every: 1h\n    auth: "+auth, 1)
		if _, err := Parse([]byte(data), configMeta()); err != nil {
			t.Errorf("auth %s: %v", auth, err)
		}
	}
}
