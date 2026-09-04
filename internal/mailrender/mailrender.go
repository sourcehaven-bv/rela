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
// Two verified library behaviors force it (TKT-332QZY design review):
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
// # The logo is embedded, and raster only
//
// The operator logo is referenced as cid:<LogoCID> and travels with the message
// as an attached part, never as a URL: rela serves it from an authenticated
// endpoint, so a mail client could not fetch it.
//
// Callers must supply raster bytes (PNG/JPEG/WebP) and must NOT pass an SVG.
// SVG has near-zero support across mail clients — Gmail, Outlook and Apple Mail
// strip or fail it — and it is an active-content format that can carry a script
// element, so embedding operator-uploaded SVG would ship script-capable bytes
// into inboxes for no rendering benefit. A message renders without a logo
// rather than with an unsafe one.
//
// # Trust
//
// Content passed in is UNTRUSTED: it originates from entity bodies and
// properties. The template and its stylesheet are TRUSTED: they ship with rela.
// Palette tokens sit in between — operator-supplied, but they land in CSS, so
// they are validated as colors and rejected otherwise (see [ValidatePalette]).
//
// Nil: [Renderer.Render] rejects a nil Message; [New] rejects a nil Options.
package mailrender

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
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

	// Lang is the BCP-47 language tag of this message's content, emitted as
	// the <html lang="..."> attribute.
	//
	// It lives on Message rather than Options because language is CONTENT, not
	// branding: one deployment sends a Dutch digest and an English one from the
	// same Renderer, so a renderer-scoped value would mislabel every message
	// but one. Options.DefaultLang supplies the fallback when this is empty.
	//
	// UNTRUSTED — it reaches an HTML attribute and is validated as a language
	// tag (see [ValidateLang]), never escaped into place.
	Lang string
}

// Options configures a Renderer. The zero value is usable: it renders with
// rela's default palette and no logo.
type Options struct {
	// Palette maps CSS custom-property names (e.g. "--accent-color") to color
	// values. Values MUST be colors; see [ValidatePalette]. Keys absent from
	// the map fall back to the built-in defaults.
	Palette map[string]string

	// LogoCID, when non-empty, is the Content-ID of a logo part the caller has
	// attached to the message. The template references it as cid:<LogoCID>.
	//
	// Raster only. See "The logo is embedded, and raster only" in the package
	// doc for why an SVG must never be passed here.
	LogoCID string

	// LogoAlt is the logo's alt text. Defaults to "logo".
	LogoAlt string

	// BaseURL prefixes relative links in rendered output. Mail is read outside
	// the app, so a relative href is dead; callers that emit links should set
	// this.
	BaseURL string

	// LogoWidth and LogoHeight are the logo's intrinsic pixel dimensions,
	// emitted as width/height attributes on the <img>.
	//
	// Attributes rather than CSS because Outlook Windows ignores max-height,
	// so a large logo renders at full size there with no other constraint.
	// Zero omits the attribute rather than emitting width="0".
	LogoWidth, LogoHeight int

	// DefaultLang is the language tag used when a Message does not carry its
	// own. Defaults to "en". This is the only language value that is
	// deployment-scoped, and it is a FALLBACK — see [Message.Lang].
	DefaultLang string
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

// colorRe matches the color forms permitted in a palette value: #rgb, #rgba,
// #rrggbb, #rrggbbaa. Deliberately narrow — see ValidatePalette.
var colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)

// namedColours are the few keywords worth accepting beside hex. Kept short on
// purpose: every entry is a value that will be interpolated into CSS.
var namedColours = map[string]bool{
	"transparent": true, "black": true, "white": true, "inherit": true,
}

// ValidatePalette checks that every value is a color, and returns an error
// naming the first key that is not.
//
// This is not defensive tidiness. Palette values are interpolated into the
// stylesheet, and douceur — which runs last and validates nothing — will
// materialize whatever it finds into a style attribute. A token of
// url('javascript:alert(1)') reaches the recipient's mail client verbatim. So
// values are checked against an ALLOWLIST and REJECTED; they are never escaped
// or silently replaced with a default, because a caller that supplied a bad
// color has a bug worth surfacing.
func ValidatePalette(p map[string]string) error {
	for k, v := range p {
		// An unknown key is refused rather than ignored. Every token here is
		// consumed by name in colors(), so a misspelled one — "--dark-card-color"
		// for "--dark-card-bg" — is silently dropped and the operator sees a
		// default they explicitly tried to override, with nothing to explain it.
		// The set is closed and small, so naming the valid keys costs nothing.
		if _, known := defaultPalette[k]; !known {
			return fmt.Errorf("mailrender: palette %q is not a known token (valid: %s)",
				k, strings.Join(paletteKeys(), ", "))
		}
		val := strings.TrimSpace(strings.ToLower(v))
		if colorRe.MatchString(val) || namedColours[val] {
			continue
		}
		return fmt.Errorf("mailrender: palette %q: %q is not a color "+
			"(want #rgb/#rrggbb or one of transparent/black/white/inherit)", k, v)
	}
	return nil
}

// paletteKeys lists the recognized token names, sorted so the error message is
// stable rather than reflecting Go's map iteration order.
func paletteKeys() []string {
	out := slices.Collect(maps.Keys(defaultPalette))
	slices.Sort(out)
	return out
}

// langRe matches the SHAPE of a BCP-47 language tag: subtags of ASCII letters
// or digits joined by hyphens, first subtag letters only ("nl", "nl-NL",
// "zh-Hant-TW").
//
// Deliberately a shape check and not a registry lookup. The security
// requirement is that nothing can terminate the attribute or inject markup;
// refusing a valid-but-unusual subtag would be rela inventing a policy about
// languages it has no business holding.
var langRe = regexp.MustCompile(`^[A-Za-z]{1,8}(-[A-Za-z0-9]{1,8})*$`)

// maxLangLen bounds a language tag. Well above any real tag, low enough that a
// pathological value cannot bloat the document.
const maxLangLen = 35

// ValidateLang checks that v is shaped like a BCP-47 language tag.
//
// The empty string is accepted and means "fall back to the default"; callers
// resolve that before rendering, so an empty tag never reaches the attribute.
//
// Like [ValidatePalette] this REJECTS rather than sanitizes. A language tag
// lands in an HTML attribute, and escaping a malformed one would yield a
// well-formed document that lies about its language — the failure is worth
// surfacing to whoever wrote the config or the script.
func ValidateLang(v string) error {
	if v == "" {
		return nil
	}
	if len(v) > maxLangLen || !langRe.MatchString(v) {
		return fmt.Errorf("mailrender: %q is not a language tag (want e.g. \"en\", \"nl-NL\")", v)
	}
	return nil
}

// validateBaseURL applies the same scheme allowlist safeHref uses.
//
// Without this the allowlist is defeated by the one input it does not check:
// safeHref rejects a "javascript:" LINK, then concatenates it onto an unchecked
// BaseURL, so a BaseURL of "javascript:alert(1)//" turns every relative link in
// the message into a live script URL. Checking here closes it once, for every
// caller, rather than relying on each one to have validated first.
func validateBaseURL(u string) error {
	if u == "" {
		return nil
	}
	l := strings.ToLower(strings.TrimSpace(u))
	if !strings.HasPrefix(l, "http://") && !strings.HasPrefix(l, "https://") {
		return fmt.Errorf("mailrender: base URL must start with http:// or https://, got %q", u)
	}
	return nil
}

// validateHeaderish rejects control characters in a value that lands in a
// header-like position (a Content-ID reference).
func validateHeaderish(field, v string) error {
	if strings.ContainsAny(v, "\r\n\x00") {
		return fmt.Errorf("mailrender: %s contains a control character", field)
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
	if err := validateBaseURL(opts.BaseURL); err != nil {
		return nil, err
	}
	if err := validateHeaderish("logo CID", opts.LogoCID); err != nil {
		return nil, err
	}
	if err := ValidateLang(opts.DefaultLang); err != nil {
		return nil, err
	}

	pal := make(map[string]string, len(defaultPalette))
	maps.Copy(pal, defaultPalette)
	maps.Copy(pal, opts.Palette)

	resolved := *opts
	if resolved.DefaultLang == "" {
		resolved.DefaultLang = fallbackLang
	}

	return &Renderer{
		opts:    resolved,
		policy:  newMailPolicy(),
		md:      newMarkdown(),
		palette: pal,
	}, nil
}

// fallbackLang is used when neither the message nor the operator names one.
// Something must be emitted — an absent lang attribute is the defect this
// closes — and rela's own strings are English.
const fallbackLang = "en"

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
