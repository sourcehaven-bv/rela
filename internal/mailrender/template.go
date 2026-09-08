package mailrender

import (
	"fmt"
	"html/template"
	"strings"
)

// defaultPalette holds the color tokens the stylesheet reads. Keys match the
// SPA's CSS custom properties (see dataentryconfig.ResolvePalette) so an
// operator palette drops in unchanged; --accent-color's default is the SPA's.
var defaultPalette = map[string]string{
	"--accent-color":  "#4772fb",
	"--text-color":    "#1f2430",
	"--muted-color":   "#6b7280",
	"--bg-color":      "#f4f5f7",
	"--card-bg":       "#ffffff",
	"--border-color":  "#d7dae0",
	"--heading-color": "#111827",

	// Dark-mode overrides, used only inside the prefers-color-scheme block.
	// Not pure black: a #ffffff-on-#000000 pairing causes halation, and some
	// clients re-invert an extreme pair on the assumption it is already dark.
	"--dark-bg-color":      "#1a1d23",
	"--dark-card-bg":       "#22262e",
	"--dark-text-color":    "#e4e6eb",
	"--dark-muted-color":   "#a6adba",
	"--dark-border-color":  "#3a3f4a",
	"--dark-heading-color": "#f2f4f7",
	"--dark-accent-color":  "#8fa9ff",
}

// msoOpen/msoClose wrap the card in a fixed-width table for Outlook's Word
// rendering engine, which ignores max-width. Constants, never caller data.
const (
	msoOpen  template.HTML = `<!--[if mso]><table role="presentation" align="center" cellpadding="0" cellspacing="0" border="0" width="600"><tr><td><![endif]-->`
	msoClose template.HTML = `<!--[if mso]></td></tr></table><![endif]-->`
)

// contentWidth is the fixed body width. 600px is the long-standing safe value:
// it fits the narrowest common desktop preview pane without horizontal scroll.
const contentWidth = "600"

// docTemplate is the TRUSTED shell. Its structure is a hand-port of what MJML
// compiles to — nested tables, explicit cellpadding/cellspacing/border, a fixed
// width, and an mso conditional wrapper — so the output matches what a
// well-tested email framework produces without requiring a Node toolchain at
// runtime.
//
// Two rules for editing:
//
//   - Everything here is trusted, so it may carry a <style> block and table
//     presentation attributes. Untrusted content arrives ONLY through the
//     pre-sanitized .Intro/.Sections/.Footer fields, which are typed
//     template.HTML because they have already been through bluemonday.
//   - Values interpolated into the <style> block come from the palette and are
//     color-validated first. Do not interpolate anything else into CSS.
//
// # Padding and margin live on table cells
//
// Section headings and the empty-section note are single-cell TABLES, not
// styled divs, and vertical gaps are spacer ROWS rather than margins. Both are
// deliberate: Outlook Windows supports padding only on table cells, and margin
// is unsupported or partial across Gmail, Outlook, Yahoo and AOL. A div with
// padding renders fine everywhere the author is likely to be testing and
// collapses in the clients they are not. Do not "simplify" these back to divs.
//
// The spacer row follows a TABLE only. A prose or empty-note section gets its
// separation from the next .sect-title's own top padding instead, so adding a
// gap there would double it. That asymmetry is deliberate, not an oversight.
//
// Lists are the one exception, and they invert the rule: a bullet indent is
// expressed as margin-left rather than padding-left, because a <ul> is not a
// cell and cannot be made into one without mangling author markup. Outlook
// honors margin on a list while dropping padding, so margin is the side of the
// trade that survives.
//
// # Dark mode is defensive, and the color-scheme meta tag is deliberately absent
//
// Clients fall into three tiers: those that leave mail alone (Apple Mail, Gmail
// desktop, Yahoo, AOL), those that partially invert and DO honor
// prefers-color-scheme (the Outlook family), and those that fully invert and do
// NOT honor it (Gmail iOS/Android, Outlook Windows) — that last group rewrites
// the query to @media none, so it cannot be targeted at all.
//
// The @media block below therefore serves only the middle tier. The first tier
// is served by NOT adding <meta name="color-scheme">: adding that tag opts Apple
// Mail into inverting, so without a complete dark stylesheet it makes matters
// worse in a client that currently renders this template exactly as designed.
// The third tier is served by the palette instead — mid-tone borders and no
// pure white on pure black, so an inversion rela cannot intercept still lands
// somewhere legible.
//
// Adding the meta tag is not a free win; it is the trap. See TKT-1GA2PG.
//
// # .prose scopes the markdown table styling
//
// Table rules for markdown content are keyed on .prose, not on .pad, because
// the section scaffolding is itself made of tables living inside .pad. A
// ".pad table" rule therefore hits the layout tables too and paints borders and
// cell padding onto the scaffolding. .prose marks the three places sanitized
// markdown lands (intro, section body, footer) so the styling reaches author
// content and nothing else. Do not rewrite these selectors in terms of .pad.
//
// The <style> block is what douceur inlines; presentation attributes and the mso
// conditionals survive inlining untouched (verified).
//
// The mso conditionals arrive as .MSOOpen/.MSOClose rather than as literal
// comments in this text, because html/template STRIPS HTML comments at parse
// time — a documented behavior that silently deleted the Outlook fallbacks
// when they were written inline here. Switching to text/template would fix it
// too, but at the cost of contextual escaping for every interpolated value,
// which is not a trade worth making for two constant strings.
// columns would split single rules across lines and make the stylesheet harder
// to read than the long lines it replaces.
//
//nolint:lll // the <style> block holds CSS declarations; wrapping them at 120
var docTemplate = template.Must(template.New("mail").Parse(`<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" lang="{{.Lang}}" xml:lang="{{.Lang}}">
<head>
<meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1.0" />
<title>{{.Subject}}</title>
<style type="text/css">
body { margin:0; padding:0; background-color:{{.C.bg}}; -webkit-text-size-adjust:100%; -ms-text-size-adjust:100%; }
table { border-collapse:collapse; }
.wrap { background-color:{{.C.bg}}; width:100%; }
.card { background-color:{{.C.card}}; border:1px solid {{.C.border}}; border-radius:6px; }
.bar { background-color:{{.C.accent}}; font-size:0; line-height:0; height:4px; }
.pad { padding:24px; font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif; font-size:15px; line-height:1.5; color:{{.C.text}}; }
.pad h1 { font-size:20px; line-height:1.3; margin:0 0 12px 0; color:{{.C.heading}}; font-weight:600; }
.pad h2 { font-size:17px; line-height:1.3; margin:20px 0 8px 0; color:{{.C.heading}}; font-weight:600; }
.pad h3 { font-size:15px; margin:16px 0 6px 0; color:{{.C.heading}}; font-weight:600; }
.pad p { margin:0 0 12px 0; }
.pad a { color:{{.C.accent}}; text-decoration:underline; }
.pad ul, .pad ol { margin:0 0 12px 20px; padding-left:0; }
.sect { width:100%; }
.sect-title { font-size:17px; font-weight:600; color:{{.C.heading}}; padding:20px 0 8px 0; font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif; }
.tbl { width:100%; font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif; font-size:14px; color:{{.C.text}}; }
.gap { font-size:0; line-height:0; height:12px; }
.th { text-align:left; padding:8px 10px; border-bottom:2px solid {{.C.border}}; font-weight:600; color:{{.C.muted}}; font-size:12px; text-transform:uppercase; letter-spacing:0.03em; }
.td { padding:8px 10px; border-bottom:1px solid {{.C.border}}; vertical-align:top; }
.prose p { margin:0 0 12px 0; }
.prose table { width:100%; font-size:14px; margin:0 0 12px 0; }
.prose th { text-align:left; padding:8px 10px; border-bottom:2px solid {{.C.border}}; font-weight:600; color:{{.C.muted}}; font-size:12px; text-transform:uppercase; letter-spacing:0.03em; }
.prose td { padding:8px 10px; border-bottom:1px solid {{.C.border}}; vertical-align:top; }
.foot { padding:16px 24px 24px 24px; font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif; font-size:12px; line-height:1.5; color:{{.C.muted}}; }
.foot a { color:{{.C.muted}}; }
.logo { display:block; border:0; outline:none; text-decoration:none; max-height:32px; }
.empty { color:{{.C.muted}}; font-style:italic; padding:8px 0; font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif; font-size:14px; }
@media (prefers-color-scheme: dark) {
.wrap { background-color:{{.C.darkBg}} !important; }
.outer { background-color:{{.C.darkBg}} !important; }
.card { background-color:{{.C.darkCard}} !important; border-color:{{.C.darkBorder}} !important; }
.bar { background-color:{{.C.darkAccent}} !important; }
.pad, .pad p, .pad li, .pad td, .tbl, .td { color:{{.C.darkText}} !important; }
.pad h1, .pad h2, .pad h3, .sect-title { color:{{.C.darkHeading}} !important; }
.pad a { color:{{.C.darkAccent}} !important; }
.th, .foot, .foot a, .empty { color:{{.C.darkMuted}} !important; }
.th, .td { border-color:{{.C.darkBorder}} !important; }
.card, .pad, .foot, .sect-title, .empty { background-color:{{.C.darkCard}} !important; }
}
</style>
</head>
<body>
<table class="wrap" role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%">
<tr><td class="outer" align="center" style="padding:24px 12px;">
{{.MSOOpen}}
<table class="card" role="presentation" cellpadding="0" cellspacing="0" border="0" width="{{.Width}}" style="width:{{.Width}}px; max-width:100%;">
<tr><td class="bar" height="4">&nbsp;</td></tr>
{{if .LogoCID}}<tr><td style="padding:20px 24px 0 24px;"><img class="logo" src="cid:{{.LogoCID}}" alt="{{.LogoAlt}}"{{if .LogoWidth}} width="{{.LogoWidth}}"{{end}}{{if .LogoHeight}} height="{{.LogoHeight}}"{{end}} /></td></tr>{{end}}
<tr><td class="pad">
<h1>{{.Subject}}</h1>
{{if .Intro}}<div class="prose">{{.Intro}}</div>{{end}}
{{range .Sections}}
<table class="sect" role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%">
{{if .Title}}<tr><td class="sect-title">{{.Title}}</td></tr>{{end}}
{{if .Body}}<tr><td class="prose">{{.Body}}</td></tr>{{end}}
{{if .HasTable}}
<tr><td>
<table class="tbl" cellpadding="0" cellspacing="0" border="0" width="100%">
<tr>{{range .Columns}}<th class="th" scope="col">{{.}}</th>{{end}}</tr>
{{range .Rows}}<tr>{{range .Cells}}<td class="td">{{.}}</td>{{end}}</tr>{{end}}
</table>
</td></tr>
<tr><td class="gap" height="12">&nbsp;</td></tr>
{{else if .Empty}}<tr><td class="empty">{{.Empty}}</td></tr>{{end}}
</table>
{{end}}
</td></tr>
{{if .Footer}}<tr><td class="foot prose">{{.Footer}}</td></tr>{{end}}
</table>
{{.MSOClose}}
</td></tr>
</table>
</body>
</html>`))

// tmplRow is one table row; Cells may carry an anchor, so it is template.HTML —
// built here from escaped parts, never from caller HTML.
type tmplRow struct{ Cells []template.HTML }

type tmplSection struct {
	Title    string
	Body     template.HTML
	Columns  []string
	Rows     []tmplRow
	HasTable bool
	Empty    string
}

type tmplDoc struct {
	Subject    string
	Lang       string
	MSOOpen    template.HTML
	MSOClose   template.HTML
	Intro      template.HTML
	Footer     template.HTML
	Sections   []tmplSection
	LogoCID    string
	LogoAlt    string
	LogoWidth  int
	LogoHeight int
	Width      string
	C          map[string]string
}

// buildDocument assembles the trusted shell around sanitized content.
func (r *Renderer) buildDocument(m *Message) (string, error) {
	intro, err := r.markdownToSafeHTML(m.Intro)
	if err != nil {
		return "", err
	}
	footer, err := r.markdownToSafeHTML(m.Footer)
	if err != nil {
		return "", err
	}

	sections := make([]tmplSection, 0, len(m.Sections))
	for i := range m.Sections {
		s, secErr := r.buildSection(&m.Sections[i])
		if secErr != nil {
			return "", secErr
		}
		sections = append(sections, s)
	}

	alt := r.opts.LogoAlt
	if alt == "" {
		alt = "logo"
	}

	lang, err := r.resolveLang(m.Lang)
	if err != nil {
		return "", err
	}

	doc := tmplDoc{
		Subject:    m.Subject,
		Lang:       lang,
		Intro:      template.HTML(intro),  //nolint:gosec // G203: sanitized by bluemonday in markdownToSafeHTML
		Footer:     template.HTML(footer), //nolint:gosec // G203: sanitized by bluemonday in markdownToSafeHTML
		Sections:   sections,
		LogoCID:    r.opts.LogoCID,
		LogoAlt:    alt,
		LogoWidth:  r.opts.LogoWidth,
		LogoHeight: r.opts.LogoHeight,
		Width:      contentWidth,
		MSOOpen:    msoOpen,
		MSOClose:   msoClose,
		C:          r.colors(),
	}

	var buf strings.Builder
	if err := docTemplate.Execute(&buf, doc); err != nil {
		return "", fmt.Errorf("mailrender: execute template: %w", err)
	}
	return buf.String(), nil
}

// resolveLang picks the language tag for one message: the message's own, else
// the operator default (which New has already defaulted and validated).
//
// The message value is validated HERE rather than at the call sites because it
// arrives from two of them — operator config and untrusted Lua — and validating
// in either one would leave the other open.
func (r *Renderer) resolveLang(msgLang string) (string, error) {
	if msgLang == "" {
		return r.opts.DefaultLang, nil
	}
	if err := ValidateLang(msgLang); err != nil {
		return "", err
	}
	return msgLang, nil
}

// colors maps palette keys to the short names the template uses, so the
// stylesheet stays readable and the CSS-variable spelling lives in one place.
func (r *Renderer) colors() map[string]string {
	return map[string]string{
		"accent":  r.palette["--accent-color"],
		"text":    r.palette["--text-color"],
		"muted":   r.palette["--muted-color"],
		"bg":      r.palette["--bg-color"],
		"card":    r.palette["--card-bg"],
		"border":  r.palette["--border-color"],
		"heading": r.palette["--heading-color"],

		"darkAccent":  r.palette["--dark-accent-color"],
		"darkText":    r.palette["--dark-text-color"],
		"darkMuted":   r.palette["--dark-muted-color"],
		"darkBg":      r.palette["--dark-bg-color"],
		"darkCard":    r.palette["--dark-card-bg"],
		"darkBorder":  r.palette["--dark-border-color"],
		"darkHeading": r.palette["--dark-heading-color"],
	}
}

func (r *Renderer) buildSection(s *Section) (tmplSection, error) {
	body, err := r.markdownToSafeHTML(s.Body)
	if err != nil {
		return tmplSection{}, err
	}

	out := tmplSection{
		Title:   s.Title,
		Body:    template.HTML(body), //nolint:gosec // G203: sanitized by bluemonday in markdownToSafeHTML
		Columns: s.Columns,
	}

	// A section declaring columns but matching nothing renders an empty note
	// rather than a header-only table, which reads as a rendering fault.
	if len(s.Columns) > 0 && len(s.Rows) == 0 {
		out.Empty = "Nothing to show."
		return out, nil
	}
	if len(s.Rows) == 0 {
		return out, nil
	}

	out.HasTable = true
	out.Rows = make([]tmplRow, 0, len(s.Rows))
	for i, row := range s.Rows {
		cells := make([]template.HTML, 0, len(row))
		for j, cell := range row {
			// Cell text is ESCAPED, never markdown-rendered: a cell is a value.
			esc := template.HTMLEscapeString(cell)
			// Only the first column links, and only to a vetted absolute URL.
			if j == 0 && i < len(s.Links) && s.Links[i] != "" {
				if href, ok := r.safeHref(s.Links[i]); ok {
					esc = `<a href="` + template.HTMLEscapeString(href) + `">` + esc + `</a>`
				}
			}
			cells = append(cells, template.HTML(esc)) //nolint:gosec // G203: escaped above; href vetted by safeHref
		}
		// Normalize to the header width. A row is padded when short and
		// TRUNCATED when long, because the two failures are not symmetric: a
		// missing cell leaves a blank in a grid that still lines up, while an
		// extra one widens that row past every other and breaks the table's
		// columns for the whole message. Callers assemble rows from data, so a
		// mismatch is a caller bug — but a misshapen table is a worse way to
		// learn about it than a blank cell.
		for len(cells) < len(s.Columns) {
			cells = append(cells, "")
		}
		out.Rows = append(out.Rows, tmplRow{Cells: cells[:len(s.Columns)]})
	}
	return out, nil
}

// safeHref resolves a link against BaseURL and admits only http/https.
//
// Mail is read outside the app, so a relative href is dead on arrival; and an
// unvetted scheme is how javascript: reaches a recipient. Anything else is
// dropped (the text still renders, just unlinked) rather than passed through.
func (r *Renderer) safeHref(link string) (string, bool) {
	l := strings.TrimSpace(link)
	switch {
	case l == "":
		return "", false
	case strings.HasPrefix(l, "http://"), strings.HasPrefix(l, "https://"):
		return l, true
	case strings.HasPrefix(l, "/"):
		if r.opts.BaseURL == "" {
			return "", false
		}
		return strings.TrimRight(r.opts.BaseURL, "/") + l, true
	default:
		return "", false
	}
}
