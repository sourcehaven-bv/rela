package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// newOrderedTabApp is a recipe page whose `steps` tab lists the recipe's
// steps over has-step, seeded STP-Z=1, STP-X=2, STP-M=3, STP-Y=missing.
func newOrderedTabApp(t *testing.T, mode metamodel.OrderableMode) *App {
	t.Helper()
	app := newOrderableRelationsTestApp(t, mode)
	installPagesConfig(app, map[string]dataentryconfig.List{"steps": {EntityType: "step"}},
		map[string]dataentryconfig.Page{
			"rec": {Label: "Recipe", EntityType: "recipe", Tabs: []dataentryconfig.PageTab{
				{ID: "steps", Label: "Steps", List: "steps", Scope: &dataentryconfig.PageTabScope{
					Relation: "has-step", Direction: dataentryconfig.DirectionOutgoing,
				}},
			}},
		}, nil)
	seedOrderableFixture(t, app)
	return app
}

func orderedTabQuery(extra ...string) string {
	q := url.Values{"scope_page": {"rec"}, "scope_tab": {"steps"}, "anchor": {"REC-001"}}
	for i := 0; i+1 < len(extra); i += 2 {
		q.Add(extra[i], extra[i+1])
	}
	return q.Encode()
}

func listedIDs(resp v1.ListResponse) []string {
	ids := make([]string, 0, len(resp.Data))
	for _, e := range resp.Data {
		ids = append(ids, e.ID)
	}
	return ids
}

func TestRelationOrder_ScopedTab(t *testing.T) {
	tests := []struct {
		name      string
		mode      metamodel.OrderableMode
		query     string
		wantIDs   []string
		wantOrder *v1.RelationOrder
	}{
		{
			name:      "relation order by default",
			mode:      metamodel.OrderableOutgoing,
			query:     orderedTabQuery(),
			wantIDs:   []string{"STP-Z", "STP-X", "STP-M", "STP-Y"},
			wantOrder: &v1.RelationOrder{Relation: "has-step", Anchor: "REC-001", AnchorType: "recipe", Movable: true},
		},
		{
			name:      "paged in the same order",
			mode:      metamodel.OrderableOutgoing,
			query:     orderedTabQuery("per_page", "2", "page", "2"),
			wantIDs:   []string{"STP-M", "STP-Y"},
			wantOrder: &v1.RelationOrder{Relation: "has-step", Anchor: "REC-001", AnchorType: "recipe", Movable: true},
		},
		{
			name:    "a sort of the reader's own wins",
			mode:    metamodel.OrderableOutgoing,
			query:   orderedTabQuery("sort", "-title"),
			wantIDs: []string{"STP-Z", "STP-Y", "STP-X", "STP-M"},
		},
		{
			name:    "not orderable",
			mode:    metamodel.OrderableNone,
			query:   orderedTabQuery("sort", "title"),
			wantIDs: []string{"STP-M", "STP-X", "STP-Y", "STP-Z"},
		},
		{
			name:    "orderable on the other side only",
			mode:    metamodel.OrderableIncoming,
			query:   orderedTabQuery("sort", "title"),
			wantIDs: []string{"STP-M", "STP-X", "STP-Y", "STP-Z"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newOrderedTabApp(t, tt.mode)
			resp, rec := listUngated(t, app, "steps", "step", tt.query)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(t, tt.wantIDs, listedIDs(resp))
			require.Equal(t, tt.wantOrder, resp.Meta.RelationOrder)
		})
	}
}

// An order field the principal may not write leaves the order on show but
// not movable.
func TestRelationOrder_NotMovableWhenFieldReadOnly(t *testing.T) {
	app := newOrderedTabApp(t, metamodel.OrderableOutgoing)
	app.fieldResolver = fakeResolver{rv: RelationVerdicts{Types: map[string]RelationVerdict{
		"has-step": {Creatable: true, Removable: true, Fields: map[string]bool{metamodel.OrderPropertyOut: false}},
	}}}
	resp, rec := listUngated(t, app, "steps", "step", orderedTabQuery())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotNil(t, resp.Meta.RelationOrder)
	require.False(t, resp.Meta.RelationOrder.Movable)
}

// hiddenOrderResolver hides the order field from every principal.
type hiddenOrderResolver struct{ fakeResolver }

func (hiddenOrderResolver) RelationFieldVerdicts(
	context.Context, *entity.Entity, string, []string,
) map[string]bool {
	return map[string]bool{metamodel.OrderPropertyOut: false}
}

// Sorting by a value the principal cannot read would disclose it, so a
// hidden order field is not applied at all.
func TestRelationOrder_HiddenOrderFieldIsNotApplied(t *testing.T) {
	app := newOrderedTabApp(t, metamodel.OrderableOutgoing)
	app.fieldResolver = hiddenOrderResolver{}
	resp, rec := listUngated(t, app, "steps", "step", orderedTabQuery())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Nil(t, resp.Meta.RelationOrder)
	require.ElementsMatch(t, []string{"STP-Z", "STP-X", "STP-M", "STP-Y"}, listedIDs(resp))
}

// Prev/next on a relation-ordered tab walks the order the tab shows.
func TestRelationOrder_PositionWalksRelationOrder(t *testing.T) {
	app := newOrderedTabApp(t, metamodel.OrderableOutgoing)
	scope, err := json.Marshal(ScopeDescriptor{
		Source: "list", Type: "step", ScopePage: "rec", ScopeTab: "steps", Anchor: "REC-001",
	})
	require.NoError(t, err)
	q := url.Values{"id": {"STP-X"}, "scope": {string(scope)}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_position?"+q.Encode(), http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1EntityPosition(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var pos v1.Position
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pos))
	require.NotNil(t, pos.Prev)
	require.NotNil(t, pos.Next)
	require.Equal(t, "STP-Z", pos.Prev.ID)
	require.Equal(t, "STP-M", pos.Next.ID)
}
