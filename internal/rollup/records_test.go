package rollup_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
)

const recordsSchema = `version: "1.0"
entities:
  site:
    label: Site
    id_prefix: "SITE-"
    properties:
      last: {type: decimal, rollup: {records: reading, latest: kwh}}
      week: {type: decimal, rollup: {records: reading, sum: kwh, window: 7d}}
      avg: {type: decimal, rollup: {records: reading, avg: kwh}}
  meter:
    label: Meter
    id_prefix: "MTR-"
    properties:
      name: {type: string}
records:
  reading:
    owner: site
    fields:
      kwh: {type: decimal, scale: 1}
      note: {type: string}
`

func TestCompile_Records(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(recordsSchema))
	if err != nil {
		t.Fatal(err)
	}
	set, err := rollup.Compile(meta)
	if err != nil {
		t.Fatal(err)
	}
	cols := set.For("site")
	if len(cols) != 3 {
		t.Fatalf("columns = %d, want 3", len(cols))
	}
	for _, c := range cols {
		if c.Records != "reading" || c.Value != "kwh" || !c.Decimal {
			t.Errorf("column %s = %+v", c.Property, c)
		}
		if c.Property == "week" && c.Window != 7*24*time.Hour {
			t.Errorf("window = %v", c.Window)
		}
	}
}

func TestCompile_RecordsRejects(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		replace [2]string
		want    string
	}{
		{"unknown record type", [2]string{"records: reading, latest", "records: nope, latest"},
			`records "nope" is not a record type`},
		{"wrong owner", [2]string{"owner: site", "owner: meter"}, `is owned by "meter", not "site"`},
		{"unknown field", [2]string{"latest: kwh", "latest: wh"}, `latest "wh" is not a field`},
		{"field not numeric", [2]string{"latest: kwh", "latest: note"},
			`has type "string"; it needs integer or decimal`},
		{"type mismatch", [2]string{"last: {type: decimal", "last: {type: integer"},
			`but the roll-up is declared "integer"`},
		{"decimal needs a scale", [2]string{"kwh: {type: decimal, scale: 1}", "kwh: {type: decimal}"},
			`needs a declared scale`},
		{"avg needs decimal", [2]string{"avg: {type: decimal", "avg: {type: integer"}, `avg needs type decimal`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := strings.Replace(recordsSchema, tc.replace[0], tc.replace[1], 1)
			if src == recordsSchema {
				t.Fatalf("replacement %q not found", tc.replace[0])
			}
			meta, err := metamodel.Parse([]byte(src))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			_, err = rollup.Compile(meta)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Compile error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
