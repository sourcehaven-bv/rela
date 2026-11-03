package dataentry

import (
	"context"
	"errors"
	"iter"
	"net/http"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// goldenExportApp builds a ticket list with relation columns in both
// directions on one relation type, seeds the graph the golden test pins, and
// returns a context for a principal who may read tickets and features but
// not secrets. wrap, when non-nil, decorates the memstore.
func goldenExportApp(t *testing.T, wrap func(store.Store) store.Store) (*App, context.Context) {
	t.Helper()
	str := map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Label: "Ticket", IDPrefix: "TKT-", Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string", Required: true}, "status": {Type: "string"},
			}, PropertyOrder: []string{"title", "status"}},
			"feature": {Label: "Feature", IDPrefix: "FEAT-", Properties: str, PropertyOrder: []string{"title"}},
			"secret":  {Label: "Secret", IDPrefix: "SEC-", Properties: str, PropertyOrder: []string{"title"}},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {Label: "implements", From: []string{"ticket"}, To: []string{"feature", "secret"}},
			"blocks":     {Label: "blocks", From: []string{"ticket"}, To: []string{"ticket"}},
		},
		Transforms: map[string]metamodel.TransformDef{
			"copy": {From: "markdown", Command: []string{"cp", "{in}", "{out}"}, Produces: "text/plain"},
		},
	}
	cfg := &dataentryconfig.Config{
		App:   dataentryconfig.AppConfig{Name: "Export Golden"},
		Forms: map[string]dataentryconfig.Form{},
		Lists: map[string]dataentryconfig.List{
			"tickets": {EntityType: "ticket", Columns: []dataentryconfig.ListColumn{
				{Property: "id", Label: "ID"},
				{Property: "title", Label: "Title"},
				{Property: "status", Label: "Status"},
				{Relation: "implements", Label: "Implements"},
				{Relation: "blocks", Label: "Blocks"},
				{Relation: "blocks", Direction: dataentryconfig.DirectionIncoming, Label: "Blocked by"},
			}},
		},
		Views:      map[string]dataentryconfig.ViewConfig{},
		Kanbans:    map[string]dataentryconfig.Kanban{},
		Navigation: []dataentryconfig.NavigationEntry{{List: "tickets"}},
	}
	var st store.Store = memstore.New()
	if wrap != nil {
		st = wrap(st)
	}
	app := newAppFromParts(cfg, meta, newFixture(), appbuildtest.WithStore(st))

	for _, e := range []*entity.Entity{
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "One", "status": "open"}},
		{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "Two", "status": "done"}},
		{ID: "TKT-3", Type: "ticket", Properties: map[string]any{"title": "Three"}},
		{ID: "TKT-4", Type: "ticket", Properties: map[string]any{"title": "Four", "status": "open"}},
		{ID: "FEAT-A", Type: "feature", Properties: map[string]any{"title": "Alpha"}},
		{ID: "FEAT-B", Type: "feature", Properties: map[string]any{"title": "Beta"}},
		{ID: "SEC-1", Type: "secret", Properties: map[string]any{"title": "Hidden"}},
	} {
		seedEntity(app, e)
	}
	for _, r := range []*entity.Relation{
		{From: "TKT-1", Type: "implements", To: "FEAT-B"},
		{From: "TKT-1", Type: "implements", To: "FEAT-A"},
		{From: "TKT-1", Type: "implements", To: "SEC-1"},
		{From: "TKT-2", Type: "implements", To: "FEAT-A"},
		{From: "TKT-3", Type: "implements", To: "SEC-1"},
		{From: "TKT-2", Type: "blocks", To: "TKT-1"},
		{From: "TKT-3", Type: "blocks", To: "TKT-1"},
		{From: "TKT-1", Type: "blocks", To: "TKT-4"},
		{From: "TKT-4", Type: "blocks", To: "TKT-4"},
	} {
		seedRelation(app, r)
	}

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket", "feature"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)
	return app, gateCtxFor(aliceCtx(), t, d)
}

// TestExport_List_RelationColumnsGolden pins the exact table the list export
// renders for relation columns. The fixture covers both directions on one
// relation type, an edge between two exported rows, a self-loop (TKT-4
// blocks itself, so it is in both its Blocks and its Blocked-by cell), a
// neighbor shared by several rows, several neighbors in one cell, a hidden
// neighbor next to visible ones, and a cell whose only neighbor is hidden
// (TKT-3's Implements).
//
// The expected table was produced by the per-row edge reads that TKT-VEIMO3
// replaced with one batched read. It runs on memstore only; the batched read
// keeps the order on the other backends because every backend sorts
// relations by key, whatever the filter.
func TestExport_List_RelationColumnsGolden(t *testing.T) {
	requireCp(t)
	app, ctx := goldenExportApp(t, nil)

	rec := exportListURL(ctx, app, "transform=copy&list=tickets&sort=id")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	const want = `| ID | Title | Status | Implements | Blocks | Blocked by |
| --- | --- | --- | --- | --- | --- |
| TKT-1 | One | open | Alpha, Beta | Four | Two, Three |
| TKT-2 | Two | done | Alpha | One |  |
| TKT-3 | Three |  |  | One |  |
| TKT-4 | Four | open |  | Four | One, Four |
`
	if got := rec.Body.String(); got != want {
		t.Errorf("export table changed.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// A faced row lists only the content-scoped edges its own face owns
// (BUG-ISJHML), now that the edges come from one batched read that returns
// every face's edges of the id.
func TestExport_List_FacedRowCarriesOnlyItsOwnEdges(t *testing.T) {
	app, _ := facedExportApp(t, nil)
	s := app.State()
	cfg := &Config{
		Lists: map[string]dataentryconfig.List{"policies": {EntityType: "policy", Columns: []dataentryconfig.ListColumn{
			{Property: "id", Label: "ID"}, {Face: true}, {Relation: "cites", Label: "Cites"},
		}}},
		Navigation: []dataentryconfig.NavigationEntry{{List: "policies"}},
	}
	cfg.App.DefaultWorld = "published"
	app.schema.Publish(&Schema{Cfg: cfg, Meta: s.Meta})
	seedExportGraph(t, app,
		[]*entity.Entity{
			{ID: "FEAT-DRAFT", Type: "feature", Properties: map[string]any{"title": "Draft cite"}},
			{ID: "FEAT-PUB", Type: "feature", Properties: map[string]any{"title": "Published cite"}},
		},
		[][3]string{{"POL-1", "cites", "FEAT-DRAFT"}, {"POL-1", "cites", "FEAT-PUB"}},
		[]entity.Face{"draft", "published"})

	rec := exportRouted(t, app, "policys?list=policies")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	const want = "| ID | Face | Cites |\n| --- | --- | --- |\n| POL-1 | Published | Published cite |\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("export table:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// pageEdgeFailingStore fails the batched edge read (a RelationQuery over
// EntityIDs) and passes every other relation read through, so the ACL's
// single-entity membership walk still works.
type pageEdgeFailingStore struct{ store.Store }

func (s pageEdgeFailingStore) ListRelations(
	ctx context.Context, q store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	if q.EntityIDs != nil {
		return func(yield func(*entity.Relation, error) bool) { yield(nil, errors.New("relation read failed")) }
	}
	return s.Store.ListRelations(ctx, q)
}

// A failed edge read fails the export instead of rendering a table that
// looks complete with empty relation cells.
func TestExport_List_EdgeReadFaultFailsTheExport(t *testing.T) {
	requireCp(t)
	app, ctx := goldenExportApp(t, func(s store.Store) store.Store { return pageEdgeFailingStore{s} })

	rec := exportListURL(ctx, app, "transform=copy&list=tickets")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body)
	}
	if body := rec.Body.String(); strings.Contains(body, "TKT-1") || strings.Contains(body, "relation read failed") {
		t.Errorf("failed export leaked rows or the store error: %s", body)
	}
}
