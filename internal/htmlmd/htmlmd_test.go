package htmlmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromHTML(t *testing.T) {
	cases := []struct {
		name, in, want string
		lossy          bool
	}{
		{name: "basecamp divs", in: `<div>Hello <strong>bold</strong> and <em>it</em><br>line2</div><div><br></div><div>next</div>`,
			want: "Hello **bold** and *it*  \nline2\n\nnext"},
		{name: "heading and lists", in: `<h1>Title</h1><ul><li>a</li><li>b</li></ul>`, want: "# Title\n\n- a\n- b"},
		{name: "strikethrough", in: `<div><del>gone</del></div>`, want: "~~gone~~"},
		{name: "adjacent lists merge", in: `<ul><li>a</li></ul><ul><li>b</li></ul>`, want: "- a\n- b"},
		{name: "table", in: `<table><tr><th>a</th><th>b</th></tr><tr><td>1</td><td>2</td></tr></table>`,
			want: "| a | b |\n|---|---|\n| 1 | 2 |"},
		{name: "link kept", in: `<a href="https://x.example/a?b=1">x</a>`, want: "[x](https://x.example/a?b=1)"},
		{name: "javascript link dropped", in: `<a href="javascript:alert(1)">js</a>`, want: "js", lossy: true},
		{name: "script dropped", in: `<script>alert(1)</script>ok`, want: "ok", lossy: true},
		{name: "image dropped", in: `<img src="https://t.example/p.gif">ok`, want: "ok", lossy: true},
		{name: "attachment dropped", in: `<bc-attachment sgid="x" url="https://u/x.png"></bc-attachment>ok`,
			want: "ok", lossy: true},
		{name: "style is not a loss", in: `<div style="color:red">ok</div>`, want: "ok"},
		{name: "markdown in text escaped", in: `<div>a _b_ [c]</div>`, want: `a \_b_ \[c]`},
		{name: "entity-like text stays text", in: `<div>&amp;copy; &amp;nbsp; &amp;#39; a &amp; b</div>`,
			want: `\&copy; \&nbsp; \&#39; a & b`},
		{name: "empty", in: ``, want: ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, lossy, err := FromHTML(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
			require.Equal(t, tc.lossy, lossy)
		})
	}
}

func TestToHTML(t *testing.T) {
	cases := []struct {
		name, in, want string
		lossy          bool
	}{
		{name: "paragraphs", in: "Hello **bold**\n\nnext", want: "<p>Hello <strong>bold</strong></p>\n<p>next</p>"},
		{name: "strikethrough", in: "~~gone~~", want: "<p><del>gone</del></p>"},
		{name: "table", in: "| a |\n|---|\n| 1 |",
			want: "<table>\n<thead>\n<tr>\n<th>a</th>\n</tr>\n</thead>\n<tbody>\n<tr>\n<td>1</td>\n</tr>\n</tbody>\n</table>"},
		{name: "raw html dropped", in: "<script>alert(1)</script>\n\nok", want: "<p>ok</p>", lossy: true},
		{name: "javascript link dropped", in: "[js](javascript:alert(1))", want: "<p>js</p>", lossy: true},
		{name: "relative link dropped", in: "[x](/rel)", want: "<p>x</p>", lossy: true},
		{name: "image dropped", in: "![a](https://t.example/p.gif)", want: "<p></p>", lossy: true},
		{name: "entity text kept", in: `\&copy;`, want: "<p>&amp;copy;</p>"},
		{name: "empty", in: "", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, lossy, err := ToHTML(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
			require.Equal(t, tc.lossy, lossy)
		})
	}
}

func TestInputLimit(t *testing.T) {
	big := strings.Repeat("a", MaxInput+1)
	_, _, err := FromHTML(big)
	require.Error(t, err)
	_, _, err = ToHTML(big)
	require.Error(t, err)
}

// TestRoundTripStable pins the property sync loop prevention needs: once a
// body has crossed in one direction, converting it back and forth again
// changes nothing.
func TestRoundTripStable(t *testing.T) {
	htmls := []struct{ name, in string }{
		{"basecamp body", `<div>Hello <strong>bold</strong> and <em>it</em><br>line2</div><div><br></div><div>para <a href="https://x.example/?a=1&amp;b=2">link</a></div>`},
		{"lists", `<h1>Title</h1><ul><li>a</li><li>b <del>gone</del></li></ul><ol><li>one</li><li>two</li></ol>`},
		{"adjacent lists", `<ul><li>a</li></ul><ul><li>b</li></ul>`},
		{"code and quote", "<pre>code\n  indented *not em*</pre><blockquote>quote <em>x</em></blockquote><hr>"},
		{"markdown-like text", `<div>a * b _c_ [d] &lt;tag&gt; #notheading 1. notlist</div>`},
		{"nested emphasis", `<div>nested <strong><em>both</em></strong> and <a href="mailto:a@b.example">mail</a></div>`},
		{"nested list", `<ul><li>outer<ul><li>inner</li></ul></li></ul>`},
		{"intraword bold before punctuation", `<div>price<strong>$100</strong>now</div>`},
		{"intraword bold quote", `<div>a<strong>"q"</strong>b</div>`},
		{"intraword em paren", `<div>a<em>(x)</em>b</div>`},
		{"bold underscore", `<div>x<strong>_</strong>y</div>`},
		{"entity-like text", `<div>&amp;copy; &amp;nbsp; &amp;amp; &amp;#x41;</div>`},
		{"nbsp", `<div>a&nbsp;b&nbsp;</div>`},
		{"backticks in code", "<div><code>a`b</code> and <pre>```\nfence\n```</pre></div>"},
		{"link with parens", `<a href="https://x.example/a_(b)">w (x)</a>`},
		{"table", `<table><tr><td>a|b</td><td>*c*</td></tr></table>`},
		{"unicode", `<div>héllo — 日本 🎉</div>`},
		{"empty elements", `<div></div><strong></strong><ul><li></li></ul>`},
		{"heading in div", `<div><h1>t</h1>text</div>`},
	}
	for _, tc := range htmls {
		t.Run("html "+tc.name, func(t *testing.T) {
			md1, _, err := FromHTML(tc.in)
			require.NoError(t, err)
			html2, _, err := ToHTML(md1)
			require.NoError(t, err)
			md2, _, err := FromHTML(html2)
			require.NoError(t, err)
			require.Equal(t, md1, md2)
		})
	}
	mds := []struct{ name, in string }{
		{"mixed", "# T\n\nsome *em* and **strong** text\n\n- a\n- b\n\n1. x\n2. y"},
		{"soft break", "line one\nline two"},
		{"quote and fence", "> quoted\n\n```go\ncode\n```"},
		{"underscores", "a_b_c and 2 * 3"},
		{"list markers", "* a\n+ b"},
		{"task list", "- [ ] task\n- [x] done"},
		{"table", "| a | b |\n|---|---|\n| 1 | 2 |"},
		{"entity", `\&amp; and &copy;`},
	}
	for _, tc := range mds {
		t.Run("markdown "+tc.name, func(t *testing.T) {
			h1, _, err := ToHTML(tc.in)
			require.NoError(t, err)
			md1, _, err := FromHTML(h1)
			require.NoError(t, err)
			h2, _, err := ToHTML(md1)
			require.NoError(t, err)
			md2, _, err := FromHTML(h2)
			require.NoError(t, err)
			require.Equal(t, md1, md2)
		})
	}
}
