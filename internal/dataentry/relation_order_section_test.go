package dataentry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// buildOrderedSection runs a recipe view with the given rules and returns its
// one section's row ids and relation order.
func buildOrderedSection(
	t *testing.T, mode metamodel.OrderableMode, rules []ViewTraverse, sec ViewSection,
) ([]string, *v1.RelationOrder) {
	t.Helper()
	app := newOrderableRelationsTestApp(t, mode)
	seedOrderableFixture(t, app)
	view := ViewConfig{Entry: ViewEntry{Type: "recipe"}, Traverse: rules, Sections: []ViewSection{sec}}
	result, err := app.views.executeView(context.Background(), view, "REC-001", defaultViewWorld())
	require.NoError(t, err)
	out := app.views.buildSections(context.Background(), view.Sections, result)
	require.Len(t, out, 1)
	var ids []string
	for _, row := range out[0].Rows {
		ids = append(ids, row.EntityID)
	}
	for _, e := range out[0].Entities {
		ids = append(ids, e.ID)
	}
	return ids, out[0].RelationOrder
}

func TestRelationOrder_Section(t *testing.T) {
	follow := []ViewTraverse{{From: "entry", Follow: "has-step", CollectAs: "steps"}}
	table := ViewSection{Heading: "Steps", Source: "steps", Display: "table", Columns: []ListColumn{{Property: "title"}}}
	cards := ViewSection{Heading: "Steps", Source: "steps", Display: "cards"}
	sorted := table
	sorted.Sort = []dataentryconfig.SortSpec{{Property: "title", Direction: "desc"}}
	grouped := table
	grouped.GroupBy = "properties.title"
	ordered := &v1.RelationOrder{Relation: "has-step", Anchor: "REC-001", AnchorType: "recipe", Movable: true}
	relationOrder := []string{"STP-Z", "STP-X", "STP-M", "STP-Y"}

	tests := []struct {
		name      string
		mode      metamodel.OrderableMode
		rules     []ViewTraverse
		sec       ViewSection
		wantIDs   []string // nil: any order
		wantOrder *v1.RelationOrder
	}{
		{"table in relation order", metamodel.OrderableOutgoing, follow, table, relationOrder, ordered},
		{"cards in relation order", metamodel.OrderableOutgoing, follow, cards, relationOrder, ordered},
		{"an author's sort wins", metamodel.OrderableOutgoing, follow, sorted,
			[]string{"STP-Z", "STP-Y", "STP-X", "STP-M"}, nil},
		{"grouped is not ordered", metamodel.OrderableOutgoing, follow, grouped, nil, nil},
		{"not orderable", metamodel.OrderableNone, follow, table, nil, nil},
		{"two rules write the collection", metamodel.OrderableOutgoing, append(follow,
			ViewTraverse{From: "steps", Follow: "has-step", CollectAs: "steps"}), table, nil, nil},
		{"recursive rule", metamodel.OrderableOutgoing,
			[]ViewTraverse{{From: "entry", Follow: "has-step", CollectAs: "steps", Recursive: true}}, table, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids, order := buildOrderedSection(t, tt.mode, tt.rules, tt.sec)
			if tt.wantIDs != nil {
				require.Equal(t, tt.wantIDs, ids)
			} else if tt.sec.GroupBy == "" {
				require.ElementsMatch(t, relationOrder, ids)
			}
			require.Equal(t, tt.wantOrder, order)
		})
	}
}
