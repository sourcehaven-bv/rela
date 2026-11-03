// Tests for TKT-4Q2EIVZ: editable table columns carry the row values and
// write verdicts an inline cell needs, and only when a column opts in.

package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

func TestBuildTableSection_EditDataOnlyForEditableColumns(t *testing.T) {
	ticket := &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]any{"title": "First", "status": "open", "priority": "high"},
	}
	tests := []struct {
		name     string
		sec      ViewSection
		wantEdit bool
	}{
		{
			name: "plain columns stay read-only",
			sec:  ViewSection{Display: "table", Columns: []ListColumn{{Property: "title"}, {Property: "status"}}},
		},
		{
			name: "section render does not make a table editable",
			sec: ViewSection{Display: "table", Render: dataentryconfig.RenderInput,
				Columns: []ListColumn{{Property: "title"}}},
		},
		{
			name: "an input column adds edit data",
			sec: ViewSection{Display: "table",
				Columns: []ListColumn{{Property: "title"}, {Property: "status", Render: dataentryconfig.RenderInput}}},
			wantEdit: true,
		},
		{
			name: "grouped rows get it too",
			sec: ViewSection{Display: "table", GroupBy: "status",
				Columns: []ListColumn{{Property: "status", Render: dataentryconfig.RenderInput}}},
			wantEdit: true,
		},
		{
			name: "columns_by_type groups get it per group",
			sec: ViewSection{Display: "table",
				ColumnsByType: map[string][]ListColumn{"ticket": {{Property: "status", Render: dataentryconfig.RenderInput}}}},
			wantEdit: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := testViewApp()
			setTestResolver(t, app, (&verdictBuilder{}).Hidden("priority").ReadOnly("status").Build())
			var sd SectionData
			app.views.buildTableSection(context.Background(), app.views.schema(), tc.sec,
				[]*entity.Entity{ticket}, &sd)
			rows := sd.Rows
			for _, g := range sd.Groups {
				rows = append(rows, g.Rows...)
			}
			if len(rows) != 1 {
				t.Fatalf("got %d rows, want 1", len(rows))
			}
			row := rows[0]
			if !tc.wantEdit {
				if row.Props != nil || row.FieldVerdicts != nil {
					t.Fatalf("read-only table row carries edit data: %+v %+v", row.Props, row.FieldVerdicts)
				}
				return
			}
			if row.Props["status"] != "open" {
				t.Errorf("Props[status] = %v, want open", row.Props["status"])
			}
			if _, ok := row.Props["priority"]; ok {
				t.Errorf("hidden priority leaked into Props: %+v", row.Props)
			}
			if _, ok := row.FieldVerdicts["priority"]; ok {
				t.Errorf("hidden priority leaked into FieldVerdicts: %+v", row.FieldVerdicts)
			}
			if v := row.FieldVerdicts["status"]; v.Writable == nil || *v.Writable {
				t.Errorf("status verdict = %+v, want read-only", v)
			}
			wire := viewRowToV1(row)
			if wire.Props == nil || wire.FieldAffordances == nil {
				t.Errorf("wire row lost edit data: %+v", wire)
			}
		})
	}
}

func TestViewSectionToV1_Totals(t *testing.T) {
	sec := SectionData{Display: "table", totals: []dataentryconfig.SectionTotal{
		{Property: "subtotal"}, {Property: "total", Label: "Total", Emphasis: true},
	}}
	got := viewSectionToV1(sec, nil).Totals
	if len(got) != 2 || got[1].Property != "total" || !got[1].Emphasis || got[1].Label != "Total" {
		t.Fatalf("totals = %+v", got)
	}
	if viewSectionToV1(SectionData{Display: "table"}, nil).Totals != nil {
		t.Fatal("a section without totals must omit the key")
	}
}

func TestViewColumnsToV1_Render(t *testing.T) {
	got := viewColumnsToV1([]ListColumn{{Property: "qty", Render: dataentryconfig.RenderInput}, {Property: "title"}})
	if got[0].Render != "input" || got[1].Render != "" {
		t.Fatalf("columns = %+v", got)
	}
}
