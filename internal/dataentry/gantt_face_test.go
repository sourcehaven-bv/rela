package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// A content-scoped hierarchy edge belongs to one face of its parent
// (BUG-BZQQDP). The gantt places a child, and folds its dates, only under the
// face that owns the edge. facedGanttApp seeds PRJ-A with a draft and a
// published face; the draft face has EPIC-D (a late window) and the published
// face has EPIC-P.
func facedGanttApp(t *testing.T) *App {
	t.Helper()
	meta, err := metamodel.Parse([]byte(`
entities:
  project:
    label: Project
    id_prefix: PRJ
    display_property: title
    faces:
      draft: {}
      published: {}
    properties:
      title: { type: string }
  epic:
    label: Epic
    id_prefix: EPIC
    display_property: title
    properties:
      title: { type: string }
      start: { type: date }
      end: { type: date }
relations:
  has-epic:
    from: [project]
    to: [epic]
    scope: content
`))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	g := dataentryconfig.Gantt{
		Title:     "Plan",
		Hierarchy: []string{"has-epic"},
		Sources: map[string]dataentryconfig.GanttSource{
			"project": {},
			"epic":    {Start: "start", End: "end"},
		},
	}
	cfg := &dataentryconfig.Config{
		App:        dataentryconfig.AppConfig{Name: "Gantt Face Test"},
		Forms:      make(map[string]dataentryconfig.Form),
		Lists:      make(map[string]dataentryconfig.List),
		Views:      make(map[string]dataentryconfig.ViewConfig),
		Kanbans:    make(map[string]dataentryconfig.Kanban),
		Gantts:     map[string]dataentryconfig.Gantt{"plan": g},
		Navigation: []dataentryconfig.NavigationEntry{},
	}
	dataentryconfig.NormalizeGantts(cfg)
	app := newAppFromParts(cfg, meta, newFixture())

	for _, face := range []entity.Face{"draft", "published"} {
		seedEntity(app, &entity.Entity{ID: "PRJ-A", Type: "project", Face: face,
			Properties: map[string]any{"title": "Root " + string(face)}})
	}
	seedEntity(app, &entity.Entity{ID: "EPIC-D", Type: "epic",
		Properties: map[string]any{"title": "Draft epic", "start": "2026-01-01", "end": "2026-12-31"}})
	seedEntity(app, &entity.Entity{ID: "EPIC-P", Type: "epic",
		Properties: map[string]any{"title": "Published epic", "start": "2026-03-01", "end": "2026-04-01"}})
	for _, e := range []struct {
		to   string
		tail entity.Face
	}{{"EPIC-D", "draft"}, {"EPIC-P", "published"}} {
		if _, err := app.store.CreateRelation(context.Background(), "PRJ-A", "has-epic", e.to,
			&store.RelationData{FromFace: e.tail}); err != nil {
			t.Fatalf("seed PRJ-A has-epic %s: %v", e.to, err)
		}
	}
	return app
}

// The handler cannot reach this case yet: a faced source type loads no rows
// until the gantt reads in a world (TKT-KQXVF7), so the check is pinned at the
// edge filter, which is where the fold gets its edges.
func TestGanttEdges_ContentEdgeOnlyWithItsFace(t *testing.T) {
	app := facedGanttApp(t)
	meta := app.schema.Current().Meta
	nodes := func() map[string]*ganttNode {
		return map[string]*ganttNode{
			"PRJ-A":  {id: "PRJ-A", entType: "project", face: "published"},
			"EPIC-D": {id: "EPIC-D", entType: "epic"},
			"EPIC-P": {id: "EPIC-P", entType: "epic"},
		}
	}
	ctx := context.Background()

	edges, external, gerr := app.gantt.ganttEdgesForType(ctx, meta, "has-epic", nodes(), "")
	if gerr != nil {
		t.Fatalf("full build: %+v", gerr)
	}
	if external || len(edges) != 1 || edges[0] != [2]string{"PRJ-A", "EPIC-P"} {
		t.Errorf("full build edges = %v (external %v), want only PRJ-A -> EPIC-P", edges, external)
	}

	// The subtree closure found EPIC-D over the draft edge; the drill must
	// decline to the full build rather than render EPIC-D beside PRJ-A.
	_, external, gerr = app.gantt.ganttEdgesForType(ctx, meta, "has-epic", nodes(), "PRJ-A")
	if gerr != nil {
		t.Fatalf("subtree: %+v", gerr)
	}
	if !external {
		t.Error("subtree drill with a disowned edge in its set must decline (external)")
	}
}

// A node keeps the face its row was loaded at, since edge ownership is decided
// by it, and a second face of the same id is refused rather than overwriting
// the first.
func TestGanttNodes_KeepTheirFace(t *testing.T) {
	app := facedGanttApp(t)
	s := app.schema.Current()
	g := s.Cfg.Gantts["plan"]
	row := func(face entity.Face) *entity.Entity {
		return &entity.Entity{ID: "PRJ-A", Type: "project", Face: face, Properties: map[string]any{"title": "A"}}
	}

	nodes := map[string]*ganttNode{}
	if gerr := app.gantt.addGanttNodes(s, g, "project", []*entity.Entity{row("published")}, nodes); gerr != nil {
		t.Fatalf("addGanttNodes: %+v", gerr)
	}
	if got := nodes["PRJ-A"].face; got != "published" {
		t.Fatalf("node face = %q, want published", got)
	}

	if gerr := app.gantt.addGanttNodes(s, g, "project", []*entity.Entity{row("draft")}, nodes); gerr == nil {
		t.Error("a second face of PRJ-A was accepted; want a refusal")
	}
	if got := nodes["PRJ-A"].face; got != "published" {
		t.Errorf("node face after refusal = %q, want published", got)
	}
}

// The drill reads its root by id with no face, which a faced source type does
// not have, so it declines to the full build (see ganttHasFacedSource).
func TestGanttSubtree_DeclinesForFacedSources(t *testing.T) {
	app := facedGanttApp(t)
	s := app.schema.Current()
	f, gerr := app.gantt.buildGanttSubtree(context.Background(), s, s.Cfg.Gantts["plan"], "EPIC-P")
	if f != nil || gerr != nil {
		t.Fatalf("buildGanttSubtree = %v, %+v; want a decline (nil, nil)", f, gerr)
	}
}
