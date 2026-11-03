package check

import (
	"os"
	"reflect"
	"testing"
)

// goldenBody is the body the vale.json golden file was produced from (Vale
// 3.25.1). The emoji is one code point, so "altijd" starts at column 16.
const goldenBody = "De café 😀 moet altijd open.\nDit zou moeten werken.\n"

func TestParseVale_Golden(t *testing.T) {
	out, err := os.ReadFile("testdata/vale.json")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := parseVale(out)
	if err != nil {
		t.Fatal(err)
	}
	got := resolve("style", raw, []byte(goldenBody))
	want := []Finding{
		{
			Checker: "style", Rule: "ISMS-Vaag.Absoluut", Severity: SeverityWarning,
			Message: "'altijd' is een absolute claim.", Line: 1, Column: 16, Length: 6, Match: "altijd",
		},
		{
			Checker: "style", Rule: "ISMS-Vaag.Verplichting", Severity: SeverityError,
			Message: "Gebruik 'moet' in plaats van 'zou moeten'.", Line: 2, Column: 5, Length: 10,
			Match: "zou moeten", Replacements: []string{"moet"}, Link: "https://example.org/verplichting",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestParseVale_Empty(t *testing.T) {
	raw, err := parseVale([]byte("{}"))
	if err != nil || len(raw) != 0 {
		t.Errorf("raw = %v, err = %v", raw, err)
	}
}

func TestParseVale_SuggestionsWithoutReplaceAction(t *testing.T) {
	raw, err := parseVale([]byte(`{"f":[{"Action":{"Name":"suggest","Params":["spelling"]},
		"Suggestions":["kwartaal","kwartalen"],"Span":[1,3],"Check":"Vale.Spelling","Message":"m","Line":1}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(raw[0].Replacements, []string{"kwartaal", "kwartalen"}) {
		t.Errorf("replacements = %v", raw[0].Replacements)
	}
}

func TestParseRela(t *testing.T) {
	raw, err := parseRela([]byte(`{"findings":[{"rule":"R","severity":"suggestion","message":"m",
		"line":1,"column":4,"length":4,"replacements":["x"],"extra":true}],"version":2}`))
	if err != nil {
		t.Fatal(err)
	}
	got := resolve("c", raw, []byte("De café"))
	want := []Finding{{Checker: "c", Rule: "R", Severity: SeveritySuggestion, Message: "m",
		Line: 1, Column: 4, Length: 4, Match: "café", Replacements: []string{"x"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseMalformed(t *testing.T) {
	for _, in := range []string{"", "not json", "[]", `{"findings":[]} {}`} {
		if _, err := parseRela([]byte(in)); err == nil {
			t.Errorf("parseRela(%q) accepted", in)
		}
	}
	for _, in := range []string{"", "not json", `{"f": 1}`} {
		if _, err := parseVale([]byte(in)); err == nil {
			t.Errorf("parseVale(%q) accepted", in)
		}
	}
}

func TestResolve_Positions(t *testing.T) {
	body := []byte("één\r\ntwee\n")
	cases := []struct {
		name              string
		line, col, length int
		wantLine, wantCol int
		wantMatch         string
	}{
		{"diacritics count as one", 1, 1, 3, 1, 1, "één"},
		{"CRLF does not shift columns", 2, 1, 4, 2, 1, "twee"},
		{"a span across a line break", 1, 3, 4, 1, 3, "n\ntw"},
		{"past the body end", 2, 2, 9, 0, 0, ""},
		{"start past the line end", 1, 4, 1, 0, 0, ""},
		{"line out of range", 9, 1, 1, 0, 0, ""},
		{"zero length", 1, 1, 0, 0, 0, ""},
		{"negative column", 1, -1, 2, 0, 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolve("c", []rawFinding{{Message: "m", Line: tc.line, Column: tc.col, Length: tc.length}}, body)
			if len(got) != 1 {
				t.Fatalf("finding dropped: %v", got)
			}
			f := got[0]
			if f.Line != tc.wantLine || f.Column != tc.wantCol || f.Match != tc.wantMatch {
				t.Errorf("got line %d col %d match %q", f.Line, f.Column, f.Match)
			}
		})
	}
}

func TestResolve_DropsEmptyMessageAndNormalizesSeverity(t *testing.T) {
	got := resolve("c", []rawFinding{
		{Message: " "},
		{Message: "a", Severity: "ERROR"},
		{Message: "b", Severity: "info"},
	}, []byte("x"))
	if len(got) != 2 || got[0].Severity != SeverityError || got[1].Severity != SeverityWarning {
		t.Errorf("got %+v", got)
	}
}
