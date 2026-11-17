package dataentryconfig

import (
	"maps"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidateConfig_KanbanCellColors(t *testing.T) {
	type cells = map[string]map[string]KanbanCellColor
	board := func(c cells) Kanban {
		return Kanban{
			EntityType: "ticket", ColumnProperty: "status", SwimlaneProperty: "priority",
			Card: KanbanCard{Title: "title"}, CellColors: c,
		}
	}
	tests := []struct {
		name    string
		kanban  Kanban
		palette *PaletteConfig
		wantErr string
	}{
		{name: "tones", kanban: board(cells{
			"open":   {"high": {Color: "error"}, "low": {Color: "success"}},
			"closed": {"medium": {Color: "warning", Label: "Medium risk"}},
		})},
		{name: "empty", kanban: board(cells{})},
		{name: "kit shade with a label", kanban: board(cells{"open": {"high": {Color: "red/700", Label: "High"}}})},
		{name: "bare hue with a label", kanban: board(cells{"open": {"high": {Color: "teal", Label: "Teal"}}})},
		{name: "operator color with a label",
			kanban:  board(cells{"open": {"high": {Color: "risk", Label: "Risk"}}}),
			palette: &PaletteConfig{Colors: map[string]OperatorColor{"risk": {Light: "#aa0000"}}}},
		{name: "declared columns and lanes", kanban: func() Kanban {
			k := board(cells{"open": {"low": {Color: "info"}}})
			k.Columns = []KanbanColumn{{Value: "open"}}
			k.Swimlanes = []KanbanSwimlane{{Value: "low"}}
			return k
		}()},
		{name: "unknown column", kanban: board(cells{"nope": {"high": {Color: "error"}}}),
			wantErr: `kanban "board": cell_colors: "nope" is not a column of the board (columns: open, in-progress, closed)`},
		{name: "column the board does not show", kanban: func() Kanban {
			k := board(cells{"closed": {"high": {Color: "error"}}})
			k.Columns = []KanbanColumn{{Value: "open"}}
			return k
		}(), wantErr: `"closed" is not a column of the board (columns: open)`},
		{name: "unknown lane", kanban: board(cells{"open": {"urgent": {Color: "error"}}}),
			wantErr: `kanban "board": cell_colors.open.urgent: "urgent" is not a swimlane of the board ` +
				`(swimlanes: high, medium, low)`},
		{name: "lane the board does not show", kanban: func() Kanban {
			k := board(cells{"open": {"low": {Color: "error"}}})
			k.Swimlanes = []KanbanSwimlane{{Value: "high"}}
			return k
		}(), wantErr: `"low" is not a swimlane of the board (swimlanes: high)`},
		{name: "unknown color", kanban: board(cells{"open": {"high": {Color: "crimson"}}}),
			wantErr: `kanban "board": cell_colors.open.high: "crimson" is neither a tone`},
		{name: "empty color", kanban: board(cells{"open": {"high": {}}}),
			wantErr: `cell_colors.open.high: "" is neither a tone`},
		{name: "color without a label", kanban: board(cells{"open": {"high": {Color: "red/700"}}}),
			wantErr: `cell_colors.open.high: color "red/700" needs a label`},
		{name: "blank label", kanban: board(cells{"open": {"high": {Color: "red", Label: "  "}}}),
			wantErr: `color "red" needs a label`},
		{name: "without swimlanes", kanban: func() Kanban {
			k := board(cells{"open": {"high": {Color: "error"}}})
			k.SwimlaneProperty = ""
			return k
		}(), wantErr: `kanban "board": cell_colors needs swimlane_property`},
		{name: "with columns_from", kanban: Kanban{
			EntityType: "ticket", SwimlaneProperty: "priority",
			ColumnsFrom: &KanbanColumnsFrom{Relation: "in-category"},
			CellColors:  cells{"CAT-1": {"high": {Color: "error"}}},
		}, wantErr: `kanban "board": cell_colors is not supported with columns_from`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := testMetamodel()
			maps.Copy(meta.Relations, columnsFromMetamodel().Relations)
			cfg := &Config{Kanbans: map[string]Kanban{"board": tt.kanban}, Palette: tt.palette}
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

// TestKanbanCellColors_YAML pins both shapes an operator writes for a cell.
func TestKanbanCellColors_YAML(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte(`
version: "1.0"
kanbans:
  risks:
    entity_type: ticket
    column_property: status
    swimlane_property: priority
    cell_colors:
      open:
        high: error
        low:
          color: red/700
          label: Hoog
    card:
      title: title
`), &cfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := cfg.Kanbans["risks"].CellColors["open"]
	if c := got["high"]; c != (KanbanCellColor{Color: "error"}) {
		t.Errorf("scalar cell = %+v, want color error", c)
	}
	if c := got["low"]; c != (KanbanCellColor{Color: "red/700", Label: "Hoog"}) {
		t.Errorf("mapping cell = %+v, want red/700 with label Hoog", c)
	}
}

func TestKanbanCellColors_YAMLRejectsBadCells(t *testing.T) {
	tests := []struct {
		name, cell, wantErr string
	}{
		{name: "sequence", cell: "[error]", wantErr: "line 6: cell_colors: a cell is a color or a mapping"},
		{name: "unknown key", cell: "{color: error, lable: High}",
			wantErr: `line 6: cell_colors: unknown key "lable" in a cell (valid: color, label)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg Config
			err := yaml.Unmarshal([]byte(`
kanbans:
  risks:
    cell_colors:
      open:
        high: `+tt.cell+"\n"), &cfg)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

// A bad color is reported even while the lane property is itself invalid,
// so the operator sees both mistakes in one load.
func TestValidateConfig_KanbanCellColorsCheckedWithABadAxis(t *testing.T) {
	cfg := &Config{Kanbans: map[string]Kanban{"board": {
		EntityType: "ticket", ColumnProperty: "status", SwimlaneProperty: "nope",
		Card:       KanbanCard{Title: "title"},
		CellColors: map[string]map[string]KanbanCellColor{"open": {"high": {Color: "crimson"}}},
	}}}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, testMetamodel())
	for _, want := range []string{`swimlane_property "nope" not in entity`, `cell_colors.open.high: "crimson"`} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want error containing %q, got %v", want, err)
		}
	}
}

func TestResolveKanbanCellClasses(t *testing.T) {
	operator := map[string]OperatorColor{"risk": {Light: "#aa0000"}}
	got := ResolveKanbanCellClasses(map[string]map[string]KanbanCellColor{
		"a": {
			"tone":     {Color: "warning"},
			"hue":      {Color: "gray", Label: "Grey"},
			"shade":    {Color: "green/100", Label: "Low"},
			"operator": {Color: "risk", Label: "Risk"},
			"bad":      {Color: "crimson", Label: "?"},
		},
	}, operator)
	want := map[string]string{
		"tone": "badge-orange", "hue": "badge-gray", "shade": "kit-green-100",
		"operator": "custom-risk", "bad": "",
	}
	for lane, class := range want {
		if c := got["a"][lane].Class; c != class {
			t.Errorf("%s: class = %q, want %q", lane, c, class)
		}
	}
	if ResolveKanbanCellClasses(nil, nil) != nil {
		t.Error("nil cells should stay nil")
	}
}
