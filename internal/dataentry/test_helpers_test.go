package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/openapi"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// seedEntity writes an entity directly into the app's store.
func seedEntity(app *App, e *entity.Entity) {
	if err := app.store.CreateEntity(context.Background(), e); err != nil {
		panic(err)
	}
}

// fixture is a trivial collector of entities + relations used by test
// helpers to build an App. It replaces the historical *graph.Graph
// container and has no behavior of its own — it's just a slice pair
// that store seeders can iterate.
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
// the given store.
func seedFromFixture(st store.Store, f *fixture) {
	if st == nil || f == nil {
		return
	}
	ctx := context.Background()
	for _, e := range f.entities {
		if err := st.CreateEntity(ctx, e); err != nil {
			panic(err)
		}
	}
	for _, r := range f.relations {
		if _, err := st.CreateRelation(ctx, r.From, r.Type, r.To, nil); err != nil {
			panic(err)
		}
	}
}

// seedRelation is the relation counterpart to seedEntity.
func seedRelation(app *App, r *entity.Relation) {
	if _, err := app.store.CreateRelation(context.Background(), r.From, r.Type, r.To, nil); err != nil {
		panic(err)
	}
}

// entitiesByType returns the entities of a given type currently held
// by the app's store.
func entitiesByType(app *App, entityType string) []*entity.Entity {
	out := make([]*entity.Entity, 0)
	for e, err := range app.store.ListEntities(
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

// bindRepo rewires the given app to a workspace rooted at root, preserving
// the current app's entities and relations. Uses an OS-backed SafeFS so
// handlers that touch disk (e.g., cache writes) find a real filesystem.
func bindRepo(app *App, root string) {
	bindRepoWithFS(app,
		storage.NewSafeFS(storage.NewOsFS()),
		&project.Context{Root: root},
	)
}

// bindRepoWithFS rewires the given app to a workspace rooted at the given
// filesystem + paths, preserving fixtures. Use when the test needs to
// share a specific filesystem (e.g., an in-memory FS across multiple
// App instances).
func bindRepoWithFS(app *App, fs storage.FS, paths *project.Context) {
	newWs := workspace.NewForTest(app.Meta(), workspace.WithFS(fs, paths))
	reseedStore(newWs.Store(), app.store)
	rebindApp(app, fs, paths, newWs)
}

// rebindApp repoints the app's service fields at the given workspace.
// Used by bindRepoWithFS.
func rebindApp(app *App, fs storage.FS, paths *project.Context, ws *workspace.Workspace) {
	app.fs = fs
	app.paths = paths
	app.store = ws.Store()
	app.entityManager = ws.EntityManager()
	app.searcher = ws.Searcher()
	app.tracer = ws.Tracer()
	app.validator = ws.Validator()
	app.templater = ws.Templater()
	app.cfgLoader = ws.Config()
	app.kv = ws.State()
	app.startWatching = ws.StartWatching
	// Wire a minimal documentService for tests that hit the documents
	// handler. Script engine can be the real one (tests that use script:
	// configs will need to seed scripts on disk).
	if app.scriptEngine != nil {
		app.documents = newDocumentService(app.store, app.kv, "/", app.scriptEngine, app.luaWriteDeps)
	}
}

// reseedStore copies every entity and relation from src into dst.
func reseedStore(dst, src store.Store) {
	if src == nil || dst == nil {
		return
	}
	ctx := context.Background()
	for e, err := range src.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			continue
		}
		if err := dst.CreateEntity(ctx, e); err != nil {
			panic(err)
		}
	}
	for r, err := range src.ListRelations(ctx, store.RelationQuery{}) {
		if err != nil {
			continue
		}
		if _, err := dst.CreateRelation(ctx, r.From, r.Type, r.To, nil); err != nil {
			panic(err)
		}
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
	app := &App{scriptEngine: script.NewEngine()}
	if meta != nil {
		// Use an in-memory FS + project context so the workspace's
		// templater has paths it can dereference. Without this,
		// CreateRelation panics inside RelationTemplate when it tries
		// to compute a path against a nil *project.Context.
		fs := storage.NewMemFS()
		ctx := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
		_ = fs.MkdirAll(ctx.CacheDir, 0o755)
		ws := workspace.NewForTest(meta, workspace.WithFS(fs, ctx))
		rebindApp(app, fs, ctx, ws)
		seedFromFixture(app.store, f)
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

	ws := workspace.NewForTest(meta, workspace.WithFS(fs, ctx))
	seedFromFixture(ws.Store(), g)

	app := &App{}
	rebindApp(app, fs, ctx, ws)
	// Make sure kv hits the real filesystem through state.KV, matching production.
	kvRoot, err := storage.NewRootedFS(fs, ctx.CacheDir)
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	app.kv = state.NewFSKV(kvRoot)
	app.state.Store(&AppState{
		Cfg:         cfg,
		Meta:        meta,
		StyleMap:    styleMap,
		StyledTypes: styledTypes,
	})
	return app
}
