package safexml

import (
	"errors"
	"strings"
	"testing"
)

func TestDecode_Tree(t *testing.T) {
	doc := `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:x="urn:x">
  <title type="text">Hello &amp; &nbsp;bye</title>
  <x:item x:id="7">a<![CDATA[<b>]]>c<child/>d</x:item>
  <!-- ignored -->
</feed>`
	root, err := Decode([]byte(doc), DefaultLimits)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if root.Name != "feed" || root.Space != "http://www.w3.org/2005/Atom" {
		t.Fatalf("root = %s/%s", root.Space, root.Name)
	}
	if len(root.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(root.Children))
	}
	title := root.Children[0]
	if title.Text != "Hello &  bye" {
		t.Errorf("title text = %q", title.Text)
	}
	if len(title.Attrs) != 1 || title.Attrs[0].Name != "type" || title.Attrs[0].Value != "text" {
		t.Errorf("title attrs = %+v", title.Attrs)
	}
	item := root.Children[1]
	if item.Space != "urn:x" || item.Name != "item" {
		t.Errorf("item = %s/%s", item.Space, item.Name)
	}
	if item.Text != "a<b>c" || item.Children[0].Tail != "d" {
		t.Errorf("item text = %q, child tail = %q; want \"a<b>c\" and \"d\"", item.Text, item.Children[0].Tail)
	}
	if len(root.Attrs) != 0 {
		t.Errorf("namespace declarations kept as attributes: %+v", root.Attrs)
	}
	if item.Attrs[0].Space != "urn:x" || item.Attrs[0].Value != "7" {
		t.Errorf("item attr = %+v", item.Attrs[0])
	}
	if len(item.Children) != 1 || item.Children[0].Name != "child" {
		t.Errorf("item children = %+v", item.Children)
	}
}

func TestDecode_Encodings(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"utf8 bom", append([]byte{0xEF, 0xBB, 0xBF}, `<a>é</a>`...), "é"},
		{"latin1", []byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><a>caf\xe9</a>"), "café"},
		{"windows-1252 bytes under a latin1 label",
			[]byte("<?xml version=\"1.0\" encoding=\"iso-8859-1\"?><a>\x93q\x94 \x80</a>"), "“q” €"},
		{"us-ascii", []byte(`<?xml version="1.0" encoding="US-ASCII"?><a>x</a>`), "x"},
		{"utf8 without hyphen", []byte(`<?xml version="1.0" encoding="utf8"?><a>é</a>`), "é"},
		{"utf8 declared", []byte(`<?xml version="1.0" encoding="utf-8"?><a>é</a>`), "é"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, err := Decode(tc.data, DefaultLimits)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if root.Text != tc.want {
				t.Errorf("text = %q, want %q", root.Text, tc.want)
			}
		})
	}
}

func TestDecode_Refused(t *testing.T) {
	small := Limits{MaxBytes: 4096, MaxDepth: 4, MaxElements: 10, MaxAttrs: 3}
	tests := []struct {
		name  string
		doc   string
		lim   Limits
		limit string // want a LimitError with this Limit; empty for a syntax error
		msg   string
	}{
		{
			name: "billion laughs",
			doc: `<?xml version="1.0"?>
<!DOCTYPE lolz [<!ENTITY lol "lol"><!ENTITY lol2 "&lol;&lol;&lol;&lol;">]>
<lolz>&lol2;</lolz>`,
			lim: DefaultLimits, msg: "entity",
		},
		{
			name: "external entity",
			doc: `<?xml version="1.0"?>
<!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///etc/passwd">]>
<foo>&xxe;</foo>`,
			lim: DefaultLimits, msg: "entity",
		},
		{
			name: "undeclared entity without doctype",
			doc:  `<foo>&xxe;</foo>`,
			lim:  DefaultLimits, msg: "entity",
		},
		{name: "too deep", doc: strings.Repeat("<a>", 5) + strings.Repeat("</a>", 5), lim: small, limit: "depth", msg: "deeper than 4"},
		{name: "too many elements", doc: "<r>" + strings.Repeat("<a/>", 10) + "</r>", lim: small, limit: "elements", msg: "more than 10"},
		{name: "too many attributes", doc: `<r a="1" b="2" c="3" d="4"/>`, lim: small, limit: "attributes", msg: "4 attributes"},
		{name: "namespace declarations count as attributes", doc: `<r xmlns:a="1" xmlns:b="2" xmlns:c="3" xmlns:d="4"/>`, lim: small, limit: "attributes", msg: "attributes"},
		{name: "oversize", doc: "<r>" + strings.Repeat("x", 5000) + "</r>", lim: small, limit: "bytes", msg: "bytes"},
		{name: "utf-16", doc: `<?xml version="1.0" encoding="UTF-16"?><a/>`, lim: DefaultLimits, limit: "charset", msg: "charset"},
		{name: "windows-1251", doc: `<?xml version="1.0" encoding="windows-1251"?><a/>`, lim: DefaultLimits, limit: "charset", msg: "charset"},
		{name: "empty", doc: "", lim: DefaultLimits, msg: "no root"},
		{name: "whitespace", doc: "  \n ", lim: DefaultLimits, msg: "no root"},
		{name: "unclosed", doc: "<a><b></a>", lim: DefaultLimits, msg: "safexml"},
		{name: "second root", doc: "<a/><b/>", lim: DefaultLimits, msg: "after the root"},
		{name: "text only", doc: "hello", lim: DefaultLimits, msg: "no root"},
		{name: "control character", doc: "<a>\x0b</a>", lim: DefaultLimits, msg: "illegal character"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode([]byte(tc.doc), tc.lim)
			if err == nil {
				t.Fatal("Decode succeeded, want error")
			}
			got := ""
			if le, ok := errors.AsType[*LimitError](err); ok {
				got = le.Limit
			}
			if got != tc.limit {
				t.Errorf("limit = %q, want %q (err: %v)", got, tc.limit, err)
			}
			if !strings.Contains(err.Error(), tc.msg) {
				t.Errorf("error %q does not mention %q", err, tc.msg)
			}
		})
	}
}

func TestDecode_ZeroLimitRejected(t *testing.T) {
	lim := DefaultLimits
	lim.MaxDepth = 0
	if _, err := Decode([]byte("<a/>"), lim); err == nil {
		t.Fatal("zero MaxDepth accepted")
	}
}

func TestDecode_AtLimitsSucceeds(t *testing.T) {
	lim := Limits{MaxBytes: 4096, MaxDepth: 3, MaxElements: 3, MaxAttrs: 2}
	if _, err := Decode([]byte(`<a x="1" y="2"><b><c/></b></a>`), lim); err != nil {
		t.Fatalf("document exactly at every limit refused: %v", err)
	}
}
