// Package htmlmd converts between rich-text HTML and markdown, for sync
// connectors whose remote system stores HTML (TKT-LAMZ8F).
//
// It is a leaf: pure text to text, no state, no internal imports.
//
// # Both directions sanitize
//
// [FromHTML] reads HTML a remote system sent, so it is untrusted. It is
// sanitized before conversion, so a javascript: link or a script cannot
// become markdown that rela later renders. [ToHTML] renders markdown that a
// rela user wrote and sanitizes the result with the same allowlist, so the
// remote system receives only the subset both sides express.
//
// # Round trips converge
//
// A connector prevents sync loops by comparing each side with the last
// agreed state. That holds only when converting back and forth stops
// changing the text. For any HTML h, FromHTML(ToHTML(FromHTML(h))) equals
// FromHTML(h); TestRoundTripStable pins it. Formatting outside the allowlist,
// such as colors, images and attachments, is dropped on the way in, so a
// connector must not push a body it could not represent (see the Basecamp
// example).
package htmlmd

import (
	"bytes"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// policy is the allowlist both directions share: block structure, emphasis,
// strikethrough, lists, code, quotes and http(s)/mailto links. Images are
// left out because a remote image in a body is a tracking pixel.
var policy = newPolicy()

func newPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "div", "br", "hr", "h1", "h2", "h3", "h4", "h5", "h6",
		"strong", "b", "em", "i", "del", "s", "strike",
		"ul", "ol", "li", "pre", "code", "blockquote")
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto")
	p.RequireParseableURLs(true)
	// rel=nofollow would be added to every link and come back as a change.
	p.RequireNoFollowOnLinks(false)
	return p
}

var toMarkdown = converter.NewConverter(converter.WithPlugins(
	base.NewBasePlugin(),
	commonmark.NewCommonmarkPlugin(),
	strikethrough.NewStrikethroughPlugin(),
))

// goldmark without html.WithUnsafe: raw HTML in the markdown is not passed
// through, so a user cannot smuggle markup past the allowlist's intent.
var toHTML = goldmark.New(goldmark.WithExtensions(extension.Strikethrough))

// FromHTML converts untrusted rich-text HTML to markdown. Markup outside the
// allowlist is dropped and its text kept.
func FromHTML(html string) (string, error) {
	md, err := toMarkdown.ConvertString(policy.Sanitize(html))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(md), nil
}

// ToHTML renders markdown as sanitized HTML.
func ToHTML(md string) (string, error) {
	var buf bytes.Buffer
	if err := toHTML.Convert([]byte(md), &buf); err != nil {
		return "", err
	}
	return strings.TrimSpace(policy.Sanitize(buf.String())), nil
}
