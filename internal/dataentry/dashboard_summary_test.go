package dataentry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestFoldSummary(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    properties:
      cents: {type: integer}
      cost: {type: decimal}
      due: {type: date}
`))
	require.NoError(t, err)
	def, _ := meta.GetEntityDef("ticket")
	row := func(id string, props map[string]any, redacted ...string) *entity.Entity {
		return &entity.Entity{ID: id, Type: "ticket", Properties: props, Redacted: redacted}
	}
	rows := []*entity.Entity{
		row("TKT-1", map[string]any{"cents": 150, "cost": "12.50", "due": "2026-03-01"}),
		row("TKT-2", map[string]any{"cents": 25, "cost": 1.5, "due": time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC)}),
		row("TKT-3", map[string]any{"cost": "0.125"}),
		row("TKT-4", map[string]any{}, "cents", "cost", "due"),
	}
	tests := []struct {
		fn   dataentryconfig.SummaryFunction
		prop string
		want any
	}{
		{dataentryconfig.SummarySum, "cents", int64(175)},
		{dataentryconfig.SummaryAvg, "cents", "87.50"},
		{dataentryconfig.SummarySum, "cost", "14.125"},
		{dataentryconfig.SummaryAvg, "cost", "4.708"},
		{dataentryconfig.SummaryMin, "cost", "0.125"},
		{dataentryconfig.SummaryMax, "cost", "12.50"},
		{dataentryconfig.SummaryMin, "due", "2026-01-09"},
		{dataentryconfig.SummaryMax, "due", "2026-03-01"},
	}
	for _, tc := range tests {
		t.Run(string(tc.fn)+" "+tc.prop, func(t *testing.T) {
			got, partial := foldSummary(tc.fn, meta, def, tc.prop, rows)
			require.Equal(t, tc.want, got)
			require.True(t, partial, "TKT-4 hides the property")
		})
	}
	got, partial := foldSummary(dataentryconfig.SummarySum, meta, def, "cents", rows[2:3])
	require.Nil(t, got, "no row has a value")
	require.False(t, partial)
}
