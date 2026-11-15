package mapimport

import (
	"errors"
	"strings"
	"testing"
)

// TestDecode_RefusesHostileInput pins the safety caps: no DTD (so no
// external entity and no entity expansion), and size, depth and node caps.
func TestDecode_RefusesHostileInput(t *testing.T) {
	laughs := `<?xml version="1.0"?>
<!DOCTYPE lolz [
 <!ENTITY lol "lol">
 <!ENTITY lol2 "&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;">
 <!ENTITY lol3 "&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;">
]>
<lolz>&lol3;</lolz>`
	xxe := `<?xml version="1.0"?>
<!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///etc/passwd">]>
<foo>&xxe;</foo>`
	deepJSON := strings.Repeat("[", 200) + strings.Repeat("]", 200)
	yamlBomb := "a: &a [x, x, x, x, x, x, x, x, x]\n" +
		"b: &b [*a, *a, *a, *a, *a, *a, *a, *a, *a]\n" +
		"c: &c [*b, *b, *b, *b, *b, *b, *b, *b, *b]\n" +
		"d: [*c, *c, *c, *c, *c, *c, *c, *c, *c]\n"
	cases := []struct {
		name   string
		data   string
		format Format
		lim    Limits
		limit  bool // want a *LimitError
	}{
		{"billion laughs", laughs, FormatXML, Limits{}, false},
		{"external entity", xxe, FormatXML, Limits{}, false},
		{"oversized xml", "<a>" + strings.Repeat("x", 2048) + "</a>", FormatXML, Limits{MaxBytes: 1024}, true},
		{"oversized json", `"` + strings.Repeat("x", 2048) + `"`, FormatJSON, Limits{MaxBytes: 1024}, true},
		{"deep json", deepJSON, FormatJSON, Limits{}, true},
		{"too many nodes", `[1,2,3,4,5,6,7,8,9,10]`, FormatJSON, Limits{MaxNodes: 5}, true},
		{"yaml alias bomb", yamlBomb, FormatYAML, Limits{MaxNodes: 500}, true},
		{"trailing json", `{} {}`, FormatJSON, Limits{}, false},
		{"csv without header", "", FormatCSV, Limits{}, false},
		{"csv duplicate header", "a,a\n1,2\n", FormatCSV, Limits{}, false},
		{"unknown format", "x", Format("toml"), Limits{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, err := Decode([]byte(tc.data), tc.format, tc.lim)
			if err == nil {
				t.Fatalf("Decode accepted it: %+v", n)
			}
			var le *LimitError
			if tc.limit != errors.As(err, &le) {
				t.Fatalf("err = %v, want a limit error: %v", err, tc.limit)
			}
		})
	}
}

func TestDecode_Shapes(t *testing.T) {
	cases := []struct {
		name, data, path string
		format           Format
		want             []string
	}{
		{"json", `{"a": [{"b": 1}, {"b": true}, {"b": null}]}`, "$.a.b", FormatJSON, []string{"1", "true", ""}},
		{"yaml", "a:\n  - b: x\n  - b: 2026-01-02\n", "$.a.b", FormatYAML, []string{"x", "2026-01-02"}},
		{"xml attrs and text", `<r><i k="1">one</i><i k="2"> two  words </i></r>`, "$.r.i.@k", FormatXML,
			[]string{"1", "2"}},
		{"xml text", `<r><i k="1">one</i><i k="2"> two  words </i></r>`, "$.r.i", FormatXML,
			[]string{"one", "two words"}},
		{"xml bom", "\ufeff<r><i>x</i></r>", "$.r.i", FormatXML, []string{"x"}},
		{"xml empty element", `<r><i/><i>  </i><i>x</i></r>`, "$.r.i", FormatXML, []string{"", "", "x"}},
		{"xml #text", `<r><i>a <b>b</b> c</i></r>`, "$.r.i.#text", FormatXML, []string{"a b c"}},
		{"xml structure without text", `<r><i><b/></i></r>`, "$.r.i.#text", FormatXML, nil},
		{"csv", "id,name\n1,a\n2,b\n", "$.name", FormatCSV, []string{"a", "b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, err := Decode([]byte(tc.data), tc.format, Limits{})
			if err != nil {
				t.Fatal(err)
			}
			p, err := CompilePath(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, n := range p.Eval(root, root) {
				s, _ := n.text()
				got = append(got, s)
			}
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDetectFormat(t *testing.T) {
	for in, want := range map[string]Format{
		"a.json": FormatJSON, "a.YAML": FormatYAML, "a.yml": FormatYAML, "a.xml": FormatXML, "a.csv": FormatCSV,
		"a.txt": "",
	} {
		if got := DetectFormat(in); got != want {
			t.Errorf("DetectFormat(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLocation_CarriesLine(t *testing.T) {
	root, err := Decode([]byte("id,name\n1,a\n2,b\n"), FormatCSV, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if loc := root.items[1].Location(); loc != "$[1] (line 3)" {
		t.Fatalf("location = %q", loc)
	}
}
