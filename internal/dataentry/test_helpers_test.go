package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/openapi"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// graphForTest returns the concrete *graph.Graph from the app for test setup.
func graphForTest(app *App) *graph.Graph {
	return app.Graph().(*graph.Graph)
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
func newAppFromParts(cfg *Config, meta *metamodel.Metamodel, g *graph.Graph) *App {
	app := &App{}
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
		Graph:        g,
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
	g.AddEdge(model.NewRelation("TKT-001", "depends_on", "TKT-002"))

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

	// Set up a repo for tests that need to read/write cache files
	fs := storage.NewMemFS()
	ctx := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	repo := repository.New(fs, ctx)

	ws := workspace.NewWithGraph(repo, meta, g)

	app := &App{ws: ws}
	app.state.Store(&AppState{
		Cfg:         cfg,
		Meta:        meta,
		Graph:       g,
		StyleMap:    styleMap,
		StyledTypes: styledTypes,
	})
	return app
}
