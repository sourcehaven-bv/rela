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
// changing the text. [FromHTML] therefore returns a fixed point: converting
// its result to HTML and back yields the same markdown. Markup the two
// libraries disagree on, such as bold inside a word next to punctuation,
// settles as literal text rather than changing on every sync.
// TestRoundTripStable pins it.
//
// # Loss is reported, not hidden
//
// Both functions say when they dropped something: an element outside the
// allowlist, a link with another scheme, or (for markdown) an image, raw
// HTML or a task checkbox. A connector must not push a body whose
// conversion lost content, because the remote copy would lose it too.
package htmlmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/marker"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"golang.org/x/net/html"
)

// allowed are the elements both directions keep: block structure,
// emphasis, strikethrough, lists, code, quotes, tables and links. Images
// are left out because a remote image in a body is a tracking pixel.
var allowed = map[string]bool{
	"p": true, "div": true, "br": true, "hr": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"strong": true, "b": true, "em": true, "i": true, "del": true, "s": true, "strike": true,
	"ul": true, "ol": true, "li": true, "pre": true, "code": true, "blockquote": true,
	"table": true, "thead": true, "tbody": true, "tr": true, "th": true, "td": true,
	"a": true,
}

// schemes are the link schemes kept; a link with another one loses its URL.
var schemes = map[string]bool{"http": true, "https": true, "mailto": true}

// policy sanitizes both directions with the allowlist above.
var policy = newPolicy()

func newPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	for el := range allowed {
		if el != "a" {
			p.AllowElements(el)
		}
	}
	p.AllowAttrs("href").OnElements("a")
	for s := range schemes {
		p.AllowURLSchemes(s)
	}
	p.RequireParseableURLs(true)
	// rel=nofollow would be added to every link and come back as a change.
	p.RequireNoFollowOnLinks(false)
	return p
}

var toMarkdown = newConverter()

func newConverter() *converter.Converter {
	conv := converter.NewConverter(converter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(),
		strikethrough.NewStrikethroughPlugin(),
		table.NewTablePlugin(),
	))
	// Text that reads as an HTML entity, such as a literal "&copy;", must
	// stay text: unescaped, the next markdown parse would decode it.
	conv.Register.EscapedChar('&')
	conv.Register.UnEscaper(isEntity, converter.PriorityStandard)
	return conv
}

// isEntity is an html-to-markdown unescape handler: it keeps the escape on
// an & that starts an entity-like run (&name; or &#123;). chars holds escape
// placeholders before other escapable characters, such as '#'.
func isEntity(chars []byte, index int) int {
	if chars[index] != '&' {
		return -1
	}
	n := 0
	for i := index + 1; i < len(chars); i++ {
		c := chars[i]
		switch {
		case c == marker.BytesMarkerEscaping[0]:
		case c == ';':
			if n > 0 {
				return 1
			}
			return -1
		case c == '#' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			n++
		default:
			return -1
		}
	}
	return -1
}

// goldmark without html.WithUnsafe: raw HTML in the markdown is not passed
// through but reported as a loss.
var toHTML = goldmark.New(goldmark.WithExtensions(extension.Strikethrough, extension.Table))

// MaxInput is the largest input either function accepts, in bytes. A rich
// text body is far smaller; the cap bounds the cost of a hostile one.
const MaxInput = 1 << 20

// maxPasses bounds the fixed-point search in FromHTML. Every input seen so
// far settles within two.
const maxPasses = 4

// FromHTML converts untrusted rich-text HTML to markdown. Markup outside the
// allowlist is dropped and its text kept; lossy reports that this happened.
func FromHTML(in string) (md string, lossy bool, err error) {
	if len(in) > MaxInput {
		return "", false, fmt.Errorf("htmlmd: input is %d bytes; the limit is %d", len(in), MaxInput)
	}
	lossy, err = outsideAllowlist(in)
	if err != nil {
		return "", false, err
	}
	md, err = convert(in)
	if err != nil {
		return "", false, err
	}
	for range maxPasses {
		h, err := render(md)
		if err != nil {
			return "", false, err
		}
		next, err := convert(h)
		if err != nil {
			return "", false, err
		}
		if next == md {
			return md, lossy, nil
		}
		md = next
	}
	return "", false, fmt.Errorf("htmlmd: conversion did not settle in %d passes", maxPasses)
}

// ToHTML renders markdown as sanitized HTML; lossy reports that something
// the markdown held was dropped.
func ToHTML(md string) (out string, lossy bool, err error) {
	if len(md) > MaxInput {
		return "", false, fmt.Errorf("htmlmd: input is %d bytes; the limit is %d", len(md), MaxInput)
	}
	raw, err := render(md)
	if err != nil {
		return "", false, err
	}
	lossy, err = outsideAllowlist(raw)
	if err != nil {
		return "", false, err
	}
	return strings.TrimSpace(policy.Sanitize(raw)), lossy, nil
}

// listEnd is the comment html-to-markdown puts between two adjacent lists to
// keep them apart. It would show in rela's editor, so it is removed; the two
// lists then become one, and the fixed-point search in FromHTML settles there.
var listEnd = regexp.MustCompile(`\n*<!--THE END-->\n*`)

func convert(in string) (string, error) {
	md, err := toMarkdown.ConvertString(policy.Sanitize(in))
	if err != nil {
		return "", fmt.Errorf("htmlmd: to markdown: %w", err)
	}
	return strings.TrimSpace(listEnd.ReplaceAllString(md, "\n\n")), nil
}

func render(md string) (string, error) {
	var buf bytes.Buffer
	if err := toHTML.Convert([]byte(md), &buf); err != nil {
		return "", fmt.Errorf("htmlmd: to html: %w", err)
	}
	return buf.String(), nil
}

// rawOmitted is what goldmark writes in place of raw HTML it refused.
var rawOmitted = regexp.MustCompile(`<!--\s*raw HTML omitted\s*-->`)

// outsideAllowlist reports whether the HTML holds an element, link or raw
// HTML the policy drops. Attributes other than href are ignored: dropping a
// style or class loses looks, not content.
func outsideAllowlist(in string) (bool, error) {
	if rawOmitted.MatchString(in) {
		return true, nil
	}
	z := html.NewTokenizer(strings.NewReader(in))
	for {
		switch z.Next() {
		case html.ErrorToken:
			if err := z.Err(); !errors.Is(err, io.EOF) {
				return false, fmt.Errorf("htmlmd: read html: %w", err)
			}
			return false, nil
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			if !allowed[t.Data] || t.Data == "a" && !linkKept(t) {
				return true, nil
			}
		default:
		}
	}
}

// linkKept reports whether the policy keeps the link's href.
func linkKept(t html.Token) bool {
	for _, a := range t.Attr {
		if a.Key != "href" {
			continue
		}
		u, err := url.Parse(strings.TrimSpace(a.Val))
		return err == nil && schemes[strings.ToLower(u.Scheme)]
	}
	return true
}
