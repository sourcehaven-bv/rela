package dataentryconfig

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// columnsFromMetamodel extends testMetamodel with a single-valued
// ticket→category relation, the shape a relation-backed board needs.
func columnsFromMetamodel() *metamodel.Metamodel {
	m := testMetamodel()
	category := m.Entities["category"]
	category.Properties = map[string]metamodel.PropertyDef{
		"name":        {Type: "string", Required: true},
		"description": {Type: "string"},
		"kind":        {Type: "status"},
		"tone":        {Type: "string", Values: []string{"warm", "cold"}},
	}
	m.Entities["category"] = category
	one := 1
	m.Relations["in-category"] = metamodel.RelationDef{
		Label: "in category", From: []string{"ticket"}, To: []string{"category"}, MaxOutgoing: &one,
	}
	m.Relations["offers-category"] = metamodel.RelationDef{
		Label: "offers category", From: []string{"category"}, To: []string{"category"},
	}
	m.Relations["lists-ticket"] = metamodel.RelationDef{
		Label: "lists ticket", From: []string{"category"}, To: []string{"ticket"},
	}
	return m
}

func TestValidateConfig_KanbanColumnsFrom(t *testing.T) {
	tests := []struct {
		name    string
		kanban  Kanban
		wantErr string
	}{
		{
			name: "valid",
			kanban: Kanban{EntityType: "ticket", ColumnsFrom: &KanbanColumnsFrom{
				Relation: "in-category", OfferedBy: "offers-category", OrderBy: "name",
			}},
		},
		{
			name: "multi-valued relation",
			kanban: Kanban{EntityType: "ticket", ColumnsFrom: &KanbanColumnsFrom{
				Relation: "belongs-to",
			}},
			wantErr: "must declare max_outgoing: 1",
		},
		{
			name: "combined with column_property",
			kanban: Kanban{EntityType: "ticket", ColumnProperty: "status", ColumnsFrom: &KanbanColumnsFrom{
				Relation: "in-category",
			}},
			wantErr: "mutually exclusive",
		},
		{
			name: "unknown order_by",
			kanban: Kanban{EntityType: "ticket", ColumnsFrom: &KanbanColumnsFrom{
				Relation: "in-category", OrderBy: "rank",
			}},
			wantErr: `order_by "rank" is not a property of "category"`,
		},
		{
			name: "offered_by points elsewhere",
			kanban: Kanban{EntityType: "ticket", ColumnsFrom: &KanbanColumnsFrom{
				Relation: "in-category", OfferedBy: "lists-ticket",
			}},
			wantErr: `offered_by "lists-ticket" does not point to "category"`,
		},
		{
			name: "style_from enum type",
			kanban: Kanban{EntityType: "ticket", ColumnsFrom: &KanbanColumnsFrom{
				Relation: "in-category", StyleFrom: "kind",
			}},
		},
		{
			name: "style_from declared values",
			kanban: Kanban{EntityType: "ticket", ColumnsFrom: &KanbanColumnsFrom{
				Relation: "in-category", StyleFrom: "tone",
			}},
		},
		{
			name: "style_from unknown property",
			kanban: Kanban{EntityType: "ticket", ColumnsFrom: &KanbanColumnsFrom{
				Relation: "in-category", StyleFrom: "color",
			}},
			wantErr: `columns_from: style_from "color" is not a property of "category"`,
		},
		{
			name: "style_from not an enum",
			kanban: Kanban{EntityType: "ticket", ColumnsFrom: &KanbanColumnsFrom{
				Relation: "in-category", StyleFrom: "description",
			}},
			wantErr: `columns_from: style_from "description" on "category" is not an enum property`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Kanbans: map[string]Kanban{"board": tt.kanban}}
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, columnsFromMetamodel())
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

func TestValidateListGroupBy_Relation(t *testing.T) {
	tests := []struct {
		name    string
		groupBy ListGroupBy
		wantErr string
	}{
		{name: "valid", groupBy: ListGroupBy{Relation: "in-category", OfferedBy: "offers-category", OrderBy: "name"}},
		{name: "multi-valued", groupBy: ListGroupBy{Relation: "belongs-to"}, wantErr: "must declare max_outgoing: 1"},
		{name: "with property", groupBy: ListGroupBy{Relation: "in-category", Property: "status"}, wantErr: "mutually exclusive"},
		{name: "order_by without relation", groupBy: ListGroupBy{Property: "status", OrderBy: "name"}, wantErr: "need relation"},
		{name: "style_from", groupBy: ListGroupBy{Relation: "in-category", StyleFrom: "kind"}},
		{name: "style_from not an enum", groupBy: ListGroupBy{Relation: "in-category", StyleFrom: "name"}, wantErr: `style_from "name" on "category" is not an enum property`},
		{name: "style_from without relation", groupBy: ListGroupBy{Property: "status", StyleFrom: "kind"}, wantErr: "need relation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := tt.groupBy
			cfg := &Config{Lists: map[string]List{"tasks": {EntityType: "ticket", GroupBy: &g}}}
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, columnsFromMetamodel())
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
