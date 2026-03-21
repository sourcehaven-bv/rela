package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// newHandlerTestApp builds an App for handler tests.
func newHandlerTestApp(t *testing.T) *App {
	t.Helper()
	meta := testMeta()
	cfg := testConfig()
	g := testGraph()

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

	return &App{
		Cfg:         cfg,
		meta:        meta,
		g:           g,
		styleMap:    styleMap,
		styledTypes: styledTypes,
		ws:          ws,
	}
}
