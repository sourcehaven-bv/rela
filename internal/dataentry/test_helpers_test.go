package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/openapi"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// seedEntity writes an entity directly into the workspace's in-memory
// store(s). Use this instead of graph mutations — the signature is
// stable across the ongoing graph retirement.
func seedEntity(app *App, e *entity.Entity) {
	app.ws.SeedEntityForTest(e)
}

// fixture is a trivial collector of entities + relations used by test
// helpers to build an App. It replaces the historical *graph.Graph
// container and has no behavior of its own — it's just a slice pair
// that workspace seeders can iterate.
type fixture struct {
	entities  []*entity.Entity
	relations []*entity.Relation
}

// newFixture constructs an empty fixture.
func newFixture() *fixture { return &fixture{} }

// AddNode appends an entity. Kept named AddNode for drop-in
// compatibility with tests migrating off *graph.Graph.
func (f *fixture) AddNode(e *entity.Entity) { f.entities = append(f.entities, e) }

// AddEdge appends a relation. Kept named AddEdge for drop-in compat.
func (f *fixture) AddEdge(r *entity.Relation) { f.relations = append(f.relations, r) }

// NodesByType returns all entities of the given type in fixture order.
func (f *fixture) NodesByType(entityType string) []*entity.Entity {
	var out []*entity.Entity
	for _, e := range f.entities {
		if e.Type == entityType {
			out = append(out, e)
		}
	}
	return out
}

// seedFromFixture ingests every entity and relation of a fixture into
// the workspace's store.
func seedFromFixture(ws *workspace.Workspace, f *fixture) {
	if ws == nil || f == nil {
		return
	}
	for _, e := range f.entities {
		ws.SeedEntityForTest(e)
	}
	for _, r := range f.relations {
		ws.SeedRelationForTest(r)
	}
}

// seedRelation is the relation counterpart to seedEntity.
func seedRelation(app *App, r *entity.Relation) {
	app.ws.SeedRelationForTest(r)
}

// entitiesByType returns the entities of a given type currently held
// by the workspace. Tests use this to collect fixture IDs without
// reaching into the graph.
func entitiesByType(app *App, entityType string) []*entity.Entity {
	out := make([]*entity.Entity, 0)
	for e, err := range app.ws.Store().ListEntities(
		context.Background(),
		store.EntityQuery{Type: entityType},
	) {
		if err != nil {
			continue
		}
		out = append(out, e)
	}
	return out
}

// bindRepo replaces app.ws with a workspace rooted at the given
// project path. The current workspace's entities and relations are
// re-seeded into the new one so tests get their fixture data back
// without reaching into the graph. Uses an OS-backed SafeFS so
// handlers that actually touch disk (e.g., cache writes) find a real
// filesystem.
func bindRepo(app *App, root string) {
	bindRepoWithFS(app,
		storage.NewSafeFS(storage.NewOsFS()),
		&project.Context{Root: root},
	)
}

// bindRepoWithFS replaces app.ws with a workspace rooted at the given
// filesystem + paths, preserving fixtures. Use when the test needs to
// share a specific filesystem (e.g., an in-memory FS across multiple
// App instances).
func bindRepoWithFS(app *App, fs storage.FS, paths *project.Context) {
	prior := app.ws
	newWs := workspace.NewBare(fs, paths, app.Meta())
	reseedWorkspace(newWs, prior)
	app.ws = newWs
}

// reseedWorkspace copies every entity and relation from src into dst's
// in-memory store(s). Used by bindRepo to preserve test fixtures
// across a workspace rebind.
func reseedWorkspace(dst, src *workspace.Workspace) {
	if src == nil {
		return
	}
	ctx := context.Background()
	st := src.Store()
	if st == nil {
		return
	}
	for e, err := range st.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			continue
		}
		dst.SeedEntityForTest(e)
	}
	for r, err := range st.ListRelations(ctx, store.RelationQuery{}) {
		if err != nil {
			continue
		}
		dst.SeedRelationForTest(r)
	}
}

// newAppFromParts builds an App with a populated AppState snapshot for
// tests that previously used the struct-literal pattern
// `&App{Cfg: cfg, meta: meta, g: g}`. The App.state pointer must be
// populated because handlers now read from it; a nil snapshot would
// nil-deref inside a.State().
//
// Populates ALL AppState fields with safe defaults (UserDefaults,
// Palette, UserPalette, OpenAPIGen) so handlers that touch the
// less-common fields don't nil-deref in tests that didn't ask for them.
func newAppFromParts(cfg *Config, meta *metamodel.Metamodel, f *fixture) *App {
	app := &App{}
	if meta != nil {
		app.ws = workspace.NewBare(nil, nil, meta)
		seedFromFixture(app.ws, f)
	}
	if cfg == nil {
		cfg = &Config{}
	}
	var styleMap map[string]map[string]string
	var styledTypes map[string]bool
	if meta != nil {
		styleMap, styledTypes = buildStyleMap(cfg, meta)
	}
	var openAPIGen *openapi.Generator
	if meta != nil {
		openAPIGen = openapi.New(meta, openapi.Config{Title: cfg.App.Name})
	}
	app.state.Store(&AppState{
		Cfg:          cfg,
		Meta:         meta,
		StyleMap:     styleMap,
		StyledTypes:  styledTypes,
		UserDefaults: &UserDefaults{},
		Palette:      ResolvePalette(cfg.Palette, nil),
		UserPalette:  &PaletteConfig{},
		OpenAPIGen:   openAPIGen,
	})
	return app
}

// newHandlerTestApp builds an App for handler tests.
func newHandlerTestApp(t *testing.T) *App {
	t.Helper()
	meta := testMeta()
	cfg := testConfig()
	g, _ := testGraph(meta)

	// Add a relation for testing edge display
	g.AddEdge(entity.NewRelation("TKT-001", "depends_on", "TKT-002"))

	// Add view config
	cfg.Views = map[string]ViewConfig{
		"ticket_detail": {
			Title: "Ticket Detail",
			Entry: ViewEntry{Type: "ticket"},
			Traverse: []ViewTraverse{
				{From: "entry", Follow: "belongs_to", CollectAs: "components"},
			},
			Sections: []ViewSection{
				{Heading: "Properties", Source: "entry", Display: "properties", Fields: []ViewSectionField{
					{Property: "title"}, {Property: "status"},
				}},
				{Heading: "Components", Source: "components", Display: "list"},
			},
		},
	}

	// Add dashboard config
	cfg.Dashboard = &DashboardConfig{
		Title: "Dashboard",
		Cards: []DashboardCard{
			{Title: "All Tickets", Query: "type:ticket", Display: "count"},
		},
	}

	styleMap, styledTypes := buildStyleMap(cfg, meta)

	// Set up a filesystem for tests that need to read/write cache files
	fs := storage.NewMemFS()
	ctx := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)

	ws := workspace.NewBare(fs, ctx, meta)
	seedFromFixture(ws, g)

	app := &App{ws: ws}
	app.state.Store(&AppState{
		Cfg:         cfg,
		Meta:        meta,
		StyleMap:    styleMap,
		StyledTypes: styledTypes,
	})
	return app
}
