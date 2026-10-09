package htmlmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromHTML(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"basecamp divs", `<div>Hello <strong>bold</strong> and <em>it</em><br>line2</div><div><br></div><div>next</div>`,
			"Hello **bold** and *it*  \nline2\n\nnext"},
		{"heading and lists", `<h1>Title</h1><ul><li>a</li><li>b</li></ul>`, "# Title\n\n- a\n- b"},
		{"strikethrough", `<div><del>gone</del></div>`, "~~gone~~"},
		{"link kept", `<a href="https://x.example/a?b=1">x</a>`, "[x](https://x.example/a?b=1)"},
		{"javascript link dropped", `<a href="javascript:alert(1)">js</a>`, "js"},
		{"script dropped", `<script>alert(1)</script>ok`, "ok"},
		{"image dropped", `<img src="https://t.example/p.gif">ok`, "ok"},
		{"attachment dropped", `<bc-attachment sgid="x" url="https://u/x.png"></bc-attachment>ok`, "ok"},
		{"markdown in text escaped", `<div>a _b_ [c]</div>`, `a \_b_ \[c]`},
		{"empty", ``, ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FromHTML(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestToHTML(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"paragraphs", "Hello **bold**\n\nnext", "<p>Hello <strong>bold</strong></p>\n<p>next</p>"},
		{"strikethrough", "~~gone~~", "<p><del>gone</del></p>"},
		{"raw html not passed", "<script>alert(1)</script>\n\nok", "<p>ok</p>"},
		{"javascript link dropped", "[js](javascript:alert(1))", "<p>js</p>"},
		{"image dropped", "![a](https://t.example/p.gif)", "<p></p>"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ToHTML(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestRoundTripStable pins the property sync loop prevention needs: once a
// body has crossed in one direction, converting it back and forth again
// changes nothing.
func TestRoundTripStable(t *testing.T) {
	htmls := []string{
		`<div>Hello <strong>bold</strong> and <em>it</em><br>line2</div><div><br></div><div>para <a href="https://x.example/?a=1&amp;b=2">link</a></div>`,
		`<h1>Title</h1><ul><li>a</li><li>b <del>gone</del></li></ul><ol><li>one</li><li>two</li></ol>`,
		`<ul><li>a</li></ul><ul><li>b</li></ul>`,
		`<pre>code
  indented *not em*</pre><blockquote>quote <em>x</em></blockquote><hr>`,
		`<div>a * b _c_ [d] &lt;tag&gt; #notheading 1. notlist</div>`,
		`<div>nested <strong><em>both</em></strong> and <a href="mailto:a@b.example">mail</a></div>`,
		`<ul><li>outer<ul><li>inner</li></ul></li></ul>`,
	}
	for _, h := range htmls {
		md1, err := FromHTML(h)
		require.NoError(t, err)
		html2, err := ToHTML(md1)
		require.NoError(t, err)
		md2, err := FromHTML(html2)
		require.NoError(t, err)
		require.Equal(t, md1, md2, "html %q", h)
	}
	mds := []string{
		"# T\n\nsome *em* and **strong** text\n\n- a\n- b\n\n1. x\n2. y",
		"line one\nline two",
		"> quoted\n\n```\ncode\n```",
		"a_b_c and 2 * 3",
	}
	for _, md := range mds {
		h1, err := ToHTML(md)
		require.NoError(t, err)
		md1, err := FromHTML(h1)
		require.NoError(t, err)
		h2, err := ToHTML(md1)
		require.NoError(t, err)
		md2, err := FromHTML(h2)
		require.NoError(t, err)
		require.Equal(t, md1, md2, "markdown %q", md)
	}
}
