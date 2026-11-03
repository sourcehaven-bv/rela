package dataentryconfig

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TKT-4Q2EIVZ: editable table columns and section totals.

func tableSectionConfig(sec ViewSection) *Config {
	return &Config{
		Version: "1.0",
		App:     AppConfig{Name: "Test App"},
		Views: map[string]ViewConfig{
			"ticket_view": {
				Title:    "Ticket",
				Entry:    ViewEntry{Type: "ticket"},
				Sections: []ViewSection{sec},
			},
		},
	}
}

func TestValidateConfig_TableColumnRenderAndTotals(t *testing.T) {
	tests := []struct {
		name    string
		sec     ViewSection
		wantErr string // substring; empty means none of this feature's errors
	}{
		{
			name: "input on a property column of a table",
			sec: ViewSection{Source: "entry", Display: "table",
				Columns: []ListColumn{{Property: "score", Render: RenderInput}}},
		},
		{
			name: "input in columns_by_type",
			sec: ViewSection{Source: "entry", Display: "table",
				ColumnsByType: map[string][]ListColumn{"ticket": {{Property: "score", Render: RenderInput}}}},
		},
		{
			name: "invalid render mode",
			sec: ViewSection{Source: "entry", Display: "table",
				Columns: []ListColumn{{Property: "score", Render: "bogus"}}},
			wantErr: `column[0] has invalid render mode "bogus"`,
		},
		{
			name: "column render outside a table",
			sec: ViewSection{Source: "entry", Display: "list",
				Columns: []ListColumn{{Property: "score", Render: RenderInput}}},
			wantErr: `column render applies to display: table only`,
		},
		{
			name: "input on a relation column",
			sec: ViewSection{Source: "entry", Display: "table",
				Columns: []ListColumn{{Relation: "blocks", Render: RenderInput}}},
			wantErr: `render: input needs a property column`,
		},
		{
			name: "input on a face column",
			sec: ViewSection{Source: "entry", Display: "table",
				Columns: []ListColumn{{Face: true, Render: RenderInput}}},
			wantErr: `render: input needs a property column`,
		},
		{
			name: "input with link",
			sec: ViewSection{Source: "entry", Display: "table",
				Columns: []ListColumn{{Property: "title", Link: "self", Render: RenderInput}}},
			wantErr: `cannot be combined with link`,
		},
		{
			name: "render on a nested level",
			sec: ViewSection{Source: "entry", Display: DisplayNested,
				ParentColumns: map[string][]ListColumn{"ticket": {{Property: "score", Render: RenderInput}}}},
			wantErr: `parent_columns.ticket[0]: render is only supported`,
		},
		{
			name: "totals on a table",
			sec: ViewSection{Source: "entry", Display: "table",
				Totals: []SectionTotal{{Property: "score", Emphasis: true}}},
		},
		{
			name:    "totals outside a table",
			sec:     ViewSection{Source: "entry", Display: "list", Totals: []SectionTotal{{Property: "score"}}},
			wantErr: `totals apply to display: table only`,
		},
		{
			name:    "totals without a property",
			sec:     ViewSection{Source: "entry", Display: "table", Totals: []SectionTotal{{Label: "Sum"}}},
			wantErr: `totals[0]: property is required`,
		},
		{
			name:    "totals naming an undeclared property",
			sec:     ViewSection{Source: "entry", Display: "table", Totals: []SectionTotal{{Property: "nope"}}},
			wantErr: `totals[0]: entity "ticket" has no property "nope"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConfig([]byte(`version: "1.0"`), tableSectionConfig(tc.sec), testMetamodel())
			if tc.wantErr == "" {
				if err != nil && (strings.Contains(err.Error(), "render") || strings.Contains(err.Error(), "totals")) {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %v does not contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidateConfig_ColumnRenderRefusedOffTables(t *testing.T) {
	t.Run("list column", func(t *testing.T) {
		cfg := &Config{Lists: map[string]List{
			"tickets": {EntityType: "ticket", Columns: []ListColumn{{Property: "title", Render: RenderInput}}},
		}}
		err := ValidateConfig([]byte(`version: "1.0"`), cfg, testMetamodel())
		if err == nil || !strings.Contains(err.Error(), `list "tickets": column[0]: render is only supported`) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("side panel section", func(t *testing.T) {
		cfg := &Config{Forms: map[string]Form{
			"ticket_form": {
				EntityType: "ticket",
				SidePanel: &SidePanelConfig{Sections: []ViewSection{{
					Source: "entry", Display: "table",
					Columns: []ListColumn{{Property: "score", Render: RenderInput}},
					ColumnsByType: map[string][]ListColumn{
						"ticket": {{Property: "score", Render: RenderInput}},
					},
					Totals: []SectionTotal{{Property: "score"}},
				}}},
			},
		}}
		err := ValidateConfig([]byte(`version: "1.0"`), cfg, testMetamodel())
		if err == nil {
			t.Fatal("expected errors")
		}
		for _, want := range []string{
			"sets totals, which a side panel",
			"column[0] sets render, which a side panel",
			"columns_by_type.ticket[0] sets render, which a side panel",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not contain %q", err, want)
			}
		}
	})
}

func TestValidateConfig_InputColumnPropertyTypes(t *testing.T) {
	meta := testMetamodel()
	ticket := meta.Entities["ticket"]
	ticket.Properties["attachment"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeFile}
	ticket.Properties["double"] = metamodel.PropertyDef{Type: "integer", Computed: "score * 2"}
	ticket.Properties["children"] = metamodel.PropertyDef{
		Type: "integer", Rollup: &metamodel.RollupDef{Via: "blocks", Count: true},
	}
	ticket.Properties["jira"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeExternalRef}
	meta.Entities["ticket"] = ticket

	tests := []struct {
		name    string
		col     ListColumn
		byType  bool
		wantErr string
	}{
		{name: "plain property", col: ListColumn{Property: "score", Render: RenderInput}},
		{name: "file", col: ListColumn{Property: "attachment", Render: RenderInput}, wantErr: "is a file property"},
		{name: "computed", col: ListColumn{Property: "double", Render: RenderInput}, wantErr: "is computed"},
		{name: "roll-up", col: ListColumn{Property: "children", Render: RenderInput}, wantErr: "is a roll-up"},
		{name: "external ref", col: ListColumn{Property: "jira", Render: RenderInput}, wantErr: "is an external reference"},
		{name: "file shown read-only", col: ListColumn{Property: "attachment"}},
		{
			name: "file in columns_by_type", col: ListColumn{Property: "attachment", Render: RenderInput},
			byType: true, wantErr: "columns_by_type.ticket[0]: render: input on ticket.attachment",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sec := ViewSection{Source: "entry", Display: "table", Columns: []ListColumn{tc.col}}
			if tc.byType {
				sec = ViewSection{Source: "entry", Display: "table",
					ColumnsByType: map[string][]ListColumn{"ticket": {tc.col}}}
			}
			err := ValidateConfig([]byte(`version: "1.0"`), tableSectionConfig(sec), meta)
			if tc.wantErr == "" {
				if err != nil && strings.Contains(err.Error(), "render: input on") {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %v does not contain %q", err, tc.wantErr)
			}
		})
	}
}
