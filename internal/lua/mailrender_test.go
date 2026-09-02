package lua

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/mailrender"
)

// baseURLSender is a sender that also declares a base URL, so the optional
// BaseURLCarrier capability is exercised rather than merely declared.
type baseURLSender struct {
	MailSender
	base string
}

func (s baseURLSender) MailBaseURL() string { return s.base }

// runRender executes a script and returns whatever it assigned to the globals
// `h` and `t`, so a test can assert on rendered output without the binding
// needing a Go-visible return path.
func runRender(t *testing.T, rt *Runtime, script string) (html, text string) {
	t.Helper()
	require.NoError(t, rt.RunString(script))
	return rt.L.GetGlobal("h").String(), rt.L.GetGlobal("t").String()
}

// TestMailRender_MatchesDirectRenderer is the load-bearing integration test.
//
// It pins the Lua path to the Go path BYTE FOR BYTE. Without it the binding
// could drift — a field silently dropped in conversion, an option not carried —
// and the only symptom would be mail that looks subtly wrong, which no unit
// test on either side would catch. With it, a change to mailrender that the
// binding fails to track breaks the build.
func TestMailRender_MatchesDirectRenderer(t *testing.T) {
	t.Parallel()

	rt, _ := newMailRuntime(t, baseURLSender{&recordingMailSender{}, "https://app.example"})
	gotHTML, gotText := runRender(t, rt, `
h, t = mail.render{
  subject = "Weekly digest",
  lang = "nl",
  intro = "You have **3** items.",
  sections = {
    { title = "Overdue", body = "These are _past_ due.",
      columns = {"Title", "Due"},
      rows = {{"Ship the thing", "2026-08-01"}},
      links = {"https://app.example/e/TKT-1"} },
    { title = "Empty", columns = {"Title"} },
  },
  footer = "Sent by rela.",
}`)

	r, err := mailrender.New(&mailrender.Options{BaseURL: "https://app.example"})
	require.NoError(t, err)
	wantHTML, wantText, err := r.Render(&mailrender.Message{
		Subject: "Weekly digest",
		Lang:    "nl",
		Intro:   "You have **3** items.",
		Sections: []mailrender.Section{{
			Title:   "Overdue",
			Body:    "These are _past_ due.",
			Columns: []string{"Title", "Due"},
			Rows:    [][]string{{"Ship the thing", "2026-08-01"}},
			Links:   []string{"https://app.example/e/TKT-1"},
		}, {
			Title:   "Empty",
			Columns: []string{"Title"},
		}},
		Footer: "Sent by rela.",
	})
	require.NoError(t, err)

	require.Equal(t, string(wantHTML), gotHTML, "the Lua path must render exactly what mailrender does")
	require.Equal(t, string(wantText), gotText)
}

// TestMailRender_SanitizesUntrustedContent pins that script-supplied markdown
// is treated exactly like entity content.
//
// A script is not more trusted than an entity body: both are author-supplied
// and both reach a recipient's mail client.
func TestMailRender_SanitizesUntrustedContent(t *testing.T) {
	t.Parallel()

	rt, _ := newMailRuntime(t, &recordingMailSender{})
	html, text := runRender(t, rt, `
local hostile = "<script>alert(1)</script>" ..
  "<img src=x onerror=\"steal()\">" ..
  "<a href=\"javascript:bad()\">click</a>" ..
  "<iframe src=\"https://evil.example\"></iframe>"
h, t = mail.render{
  subject = "Digest",
  intro = hostile,
  sections = {{ title = "S", body = hostile }},
  footer = hostile,
}`)

	for _, bad := range []string{"<script", "onerror", "javascript:", "<iframe", "steal()", "alert(1)"} {
		assert.NotContains(t, html, bad, "hostile token %q survived into HTML", bad)
		assert.NotContains(t, text, bad, "hostile token %q survived into text", bad)
	}
}

// TestMailRender_KeepsInlineStyles is the ordering-inversion canary from
// TKT-332QZY, re-asserted on this path.
//
// Sanitizing the assembled document instead of the content fragment still
// strips every hostile token above, so the sanitization test alone would keep
// passing while the mail shipped unstyled. Only an assertion that styling
// SURVIVED distinguishes the two.
func TestMailRender_KeepsInlineStyles(t *testing.T) {
	t.Parallel()

	rt, _ := newMailRuntime(t, &recordingMailSender{})
	html, _ := runRender(t, rt, `h, t = mail.render{subject = "S", intro = "hi"}`)

	require.Contains(t, html, `style="`, "no inline styles: CSS inlining did not run on this path")
	require.Contains(t, html, "<table", "the branded table layout is missing")
}

// TestMailRender_LinkSafety covers the href allowlist on script-supplied links.
//
// The text must survive in every case: a dropped link should cost the hyperlink,
// never the row.
func TestMailRender_LinkSafety(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		link     string
		wantHref bool
	}{
		{"absolute https", "https://app.example/x", true},
		{"root relative resolves against base", "/e/TKT-1", true},
		{"javascript scheme", "javascript:alert(1)", false},
		{"data scheme", "data:text/html,x", false},
		{"bare word", "not-a-url", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rt, _ := newMailRuntime(t, baseURLSender{&recordingMailSender{}, "https://app.example"})
			html, _ := runRender(t, rt, `
h, t = mail.render{
  subject = "S",
  sections = {{ columns = {"Title"}, rows = {{"CELLTEXT"}}, links = {"`+tc.link+`"} }},
}`)

			require.Contains(t, html, "CELLTEXT", "cell text must render whether or not the link is kept")
			if tc.wantHref {
				assert.Contains(t, html, "<a href=", "a vetted link should render as an anchor")
			} else {
				assert.NotContains(t, html, "<a href=", "an unvetted link must not become an anchor")
				assert.NotContains(t, html, tc.link)
			}
		})
	}
}

// TestMailRender_NoBaseURLDropsRelativeLinks pins the fail-safe direction of the
// optional BaseURLCarrier: a sender that cannot answer costs a hyperlink, never
// a broken or forged one.
func TestMailRender_NoBaseURLDropsRelativeLinks(t *testing.T) {
	t.Parallel()

	rt, _ := newMailRuntime(t, &recordingMailSender{}) // plain sender: no carrier
	html, _ := runRender(t, rt, `
h, t = mail.render{
  subject = "S",
  sections = {{ columns = {"Title"}, rows = {{"CELLTEXT"}}, links = {"/e/TKT-1"} }},
}`)

	require.Contains(t, html, "CELLTEXT")
	require.NotContains(t, html, "<a href=", "a relative link with no base must render unlinked")
}

// TestMailRender_WorksWithoutMailConfigured covers the registration policy.
//
// mail.render is pure formatting, so refusing it when no transport is wired
// would answer a question the caller never asked. A script may legitimately
// render a message to log or inspect it.
func TestMailRender_WorksWithoutMailConfigured(t *testing.T) {
	t.Parallel()

	rt, _ := newMailRuntime(t, nil) // no sender at all
	html, text := runRender(t, rt, `h, t = mail.render{subject = "S", intro = "hi"}`)

	require.Contains(t, html, "<table")
	require.Contains(t, text, "hi")
}

// TestMailRender_ComposesWithSend is the end-to-end shape the binding exists
// for: render, then send what was rendered.
func TestMailRender_ComposesWithSend(t *testing.T) {
	t.Parallel()

	sender := &recordingMailSender{}
	rt, _ := newMailRuntime(t, sender)

	require.NoError(t, rt.RunString(`
local html, text = mail.render{
  subject = "Wekelijks MT",
  lang = "nl",
  sections = {{ title = "Open acties", columns = {"Taak"}, rows = {{"Doe dit"}} }},
}
local ok, err = mail.send{to = "maaike@example.nl", subject = "Wekelijks MT", html = html, text = text}
assert(err == nil, "unexpected error")
assert(ok == true, "expected true")`))

	got := sender.messages()
	require.Len(t, got, 1)
	assert.Contains(t, got[0].HTML, `lang="nl"`)
	assert.Contains(t, got[0].HTML, "Doe dit")
	assert.NotEmpty(t, got[0].Text, "the text alternative must be carried through")
}

// TestMailRender_MalformedCallRaises covers the error convention.
//
// Every failure here is a malformed argument — nothing touches the network — so
// it RAISES rather than returning an error table. That is the opposite of
// mail.send, where a delivery failure is a fact about the world the script may
// reasonably handle.
func TestMailRender_MalformedCallRaises(t *testing.T) {
	t.Parallel()

	tests := map[string]struct{ script, want string }{
		"missing subject":    {`mail.render{}`, "subject must be a string"},
		"numeric subject":    {`mail.render{subject = 5}`, "subject must be a string"},
		"numeric intro":      {`mail.render{subject = "S", intro = 5}`, "intro must be a string"},
		"sections not table": {`mail.render{subject = "S", sections = "x"}`, "sections must be a table"},
		"section not table":  {`mail.render{subject = "S", sections = {"x"}}`, "sections[1] must be a table"},
		"columns not table":  {`mail.render{subject = "S", sections = {{columns = 5}}}`, "columns must be a table"},
		"non-string column":  {`mail.render{subject = "S", sections = {{columns = {5}}}}`, "columns[1] must be a string"},
		"rows not table":     {`mail.render{subject = "S", sections = {{rows = 5}}}`, "rows must be a table"},
		"non-string cell":    {`mail.render{subject = "S", sections = {{rows = {{5}}}}}`, "must be a string"},
		"invalid lang":       {`mail.render{subject = "S", lang = "en\" onload=x"}`, "language tag"},
		"no argument at all": {`mail.render()`, "table expected"},
		"non-table argument": {`mail.render("x")`, "table expected"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rt, _ := newMailRuntime(t, &recordingMailSender{})
			err := rt.RunString(tc.script)
			require.Error(t, err, "malformed call must raise")
			require.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tc.want))
		})
	}
}

// TestMailRender_TolerantOfOptionalShapes pins the cases that are legitimately
// underspecified rather than wrong, so a script author is not forced to supply
// empty tables to say "nothing here".
func TestMailRender_TolerantOfOptionalShapes(t *testing.T) {
	t.Parallel()

	scripts := map[string]string{
		"no sections":        `h, t = mail.render{subject = "S"}`,
		"empty sections":     `h, t = mail.render{subject = "S", sections = {}}`,
		"section with title": `h, t = mail.render{subject = "S", sections = {{title = "T"}}}`,
		// A ragged row is accepted, not rejected — but it is normalized to the
		// header width rather than rendered as-is; see
		// TestRender_RaggedRowsMatchHeaderWidth for why the two directions are
		// treated differently.
		"ragged rows":   `h, t = mail.render{subject = "S", sections = {{columns = {"a","b"}, rows = {{"1"}}}}}`,
		"fewer links":   `h, t = mail.render{subject = "S", sections = {{columns = {"a"}, rows = {{"1"},{"2"}}, links = {"https://x.example"}}}}`,
		"empty subject": `h, t = mail.render{subject = ""}`,
	}

	for name, script := range scripts {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rt, _ := newMailRuntime(t, &recordingMailSender{})
			html, _ := runRender(t, rt, script)
			require.Contains(t, html, "<table", "a valid but sparse message must still render")
		})
	}
}

// TestMailRender_NoRawHTMLField pins the absence of a sanitizer bypass.
//
// The whole point of this binding is to give authors an alternative to handing
// raw HTML to mail.send. An `html` field here would reintroduce that hole while
// looking like a convenience, so its absence is asserted rather than left to
// reviewer vigilance.
func TestMailRender_NoRawHTMLField(t *testing.T) {
	t.Parallel()

	rt, _ := newMailRuntime(t, &recordingMailSender{})
	html, _ := runRender(t, rt, `
h, t = mail.render{
  subject = "S",
  html = "<div id=\"SMUGGLED\">raw</div>",
  css = "body{background:url(javascript:alert(1))}",
}`)

	require.NotContains(t, html, "SMUGGLED", "a raw html field must not be honored")
	require.NotContains(t, html, "javascript:", "a raw css field must not reach douceur")
}

// TestMailRender_RejectsNilHolesInArrays covers a defect found in code review:
// a nil hole in `rows` emitted a bare <tr></tr> between the real rows, with no
// error.
//
// The cause was a rule leaking out of its context. `stringArray` maps LNil to
// "this optional array was omitted", which is correct for `columns` and
// `links` — both genuinely optional — but meaningless for a row SLOT, where
// absence is not an option but a malformed array. Lua's `#` is undefined on a
// table with a hole, so the length itself cannot be trusted either.
//
// `links` is deliberately NOT in this list: a short or sparse links array means
// "these rows are unlinked", which is documented behavior of buildSection, not
// a malformed input.
func TestMailRender_RejectsNilHolesInArrays(t *testing.T) {
	t.Parallel()

	tests := map[string]struct{ script, want string }{
		"hole in rows": {
			`mail.render{subject="S", sections={{columns={"a"}, rows={{"x"},nil,{"z"}}}}}`,
			"rows[2] is missing",
		},
		"hole in sections": {
			`mail.render{subject="S", sections={{title="A"},nil,{title="B"}}}`,
			"sections[2] must be a table",
		},
		"hole in columns": {
			`mail.render{subject="S", sections={{columns={"a",nil,"c"}, rows={{"x"}}}}}`,
			"columns[2] must be a string",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rt, _ := newMailRuntime(t, &recordingMailSender{})
			err := rt.RunString(tc.script)
			require.Error(t, err, "a nil hole must be refused, not rendered as an empty row")
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestMailRender_SparseLinksLeaveRowsUnlinked is the other half of the rule
// above: `links` may legitimately be shorter than `rows`, and the trailing rows
// simply render without a hyperlink.
func TestMailRender_SparseLinksLeaveRowsUnlinked(t *testing.T) {
	t.Parallel()

	rt, _ := newMailRuntime(t, &recordingMailSender{})
	html, _ := runRender(t, rt, `
h, t = mail.render{
  subject = "S",
  sections = {{ columns = {"a"}, rows = {{"LINKED"},{"PLAIN"}},
                links = {"https://app.example/x"} }},
}`)

	require.Contains(t, html, `<a href="https://app.example/x"`)
	require.Contains(t, html, "PLAIN", "a row past the end of links still renders")
	require.NotContains(t, html, "<tr></tr>", "no empty row is emitted")
}
