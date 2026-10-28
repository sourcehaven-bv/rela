package dataentryconfig

import (
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
