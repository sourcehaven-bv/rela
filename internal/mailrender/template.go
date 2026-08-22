package mailrender

import (
	"fmt"
	"html/template"
	"strings"
)

// defaultPalette holds the colour tokens the stylesheet reads. Keys match the
// SPA's CSS custom properties (see dataentryconfig.ResolvePalette) so an
// operator palette drops in unchanged; --accent-color's default is the SPA's.
var defaultPalette = map[string]string{
	"--accent-color":  "#4772fb",
	"--text-color":    "#1f2430",
	"--muted-color":   "#6b7280",
	"--bg-color":      "#f4f5f7",
	"--card-bg":       "#ffffff",
	"--border-color":  "#e5e7eb",
	"--heading-color": "#111827",
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
//     colour-validated first. Do not interpolate anything else into CSS.
//
// The <style> block is what douceur inlines; presentation attributes and the mso
// conditionals survive inlining untouched (verified).
//
// The mso conditionals arrive as .MSOOpen/.MSOClose rather than as literal
// comments in this text, because html/template STRIPS HTML comments at parse
// time — a documented behaviour that silently deleted the Outlook fallbacks
// when they were written inline here. Switching to text/template would fix it
// too, but at the cost of contextual escaping for every interpolated value,
// which is not a trade worth making for two constant strings.
var docTemplate = template.Must(template.New("mail").Parse(`<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml">
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
.pad ul, .pad ol { margin:0 0 12px 0; padding-left:20px; }
.sect { margin:0; }
.sect-title { font-size:17px; font-weight:600; color:{{.C.heading}}; padding:20px 0 8px 0; font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif; }
.tbl { width:100%; font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif; font-size:14px; color:{{.C.text}}; }
.th { text-align:left; padding:8px 10px; border-bottom:2px solid {{.C.border}}; font-weight:600; color:{{.C.muted}}; font-size:12px; text-transform:uppercase; letter-spacing:0.03em; }
.td { padding:8px 10px; border-bottom:1px solid {{.C.border}}; vertical-align:top; }
.foot { padding:16px 24px 24px 24px; font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif; font-size:12px; line-height:1.5; color:{{.C.muted}}; }
.foot a { color:{{.C.muted}}; }
.logo { display:block; border:0; outline:none; text-decoration:none; max-height:32px; }
.empty { color:{{.C.muted}}; font-style:italic; padding:8px 0; font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif; font-size:14px; }
</style>
</head>
<body>
<table class="wrap" role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%">
<tr><td align="center" style="padding:24px 12px;">
{{.MSOOpen}}
<table class="card" role="presentation" cellpadding="0" cellspacing="0" border="0" width="{{.Width}}" style="width:{{.Width}}px; max-width:100%;">
<tr><td class="bar" height="4">&nbsp;</td></tr>
{{if .LogoCID}}<tr><td style="padding:20px 24px 0 24px;"><img class="logo" src="cid:{{.LogoCID}}" alt="{{.LogoAlt}}" /></td></tr>{{end}}
<tr><td class="pad">
<h1>{{.Subject}}</h1>
{{if .Intro}}{{.Intro}}{{end}}
{{range .Sections}}
<div class="sect">
{{if .Title}}<div class="sect-title">{{.Title}}</div>{{end}}
{{if .Body}}{{.Body}}{{end}}
{{if .HasTable}}
<table class="tbl" role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%">
<tr>{{range .Columns}}<th class="th">{{.}}</th>{{end}}</tr>
{{range .Rows}}<tr>{{range .Cells}}<td class="td">{{.}}</td>{{end}}</tr>{{end}}
</table>
{{else if .Empty}}<div class="empty">{{.Empty}}</div>{{end}}
</div>
{{end}}
</td></tr>
{{if .Footer}}<tr><td class="foot">{{.Footer}}</td></tr>{{end}}
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
	Subject  string
	MSOOpen  template.HTML
	MSOClose template.HTML
	Intro    template.HTML
	Footer   template.HTML
	Sections []tmplSection
	LogoCID  string
	LogoAlt  string
	Width    string
	C        map[string]string
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

	doc := tmplDoc{
		Subject:  m.Subject,
		Intro:    template.HTML(intro),  //nolint:gosec // G203: sanitized by bluemonday in markdownToSafeHTML
		Footer:   template.HTML(footer), //nolint:gosec // G203: sanitized by bluemonday in markdownToSafeHTML
		Sections: sections,
		LogoCID:  r.opts.LogoCID,
		LogoAlt:  alt,
		Width:    contentWidth,
		MSOOpen:  msoOpen,
		MSOClose: msoClose,
		C:        r.colours(),
	}

	var buf strings.Builder
	if err := docTemplate.Execute(&buf, doc); err != nil {
		return "", fmt.Errorf("mailrender: execute template: %w", err)
	}
	return buf.String(), nil
}

// colours maps palette keys to the short names the template uses, so the
// stylesheet stays readable and the CSS-variable spelling lives in one place.
func (r *Renderer) colours() map[string]string {
	return map[string]string{
		"accent":  r.palette["--accent-color"],
		"text":    r.palette["--text-color"],
		"muted":   r.palette["--muted-color"],
		"bg":      r.palette["--bg-color"],
		"card":    r.palette["--card-bg"],
		"border":  r.palette["--border-color"],
		"heading": r.palette["--heading-color"],
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
		out.Rows = append(out.Rows, tmplRow{Cells: cells})
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
