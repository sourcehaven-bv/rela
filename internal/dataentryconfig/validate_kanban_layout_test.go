package dataentryconfig

import (
	"strings"
	"testing"
)

func TestValidateConfig_KanbanLayout(t *testing.T) {
	tests := []struct {
		name    string
		kanban  Kanban
		wantErr string
	}{
		{
			name:   "plain wall without columns",
			kanban: Kanban{EntityType: "ticket", Layout: LayoutWall, Card: KanbanCard{Style: CardStyleNote}},
		},
		{
			name: "wall with columns_from",
			kanban: Kanban{EntityType: "ticket", Layout: LayoutWall, ColumnsFrom: &KanbanColumnsFrom{
				Relation: "in-category", OfferedBy: "offers-category",
			}},
		},
		{
			name:   "wall with column_property",
			kanban: Kanban{EntityType: "ticket", Layout: LayoutWall, ColumnProperty: "status"},
		},
		{
			name:    "columns layout still needs columns",
			kanban:  Kanban{EntityType: "ticket"},
			wantErr: "column_property is required",
		},
		{
			name:    "unknown layout",
			kanban:  Kanban{EntityType: "ticket", ColumnProperty: "status", Layout: "grid"},
			wantErr: `layout "grid" is not valid`,
		},
		{
			name:    "unknown card style",
			kanban:  Kanban{EntityType: "ticket", ColumnProperty: "status", Card: KanbanCard{Style: "postit"}},
			wantErr: `card.style "postit" is not valid`,
		},
		{
			name: "plain wall with columns",
			kanban: Kanban{
				EntityType: "ticket", Layout: LayoutWall, Columns: []KanbanColumn{{Value: "open"}},
			},
			wantErr: "columns need column_property",
		},
		{
			name: "wall with swimlanes",
			kanban: Kanban{
				EntityType: "ticket", Layout: LayoutWall, ColumnProperty: "status", SwimlaneProperty: "status",
			},
			wantErr: "swimlanes are not supported with layout: wall",
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
