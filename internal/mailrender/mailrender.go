// Package mailrender turns a message model into the two body parts an email
// carries: sanitized, CSS-inlined HTML and a plain-text alternative.
//
// It is a leaf, in the sense internal/calfeed is: pure model to bytes, holding
// no state, reading nothing from a store, and importing no storage, metamodel
// or application types. Callers assemble the model from data they have already
// read (and, from TKT-U2R7GU, already gated); this package only formats it.
//
// # The pipeline order is load-bearing
//
// Rendering runs in exactly this order, and the order is a security property,
// not a convenience:
//
//	markdown -> goldmark -> bluemonday(CONTENT ONLY) -> trusted template -> douceur inline
//
// Two verified library behaviours force it (TKT-332QZY design review):
//
//   - bluemonday strips style attributes unconditionally, and AllowStyling does
//     not restore them. Sanitizing after inlining therefore deletes every
//     inlined declaration and produces unstyled mail. Sanitizing the assembled
//     document additionally strips the cellpadding/cellspacing/border/role
//     attributes that table-based email layout depends on, and drops cid:
//     image sources, which would break the embedded logo.
//   - douceur performs no CSS value validation whatsoever. It will happily
//     materialize url('javascript:...'), behavior:url(...) and expression(...)
//     into style attributes. Because it runs last, nothing sanitizes its
//     output, so only trusted CSS may reach it.
//
// The consequences for anyone editing this package: sanitize the untrusted
// fragment, never the assembled document; keep the <style> block operator- and
// template-authored; and validate every value interpolated into CSS. Reversing
// any of those is a silent downgrade — mail still sends, it is merely unstyled
// or unsafe — which is why this is written down rather than left to be
// rediscovered.
//
// # Trust
//
// Content passed in is UNTRUSTED: it originates from entity bodies and
// properties. The template and its stylesheet are TRUSTED: they ship with rela.
// Palette tokens sit in between — operator-supplied, but they land in CSS, so
// they are validated as colours and rejected otherwise (see [ValidatePalette]).
//
// Nil: [Render] rejects a nil Message; [New] rejects a nil Options.
package mailrender

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strings"

	"github.com/aymerick/douceur/inliner"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

// Section is one block of a message body. A Section renders either as a
// paragraph of prose (Body only) or as a table (Rows non-empty); a Section with
// both renders the prose above the table.
type Section struct {
	// Title is an optional heading rendered above the section.
	Title string

	// Body is markdown. It is UNTRUSTED and is sanitized during rendering.
	Body string

	// Columns are table header labels. Empty means no table.
	Columns []string

	// Rows are table cells, each row aligned to Columns. Cell text is escaped,
	// not markdown-rendered — a table cell is a value, not a document.
	Rows [][]string

	// Links, when set, gives each row an href. A nil or short slice leaves the
	// corresponding rows unlinked.
	Links []string
}

// Message is the model a caller assembles and this package formats.
type Message struct {
	// Subject is the mail subject. Header-safety (no CR/LF) is the sending
	// package's concern, not the renderer's.
	Subject string

	// Intro is optional markdown shown above the sections. UNTRUSTED.
	Intro string

	// Sections are rendered in order.
	Sections []Section

	// Footer is optional markdown shown below the sections. UNTRUSTED.
	Footer string
}

// Options configures a Renderer. The zero value is usable: it renders with
// rela's default palette and no logo.
type Options struct {
	// Palette maps CSS custom-property names (e.g. "--accent-color") to colour
	// values. Values MUST be colours; see [ValidatePalette]. Keys absent from
	// the map fall back to the built-in defaults.
	Palette map[string]string

	// LogoCID, when non-empty, is the Content-ID of a logo part the caller has
	// attached to the message. The template references it as cid:<LogoCID>.
	// Callers must only set this for raster images — see the package docs on
	// why SVG is refused.
	LogoCID string

	// LogoAlt is the logo's alt text. Defaults to "logo".
	LogoAlt string

	// BaseURL prefixes relative links in rendered output. Mail is read outside
	// the app, so a relative href is dead; callers that emit links should set
	// this.
	BaseURL string
}

// Renderer formats messages. It is immutable after construction and safe for
// concurrent use.
type Renderer struct {
	opts    Options
	policy  *bluemonday.Policy
	md      goldmark.Markdown
	palette map[string]string
}

// ErrNilMessage is returned by Render when handed a nil message.
var ErrNilMessage = errors.New("mailrender: nil message")

// colourRe matches the colour forms permitted in a palette value: #rgb, #rgba,
// #rrggbb, #rrggbbaa. Deliberately narrow — see ValidatePalette.
var colourRe = regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)

// namedColours are the few keywords worth accepting beside hex. Kept short on
// purpose: every entry is a value that will be interpolated into CSS.
var namedColours = map[string]bool{
	"transparent": true, "black": true, "white": true, "inherit": true,
}

// ValidatePalette checks that every value is a colour, and returns an error
// naming the first key that is not.
//
// This is not defensive tidiness. Palette values are interpolated into the
// stylesheet, and douceur — which runs last and validates nothing — will
// materialize whatever it finds into a style attribute. A token of
// url('javascript:alert(1)') reaches the recipient's mail client verbatim. So
// values are checked against an ALLOWLIST and REJECTED; they are never escaped
// or silently replaced with a default, because a caller that supplied a bad
// colour has a bug worth surfacing.
func ValidatePalette(p map[string]string) error {
	for k, v := range p {
		val := strings.TrimSpace(strings.ToLower(v))
		if colourRe.MatchString(val) || namedColours[val] {
			continue
		}
		return fmt.Errorf("mailrender: palette %q: %q is not a colour "+
			"(want #rgb/#rrggbb or one of transparent/black/white/inherit)", k, v)
	}
	return nil
}

// New returns a Renderer.
//
// Nil: rejected — pass a zero Options rather than nil, so "defaults" is a
// deliberate choice at the call site rather than an accident.
func New(opts *Options) (*Renderer, error) {
	if opts == nil {
		return nil, errors.New("mailrender: nil options")
	}
	if err := ValidatePalette(opts.Palette); err != nil {
		return nil, err
	}

	pal := make(map[string]string, len(defaultPalette))
	maps.Copy(pal, defaultPalette)
	maps.Copy(pal, opts.Palette)

	return &Renderer{
		opts:    *opts,
		policy:  newMailPolicy(),
		md:      newMarkdown(),
		palette: pal,
	}, nil
}

// newMarkdown builds the goldmark instance used for UNTRUSTED content.
//
// WithUnsafe is deliberately NOT set. internal/dataentry's converter sets it
// because its input is operator-authored schema prose; mail bodies carry entity
// content, so raw HTML must not pass through. bluemonday runs afterwards
// regardless — the two are belt and braces, not alternatives.
func newMarkdown() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
}

// newMailPolicy returns the sanitizer applied to untrusted content fragments.
//
// UGCPolicy plus tables. Note what is NOT granted: style attributes stay
// forbidden, so untrusted content can never contribute CSS for douceur to
// inline. That is the invariant the package doc describes; granting style here
// would defeat it.
func newMailPolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowTables()
	return p
}

// Render produces the HTML and plain-text parts of m.
//
// Nil: a nil message is rejected with [ErrNilMessage].
func (r *Renderer) Render(m *Message) (html, text []byte, err error) {
	if m == nil {
		return nil, nil, ErrNilMessage
	}

	doc, err := r.buildDocument(m)
	if err != nil {
		return nil, nil, err
	}

	// Inline LAST. Nothing may sanitize after this point.
	inlined, err := inliner.Inline(doc)
	if err != nil {
		return nil, nil, fmt.Errorf("mailrender: inline css: %w", err)
	}

	return []byte(inlined), r.renderText(m), nil
}

// markdownToSafeHTML converts UNTRUSTED markdown to a sanitized HTML fragment.
// This is the only place untrusted input becomes HTML, and it is always
// followed by the sanitizer.
func (r *Renderer) markdownToSafeHTML(src string) (string, error) {
	if strings.TrimSpace(src) == "" {
		return "", nil
	}
	var buf bytes.Buffer
	if err := r.md.Convert([]byte(src), &buf); err != nil {
		return "", fmt.Errorf("mailrender: convert markdown: %w", err)
	}
	return r.policy.Sanitize(buf.String()), nil
}
