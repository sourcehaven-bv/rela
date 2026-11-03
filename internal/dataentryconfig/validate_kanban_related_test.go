package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// relatedPropertyMetamodel adds a `segment` enum to testMetamodel's category
// and a `lead` type that links to tickets, so a ticket board has an incoming
// relation to read a property through.
func relatedPropertyMetamodel() *metamodel.Metamodel {
	m := testMetamodel()
	category := m.Entities["category"]
	category.Properties["segment"] = metamodel.PropertyDef{Type: "priority"}
	m.Entities["category"] = category
	m.Entities["lead"] = metamodel.EntityDef{
		Label: "Lead", IDPrefix: "LEAD-",
		Properties: map[string]metamodel.PropertyDef{"segment": {Type: "priority"}},
	}
	m.Relations["ghost-link"] = metamodel.RelationDef{
		Label: "ghost link", From: []string{"category"}, To: []string{"lead"},
	}
	m.Relations["has-ticket"] = metamodel.RelationDef{
		Label: "has ticket", From: []string{"category", "lead"}, To: []string{"ticket"},
	}
	return m
}

func TestValidateConfig_KanbanRelatedProperty(t *testing.T) {
	board := func(fields ...KanbanCardField) Kanban {
		return Kanban{
			EntityType: "ticket", ColumnProperty: "status",
			Card: KanbanCard{Title: "title", Fields: fields},
		}
	}
	tests := []struct {
		name    string
		field   KanbanCardField
		wantErr string
	}{
		{name: "outgoing, inferred direction",
			field: KanbanCardField{Relation: "belongs-to", Property: "segment"}},
		{name: "outgoing, explicit direction",
			field: KanbanCardField{Relation: "belongs-to", Direction: DirectionOutgoing, Property: "description"}},
		{name: "incoming from several source types",
			field: KanbanCardField{Relation: "has-ticket", Direction: DirectionIncoming, Property: "segment"}},
		{name: "incoming, inferred direction",
			field: KanbanCardField{Relation: "has-ticket", Property: "segment"}},
		{name: "self relation with explicit direction",
			field: KanbanCardField{Relation: "blocks", Direction: DirectionIncoming, Property: "priority"}},
		{name: "unknown relation",
			field:   KanbanCardField{Relation: "ghost", Property: "segment"},
			wantErr: `card.fields[0] references unknown relation "ghost"`},
		{name: "wrong side for outgoing",
			field: KanbanCardField{Relation: "has-ticket", Direction: DirectionOutgoing, Property: "segment"},
			wantErr: `card.fields[0]: entity type "ticket" is not a from type of relation "has-ticket", ` +
				`so it has no outgoing "has-ticket" edges (valid from types: category, lead)`},
		{name: "neither side, no direction",
			field: KanbanCardField{Relation: "ghost-link", Property: "segment"},
			wantErr: `card.fields[0]: entity type "ticket" is on neither side of relation "ghost-link" ` +
				`(from: category; to: lead)`},
		{name: "wrong side for incoming",
			field:   KanbanCardField{Relation: "belongs-to", Direction: DirectionIncoming, Property: "segment"},
			wantErr: `entity type "ticket" is not a to type of relation "belongs-to"`},
		{name: "property missing on one target type",
			field: KanbanCardField{Relation: "relates-to", Direction: DirectionOutgoing, Property: "assignee"},
			wantErr: `card.fields[0]: property "assignee" is not declared on entity type "category", ` +
				`which outgoing "relates-to" edges can lead to`},
		{name: "property missing on every target type names each",
			field:   KanbanCardField{Relation: "relates-to", Direction: DirectionOutgoing, Property: "ghost"},
			wantErr: `property "ghost" is not declared on entity type "ticket", "category"`},
		{name: "property of the board type, not the related one",
			field:   KanbanCardField{Relation: "belongs-to", Property: "assignee"},
			wantErr: `property "assignee" is not declared on entity type "category"`},
		{name: "ambiguous direction is reported once",
			field:   KanbanCardField{Relation: "blocks", Property: "priority"},
			wantErr: "needs an explicit `direction:`"},
		{name: "with display count",
			field:   KanbanCardField{Relation: "belongs-to", Property: "segment", Display: "count"},
			wantErr: "card.fields[0]: display cannot be combined with a related property"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Kanbans: map[string]Kanban{"board": board(tt.field)}}
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, relatedPropertyMetamodel())
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

// TestKanbanRelatedProperty_YAML pins the keys an operator writes and the
// label derived when none is given.
func TestKanbanRelatedProperty_YAML(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte(`
version: "1.0"
kanbans:
  pipeline:
    entity_type: ticket
    column_property: status
    card:
      title: title
      fields:
        - relation: has-ticket
          direction: incoming
          property: segment
          show_label: false
`), &cfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	f := cfg.Kanbans["pipeline"].Card.Fields[0]
	if f.Relation != "has-ticket" || f.Direction != DirectionIncoming || f.Property != "segment" || f.LabelShown() {
		t.Errorf("field = %+v", f)
	}
	if got := f.DisplayLabel(); got != "segment" {
		t.Errorf("DisplayLabel() = %q, want the property name", got)
	}
	if got := (KanbanCardField{Relation: "has-ticket"}).DisplayLabel(); got != "has-ticket" {
		t.Errorf("relation-only DisplayLabel() = %q, want the relation name", got)
	}
}
