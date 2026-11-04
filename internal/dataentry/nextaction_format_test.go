package dataentry

import (
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestNextActionValueFormatter(t *testing.T) {
	t.Parallel()
	meta := &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"stage": {Values: []string{"new", "won"}, Labels: map[string]string{"new": "Nieuw", "won": "Gewonnen"}},
		},
		Entities: map[string]metamodel.EntityDef{
			"deal": {Properties: map[string]metamodel.PropertyDef{
				"stage":  {Type: "stage"},
				"stages": {Type: "stage", List: true},
				"prio":   {Type: "enum", Values: []string{"hi"}, Labels: map[string]string{"hi": "Hoog"}},
				"due":    {Type: "date", Format: "02-01-2006"},
				"plain":  {Type: "date"},
				"note":   {Type: "string"},
			}},
		},
	}
	day := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name, typ, prop string
		v               any
		want            string
		wantOK          bool
	}{
		{"custom type label", "deal", "stage", "won", "Gewonnen", true},
		{"inline enum label", "deal", "prio", "hi", "Hoog", true},
		{"list of labels", "deal", "stages", []any{"new", "won"}, "Nieuw, Gewonnen", true},
		{"value without a label falls back", "deal", "stage", "lost", "", false},
		{"date in its format", "deal", "due", day, "03-11-2026", true},
		{"date without a format falls back", "deal", "plain", day, "", false},
		{"no labels falls back", "deal", "note", "x", "", false},
		{"unknown property falls back", "deal", "gone", "x", "", false},
		{"unknown type falls back", "ghost", "stage", "won", "", false},
	}
	format := nextActionValueFormatter(meta)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := format(tc.typ, tc.prop, tc.v)
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("format(%s.%s) = %q, %v; want %q, %v", tc.typ, tc.prop, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// The handler wires the schema-aware formatter: a labeled enum reads as its
// label in the served message.
func TestNextAction_MessageUsesEnumLabel(t *testing.T) {
	app := newTestAppV1(t)
	seedNextActionTickets(t, app, "TKT-001")
	withPlacement(t, app, nil)
	withNextActions(t, app, oneBand, map[string]dataentryconfig.NextActionSource{
		"about-this": {Band: "b", Context: "ticket", Suggest: "Status: {status}"},
	})

	s := app.State()
	meta := *s.Meta
	meta.Entities = maps.Clone(s.Meta.Entities)
	ticket := meta.Entities["ticket"]
	ticket.Properties = maps.Clone(ticket.Properties)
	status := ticket.Properties["status"]
	status.Labels = map[string]string{"open": "Openstaand"}
	ticket.Properties["status"] = status
	meta.Entities["ticket"] = ticket
	app.schema.Publish(&Schema{
		Cfg: s.Cfg, Meta: &meta, StyleMap: s.StyleMap,
		StyledTypes: s.StyledTypes, OpenAPIGen: s.OpenAPIGen,
	})

	got := decodeNextAction(t, getNextActionOn(aliceCtx(), t, app, entityPage("TKT-001")))
	require.NotNil(t, got.Suggestion)
	require.Equal(t, "Status: Openstaand", got.Suggestion.Message)
}
