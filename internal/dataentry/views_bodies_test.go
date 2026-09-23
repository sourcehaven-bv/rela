package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// bodiesApp: one entry ticket linked to two tickets that carry bodies.
func bodiesApp(t *testing.T) (*App, *storetest.Counting) {
	t.Helper()
	counting := storetest.NewCounting(memstore.New())
	ctx := context.Background()
	for _, id := range []string{"TKT-ENTRY", "TKT-A", "TKT-B"} {
		e := entity.New(id, "ticket")
		e.SetString("title", id)
		e.Content = "body of " + id
		if err := counting.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	for _, to := range []string{"TKT-A", "TKT-B"} {
		if _, err := counting.CreateRelation(ctx, "TKT-ENTRY", "links", to, nil); err != nil {
			t.Fatal(err)
		}
	}
	app := newAppFromParts(&Config{App: AppConfig{Name: "Test"}}, traversalLeakMeta(), newFixture(),
		appbuildtest.WithStore(counting))
	counting.Reset()
	return app, counting
}

func bodiesView(sections ...ViewSection) ViewConfig {
	return ViewConfig{
		Entry: ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{
			{From: "entry", Follow: "links", CollectAs: "rows"},
			{From: "entry", Follow: "links", CollectAs: "cards"},
		},
		Sections: sections,
	}
}

// A view that renders its collections as tables must not read a single body
// (TKT-U9DYW4): no full-entity list read, and no Content on the rows.
func TestViewBodies_TableOnlyReadsNoBodies(t *testing.T) {
	app, counting := bodiesApp(t)
	view := bodiesView(ViewSection{Source: "rows", Display: "table"})
	result, err := app.views.executeView(context.Background(), view, "TKT-ENTRY", defaultViewWorld())
	if err != nil {
		t.Fatal(err)
	}
	if n := counting.Calls()["ListEntities"]; n != 0 {
		t.Errorf("table-only view issued %d full-entity list reads (%s)", n, counting.String())
	}
	if len(result.Collections["rows"]) != 2 {
		t.Fatalf("rows = %d, want 2", len(result.Collections["rows"]))
	}
	for _, e := range result.Collections["rows"] {
		if e.Content != "" {
			t.Errorf("%s carries a body in a table collection", e.ID)
		}
	}
}

// The decision is per collection, not per entity or per rule: the same
// entities sit in a table collection AND a cards collection, and only the
// latter carries bodies — loaded in one batched read.
func TestViewBodies_PerCollection(t *testing.T) {
	app, counting := bodiesApp(t)
	view := bodiesView(
		ViewSection{Source: "rows", Display: "table"},
		ViewSection{Source: "cards", Display: "cards"},
	)
	result, err := app.views.executeView(context.Background(), view, "TKT-ENTRY", defaultViewWorld())
	if err != nil {
		t.Fatal(err)
	}
	if n := counting.Calls()["ListEntities"]; n != 1 {
		t.Errorf("bodies took %d list reads, want exactly 1 (%s)", n, counting.String())
	}
	for _, e := range result.Collections["cards"] {
		if e.Content != "body of "+e.ID {
			t.Errorf("cards %s: body %q", e.ID, e.Content)
		}
	}
	for _, e := range result.Collections["rows"] {
		if e.Content != "" {
			t.Errorf("rows %s carries a body; the cards collection must not write through a shared pointer", e.ID)
		}
	}
}

// An unknown display mode loads bodies: the content-free list is an allowlist.
func TestViewBodies_UnknownDisplayFailsTowardLoading(t *testing.T) {
	b := viewBodyCollections([]ViewSection{
		{Source: "a", Display: "table"}, {Source: "b", Display: "some-future-mode"},
		{Source: "c", Display: "content"}, {Source: "d", Display: "nested", Children: "e"},
	})
	for name, want := range map[string]bool{"a": false, "b": true, "c": true, "d": false, "e": false} {
		if b.wants(name) != want {
			t.Errorf("collection %q: wants=%v, want %v", name, b.wants(name), want)
		}
	}
}

// The command runner pipes collections to an operator script, so it gets
// whole entities whatever the view's sections render.
func TestViewBodies_CommandRunnerGetsWholeEntities(t *testing.T) {
	app, _ := bodiesApp(t)
	view := bodiesView(ViewSection{Source: "rows", Display: "table"})
	result, err := app.views.executeViewWhole(context.Background(), view, "TKT-ENTRY", defaultViewWorld())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"rows", "cards"} {
		for _, e := range result.Collections[name] {
			if e.Content != "body of "+e.ID {
				t.Errorf("%s/%s: body %q", name, e.ID, e.Content)
			}
		}
	}
}
