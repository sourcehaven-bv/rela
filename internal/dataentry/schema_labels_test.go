package dataentry

import (
	"testing"

	dec "github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func labelsMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	mm, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  account:
    label: Account
    id_prefix: "ACC-"
    properties:
      status: {type: string, label: Account status}
      plan: {type: string}
  invoice:
    label: Invoice
    id_prefix: "INV-"
    properties:
      status: {type: string, label: Payment status}
relations:
  billed-to:
    label: billed to
    from: [invoice]
    to: [account]
    inverse: {id: invoices, label: Invoices}
  owned-by:
    label: owned by
    from: [account]
    to: [account]
`))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	return mm
}

// TestWithSchemaLabels pins that a column without its own `label:` shows the
// schema's label, the one forms show, so a list and a form name a property
// the same way (finding: "Status" in a form, "Account status" on a page).
func TestWithSchemaLabels(t *testing.T) {
	mm := labelsMeta(t)
	tests := []struct {
		name  string
		types []string
		col   dec.ListColumn
		want  string
	}{
		{"schema property label", []string{"account"}, dec.ListColumn{Property: "status"}, "Account status"},
		{"own label wins", []string{"account"}, dec.ListColumn{Property: "status", Label: "State"}, "State"},
		{"no schema label stays empty", []string{"account"}, dec.ListColumn{Property: "plan"}, ""},
		{"first type that labels it", []string{"invoice", "account"}, dec.ListColumn{Property: "status"}, "Payment status"},
		{"outgoing relation label", []string{"invoice"}, dec.ListColumn{Relation: "billed-to"}, "billed to"},
		{"incoming relation inverse label", []string{"account"}, dec.ListColumn{Relation: "billed-to"}, "Invoices"},
		{
			"explicit incoming without inverse label", []string{"account"},
			dec.ListColumn{Relation: "owned-by", Direction: dec.DirectionIncoming}, "owned by",
		},
		{"unknown relation", []string{"account"}, dec.ListColumn{Relation: "nope"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := []dec.ListColumn{tt.col}
			got := withSchemaLabels(mm, tt.types, in)
			if got[0].Label != tt.want {
				t.Errorf("label = %q, want %q", got[0].Label, tt.want)
			}
			if in[0].Label != tt.col.Label {
				t.Errorf("input changed: label = %q", in[0].Label)
			}
		})
	}
}

// TestBuildSectionFieldData_SchemaLabel pins the same fallback for a view
// field: own label, then schema label, then the raw property name.
func TestBuildSectionFieldData_SchemaLabel(t *testing.T) {
	mm := labelsMeta(t)
	def, _ := mm.GetEntityDef("account")
	e := &entity.Entity{ID: "ACC-001", Type: "account", Properties: map[string]any{"status": "active"}}
	tests := []struct {
		field ViewSectionField
		want  string
	}{
		{ViewSectionField{Property: "status"}, "Account status"},
		{ViewSectionField{Property: "status", Label: "State"}, "State"},
		{ViewSectionField{Property: "plan"}, "plan"},
	}
	for _, tt := range tests {
		got := buildSectionFieldData(tt.field, e, def, "")
		if got.Label != tt.want {
			t.Errorf("%+v: label = %q, want %q", tt.field, got.Label, tt.want)
		}
	}
}

func TestEntityTypesOf(t *testing.T) {
	got := entityTypesOf([]*entity.Entity{{Type: "b"}, {Type: "a"}, {Type: "b"}})
	if len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Errorf("types = %v, want [b a]", got)
	}
}

// TestResolveListDirections_FillsLabels pins that /_config serves list
// columns with the schema label filled in, without touching the loaded
// config.
func TestResolveListDirections_FillsLabels(t *testing.T) {
	s := &Schema{Meta: labelsMeta(t)}
	lists := map[string]dec.List{"accounts": {
		EntityType: "account",
		Columns:    []dec.ListColumn{{Property: "status"}, {Relation: "billed-to"}},
	}}
	got := resolveListDirections(s, lists)["accounts"].Columns
	if got[0].Label != "Account status" || got[1].Label != "Invoices" {
		t.Errorf("labels = %q, %q; want Account status, Invoices", got[0].Label, got[1].Label)
	}
	if lists["accounts"].Columns[0].Label != "" {
		t.Error("the loaded config changed")
	}
}
