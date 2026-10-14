package ingest

import "testing"

// TestInterpolate pins the flattening seam for every source: a resolved value
// cannot carry a line break or NUL into the document, while the operator's
// own template keeps its newlines.
func TestInterpolate(t *testing.T) {
	values := map[string]string{
		"nl":    "one\ntwo",
		"cr":    "one\rtwo",
		"nul":   "one\x00two",
		"brace": "{{nl}}",
		"plain": "x",
	}
	resolve := func(ref string) string { return values[ref] }

	for _, tc := range []struct {
		name, tmpl, want string
	}{
		{"no reference", "plain text", "plain text"},
		{"newline", "- {{nl}}", "- one two"},
		{"carriage return", "- {{cr}}", "- one two"},
		{"NUL", "- {{nul}}", "- onetwo"},
		{"operator newline kept", "#### h\n- {{nl}}", "#### h\n- one two"},
		{"no recursive interpolation", "- {{brace}}", "- {{nl}}"},
		{"unresolved is empty", "[{{missing}}]", "[]"},
		{"spaces trimmed", "{{ plain }}", "x"},
		{"unterminated kept verbatim", "a {{plain", "a {{plain"},
		{"two references", "{{plain}}{{plain}}", "xx"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Interpolate(tc.tmpl, resolve); got != tc.want {
				t.Errorf("Interpolate(%q) = %q, want %q", tc.tmpl, got, tc.want)
			}
		})
	}
}

// TestFlattenToLine_LeavesNonCommonMarkBreaks pins that only \n and \r are
// rewritten: the others cannot start a CommonMark line.
func TestFlattenToLine_LeavesNonCommonMarkBreaks(t *testing.T) {
	in := "a\vb\fc\u0085d e"
	if got := FlattenToLine(in); got != in {
		t.Errorf("FlattenToLine(%q) = %q, want it unchanged", in, got)
	}
}
