package dataentryconfig

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// quickAddMetamodel extends testMetamodel with the shapes quick-add has to
// judge: a required property with and without a default, a manual-id type
// and a computed title.
func quickAddMetamodel() *metamodel.Metamodel {
	m := columnsFromMetamodel()
	m.Entities["risk"] = metamodel.EntityDef{
		Label: "Risk", IDPrefix: "RSK-",
		Properties: map[string]metamodel.PropertyDef{
			"title":  {Type: "string", Required: true},
			"status": {Type: "status", Required: true}, // custom type default
			"impact": {Type: "string", Required: true, Default: "low"},
			"owner":  {Type: "string", Required: true},
			"lane":   {Type: "priority", Required: true},
		},
	}
	m.Entities["code"] = metamodel.EntityDef{
		Label: "Code", IDType: metamodel.IDTypeManual,
		Properties: map[string]metamodel.PropertyDef{
			"title":  {Type: "string", Required: true},
			"status": {Type: "status"},
		},
	}
	m.Entities["derived"] = metamodel.EntityDef{
		Label: "Derived", IDPrefix: "DRV-",
		Properties: map[string]metamodel.PropertyDef{
			"name":   {Type: "string"},
			"label":  {Type: "string", Computed: "entity.name"},
			"status": {Type: "status"},
		},
	}
	return m
}

func TestValidateConfig_KanbanQuickAdd(t *testing.T) {
	tests := []struct {
		name    string
		kanban  Kanban
		wantErr []string
	}{
		{
			name: "off is never checked",
			kanban: Kanban{EntityType: "code", ColumnProperty: "status",
				Card: KanbanCard{Title: "title"}},
		},
		{
			name: "column board",
			kanban: Kanban{EntityType: "ticket", ColumnProperty: "status", QuickAdd: true,
				Card: KanbanCard{Title: "title"}},
		},
		{
			name:   "title from the primary property",
			kanban: Kanban{EntityType: "ticket", ColumnProperty: "status", QuickAdd: true},
		},
		{
			name: "plain wall",
			kanban: Kanban{EntityType: "ticket", Layout: LayoutWall, QuickAdd: true,
				Card: KanbanCard{Title: "title"}},
		},
		{
			name: "columns_from board",
			kanban: Kanban{EntityType: "ticket", QuickAdd: true, Card: KanbanCard{Title: "title"},
				ColumnsFrom: &KanbanColumnsFrom{Relation: "in-category"}},
		},
		{
			name: "swimlane fills a required property",
			kanban: Kanban{EntityType: "risk", ColumnProperty: "status", SwimlaneProperty: "lane",
				QuickAdd: true, Card: KanbanCard{Title: "title"}},
			wantErr: []string{`required property "owner" of "risk" has no value`},
		},
		{
			name: "several required properties without a source",
			kanban: Kanban{EntityType: "risk", ColumnProperty: "status", QuickAdd: true,
				Card: KanbanCard{Title: "title"}},
			wantErr: []string{`required properties "lane", "owner" of "risk" have no value`},
		},
		{
			name: "manual ids",
			kanban: Kanban{EntityType: "code", ColumnProperty: "status", QuickAdd: true,
				Card: KanbanCard{Title: "title"}},
			wantErr: []string{`entity "code" uses manual ids`},
		},
		{
			name: "computed title",
			kanban: Kanban{EntityType: "derived", ColumnProperty: "status", QuickAdd: true,
				Card: KanbanCard{Title: "label"}},
			wantErr: []string{`title property "label" is computed`},
		},
		{
			name:    "no title property",
			kanban:  Kanban{EntityType: "derived", ColumnProperty: "status", QuickAdd: true},
			wantErr: []string{"quick_add: needs a title property"},
		},
		{
			name: "unknown title property",
			kanban: Kanban{EntityType: "ticket", ColumnProperty: "status", QuickAdd: true,
				Card: KanbanCard{Title: "headline"}},
			wantErr: []string{`quick_add: title property "headline" is not a property of "ticket"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Kanbans: map[string]Kanban{"board": tt.kanban}}
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, quickAddMetamodel())
			if len(tt.wantErr) == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want errors %q, got none", tt.wantErr)
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("want error containing %q, got %v", want, err)
				}
			}
		})
	}
}
