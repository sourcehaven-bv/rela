package mailrender_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/mailrender"
)

// updateGolden follows the internal/mcp convention: an env var, not a flag, so
// the knob exists only for this package's test binary.
var updateGolden = os.Getenv("UPDATE_GOLDEN") == "1"

func newRenderer(t *testing.T, opts *mailrender.Options) *mailrender.Renderer {
	t.Helper()
	r, err := mailrender.New(opts)
	require.NoError(t, err)
	return r
}

func render(t *testing.T, r *mailrender.Renderer, m *mailrender.Message) (html, text string) {
	t.Helper()
	h, txt, err := r.Render(m)
	require.NoError(t, err)
	return string(h), string(txt)
}

// TestRender_SanitizesUntrustedContent covers AC 6: hostile entity content must
// not survive into the HTML part.
func TestRender_SanitizesUntrustedContent(t *testing.T) {
	t.Parallel()

	hostile := "# Agenda\n\n" +
		"<script>alert(1)</script>\n\n" +
		`<img src=x onerror="steal()">` + "\n\n" +
		`<a href="javascript:bad()">click</a>` + "\n\n" +
		`<p style="background:url(javascript:x)">styled</p>` + "\n\n" +
		`<iframe src="https://evil.example"></iframe>`

	r := newRenderer(t, &mailrender.Options{})
	html, text := render(t, r, &mailrender.Message{
		Subject:  "Digest",
		Intro:    hostile,
		Sections: []mailrender.Section{{Title: "S", Body: hostile}},
		Footer:   hostile,
	})

	for _, bad := range []string{
		"<script", "onerror", "javascript:", "<iframe", "steal()", "alert(1)",
	} {
		require.NotContains(t, html, bad, "hostile token %q survived into HTML", bad)
		require.NotContains(t, text, bad, "hostile token %q survived into text", bad)
	}
}

// TestRender_TextPartStripsEncodedMarkup covers the sanitizer bypass found in
// code review: the text part decoded entities AFTER stripping tags, so
// "&lt;script&gt;" in an entity property was re-materialized as a live tag in
// the delivered body.
//
// The double-encoded case matters too — a single decode/strip pass leaves it
// one decode away from the same outcome.
func TestRender_TextPartStripsEncodedMarkup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
	}{
		{"entity encoded", "&lt;script&gt;alert(1)&lt;/script&gt;"},
		{"double encoded", "&amp;lt;script&amp;gt;alert(1)&amp;lt;/script&amp;gt;"},
		{"numeric entities", "&#60;script&#62;alert(1)&#60;/script&#62;"},
		{"encoded anchor", `&lt;a href=&quot;https://evil.example&quot;&gt;click&lt;/a&gt;`},
		{"raw tag", "<script>alert(1)</script>"},
	}

	r := newRenderer(t, &mailrender.Options{})
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, text := render(t, r, &mailrender.Message{
				Subject: tc.in,
				Sections: []mailrender.Section{{
					Title:   tc.in,
					Columns: []string{"C"},
					Rows:    [][]string{{tc.in}},
				}},
			})
			require.NotContains(t, text, "<script", "live tag reached the text part")
			require.NotContains(t, text, "<a href", "live anchor reached the text part")
		})
	}
}

// TestNew_RejectsHostileBaseURL covers the scheme-allowlist bypass found in
// code review: safeHref rejected a "javascript:" LINK, then concatenated it
// onto an unvalidated BaseURL, so every relative link became a script URL.
func TestNew_RejectsHostileBaseURL(t *testing.T) {
	t.Parallel()

	for _, bad := range []string{
		"javascript:alert(1)//",
		"data:text/html,x",
		"app.example.com",
		"//evil.example",
	} {
		_, err := mailrender.New(&mailrender.Options{BaseURL: bad})
		require.Error(t, err, "BaseURL %q must be refused", bad)
	}

	for _, ok := range []string{"", "https://app.example", "http://localhost:8080"} {
		_, err := mailrender.New(&mailrender.Options{BaseURL: ok})
		require.NoError(t, err, "BaseURL %q must be accepted", ok)
	}
}

// TestRender_LinkWithParensInURL covers the link-parsing bug found in code
// review: taking the first ")" as the terminator truncated any URL containing
// parens and left stray text in the body.
func TestRender_LinkWithParensInURL(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, &mailrender.Options{BaseURL: "https://app.example"})
	_, text := render(t, r, &mailrender.Message{
		Subject: "S",
		Intro:   "see [w](https://en.wikipedia.org/wiki/Foo_(bar)) end",
	})

	require.Contains(t, text, "https://en.wikipedia.org/wiki/Foo_(bar)")
	require.Contains(t, text, "end", "text after the link must survive")
	require.NotContains(t, text, "w)", "no stray paren residue")
}

// TestRender_PartsAgreeOnEmphasis pins that the two alternatives render the
// same content. The text part walks goldmark's AST rather than pattern-matching
// markers, so what it treats as emphasis is exactly what the HTML part does —
// including CommonMark's rule that "5*3*2" IS emphasis and "2 * 3 * 4" is not.
func TestRender_PartsAgreeOnEmphasis(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in           string
		wantHTMLEmph bool
		wantText     string
	}{
		{"cost is 5*3*2 dollars", true, "cost is 532 dollars"},
		{"2 * 3 * 4", false, "2 * 3 * 4"},
		{"a_b_c stays", false, "a_b_c stays"},
		{"**bold** words", true, "bold words"},
	}

	r := newRenderer(t, &mailrender.Options{})
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			html, text := render(t, r, &mailrender.Message{Subject: "S", Intro: tc.in})

			gotEmph := strings.Contains(html, "<em>") || strings.Contains(html, "<strong>")
			require.Equal(t, tc.wantHTMLEmph, gotEmph, "HTML emphasis")
			require.Contains(t, text, tc.wantText, "text part must match what HTML renders")
		})
	}
}

// TestRender_MarkdownStructures exercises the AST walk across block types that
// a real digest body carries.
func TestRender_MarkdownStructures(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, &mailrender.Options{BaseURL: "https://app.example"})
	_, text := render(t, r, &mailrender.Message{
		Subject: "S",
		Intro: "## Heading\n\n- one\n- two\n\n> quoted\n\n" +
			"| a | b |\n|---|---|\n| 1 | 2 |\n\n`code span` and\n\n```\nblock\n```",
	})

	for _, want := range []string{"Heading", "- one", "- two", "> quoted", "a | b", "1 | 2", "code span", "block"} {
		require.Contains(t, text, want)
	}
}

// TestRender_KeepsInlineStyles covers AC 6a — the regression guard for the
// pipeline-ordering inversion.
//
// This is the test that fails if someone sanitizes the ASSEMBLED document
// instead of the content fragment. That change leaves every assertion in
// TestRender_SanitizesUntrustedContent passing (a stricter sanitize still
// strips hostile tokens) while silently shipping unstyled mail, because
// bluemonday removes style attributes. Only an assertion that styling SURVIVED
// can catch it.
func TestRender_KeepsInlineStyles(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, &mailrender.Options{})
	html, _ := render(t, r, &mailrender.Message{
		Subject:  "Styled",
		Sections: []mailrender.Section{{Title: "S", Body: "hello"}},
	})

	require.Contains(t, html, `style="`, "no inline styles: CSS inlining did not run, or a sanitizer ran after it")

	// Every INLINABLE rule must have been consumed. What legitimately remains
	// in <style> is the @media block, which cannot be expressed as a style
	// attribute (TKT-1GA2PG) — so assert that nothing survives OUTSIDE that
	// block, rather than asserting the tag is gone, or this stops catching an
	// inlining failure.
	require.Empty(t, styleRulesOutsideMedia(t, html),
		"an inlinable rule was left in <style>: CSS inlining did not run to completion")

	// The accent color reaches the output, so the palette is genuinely wired
	// through rather than the template merely carrying static markup.
	require.Contains(t, strings.ToLower(html), "#4772fb")
}

// TestRender_StylesMarkdownTables covers a defect found only by LOOKING at the
// rendered mail: a GFM table written in prose emits bare <th>/<td> with no
// class, so the .th/.td rules never matched it and it arrived unpadded and
// collapsed — "col acol b" with the headers run together.
//
// Byte-level assertions all passed; the table was present and correct in
// structure. Only the visual check caught it.
// It runs over ALL THREE places sanitized markdown lands, not just the section
// body. The table styling is keyed on a `.prose` class that has to be applied
// at each site by hand (TKT-1GA2PG), so a missing class on any one of them
// unstyles every table there — and only the golden file would notice, which is
// the artifact most likely to be regenerated on autopilot when it goes red.
func TestRender_StylesMarkdownTables(t *testing.T) {
	t.Parallel()

	const table = "| col a | col b |\n|---|---|\n| 1 | 2 |"

	sites := map[string]mailrender.Message{
		"section body": {Subject: "S", Sections: []mailrender.Section{{Body: table}}},
		"intro":        {Subject: "S", Intro: table},
		"footer":       {Subject: "S", Footer: table},
	}

	r := newRenderer(t, &mailrender.Options{})
	for name, msg := range sites {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			html, _ := render(t, r, &msg)

			// Every cell in a prose table carries padding, so headers cannot
			// run together.
			for _, frag := range []string{"<th style=", "<td style="} {
				require.Contains(t, html, frag, "markdown table cells must be styled")
			}
			// Assert on the table itself. A bare "<td>" also appears inside the
			// mso conditional comment, which is a literal string rather than a
			// rendered cell, so a document-wide check would fail for the wrong
			// reason.
			idx := strings.Index(html, "<thead>")
			require.GreaterOrEqual(t, idx, 0, "no table rendered at all")
			body := html[idx:]
			require.NotContains(t, body, "<th>", "an unstyled header cell means the rules did not match")
			require.NotContains(t, body, "<td>", "an unstyled data cell means the rules did not match")
		})
	}
}

// TestRender_PreservesMSOConditionals covers AC 6b. Outlook's fallbacks live in
// conditional comments, which a sanitizer would drop and which the inliner must
// leave alone.
func TestRender_PreservesMSOConditionals(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, &mailrender.Options{})
	html, _ := render(t, r, &mailrender.Message{Subject: "S"})

	require.Contains(t, html, "[if mso]")
	require.Contains(t, html, "<![endif]-->")
}

// styleAttrRe extracts the value of every style attribute in the output.
var styleAttrRe = regexp.MustCompile(`style="([^"]*)"`)

// TestRender_NoDangerousCSSReachesStyleAttributes covers AC 6c.
//
// douceur validates nothing and runs last, so this asserts on the FINAL output
// rather than on any intermediate stage: whatever ends up in a style attribute
// is what the recipient's mail client parses.
func TestRender_NoDangerousCSSReachesStyleAttributes(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, &mailrender.Options{})
	html, _ := render(t, r, &mailrender.Message{
		Subject: "S",
		Intro:   `<p style="background:url(javascript:alert(1));behavior:url(x.htc)">x</p>`,
		Sections: []mailrender.Section{{
			Body: `<div style="width:expression(alert(1))">y</div>`,
		}},
	})

	for _, m := range styleAttrRe.FindAllStringSubmatch(html, -1) {
		v := strings.ToLower(m[1])
		require.NotContains(t, v, "javascript:")
		require.NotContains(t, v, "behavior:")
		require.NotContains(t, v, "expression(")
	}
}

// TestValidatePalette covers AC 6d: a palette token is interpolated into CSS,
// so a non-color must be rejected at the boundary rather than escaped or
// silently defaulted.
func TestValidatePalette(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		palette map[string]string
		wantErr bool
	}{
		{"nil", nil, false},
		{"empty", map[string]string{}, false},
		{"hex6", map[string]string{"--accent-color": "#4772fb"}, false},
		{"hex3", map[string]string{"--accent-color": "#abc"}, false},
		{"hex8 alpha", map[string]string{"--accent-color": "#4772fbff"}, false},
		{"uppercase hex", map[string]string{"--accent-color": "#ABCDEF"}, false},
		{"named", map[string]string{"--bg-color": "transparent"}, false},
		{"padded", map[string]string{"--bg-color": "  #fff  "}, false},

		{"javascript url", map[string]string{"--accent-color": "url('javascript:alert(1)')"}, true},
		{"expression", map[string]string{"--accent-color": "expression(alert(1))"}, true},
		{"css escape", map[string]string{"--accent-color": "red;}body{background:url(javascript:1)"}, true},
		{"rgb function", map[string]string{"--accent-color": "rgb(1,2,3)"}, true},
		{"arbitrary word", map[string]string{"--accent-color": "chartreuse"}, true},
		{"empty value", map[string]string{"--accent-color": ""}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := mailrender.ValidatePalette(tc.palette)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestNew_RejectsBadPalette pins that the rejection happens at construction, so
// a bad palette cannot reach a render.
func TestNew_RejectsBadPalette(t *testing.T) {
	t.Parallel()

	_, err := mailrender.New(&mailrender.Options{
		Palette: map[string]string{"--accent-color": "url('javascript:alert(1)')"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a color")
}

// TestNew_RejectsNilOptions covers the house rule that constructors reject nil
// required arguments rather than substituting a default.
func TestNew_RejectsNilOptions(t *testing.T) {
	t.Parallel()

	_, err := mailrender.New(nil)
	require.Error(t, err)
}

func TestRender_RejectsNilMessage(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, &mailrender.Options{})
	_, _, err := r.Render(nil)
	require.ErrorIs(t, err, mailrender.ErrNilMessage)
}

// TestRender_AlwaysHasTextPart covers AC 7. A missing text/plain part is both a
// legibility problem and a spam signal, so it must hold for every shape of
// message — including degenerate ones.
func TestRender_AlwaysHasTextPart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  mailrender.Message
	}{
		{"subject only", mailrender.Message{Subject: "Hi"}},
		{"intro only", mailrender.Message{Intro: "hello"}},
		{"empty section", mailrender.Message{Subject: "S", Sections: []mailrender.Section{{}}}},
		{"table", mailrender.Message{Subject: "S", Sections: []mailrender.Section{{
			Columns: []string{"Title"}, Rows: [][]string{{"a"}},
		}}}},
		{"html-only body", mailrender.Message{Subject: "S", Sections: []mailrender.Section{{
			Body: "<div>only html</div>",
		}}}},
	}

	r := newRenderer(t, &mailrender.Options{})
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, text := render(t, r, &tc.msg)
			require.NotEmpty(t, strings.TrimSpace(text))
		})
	}
}

// TestRender_TableCellsAreEscapedNotRendered pins that a cell is a value: markup
// in a cell must not become markup in the output.
func TestRender_TableCellsAreEscapedNotRendered(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, &mailrender.Options{})
	html, text := render(t, r, &mailrender.Message{
		Subject: "S",
		Sections: []mailrender.Section{{
			Columns: []string{"Title"},
			Rows:    [][]string{{`<script>alert(1)</script>`}, {"**not bold**"}},
		}},
	})

	require.NotContains(t, html, "<script>")
	require.Contains(t, html, "&lt;script&gt;")
	// Markdown in a cell stays literal.
	require.Contains(t, html, "**not bold**")
	require.Contains(t, text, "**not bold**")
}

// TestRender_LinkSafety pins safeHref: mail is read outside the app, so a
// relative link without a BaseURL is dead, and a non-http scheme is an attack.
func TestRender_LinkSafety(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseURL string
		link    string
		wantIn  bool
		want    string
	}{
		{"absolute https", "", "https://ok.example/e/1", true, "https://ok.example/e/1"},
		{"relative with base", "https://app.example", "/e/1", true, "https://app.example/e/1"},
		{"relative base trailing slash", "https://app.example/", "/e/1", true, "https://app.example/e/1"},
		{"relative without base", "", "/e/1", false, ""},
		{"javascript scheme", "", "javascript:alert(1)", false, ""},
		{"data scheme", "", "data:text/html,x", false, ""},
		{"empty", "", "", false, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, &mailrender.Options{BaseURL: tc.baseURL})
			html, _ := render(t, r, &mailrender.Message{
				Subject: "S",
				Sections: []mailrender.Section{{
					Columns: []string{"Title"},
					Rows:    [][]string{{"Row"}},
					Links:   []string{tc.link},
				}},
			})
			if tc.wantIn {
				require.Contains(t, html, `href="`+tc.want+`"`)
				return
			}
			require.NotContains(t, html, "<a href=")
		})
	}
}

// TestRender_TextPartResolvesLinks pins that the text alternative gets the same
// link treatment the HTML does. A relative href left as "/board" is meaningless
// in a mail client, and an unsafe scheme must be dropped from both parts.
func TestRender_TextPartResolvesLinks(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, &mailrender.Options{BaseURL: "https://app.example"})
	_, text := render(t, r, &mailrender.Message{
		Subject: "S",
		Intro:   "See [the board](/board) and [docs](https://docs.example).",
		Footer:  "Bad [link](javascript:alert(1)).",
	})

	require.Contains(t, text, "https://app.example/board", "relative link must resolve against BaseURL")
	require.Contains(t, text, "https://docs.example", "absolute link survives")
	require.NotContains(t, text, "(/board)", "no bare relative link may remain")
	require.NotContains(t, text, "javascript:", "unsafe scheme is dropped from the text part too")
}

// TestRender_EmptySectionRendersNote pins that a section matching nothing reads
// as "nothing to show" rather than a header-only table, which looks broken.
func TestRender_EmptySectionRendersNote(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, &mailrender.Options{})
	html, text := render(t, r, &mailrender.Message{
		Subject:  "S",
		Sections: []mailrender.Section{{Title: "Overdue", Columns: []string{"Title", "Due"}}},
	})

	require.Contains(t, html, "Nothing to show.")
	require.NotContains(t, html, "<th", "header-only table should not render")
	require.Contains(t, text, "Nothing to show.")
}

// TestRender_UnicodeSubjectAndBody guards the non-ASCII path end to end. The
// text part underlines by RUNE count, so a multi-byte heading must not produce
// an over-long rule.
func TestRender_UnicodeSubjectAndBody(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, &mailrender.Options{})
	html, text := render(t, r, &mailrender.Message{
		Subject: "Tâches — 期限切れ",
		Intro:   "Café ☕ naïve — 日本語のテキスト",
	})

	require.Contains(t, html, "Tâches")
	require.Contains(t, text, "Tâches")
	require.Contains(t, text, "日本語のテキスト")

	lines := strings.Split(strings.TrimSpace(text), "\n")
	require.GreaterOrEqual(t, len(lines), 2)
	require.Len(t, []rune(lines[1]), len([]rune(lines[0])),
		"underline should match the heading's rune width")
}

// TestRender_LogoCID pins that a logo is referenced by CID, never by URL: the
// stored logo sits behind the auth gate, so a mail client cannot fetch it.
func TestRender_LogoCID(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, &mailrender.Options{LogoCID: "logo@rela", LogoAlt: "Acme"})
	html, _ := render(t, r, &mailrender.Message{Subject: "S"})

	require.Contains(t, html, `src="cid:logo@rela"`)
	require.Contains(t, html, `alt="Acme"`)

	// Absent CID renders no img at all rather than a broken one.
	r2 := newRenderer(t, &mailrender.Options{})
	html2, _ := render(t, r2, &mailrender.Message{Subject: "S"})
	require.NotContains(t, html2, "<img")
}

// TestRender_Golden covers AC 5. Nondeterminism is normalized at capture time
// rather than by weakening the comparison; this renderer is deterministic, so
// nothing needs normalizing today — if that changes, normalize here.
func TestRender_Golden(t *testing.T) {
	t.Parallel()

	msg := &mailrender.Message{
		Subject: "Tasks due today",
		Intro:   "You have **3** items needing attention. See [the board](https://app.example/board).",
		Sections: []mailrender.Section{
			{
				Title:   "Overdue",
				Body:    "These are _past_ their due date.",
				Columns: []string{"Title", "Due", "Owner"},
				Rows: [][]string{
					{"Ship the thing", "2026-08-01", "jeroen"},
					{"Review PR #1390", "2026-08-14", "alex"},
				},
				Links: []string{"/e/TKT-1", "https://app.example/e/TKT-2"},
			},
			{
				Title: "Notes",
				Body:  "## Heading\n\n- one\n- two\n\n| a | b |\n|---|---|\n| 1 | 2 |",
			},
			{Title: "Empty", Columns: []string{"Title"}},
		},
		Footer: "Sent by rela.",
	}

	r := newRenderer(t, &mailrender.Options{
		BaseURL: "https://app.example",
		LogoCID: "logo@rela",
		Palette: map[string]string{"--accent-color": "#2b6cb0"},
	})
	html, text := render(t, r, msg)

	checkGolden(t, "digest.golden.html", []byte(html))
	checkGolden(t, "digest.golden.txt", []byte(text))
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()

	path := filepath.Join("testdata", name)
	if updateGolden {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o644))
		t.Logf("wrote %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v\n\nFirst run? capture it with:\n"+
			"    UPDATE_GOLDEN=1 go test ./internal/mailrender -run TestRender_Golden", path, err)
	}

	// Byte-exact, with no trailing-newline fixup on either side: the text part
	// legitimately ends in a blank line, and normalizing that away would let a
	// real change in trailing whitespace pass unnoticed.
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from the committed golden.\n"+
			"Review the diff and record the accepted delta on the ticket before "+
			"regenerating — never regenerate just to make it green.\n"+
			"To accept: UPDATE_GOLDEN=1 go test ./internal/mailrender -run TestRender_Golden", name)
	}
}

// styleRulesOutsideMedia returns whatever CSS survives in the <style> block
// after the @media at-rules are removed.
//
// douceur cannot inline an at-rule, so the dark-mode block legitimately remains
// in <head>. Anything ELSE left behind means inlining did not run to completion,
// which is the failure TestRender_KeepsInlineStyles exists to catch.
func styleRulesOutsideMedia(t *testing.T, html string) string {
	t.Helper()

	start := strings.Index(html, "<style")
	if start < 0 {
		return ""
	}
	open := strings.Index(html[start:], ">")
	end := strings.Index(html, "</style>")
	if open < 0 || end < 0 {
		t.Fatalf("malformed <style> block")
	}
	css := html[start+open+1 : end]

	// Drop each @media block by walking its braces; the rules inside are
	// nested, so a regexp would stop at the first closing brace.
	var out strings.Builder
	for {
		at := strings.Index(css, "@media")
		if at < 0 {
			out.WriteString(css)
			break
		}
		out.WriteString(css[:at])
		rest := css[at:]
		depth, i := 0, 0
		for ; i < len(rest); i++ {
			switch rest[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					i++
					goto done
				}
			}
		}
	done:
		if i >= len(rest) {
			break
		}
		css = rest[i:]
	}
	return strings.TrimSpace(out.String())
}

// TestRender_LangIsPerMessage is the regression guard for the design decision
// in TKT-1GA2PG: language is CONTENT, so it lives on Message.
//
// The load-bearing case is the first one. A single Renderer is built once per
// deployment from mail config, so if the language ever migrates onto Options,
// every message an instance sends gets one language — and a Dutch digest and an
// English one cannot both be right. Rendering twice from ONE renderer is what
// makes that regression fail here instead of in someone's inbox.
func TestRender_LangIsPerMessage(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, &mailrender.Options{DefaultLang: "en"})

	nl, _ := render(t, r, &mailrender.Message{Subject: "Agenda", Lang: "nl"})
	en, _ := render(t, r, &mailrender.Message{Subject: "Digest", Lang: "en-GB"})

	require.Contains(t, nl, `lang="nl"`)
	require.Contains(t, en, `lang="en-GB"`)
	require.NotContains(t, nl, `lang="en-GB"`, "one renderer must not force one language on every message")
}

// TestRender_LangFallsBackToDefault covers the resolution order and, in the
// last case, that an absent language never becomes an EMPTY attribute — an
// empty lang is a worse claim than a wrong one.
func TestRender_LangFallsBackToDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		defaultLang string
		msgLang     string
		want        string
	}{
		{"message wins", "en", "nl", `lang="nl"`},
		{"falls back to operator default", "nl", "", `lang="nl"`},
		{"falls back to en when nothing is set", "", "", `lang="en"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, &mailrender.Options{DefaultLang: tc.defaultLang})
			html, _ := render(t, r, &mailrender.Message{Subject: "S", Lang: tc.msgLang})

			require.Contains(t, html, tc.want)
			require.NotContains(t, html, `lang=""`, "an empty lang attribute is never correct")
		})
	}
}

// TestRender_RejectsHostileLang covers the attribute-breakout case.
//
// A language tag is interpolated into markup, so it is validated and REJECTED
// rather than escaped: escaping a malformed tag yields a well-formed document
// that lies about its language, which is not an improvement.
func TestRender_RejectsHostileLang(t *testing.T) {
	t.Parallel()

	bad := []string{
		`en" onload="alert(1)`,
		`<script>`,
		"../../etc/passwd",
		"en_US", // underscore is not a BCP-47 separator
		strings.Repeat("a", 64),
		"nl\nen",
	}

	r := newRenderer(t, &mailrender.Options{})
	for _, tc := range bad {
		t.Run(tc, func(t *testing.T) {
			t.Parallel()
			_, _, err := r.Render(&mailrender.Message{Subject: "S", Lang: tc})
			require.Error(t, err, "hostile lang %q must be refused", tc)
		})
	}

	// A bad operator default is refused at construction rather than at send,
	// so a misconfigured deployment fails fast.
	_, err := mailrender.New(&mailrender.Options{DefaultLang: `en" x="`})
	require.Error(t, err)

	for _, good := range []string{"en", "nl-NL", "zh-Hant-TW", "de"} {
		_, _, err := r.Render(&mailrender.Message{Subject: "S", Lang: good})
		require.NoError(t, err, "valid tag %q must be accepted", good)
	}
}

// TestRender_FooterParagraphsAreSpaced covers a defect found in code review and
// pre-dating this ticket: `.pad p` never reached the footer, because `.foot` is
// a sibling cell rather than a descendant of `.pad`. A multi-paragraph footer
// therefore rendered with its paragraphs jammed together.
//
// The `.prose` class introduced for table scoping is the natural fix — it marks
// exactly the three places author markdown lands, so one `.prose p` rule spaces
// all of them uniformly instead of `.pad` covering two and missing the third.
func TestRender_FooterParagraphsAreSpaced(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, &mailrender.Options{})
	html, _ := render(t, r, &mailrender.Message{
		Subject: "S",
		Intro:   "intro one\n\nintro two",
		Footer:  "footer one\n\nfooter two",
	})

	// A bare <p> means no rule matched. Every rendered paragraph must carry a
	// margin, wherever in the document it landed.
	require.NotContains(t, html, "<p>",
		"an unstyled paragraph means .prose p did not reach every markdown site")
	require.Equal(t, 4, strings.Count(html, `<p style="margin: 0 0 12px 0;">`),
		"all four paragraphs (two intro, two footer) must be spaced")
}

// TestValidatePalette_RejectsUnknownKeys covers a diagnostic gap found in code
// review: an unrecognized token passed validation and was then silently dropped
// by colors().
//
// The failure mode is quiet and confusing rather than dangerous — the value is
// still color-checked, so nothing unsafe reaches CSS. But an operator who typos
// "--dark-card-color" for "--dark-card-bg" sees the default they explicitly
// tried to override, with nothing anywhere to explain why. The token set is
// closed and small, so naming the valid keys is free.
func TestValidatePalette_RejectsUnknownKeys(t *testing.T) {
	t.Parallel()

	err := mailrender.ValidatePalette(map[string]string{"--dark-card-color": "#222222"})
	require.Error(t, err, "an unknown token must not be silently ignored")
	require.Contains(t, err.Error(), "not a known token")
	require.Contains(t, err.Error(), "--dark-card-bg", "the error should name the valid keys")

	// Every key the template actually reads must be accepted, or this check
	// would reject a legitimate override.
	for _, k := range []string{
		"--accent-color", "--text-color", "--muted-color", "--bg-color",
		"--card-bg", "--border-color", "--heading-color",
		"--dark-accent-color", "--dark-text-color", "--dark-muted-color",
		"--dark-bg-color", "--dark-card-bg", "--dark-border-color",
		"--dark-heading-color",
	} {
		require.NoError(t, mailrender.ValidatePalette(map[string]string{k: "#123456"}),
			"token %q is read by the template and must validate", k)
	}
}

// TestRender_RaggedRowsMatchHeaderWidth covers a finding from code review: a
// row with the wrong cell count was rendered as-is.
//
// The two directions are not symmetric, which is why they get different
// treatment. A SHORT row leaves a blank in a grid that still lines up; a LONG
// row widens itself past every other row and breaks the column alignment of the
// whole table. So short rows are padded and long rows truncated — a caller that
// hands over a mismatched row has a bug either way, but a misshapen table is a
// worse way to find out than an empty cell.
func TestRender_RaggedRowsMatchHeaderWidth(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, &mailrender.Options{})
	html, _ := render(t, r, &mailrender.Message{
		Subject: "S",
		Sections: []mailrender.Section{{
			Columns: []string{"a", "b"},
			Rows: [][]string{
				{"short"},
				{"just", "right"},
				{"too", "many", "OVERFLOW"},
				{},
			},
		}},
	})

	// Every body row carries exactly as many cells as there are columns.
	// Matched on class="td", which is the data-cell class — the bar and gap
	// spacer rows are also <tr><td>, and counting those would test the
	// scaffolding rather than the rows.
	rowRe := regexp.MustCompile(`<tr>((?:<td class="td"[^>]*>.*?</td>)+)</tr>`)
	cellRe := regexp.MustCompile(`<td class="td"`)
	rows := rowRe.FindAllStringSubmatch(html, -1)
	require.Len(t, rows, 4, "every row must render, including the empty one")
	for i, row := range rows {
		require.Len(t, cellRe.FindAllString(row[1], -1), 2,
			"row %d does not match the 2-column header", i)
	}

	require.NotContains(t, html, "OVERFLOW", "a cell past the header width is dropped")
	require.Contains(t, html, "short", "a short row still renders its real cells")
}
