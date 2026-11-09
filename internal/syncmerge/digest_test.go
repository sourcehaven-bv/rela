package syncmerge

import (
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestDigest_AgreesWithEqual(t *testing.T) {
	fs := fields(t, "title", "points", "price", "due", "tags", "state")
	by := map[string]Field{}
	for _, f := range fs {
		by[f.Name] = f
	}
	cases := []struct {
		field string
		a, b  any
	}{
		{"title", "a", "a"},
		{"title", "a", "b"},
		{"points", 3, "3"},
		{"points", 3, 4},
		{"price", "1.50", "1.5"},
		{"due", "2026-01-02", "2026-01-02"},
		{"due", "2026-01-02", "2026-01-03"},
		{"tags", []any{"x", "y"}, []any{"y", "x"}},
		{"tags", []any{"x", "y"}, []any{"x"}},
		{"tags", []any{"ab", "c"}, []any{"a", "bc"}},
		{"state", "open", "done"},
	}
	for _, tc := range cases {
		f := by[tc.field]
		eq := Equal(f, tc.a, tc.b)
		same := Digest(f, tc.a) == Digest(f, tc.b)
		if eq != same {
			t.Errorf("%s %v vs %v: Equal=%v, digests equal=%v", tc.field, tc.a, tc.b, eq, same)
		}
	}
	if d := Digest(by["title"], ""); d != "" {
		t.Errorf("empty value digest = %q", d)
	}
	if d := Digest(by["tags"], []any{}); d != "" {
		t.Errorf("empty list digest = %q", d)
	}
	if len(Digest(by["title"], "x")) != 32 {
		t.Error("digest length")
	}
}

const relSchema = `
version: "1.0"
entities:
  a: {label: A, id_prefix: "A-", properties: {title: {type: string}}}
relations:
  links:
    from: [a]
    to: [a]
    content: true
    properties:
      weight: {type: integer}
  plain:
    from: [a]
    to: [a]
`

func TestFieldsForRelation(t *testing.T) {
	m, err := metamodel.Parse([]byte(relSchema))
	if err != nil {
		t.Fatal(err)
	}
	fs, err := FieldsForRelation(m, "links", []string{"weight", ContentField})
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 2 || fs[0].Kind != KindInteger || fs[1].Kind != KindContent {
		t.Fatalf("fields = %+v", fs)
	}
	bad := []struct {
		rel   string
		names []string
	}{
		{"nope", nil},
		{"links", []string{"weight", "weight"}},
		{"links", []string{"missing"}},
		{"plain", []string{ContentField}},
	}
	for _, tc := range bad {
		if _, err := FieldsForRelation(m, tc.rel, tc.names); !errors.Is(err, ErrField) {
			t.Errorf("%s %v: err = %v, want ErrField", tc.rel, tc.names, err)
		}
	}
}
