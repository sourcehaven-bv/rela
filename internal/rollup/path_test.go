package rollup_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// pathSchema is the GRC shape of TKT-LTSNHD: a requirement maps to ISO
// controls, measures implement them, and a framework holds requirements.
const pathSchema = `version: "1.0"
entities:
  nalevingskader:
    label: Kader
    id_prefix: "NK-"
    properties:
      maatregelen:
        type: integer
        rollup: { via: [bevat, verwijst_naar, wordt_geimplementeerd_door], count: true }
  nalevingseis:
    label: Eis
    id_prefix: "NE-"
    properties:
      gedekt:
        type: boolean
        rollup: { via: [verwijst_naar, wordt_geimplementeerd_door], exists: true, where: { status: actief } }
      VIA_UNDER_TEST
  iso_beheersmaatregel:
    label: ISO
    id_prefix: "ISO-"
    properties:
      titel: {type: string}
  maatregel:
    label: Maatregel
    id_prefix: "MAAT-"
    properties:
      status: {type: enum, values: [actief, gepland]}
  document:
    label: Document
    id_prefix: "DOC-"
    properties:
      titel: {type: string}
relations:
  bevat:
    from: [nalevingskader]
    to: [nalevingseis]
  verwijst_naar:
    from: [nalevingseis]
    to: [iso_beheersmaatregel]
  implementeert:
    from: [maatregel]
    to: [iso_beheersmaatregel]
    inverse: wordt_geimplementeerd_door
  raakt:
    from: [nalevingseis]
    to: [iso_beheersmaatregel, maatregel]
  onderbouwt:
    from: [document]
    to: [maatregel, iso_beheersmaatregel]
    inverse: onderbouwd_door
  concept_van:
    from: [document]
    to: [iso_beheersmaatregel]
    scope: content
    inverse: heeft_concept
`

func compilePath(t *testing.T, via string) (*rollup.Set, error) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(strings.Replace(pathSchema, "VIA_UNDER_TEST", via, 1)))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return rollup.Compile(meta)
}

func TestCompile_Path(t *testing.T) {
	t.Parallel()
	set, err := compilePath(t, "")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	cols := set.For("nalevingseis")
	if len(cols) != 1 {
		t.Fatalf("nalevingseis roll-ups = %d, want 1", len(cols))
	}
	c := cols[0]
	want := []rollup.Hop{{Relation: "verwijst_naar", Target: "iso_beheersmaatregel"}}
	if len(c.Path) != 1 || c.Path[0] != want[0] {
		t.Errorf("gedekt path = %+v, want %+v", c.Path, want)
	}
	if c.Relation != "implementeert" || !c.SourceIsTo || c.Target != "maatregel" ||
		c.StoreFunc() != store.RollupCount || len(c.Where) != 1 || c.Where[0].Property != "status" {

		t.Errorf("gedekt compiled to %+v", c)
	}
	k := set.For("nalevingskader")
	if len(k) != 1 || len(k[0].Path) != 2 || k[0].Path[1].Target != "iso_beheersmaatregel" ||
		k[0].Target != "maatregel" {

		t.Errorf("maatregelen compiled to %+v", k)
	}
}

func TestCompile_PathRejects(t *testing.T) {
	t.Parallel()
	prop := func(via string) string {
		return "n:\n        type: integer\n        rollup: { via: " + via + ", count: true }"
	}
	tests := []struct {
		name string
		via  string
		want string
	}{
		{
			name: "unknown relation on the path",
			via:  "[verwijst_naar, bestaat_niet]",
			want: `via ["verwijst_naar", "bestaat_niet"]: unknown relation type "bestaat_niet"`,
		},
		{
			name: "hop does not start where the last ended",
			via:  "[verwijst_naar, verwijst_naar]",
			want: `relation "verwijst_naar" does not start from "iso_beheersmaatregel"`,
		},
		{
			name: "intermediate hop with several far types",
			via:  "[raakt, wordt_geimplementeerd_door]",
			want: `intermediate relation "raakt" has 2 target types`,
		},
		{
			name: "last hop with several far types and no type",
			via:  "[verwijst_naar, onderbouwd_door, onderbouwt]",
			want: `relation "onderbouwt" has 2 target types`,
		},
		{
			name: "content-scoped hop",
			via:  "[verwijst_naar, heeft_concept]",
			want: `relation "concept_van" is content-scoped`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := compilePath(t, prop(tc.via))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Compile error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
