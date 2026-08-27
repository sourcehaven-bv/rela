package metamodel

import (
	"strings"
	"testing"
)

func TestParse_ComputedPropertyShape(t *testing.T) {
	m, err := Parse([]byte(`
version: "1.0"
entities:
  item:
    label: Item
    id_prefix: I-
    properties:
      source: {type: integer}
      doubled:
        type: integer
        computed: entity.source * 2
relations: {}
types: {}
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := m.Entities["item"].Properties["doubled"].Computed; got != "entity.source * 2" {
		t.Fatalf("Computed = %q", got)
	}
	before := m.ShapeProjection()
	p := m.Entities["item"].Properties["doubled"]
	p.Computed = "entity.source * 3"
	item := m.Entities["item"]
	item.Properties["doubled"] = p
	m.Entities["item"] = item
	after := m.ShapeProjection()
	if before.Hash() == after.Hash() {
		t.Fatal("computed expression must affect shape hash")
	}
	report := CompareShapes(before, after)
	if len(report.Deltas) != 1 || report.Deltas[0].Kind != "property_computed_changed" {
		t.Fatalf("deltas = %+v", report.Deltas)
	}
}

func TestParse_RejectsInvalidComputedPlacement(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"list", "list: true\n        computed: entity.source", "computed list"},
		{"default", "default: x\n        computed: entity.source", "mutually exclusive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte("version: '1.0'\nentities:\n  item:\n    label: Item\n    id_prefix: I-\n    properties:\n      source: {type: string}\n      target:\n        type: string\n        " + tc.body + "\nrelations: {}\ntypes: {}\n"))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v", err)
			}
		})
	}
	_, err := Parse([]byte("version: '1.0'\nentities: {}\nrelations:\n  relates:\n    label: relates\n    from: [item]\n    to: [item]\n    properties:\n      score:\n        type: integer\n        computed: '1'\ntypes: {}\n"))
	if err == nil || !strings.Contains(err.Error(), "only supported on entity") {
		t.Fatalf("relation error = %v", err)
	}
}
