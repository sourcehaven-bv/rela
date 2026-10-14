package dataentryconfig

import (
	"maps"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// amountsMetamodel adds an integer `value` to testMetamodel's ticket.
func amountsMetamodel() *metamodel.Metamodel {
	m := testMetamodel()
	ticket := m.Entities["ticket"]
	ticket.Properties["value"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeInteger}
	ticket.Properties["scores"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeInteger, List: true}
	m.Entities["ticket"] = ticket
	return m
}

func TestValidateConfig_KanbanAmounts(t *testing.T) {
	board := func(mut func(k *Kanban)) Kanban {
		k := Kanban{EntityType: "ticket", ColumnProperty: "status", Card: KanbanCard{Title: "title"}}
		mut(&k)
		return k
	}
	total := func(prop, currency string) func(k *Kanban) {
		return func(k *Kanban) {
			k.ColumnTotal = &KanbanColumnTotal{Property: prop, Label: "Totaal", Currency: currency}
		}
	}
	field := func(f KanbanCardField) func(k *Kanban) {
		return func(k *Kanban) { k.Card.Fields = []KanbanCardField{f} }
	}
	tests := []struct {
		name    string
		kanban  Kanban
		wantErr string
	}{
		{name: "total with currency", kanban: board(total("value", "EUR"))},
		{name: "total without currency or label", kanban: board(func(k *Kanban) {
			k.ColumnTotal = &KanbanColumnTotal{Property: "value"}
		})},
		{name: "total on a columns_from board", kanban: Kanban{
			EntityType:  "ticket",
			ColumnsFrom: &KanbanColumnsFrom{Relation: "in-category"},
			ColumnTotal: &KanbanColumnTotal{Property: "value"},
		}},
		{name: "total without property", kanban: board(total("", "")), wantErr: "column_total: property is required"},
		{name: "total unknown property", kanban: board(total("amount", "")), wantErr: `column_total: property "amount" not in entity "ticket"`},
		{name: "total on a string property", kanban: board(total("assignee", "")), wantErr: `column_total: property "assignee" must be an integer property, is "string"`},
		{name: "total lowercase currency", kanban: board(total("value", "eur")), wantErr: `column_total: currency "eur" is not an ISO 4217 code`},
		{name: "total four-letter currency", kanban: board(total("value", "EURO")), wantErr: `currency "EURO" is not an ISO 4217 code`},
		{name: "total with swimlanes", kanban: board(func(k *Kanban) {
			total("value", "")(k)
			k.SwimlaneProperty = "priority"
		}), wantErr: "column_total is not supported on a board with swimlanes"},
		{name: "total on a list property", kanban: board(total("scores", "")), wantErr: `column_total: property "scores" is a list`},
		{name: "card currency on a list property", kanban: board(field(KanbanCardField{Property: "scores", Currency: "EUR"})), wantErr: `card.fields[0]: property "scores" is a list`},
		{name: "card currency", kanban: board(field(KanbanCardField{Property: "value", Currency: "EUR"}))},
		{name: "card currency on a string", kanban: board(field(KanbanCardField{Property: "assignee", Currency: "EUR"})), wantErr: `card.fields[0]: property "assignee" must be an integer property`},
		{name: "card currency on a relation", kanban: board(field(KanbanCardField{Relation: "belongs-to", Currency: "EUR"})), wantErr: "currency needs a property field"},
		{name: "card currency on comments", kanban: board(field(KanbanCardField{Comments: true, Currency: "EUR"})), wantErr: "currency needs a property field"},
		{name: "card currency malformed", kanban: board(field(KanbanCardField{Property: "value", Currency: "€"})), wantErr: `currency "€" is not an ISO 4217 code`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := amountsMetamodel()
			// The columns_from case needs the relation from columnsFromMetamodel.
			maps.Copy(meta.Relations, columnsFromMetamodel().Relations)
			cfg := &Config{Kanbans: map[string]Kanban{"board": tt.kanban}}
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
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

// TestKanbanAmounts_YAML pins the key names an operator writes.
func TestKanbanAmounts_YAML(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte(`
version: "1.0"
kanbans:
  pipeline:
    entity_type: ticket
    column_property: status
    column_total:
      property: value
      label: Totaal
      currency: EUR
    card:
      title: title
      fields:
        - property: value
          currency: EUR
`), &cfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	k := cfg.Kanbans["pipeline"]
	want := KanbanColumnTotal{Property: "value", Label: "Totaal", Currency: "EUR"}
	if k.ColumnTotal == nil || *k.ColumnTotal != want {
		t.Errorf("column_total = %+v, want %+v", k.ColumnTotal, want)
	}
	if got := k.Card.Fields[0].Currency; got != "EUR" {
		t.Errorf("card field currency = %q, want EUR", got)
	}
}

// TestKanbanAmounts_UnknownCardPropertyReportedOnce: the card field loop
// already reports an unknown property, so `currency:` adds no second error.
func TestKanbanAmounts_UnknownCardPropertyReportedOnce(t *testing.T) {
	cfg := &Config{Kanbans: map[string]Kanban{"board": {
		EntityType: "ticket", ColumnProperty: "status", Card: KanbanCard{
			Title:  "title",
			Fields: []KanbanCardField{{Property: "amount", Currency: "EUR"}},
		},
	}}}
	errs := validateKanbans(cfg, amountsMetamodel())
	n := 0
	for _, e := range errs {
		if strings.Contains(e, `"amount"`) {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("want one error naming the property, got %d: %v", n, errs)
	}
}
