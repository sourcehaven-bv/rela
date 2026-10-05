package metamodel

import (
	"slices"
	"testing"
)

func TestFileNames(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   any
		want []string
	}{
		{"nil", nil, []string{}},
		{"single stamped path", "attachments/T-1/spec/a.txt", []string{"a.txt"}},
		{"bare name", "a.txt", []string{"a.txt"}},
		{"windows separators", `attachments\T-1\spec\a.txt`, []string{"a.txt"}},
		{"string list sorted and deduplicated", []string{"x/b.txt", "a.txt", "y/b.txt"}, []string{"a.txt", "b.txt"}},
		{"yaml list skips non-strings and empties", []any{"p/a.txt", 7, "", "q/c.txt"}, []string{"a.txt", "c.txt"}},
		{"dot entries reference nothing", []string{".", "..", "/"}, []string{}},
		{"unsupported type", 42, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := FileNames(tc.in); !slices.Equal(got, tc.want) {
				t.Errorf("FileNames(%#v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestFileProperties(t *testing.T) {
	m, err := Parse([]byte(`version: "1"
entities:
  doc:
    label: Doc
    id_prefix: DOC
    properties:
      title: {type: string}
      spec: {type: file}
      art: {type: file}
  note:
    label: Note
    id_prefix: NOTE
    properties:
      title: {type: string}
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := FileProperties(m, "doc"); !slices.Equal(got, []string{"art", "spec"}) {
		t.Errorf("FileProperties(doc) = %v", got)
	}
	if got := FileProperties(m, "note"); got != nil {
		t.Errorf("FileProperties(note) = %v, want nil", got)
	}
	if got := FileProperties(m, "missing"); got != nil {
		t.Errorf("FileProperties(missing) = %v, want nil", got)
	}
	if !HasFileProperties(m) || HasFileProperties(nil) {
		t.Error("HasFileProperties is wrong")
	}
}
