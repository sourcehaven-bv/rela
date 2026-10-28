package dataentryconfig

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// List-sourced, matrix and ratio cards (TKT-2SGPJZ).

func TestDashboardCard_ColumnsByShape(t *testing.T) {
	cfg, err := loadAndValidate(t, `
version: "1.0"
lists:
  all: {entity_type: ticket}
dashboard:
  cards:
    - {title: Grid, list: all, display: matrix, rows: priority, columns: status, tone_by: priority, colors: {high: error, medium: orange/300}}
    - {title: Rows, list: all, display: table, columns: [{property: title}]}
    - {title: Share, list: all, display: breakdown, group_by: status}
    - {title: Count, query: "type:ticket", display: count}
`)
	require.NoError(t, err)
	cards := cfg.Dashboard.Cards
	assert.Equal(t, "status", cards[0].MatrixColumns)
	assert.Empty(t, cards[0].Columns)
	assert.Equal(t, "title", cards[1].Columns[0].Property)
	assert.Empty(t, cards[1].MatrixColumns)
}

// TestDashboardCard_MatrixAxisOptions pins that the axis titles and orders
// load on a matrix card over a declared integer range (TKT-BRAM1A).
func TestDashboardCard_MatrixAxisOptions(t *testing.T) {
	cfg, err := loadAndValidate(t, `
version: "1.0"
lists:
  all: {entity_type: ticket}
dashboard:
  cards:
    - {title: Risks, list: all, display: matrix, rows: impact, columns: priority,
       rows_label: Impact, columns_label: Likelihood, rows_order: desc, columns_order: asc}
`)
	require.NoError(t, err)
	card := cfg.Dashboard.Cards[0]
	assert.Equal(t, "Impact", card.RowsLabel)
	assert.Equal(t, "Likelihood", card.ColumnsLabel)
	assert.Equal(t, MatrixOrderDesc, card.RowsOrder)
	assert.Equal(t, MatrixOrderAsc, card.ColumnsOrder)
}

func TestMatrixAxisTooWide(t *testing.T) {
	tests := []struct {
		name   string
		lo, hi int64
		want   bool
	}{
		{"one value", 3, 3, false},
		{"exactly the cap", 1, 50, false},
		{"one past the cap", 1, 51, true},
		{"negative range", -60, -11, false},
		{"full int64 range", math.MinInt64, math.MaxInt64, true},
		{"top of int64", math.MaxInt64 - 1, math.MaxInt64, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, MatrixAxisTooWide(tc.lo, tc.hi))
		})
	}
}

func TestValidateDashboardCardSource(t *testing.T) {
	tests := []struct {
		name string
		card string
		want string
	}{
		{"both sources", `{title: X, list: all, query: "type:ticket", display: count}`, "sets both list: and query:"},
		{"no source", `{title: X, display: count}`, "sets neither list: nor query:"},
		{"unknown list", `{title: X, list: nope, display: count}`, `list "nope" is not configured`},
		{"matrix needs list", `{title: X, query: "type:ticket", display: matrix}`, "uses matrix display, which needs list:"},
		{"ratio needs list", `{title: X, query: "type:ticket", display: ratio}`, "uses ratio display, which needs list:"},
		{"matrix without rows", `{title: X, list: all, display: matrix, columns: status}`, "has no rows:"},
		{"axis not enum or integer", `{title: X, list: all, display: matrix, rows: title, columns: status}`,
			`rows "title" must be a single-valued enum or integer property`},
		{"unknown axis", `{title: X, list: all, display: matrix, rows: nope, columns: status}`,
			`rows "nope" is not a property of "ticket"`},
		{"tone_by not enum", `{title: X, list: all, display: matrix, rows: status, columns: priority, tone_by: assignee}`,
			`tone_by "assignee" must be a single-valued enum property`},
		{"colors without tone_by", `{title: X, list: all, display: matrix, rows: status, columns: priority, colors: {high: error}}`,
			"colors needs tone_by"},
		{"colors key not a value", `{title: X, list: all, display: matrix, rows: status, columns: priority, tone_by: priority, colors: {urgent: error}}`,
			`colors key "urgent" is not a value of "priority"`},
		{"colors value unknown", `{title: X, list: all, display: matrix, rows: status, columns: priority, tone_by: priority, colors: {high: pink}}`,
			`colors[high] "pink" is neither a tone`},
		{"ratio without match", `{title: X, list: all, display: ratio}`, "uses ratio display but has no match:"},
		{"ratio with undeclared scope", `{title: X, list: all, display: ratio, match: mine}`, `query_scope "mine" is not declared`},
		{"matrix keys on count", `{title: X, list: all, display: count, rows: status}`, "apply to matrix display only"},
		{"axis label on count", `{title: X, list: all, display: count, rows_label: Impact}`, "apply to matrix display only"},
		{"axis order on count", `{title: X, list: all, display: count, columns_order: desc}`, "apply to matrix display only"},
		{"unknown rows order", `{title: X, list: all, display: matrix, rows: impact, columns: status, rows_order: down}`,
			`rows_order "down" must be "asc" or "desc"`},
		{"declared range too wide", `{title: X, list: all, display: matrix, rows: score, columns: status}`,
			`rows "score" declares 0..100, more than the 50 values a matrix axis shows`},
		{"match on count", `{title: X, list: all, display: count, match: all}`, "match applies to ratio display only"},
		{"unknown group_by", `{title: X, list: all, display: breakdown, group_by: nope}`, `group_by "nope" is not a property`},
		{"list-valued group_by", `{title: X, list: all, display: breakdown, group_by: tags}`,
			`group_by "tags" must be a single-valued property`},
		{"ratio over all", `{title: X, list: all, display: ratio, match: all}`, `match "all" admits every row`},
		{"scope error names the card", `{title: X, list: all, display: ratio, match: mine}`,
			`dashboard: card[0] "X": match "mine": query_scope "mine" is not declared`},
		{"unknown table column", `{title: X, list: all, display: table, columns: [{property: nope}]}`,
			`column "nope" is not a property of "ticket"`},
		{"unknown table sort", `{title: X, list: all, display: table, columns: [{property: title}], sort: [{property: nope}]}`,
			`sort "nope" is not a property of "ticket"`},
		{"table limit too large", `{title: X, list: all, display: table, columns: [{property: title}], limit: 101}`,
			"limit 101 is above the maximum of 100"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadAndValidate(t, `
version: "1.0"
lists:
  all: {entity_type: ticket}
dashboard:
  cards:
    - `+tc.card+`
`)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// Card width (TKT-H21TYS).

func TestValidateDashboardCardWidth(t *testing.T) {
	tests := []struct {
		name  string
		width string
		want  string // empty: the config loads
	}{
		{"unset", "", ""},
		{"normal", "normal", ""},
		{"wide", "wide", ""},
		{"full", "full", ""},
		{"unknown name", "huge", `dashboard: card[0] "X" has invalid width "huge" (valid: normal, wide, full)`},
		{"case matters", "Wide", `invalid width "Wide"`},
		{"surrounding space", `" wide"`, `invalid width " wide"`},
		{"column count", "2",
			`invalid width "2" (valid: normal, wide, full); width takes a named size, not a column count`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			width := ""
			if tc.width != "" {
				width = ", width: " + tc.width
			}
			cfg, err := loadAndValidate(t, `
version: "1.0"
dashboard:
  cards:
    - {title: X, query: "type:ticket", display: count`+width+`}
`)
			if tc.want != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.want)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.width, cfg.Dashboard.Cards[0].Width)
		})
	}
}

func TestValidateDashboardCardWidth_NamedDashboard(t *testing.T) {
	_, err := loadAndValidate(t, `
version: "1.0"
dashboards:
  ops:
    cards:
      - {title: Y, query: "type:ticket", display: count, width: half}
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `dashboards[ops]: card[0] "Y" has invalid width "half"`)
}
