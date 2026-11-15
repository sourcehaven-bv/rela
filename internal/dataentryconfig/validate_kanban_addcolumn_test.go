package dataentryconfig

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// addColumnMetamodel adds a type whose column property is an inline enum.
// The ticket type's status is a named custom type.
func addColumnMetamodel() *metamodel.Metamodel {
	m := columnsFromMetamodel()
	m.Entities["card"] = metamodel.EntityDef{
		Label: "Card", IDPrefix: "CRD-",
		Properties: map[string]metamodel.PropertyDef{
			"title": {Type: "string"},
			"stage": {Type: "enum", Values: []string{"open", "decided"}},
		},
	}
	m.Entities["isms.card"] = m.Entities["card"]
	return m
}

func TestValidateConfig_KanbanAddColumn(t *testing.T) {
	tests := []struct {
		name    string
		kanban  Kanban
		wantErr string
	}{
		{
			name:   "off is never checked",
			kanban: Kanban{EntityType: "ticket", ColumnProperty: "status"},
		},
		{
			name:   "inline enum",
			kanban: Kanban{EntityType: "card", ColumnProperty: "stage", AddColumn: true},
		},
		{
			name:    "named custom type",
			kanban:  Kanban{EntityType: "ticket", ColumnProperty: "status", AddColumn: true},
			wantErr: `add_column needs column property "status" to be an inline enum`,
		},
		{
			name:    "namespaced type",
			kanban:  Kanban{EntityType: "isms.card", ColumnProperty: "stage", AddColumn: true},
			wantErr: `"isms.card" comes from namespace "isms"`,
		},
		{
			name: "columns_from board",
			kanban: Kanban{EntityType: "ticket", AddColumn: true,
				ColumnsFrom: &KanbanColumnsFrom{Relation: "in-category"}},
			wantErr: "add_column needs column_property",
		},
		{
			name:    "plain wall",
			kanban:  Kanban{EntityType: "ticket", Layout: LayoutWall, AddColumn: true},
			wantErr: "add_column needs column_property",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Kanbans: map[string]Kanban{"board": tt.kanban}}
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, addColumnMetamodel())
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
