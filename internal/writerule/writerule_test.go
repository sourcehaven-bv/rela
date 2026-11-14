package writerule_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/writerule"
)

const baseYAML = `
version: "1.0"
types:
  versiestatus:
    values: [concept, ter_beoordeling, goedgekeurd]
entities:
  verwerking:
    label: Verwerking
    id_prefix: "VW-"
    id_type: sequential
    properties:
      versiestatus: {type: versiestatus}
      rechtsgrond: {type: string}
      bewaartermijn: {type: integer}
  pagina:
    label: Pagina
    id_prefix: "PG-"
    id_type: sequential
    faces:
      concept: {}
      vastgesteld: {}
    properties:
      titel: {type: string}
relations: {}
`

func parse(t *testing.T, validations string) *metamodel.Metamodel {
	t.Helper()
	meta, err := metamodel.Parse([]byte(baseYAML + validations))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return meta
}

// termijnGrens has an ordered when, which fails to evaluate on nil.
const termijnGrens = `
validations:
  - name: lange-termijn
    entity_type: verwerking
    enforce: write
    when_condition: "entity.bewaartermijn > 10"
    then: ["rechtsgrond!="]
`

func TestCheck(t *testing.T) {
	const rechtsgrond = `
validations:
  - name: rechtsgrond-bij-beoordeling
    description: Een verwerking ter beoordeling heeft een rechtsgrond.
    entity_type: verwerking
    enforce: write
    when: ["versiestatus=ter_beoordeling"]
    then: ["rechtsgrond!="]
`
	tests := []struct {
		name        string
		validations string
		entity      *entity.Entity
		wantRules   []string
	}{
		{
			name:        "when matches and then fails",
			validations: rechtsgrond,
			entity: &entity.Entity{ID: "VW-1", Type: "verwerking",
				Properties: map[string]any{"versiestatus": "ter_beoordeling"}},
			wantRules: []string{"rechtsgrond-bij-beoordeling"},
		},
		{
			name:        "when matches and then holds",
			validations: rechtsgrond,
			entity: &entity.Entity{ID: "VW-1", Type: "verwerking",
				Properties: map[string]any{"versiestatus": "ter_beoordeling", "rechtsgrond": "art. 6"}},
		},
		{
			name:        "when does not match",
			validations: rechtsgrond,
			entity: &entity.Entity{ID: "VW-1", Type: "verwerking",
				Properties: map[string]any{"versiestatus": "concept"}},
		},
		{
			name:        "other type",
			validations: rechtsgrond,
			entity:      &entity.Entity{ID: "PG-1", Type: "pagina", Face: "concept"},
		},
		{
			name: "rule without enforce is not checked",
			validations: `
validations:
  - name: advisory
    entity_type: verwerking
    then: ["rechtsgrond!="]
`,
			entity: &entity.Entity{ID: "VW-1", Type: "verwerking", Properties: map[string]any{}},
		},
		{
			name: "expression conditions",
			validations: `
validations:
  - name: termijn
    entity_type: verwerking
    enforce: write
    when_condition: "entity.versiestatus == 'goedgekeurd'"
    then_condition: "entity.bewaartermijn ~= nil and entity.bewaartermijn > 0"
`,
			entity: &entity.Entity{ID: "VW-1", Type: "verwerking",
				Properties: map[string]any{"versiestatus": "goedgekeurd", "bewaartermijn": 0}},
			wantRules: []string{"termijn"},
		},
		{
			name: "unscoped rule applies to every type",
			validations: `
validations:
  - name: needs-id
    enforce: write
    then_condition: "false"
`,
			entity:    &entity.Entity{ID: "PG-1", Type: "pagina", Face: "concept"},
			wantRules: []string{"needs-id"},
		},
		{
			name: "face scope covers the face",
			validations: `
validations:
  - name: vastgesteld-titel
    entity_type: pagina
    faces: [vastgesteld]
    enforce: write
    then: ["titel!="]
`,
			entity:    &entity.Entity{ID: "PG-1", Type: "pagina", Face: "vastgesteld"},
			wantRules: []string{"vastgesteld-titel"},
		},
		{
			name: "face scope leaves another face out",
			validations: `
validations:
  - name: vastgesteld-titel
    entity_type: pagina
    faces: [vastgesteld]
    enforce: write
    then: ["titel!="]
`,
			entity: &entity.Entity{ID: "PG-1", Type: "pagina", Face: "concept"},
		},
		{
			name:        "when that fails on a set value fails closed",
			validations: termijnGrens,
			entity: &entity.Entity{ID: "VW-1", Type: "verwerking",
				Properties: map[string]any{"bewaartermijn": "twintig"}},
			wantRules: []string{"lange-termijn"},
		},
		{
			name:        "when on an unset property leaves the rule out",
			validations: termijnGrens,
			entity:      &entity.Entity{ID: "VW-1", Type: "verwerking", Properties: map[string]any{}},
		},
		{
			name:        "when matches a valid value",
			validations: termijnGrens,
			entity: &entity.Entity{ID: "VW-1", Type: "verwerking",
				Properties: map[string]any{"bewaartermijn": 20}},
			wantRules: []string{"lange-termijn"},
		},
		{
			name: "several violations in declaration order",
			validations: `
validations:
  - name: first
    entity_type: verwerking
    enforce: write
    then: ["rechtsgrond!="]
  - name: second
    entity_type: verwerking
    enforce: write
    then: ["versiestatus!="]
`,
			entity:    &entity.Entity{ID: "VW-1", Type: "verwerking", Properties: map[string]any{}},
			wantRules: []string{"first", "second"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			set, err := writerule.NewSet(parse(t, tc.validations))
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			got := set.Check(context.Background(), tc.entity)
			if len(got) != len(tc.wantRules) {
				t.Fatalf("got %d violations %v, want rules %v", len(got), got, tc.wantRules)
			}
			for i, v := range got {
				if v.Type != metamodel.ValidationErrorRule {
					t.Errorf("violation %d type = %q, want %q", i, v.Type, metamodel.ValidationErrorRule)
				}
				if v.Property != "" {
					t.Errorf("violation %d property = %q, want empty", i, v.Property)
				}
				if !strings.Contains(v.Message, `"`+tc.wantRules[i]+`"`) {
					t.Errorf("violation %d message %q does not name rule %q", i, v.Message, tc.wantRules[i])
				}
			}
		})
	}
}

// TestCheck_MessageNamesNoValue pins that the refusal quotes the rule, not
// the value that broke it: the writer may not be allowed to read it.
func TestCheck_MessageNamesNoValue(t *testing.T) {
	set, err := writerule.NewSet(parse(t, `
validations:
  - name: geen-geheim
    description: De rechtsgrond is niet geheim.
    entity_type: verwerking
    enforce: write
    then: ["rechtsgrond!=geheim-123"]
`))
	if err != nil {
		t.Fatal(err)
	}
	got := set.Check(context.Background(), &entity.Entity{ID: "VW-1", Type: "verwerking",
		Properties: map[string]any{"rechtsgrond": "geheim-123"}})
	if len(got) != 1 {
		t.Fatalf("got %v, want one violation", got)
	}
	want := `validation rule "geen-geheim" is not satisfied: De rechtsgrond is niet geheim.`
	if got[0].Message != want {
		t.Errorf("message = %q, want %q", got[0].Message, want)
	}
}

func TestNewSet_Refuses(t *testing.T) {
	tests := []struct {
		name        string
		validations string
		want        string
	}{
		{
			name: "expression that does not compile",
			validations: `
validations:
  - name: broken
    entity_type: verwerking
    enforce: write
    then_condition: "entity.nope == 1"
`,
			want: `validation "broken" on "verwerking"`,
		},
		{
			name: "related traversal on an unknown relation",
			validations: `
validations:
  - name: traverses
    entity_type: verwerking
    enforce: write
    then_condition: "related(entity, 'x')"
`,
			want: `related(...): related: unknown relation type "x"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := writerule.NewSet(parse(t, tc.validations))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Compile error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestNewSet_RefusesRollup(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  maatregel:
    label: Maatregel
    id_prefix: "MAAT-"
    id_type: sequential
    properties:
      bewijs_aantal:
        type: integer
        rollup: { via: evidencedBy, count: true }
  bewijs:
    label: Bewijs
    id_prefix: "BEW-"
    id_type: sequential
    properties:
      naam: {type: string}
relations:
  evidences:
    from: [bewijs]
    to: [maatregel]
    inverse: evidencedBy
validations:
  - name: bewijs-nodig
    entity_type: maatregel
    enforce: write
    then_condition: "entity.bewijs_aantal > 0"
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := writerule.NewSet(meta); err == nil || !strings.Contains(err.Error(), "bewijs_aantal") {
		t.Fatalf("Compile error = %v, want a roll-up refusal", err)
	}
}

func TestReads(t *testing.T) {
	set, err := writerule.NewSet(parse(t, termijnGrens))
	if err != nil {
		t.Fatal(err)
	}
	got := set.Reads()
	want := []writerule.Read{{Type: "verwerking", Rule: "lange-termijn",
		Properties: []string{"bewaartermijn", "rechtsgrond"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Reads = %+v, want %+v", got, want)
	}
	var nilSet *writerule.Set
	if nilSet.Reads() != nil {
		t.Fatal("nil set Reads is not nil")
	}
}

func TestNilSet(t *testing.T) {
	var s *writerule.Set
	if got := s.Check(context.Background(), &entity.Entity{Type: "x"}); got != nil {
		t.Fatalf("nil set Check = %v, want nil", got)
	}
	if _, err := writerule.NewSet(nil); err == nil {
		t.Fatal("NewSet(nil) succeeded, want an error")
	}
}
