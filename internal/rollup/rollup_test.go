package rollup_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// briefSchema holds the three roll-ups of the feature brief (TKT-6I2OPG).
const briefSchema = `version: "1.0"
types:
  testresultaat:
    values: [geslaagd, gefaald]
entities:
  maatregel:
    label: Maatregel
    id_prefix: "MAAT-"
    properties:
      naam: {type: string}
      bewijs_aantal:
        type: integer
        rollup: { via: evidencedBy, count: true, where: { status: geldig } }
      laatste_testresultaat:
        type: testresultaat
        rollup: { via: testedBy, pick: resultaat, latest_by: datum }
  bewijs:
    label: Bewijs
    id_prefix: "BEW-"
    properties:
      status: {type: enum, values: [geldig, verlopen]}
  test:
    label: Test
    id_prefix: "TST-"
    properties:
      resultaat: {type: testresultaat}
      datum: {type: date}
  risico:
    label: Risico
    id_prefix: "RIS-"
    properties:
      open_behandelplannen:
        type: integer
        rollup: { via: treatedBy, count: true, where: { afgerond: "false" } }
  behandelplan:
    label: Behandelplan
    id_prefix: "BP-"
    properties:
      afgerond: {type: boolean}
relations:
  evidences:
    from: [bewijs]
    to: [maatregel]
    inverse: evidencedBy
  tests:
    from: [test]
    to: [maatregel]
    inverse: testedBy
  treats:
    from: [behandelplan]
    to: [risico]
    inverse: treatedBy
`

func TestCompile_BriefExamples(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(briefSchema))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	set, err := rollup.Compile(meta)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	cols := set.For("maatregel")
	if len(cols) != 2 {
		t.Fatalf("maatregel roll-ups = %d, want 2", len(cols))
	}
	count := cols[0]
	if count.Property != "bewijs_aantal" || count.StoreFunc() != store.RollupCount ||
		count.Relation != "evidences" || !count.SourceIsTo || count.Target != "bewijs" {

		t.Errorf("bewijs_aantal compiled to %+v", count)
	}
	if len(count.Where) != 1 || count.Where[0].Property != "status" || !count.Where[0].Scalar {
		t.Errorf("bewijs_aantal where = %+v", count.Where)
	}
	latest := cols[1]
	if latest.StoreFunc() != store.RollupLatest || latest.Value != "resultaat" || latest.By != "datum" {
		t.Errorf("laatste_testresultaat compiled to %+v", latest)
	}
	if got := strings.Join(latest.Reads, ","); got != "datum" {
		t.Errorf("latest Reads = %q, want datum (the pick is checked on the winning row)", got)
	}
	open := set.For("risico")
	if len(open) != 1 || open[0].Where[0].Scalar || open[0].Where[0].Value != "false" {
		t.Errorf("open_behandelplannen compiled to %+v; a boolean where is a non-scalar equality", open)
	}
	if !rollup.Is(meta, "risico", "open_behandelplannen") || rollup.Is(meta, "risico", "naam") {
		t.Error("Is does not tell roll-ups from other properties")
	}
}

func TestCompile_Rejects(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		replace [2]string
		want    string
	}{
		{
			name:    "unknown relation",
			replace: [2]string{"via: evidencedBy,", "via: provenBy,"},
			want:    `via "provenBy"`,
		},
		{
			name:    "unknown where property",
			replace: [2]string{"where: { status: geldig }", "where: { staat: geldig }"},
			want:    `where "staat" is not a property of "bewijs"`,
		},
		{
			name:    "where value outside the enum",
			replace: [2]string{"where: { status: geldig }", "where: { status: oud }"},
			want:    `"oud" is not one of geldig, verlopen`,
		},
		{
			name:    "unknown pick",
			replace: [2]string{"pick: resultaat,", "pick: uitkomst,"},
			want:    `pick "uitkomst" is not a property of "test"`,
		},
		{
			name:    "pick type mismatch",
			replace: [2]string{"type: testresultaat\n        rollup", "type: string\n        rollup"},
			want:    `pick "resultaat" has type "testresultaat", but the roll-up is declared "string"`,
		},
		{
			name:    "latest_by not a date",
			replace: [2]string{"datum: {type: date}", "datum: {type: string}"},
			want:    `latest_by "datum" has type "string"`,
		},
		{
			name:    "latest_by a datetime",
			replace: [2]string{"datum: {type: date}", "datum: {type: datetime}"},
			want:    `latest_by "datum" has type "datetime"; it needs date in the default format`,
		},
		{
			name:    "latest_by a date in a custom format",
			replace: [2]string{"datum: {type: date}", "datum: {type: date, format: \"02-01-2006\"}"},
			want:    `latest_by "datum" has type "date"; it needs date in the default format`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := strings.Replace(briefSchema, tc.replace[0], tc.replace[1], 1)
			if src == briefSchema {
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

// TestCompile_RejectsChain pins that a roll-up cannot read another roll-up.
func TestCompile_RejectsChain(t *testing.T) {
	t.Parallel()
	src := strings.Replace(briefSchema, "where: { afgerond: \"false\" }", "", 1)
	src = strings.Replace(src, "      afgerond: {type: boolean}",
		"      afgerond: {type: boolean}\n      n:\n        type: integer\n"+
			"        rollup: { via: treats, count: true }", 1)
	src = strings.Replace(src, "rollup: { via: treatedBy, count: true,  }",
		"rollup: { via: treatedBy, max: n }", 1)
	meta, err := metamodel.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err = rollup.Compile(meta)
	if err == nil || !strings.Contains(err.Error(), "roll-ups cannot be chained") {
		t.Fatalf("Compile error = %v, want a chain refusal", err)
	}
}
