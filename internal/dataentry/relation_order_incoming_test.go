package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// newIncomingTabApp is a step page whose `recipes` tab lists the recipes
// that link to the step over has-step, seeded with incoming order
// REC-Z=1, REC-X=2, REC-M=3, REC-Y=missing.
func newIncomingTabApp(t *testing.T, mode metamodel.OrderableMode) *App {
	t.Helper()
	app := newOrderableRelationsTestApp(t, mode)
	installPagesConfig(app, map[string]dataentryconfig.List{"recipes": {EntityType: "recipe"}},
		map[string]dataentryconfig.Page{
			"stp": {Label: "Step", EntityType: "step", Tabs: []dataentryconfig.PageTab{
				{ID: "recipes", Label: "Recipes", List: "recipes", Scope: &dataentryconfig.PageTabScope{
					Relation: "has-step", Direction: dataentryconfig.DirectionIncoming,
				}},
			}},
		}, nil)
	seedIncomingFixture(t, app)
	return app
}

func seedIncomingFixture(t *testing.T, app *App) {
	t.Helper()
	seedEntity(app, &entity.Entity{ID: "STP-001", Type: "step", Properties: map[string]any{"title": "Shared"}})
	for _, s := range []struct {
		id    string
		order any
	}{{"REC-Z", 1.0}, {"REC-X", 2.0}, {"REC-M", 3.0}, {"REC-Y", nil}} {
		seedEntity(app, &entity.Entity{ID: s.id, Type: "recipe", Properties: map[string]any{"title": s.id}})
		props := map[string]any{}
		if s.order != nil {
			props[metamodel.OrderPropertyIn] = s.order
		}
		if _, err := app.store.CreateRelation(t.Context(),
			entity.RelationKey{From: s.id, Type: "has-step", To: "STP-001"},
			&store.RelationData{Properties: props}); err != nil {
			t.Fatalf("seed relation: %v", err)
		}
	}
}

func incomingTabQuery(extra ...string) string {
	q := url.Values{"scope_page": {"stp"}, "scope_tab": {"recipes"}, "anchor": {"STP-001"}}
	for i := 0; i+1 < len(extra); i += 2 {
		q.Add(extra[i], extra[i+1])
	}
	return q.Encode()
}

// patchIncoming sends an incoming relation PATCH for source --has-step--> STP-001.
func patchIncoming(t *testing.T, app *App, source, position string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/steps/STP-001/relations/has-step/"+source,
		strings.NewReader(`{"direction":"incoming","position":`+position+`}`))
	rec := httptest.NewRecorder()
	app.write.handleV1UpdateRelation(rec, req, "step", "STP-001", "has-step", source)
	return rec
}

func TestRelationOrder_IncomingTab(t *testing.T) {
	ordered := &v1.RelationOrder{
		Relation: "has-step", Anchor: "STP-001", AnchorType: "step", Movable: true, Direction: "incoming",
	}
	tests := []struct {
		name      string
		mode      metamodel.OrderableMode
		query     string
		wantIDs   []string
		wantOrder *v1.RelationOrder
	}{
		{"relation order by default", metamodel.OrderableIncoming, incomingTabQuery(),
			[]string{"REC-Z", "REC-X", "REC-M", "REC-Y"}, ordered},
		{"both sides orderable", metamodel.OrderableBoth, incomingTabQuery(),
			[]string{"REC-Z", "REC-X", "REC-M", "REC-Y"}, ordered},
		{"paged in the same order", metamodel.OrderableIncoming, incomingTabQuery("per_page", "2", "page", "2"),
			[]string{"REC-M", "REC-Y"}, ordered},
		{"a sort of the reader's own wins", metamodel.OrderableIncoming, incomingTabQuery("sort", "-title"),
			[]string{"REC-Z", "REC-Y", "REC-X", "REC-M"}, nil},
		{"orderable on the other side only", metamodel.OrderableOutgoing, incomingTabQuery("sort", "title"),
			[]string{"REC-M", "REC-X", "REC-Y", "REC-Z"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newIncomingTabApp(t, tt.mode)
			resp, rec := listUngated(t, app, "recipes", "recipe", tt.query)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(t, tt.wantIDs, listedIDs(resp))
			require.Equal(t, tt.wantOrder, resp.Meta.RelationOrder)
		})
	}
}

// denyUpdateFrom refuses an update of a relation from one source and allows
// everything else.
type denyUpdateFrom struct {
	acl.NopACL
	from string
}

func (a denyUpdateFrom) AuthorizeWrite(_ context.Context, req acl.WriteRequest) acl.Decision {
	if s, ok := req.Subject.(acl.RelationSubject); ok && req.Op == acl.OpUpdate && s.FromID == a.from {
		return acl.Decision{Allow: false, RuleKind: "test"}
	}
	return acl.Decision{Allow: true, RuleKind: "test"}
}

// A move may rewrite any visible edge into the anchor, so one source the
// principal may not update makes the whole list not movable.
func TestRelationOrder_IncomingNotMovableWhenOneSourceDenied(t *testing.T) {
	app := newIncomingTabApp(t, metamodel.OrderableIncoming)
	app.acl = denyUpdateFrom{from: "REC-M"}
	resp, rec := listUngated(t, app, "recipes", "recipe", incomingTabQuery())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotNil(t, resp.Meta.RelationOrder)
	require.False(t, resp.Meta.RelationOrder.Movable)
}

func TestRelationOrder_IncomingNotMovableWhenFieldReadOnly(t *testing.T) {
	app := newIncomingTabApp(t, metamodel.OrderableIncoming)
	app.fieldResolver = fakeResolver{rv: RelationVerdicts{Types: map[string]RelationVerdict{
		"has-step": {Creatable: true, Removable: true, Fields: map[string]bool{metamodel.OrderPropertyIn: false}},
	}}}
	resp, rec := listUngated(t, app, "recipes", "recipe", incomingTabQuery())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotNil(t, resp.Meta.RelationOrder)
	require.False(t, resp.Meta.RelationOrder.Movable)
}

// hiddenOrderInResolver hides the incoming order field from every principal.
type hiddenOrderInResolver struct{ fakeResolver }

func (hiddenOrderInResolver) RelationFieldVerdicts(
	context.Context, *entity.Entity, string, []string,
) map[string]bool {
	return map[string]bool{metamodel.OrderPropertyIn: false}
}

func TestRelationOrder_IncomingHiddenOrderFieldIsNotApplied(t *testing.T) {
	app := newIncomingTabApp(t, metamodel.OrderableIncoming)
	app.fieldResolver = hiddenOrderInResolver{}
	resp, rec := listUngated(t, app, "recipes", "recipe", incomingTabQuery())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Nil(t, resp.Meta.RelationOrder)
}

func TestRelationPosition_IncomingMoves(t *testing.T) {
	app := newIncomingTabApp(t, metamodel.OrderableIncoming)
	steps := []struct {
		source, position string
		want             []string
	}{
		{"REC-Y", `{"before":"REC-X"}`, []string{"REC-Z", "REC-Y", "REC-X", "REC-M"}},
		{"REC-Z", `{"after":"REC-M"}`, []string{"REC-Y", "REC-X", "REC-M", "REC-Z"}},
		{"REC-M", `{"step":-1}`, []string{"REC-Y", "REC-M", "REC-X", "REC-Z"}},
	}
	for _, s := range steps {
		rec := patchIncoming(t, app, s.source, s.position)
		require.Equal(t, http.StatusNoContent, rec.Code, "%s %s: %s", s.source, s.position, rec.Body)
		resp, lrec := listUngated(t, app, "recipes", "recipe", incomingTabQuery())
		require.Equal(t, http.StatusOK, lrec.Code, lrec.Body.String())
		require.Equal(t, s.want, listedIDs(resp), "after %s %s", s.source, s.position)
	}
	// The outgoing side was never written.
	rel, err := app.store.GetRelation(t.Context(), entity.RelationKey{From: "REC-Y", Type: "has-step", To: "STP-001"})
	require.NoError(t, err)
	require.NotContains(t, rel.Properties, metamodel.OrderPropertyOut)
}

func TestRelationPosition_IncomingBadRequests(t *testing.T) {
	tests := []struct {
		name     string
		mode     metamodel.OrderableMode
		position string
		code     string
	}{
		{"outgoing only", metamodel.OrderableOutgoing, `{"step":1}`, "relation_not_orderable"},
		{"relative to itself", metamodel.OrderableIncoming, `{"after":"REC-Z"}`, "order_position_invalid"},
		{"before and after", metamodel.OrderableIncoming, `{"before":"REC-X","after":"REC-M"}`, "order_position_invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newIncomingTabApp(t, tt.mode)
			rec := patchIncoming(t, app, "REC-Z", tt.position)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), tt.code)
		})
	}
}

func TestRelationPosition_IncomingOrderFieldNotWritable(t *testing.T) {
	app := newIncomingTabApp(t, metamodel.OrderableIncoming)
	app.fieldResolver = fakeResolver{rv: RelationVerdicts{Types: map[string]RelationVerdict{
		"has-step": {Creatable: true, Removable: true, Fields: map[string]bool{metamodel.OrderPropertyIn: false}},
	}}}
	rec := patchIncoming(t, app, "REC-Z", `{"step":1}`)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "meta-read-only:has-step._order_in")
}

// incomingLeakApp makes `implements` orderable on its incoming side over the
// neighbor-leak fixture: FEAT-VIS has edges from TKT-VIS, which alice may
// read, and TKT-HIDDEN, which she may not.
func incomingLeakApp(t *testing.T) (*App, *acl.Declarative) {
	t.Helper()
	app := newTestAppV1(t)
	d := seedNeighborLeakFixture(t, app)
	rd := app.State().Meta.Relations["implements"]
	rd.Orderable = metamodel.OrderableIncoming
	app.State().Meta.Relations["implements"] = rd
	return app, d
}

func patchIncomingAs(t *testing.T, app *App, d *acl.Declarative, position string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/features/FEAT-VIS/relations/implements/TKT-VIS",
		strings.NewReader(`{"direction":"incoming","position":`+position+`}`))
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.write.handleV1UpdateRelation(rec, req, "feature", "FEAT-VIS", "implements", "TKT-VIS")
	return rec
}

// A source the principal cannot read answers the same 404 as one that does
// not exist.
func TestRelationPosition_IncomingHiddenSiblingIsTheUniform404(t *testing.T) {
	app, d := incomingLeakApp(t)
	hidden := patchIncomingAs(t, app, d, `{"after":"TKT-HIDDEN"}`)
	missing := patchIncomingAs(t, app, d, `{"after":"TKT-NOPE"}`)
	require.Equal(t, http.StatusNotFound, hidden.Code, hidden.Body.String())
	require.Equal(t, http.StatusNotFound, missing.Code, missing.Body.String())
	require.Equal(t, missing.Body.String(), hidden.Body.String())
}

// A step counts only the edges from sources the principal can see, and
// writes no other edge.
func TestRelationPosition_IncomingStepIgnoresHiddenSiblings(t *testing.T) {
	app, d := incomingLeakApp(t)
	rec := patchIncomingAs(t, app, d, `{"step":-1}`)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	for _, from := range []string{"TKT-HIDDEN", "TKT-VIS"} {
		rel, err := app.store.GetRelation(t.Context(),
			entity.RelationKey{From: from, Type: "implements", To: "FEAT-VIS"})
		require.NoError(t, err)
		require.NotContains(t, rel.Properties, metamodel.OrderPropertyIn, "edge from %s", from)
	}
}

func TestRelationOrder_IncomingSection(t *testing.T) {
	follow := []ViewTraverse{{From: "entry", FollowIncoming: "has-step", CollectAs: "recipes"}}
	table := ViewSection{Heading: "Recipes", Source: "recipes", Display: "table",
		Columns: []ListColumn{{Property: "title"}}}
	sorted := table
	sorted.Sort = []dataentryconfig.SortSpec{{Property: "title", Direction: "desc"}}
	ordered := &v1.RelationOrder{
		Relation: "has-step", Anchor: "STP-001", AnchorType: "step", Movable: true, Direction: "incoming",
	}
	relationOrder := []string{"REC-Z", "REC-X", "REC-M", "REC-Y"}
	tests := []struct {
		name      string
		mode      metamodel.OrderableMode
		sec       ViewSection
		wantIDs   []string
		wantOrder *v1.RelationOrder
	}{
		{"table in relation order", metamodel.OrderableIncoming, table, relationOrder, ordered},
		{"an author's sort wins", metamodel.OrderableIncoming, sorted,
			[]string{"REC-Z", "REC-Y", "REC-X", "REC-M"}, nil},
		{"orderable on the other side only", metamodel.OrderableOutgoing, table, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newOrderableRelationsTestApp(t, tt.mode)
			seedIncomingFixture(t, app)
			view := ViewConfig{Entry: ViewEntry{Type: "step"}, Traverse: follow, Sections: []ViewSection{tt.sec}}
			result, err := app.views.executeView(context.Background(), view, "STP-001", defaultViewWorld())
			require.NoError(t, err)
			out := app.views.buildSections(context.Background(), view.Sections, result)
			require.Len(t, out, 1)
			var ids []string
			for _, row := range out[0].Rows {
				ids = append(ids, row.EntityID)
			}
			if tt.wantIDs != nil {
				require.Equal(t, tt.wantIDs, ids)
			} else {
				require.ElementsMatch(t, relationOrder, ids)
			}
			require.Equal(t, tt.wantOrder, out[0].RelationOrder)
		})
	}
}

// A faced source links to the anchor once per face. The order names each
// visible row by the face its place comes from, and a move on that address
// leaves the source's hidden face alone.
func TestRelationOrder_IncomingFacedSources(t *testing.T) {
	app, d := publishedEditor(t)
	ctx := context.Background()
	meta := app.State().Meta
	rd := meta.Relations["cites"]
	rd.Orderable = metamodel.OrderableIncoming
	meta.Relations["cites"] = rd
	for _, face := range []entity.Face{"draft", "published"} {
		require.NoError(t, app.store.CreateEntity(ctx, &entity.Entity{
			ID: "POL-2", Type: "policy", Face: face, Properties: map[string]any{"title": "P2"},
		}))
	}
	edge := func(from string, face entity.Face) entity.RelationKey {
		return entity.RelationKey{From: from, FromFace: face, Type: "cites", To: "FEAT-1"}
	}
	for k, v := range map[entity.RelationKey]float64{
		edge("POL-2", "published"): 1, edge("POL-1", "draft"): 1.5, edge("POL-1", "published"): 2,
	} {
		_, err := app.store.CreateRelation(ctx, k,
			&store.RelationData{Properties: map[string]any{metamodel.OrderPropertyIn: v}})
		require.NoError(t, err)
	}

	actx := asAlice(t, d, httptest.NewRequest(http.MethodGet, "/", http.NoBody)).Context()
	anchor, err := app.store.GetEntity(ctx, entity.Ref{ID: "FEAT-1"})
	require.NoError(t, err)
	var edges []*entity.Relation
	for rel, err := range app.store.ListRelations(ctx, store.RelationQuery{To: "FEAT-1", Type: "cites"}) {
		require.NoError(t, err)
		edges = append(edges, rel)
	}
	edges, err = app.visibleReader.readableRelations(actx, edges)
	require.NoError(t, err)
	ordering, err := newRelationOrdering(actx, app.write.affordances, meta, anchor, "cites", true, edges)
	require.NoError(t, err)
	require.Equal(t, &v1.RelationOrder{
		Relation: "cites", Anchor: "FEAT-1", AnchorType: "feature", Movable: true, Direction: "incoming",
		Addresses: map[string]string{"POL-1": "POL-1@published", "POL-2": "POL-2@published"},
	}, ordering.wire())

	rec := relationAs(t, app, d, http.MethodPatch, "feature", "features", "FEAT-1", "cites",
		"POL-1@published", `{"direction":"incoming","position":{"before":"POL-2@published"}}`)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	value := func(k entity.RelationKey) any {
		r, err := app.store.GetRelation(ctx, k)
		require.NoError(t, err)
		return r.Properties[metamodel.OrderPropertyIn]
	}
	require.InDelta(t, 0.0, value(edge("POL-1", "published")), 0)
	require.InDelta(t, 1.5, value(edge("POL-1", "draft")), 0, "a hidden face's edge must not be written")
}
