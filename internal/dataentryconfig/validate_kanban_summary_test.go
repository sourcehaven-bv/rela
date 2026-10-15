package dataentryconfig

import (
	"maps"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// summaryMetamodel adds an integer, an integer list and a date to
// testMetamodel's ticket.
func summaryMetamodel() *metamodel.Metamodel {
	m := testMetamodel()
	ticket := m.Entities["ticket"]
	ticket.Properties["value"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeInteger}
	ticket.Properties["scores"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeInteger, List: true}
	ticket.Properties["due"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeDate}
	ticket.Properties["closed_at"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeDatetime}
	m.Entities["ticket"] = ticket
	return m
}

func TestValidateConfig_KanbanColumnSummary(t *testing.T) {
	board := func(summaries ...Summary) Kanban {
		return Kanban{
			EntityType: "ticket", ColumnProperty: "status", Card: KanbanCard{Title: "title"},
			ColumnSummary: summaries,
		}
	}
	tests := []struct {
		name    string
		kanban  Kanban
		wantErr string
	}{
		{name: "count", kanban: board(Summary{Function: SummaryCount})},
		{name: "several", kanban: board(
			Summary{Function: SummaryCount, Label: "Deals"},
			Summary{Function: SummarySum, Property: "value", Label: "Total"},
			Summary{Function: SummaryAvg, Property: "value"},
		)},
		{name: "min and max of an integer", kanban: board(
			Summary{Function: SummaryMin, Property: "value"}, Summary{Function: SummaryMax, Property: "value"})},
		{name: "min of a date", kanban: board(Summary{Function: SummaryMin, Property: "due"})},
		{name: "max of a datetime", kanban: board(Summary{Function: SummaryMax, Property: "closed_at"})},
		{name: "on a columns_from board", kanban: Kanban{
			EntityType:    "ticket",
			ColumnsFrom:   &KanbanColumnsFrom{Relation: "in-category"},
			ColumnSummary: []Summary{{Function: SummarySum, Property: "value"}},
		}},
		{name: "count with a property", kanban: board(Summary{Function: SummaryCount, Property: "value"}),
			wantErr: "column_summary[0]: count takes no property"},
		{name: "unknown function", kanban: board(Summary{Function: "median", Property: "value"}),
			wantErr: `column_summary[0]: function "median" is not one of count, sum, avg, min, max`},
		{name: "missing function", kanban: board(Summary{Property: "value"}),
			wantErr: `function "" is not one of`},
		{name: "sum without property", kanban: board(Summary{Function: SummarySum}),
			wantErr: "column_summary[0]: sum needs a property"},
		{name: "unknown property", kanban: board(Summary{Function: SummarySum, Property: "amount"}),
			wantErr: `column_summary[0]: property "amount" not in entity "ticket"`},
		{name: "sum of a string", kanban: board(Summary{Function: SummarySum, Property: "assignee"}),
			wantErr: `sum needs a property of type integer; "assignee" is "string"`},
		{name: "avg of a date", kanban: board(Summary{Function: SummaryAvg, Property: "due"}),
			wantErr: `avg needs a property of type integer; "due" is "date"`},
		{name: "min of a string", kanban: board(Summary{Function: SummaryMin, Property: "assignee"}),
			wantErr: `min needs a property of type integer, date, datetime; "assignee" is "string"`},
		{name: "sum of a list", kanban: board(Summary{Function: SummarySum, Property: "scores"}),
			wantErr: `property "scores" is a list`},
		{name: "second entry indexed", kanban: board(Summary{Function: SummaryCount}, Summary{Function: SummarySum}),
			wantErr: "column_summary[1]: sum needs a property"},
		{name: "with swimlanes", kanban: func() Kanban {
			k := board(Summary{Function: SummaryCount})
			k.SwimlaneProperty = "priority"
			return k
		}(), wantErr: "column_summary is not supported on a board with swimlanes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := summaryMetamodel()
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

// TestKanbanColumnSummary_YAML pins the key names an operator writes.
func TestKanbanColumnSummary_YAML(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte(`
version: "1.0"
kanbans:
  pipeline:
    entity_type: ticket
    column_property: status
    column_summary:
      - function: count
      - function: sum
        property: value
        label: Totaal
    card:
      title: title
`), &cfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := cfg.Kanbans["pipeline"].ColumnSummary
	want := []Summary{{Function: SummaryCount}, {Function: SummarySum, Property: "value", Label: "Totaal"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("column_summary = %+v, want %+v", got, want)
	}
}
