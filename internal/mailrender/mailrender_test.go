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
	require.NotContains(t, html, "<style", "the <style> block should have been inlined away")

	// The accent colour reaches the output, so the palette is genuinely wired
	// through rather than the template merely carrying static markup.
	require.Contains(t, strings.ToLower(html), "#4772fb")
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
// so a non-colour must be rejected at the boundary rather than escaped or
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
	require.Contains(t, err.Error(), "not a colour")
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
	require.Equal(t, len([]rune(lines[0])), len([]rune(lines[1])),
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
