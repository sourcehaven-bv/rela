package dataentry

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// traversalLeakApp builds the reachability-leak fixture (BUG-9Z20WH):
//
//	TKT-ENTRY --links--> SEC-HIDDEN --links--> TKT-VISIBLE
//
// A single self-following relation `links` connects all three nodes so a
// recursive traverse walks the whole chain. SEC-HIDDEN is a distinct type
// (`secret`) that the test principal cannot read; TKT-ENTRY and TKT-VISIBLE are
// tickets it can read. TKT-VISIBLE is reachable ONLY through the hidden
// intermediary, so a source-gated traversal must never surface it.
func traversalLeakApp() *App {
	meta := traversalLeakMeta()
	return newAppFromParts(&Config{App: AppConfig{Name: "Test"}}, meta, traversalLeakFixture(meta))
}

func traversalLeakMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:    "Ticket",
				IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "string"},
				},
				PropertyOrder: []string{"title", "status"},
			},
			"secret": {
				Label:    "Secret",
				IDPrefix: "SEC-",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "string"},
				},
				PropertyOrder: []string{"title", "status"},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			// Uniform relation across both types so a single recursive follow
			// walks TKT-ENTRY -> SEC-HIDDEN -> TKT-VISIBLE.
			"links": {
				Label: "links",
				From:  []string{"ticket", "secret"},
				To:    []string{"ticket", "secret"},
			},
		},
	}
}

func traversalLeakFixture(meta *metamodel.Metamodel) *fixture {
	g := newFixture()
	g.AddNode(testutil.EntityFor(meta, "ticket").ID("TKT-ENTRY").With("title", "entry").Build())
	g.AddNode(testutil.EntityFor(meta, "secret").ID("SEC-HIDDEN").
		With("title", "hidden intermediary").With("status", "classified").Build())
	g.AddNode(testutil.EntityFor(meta, "ticket").ID("TKT-VISIBLE").
		With("title", "reachable only via the hidden node").Build())

	g.AddEdge(testutil.NewRelation("TKT-ENTRY", "links", "SEC-HIDDEN").Build())
	g.AddEdge(testutil.NewRelation("SEC-HIDDEN", "links", "TKT-VISIBLE").Build())
	return g
}

// ticketReaderPolicy grants read on `ticket` only — `secret` is unreadable, so
// SEC-HIDDEN is row-gated for alice.
func ticketReaderPolicy() *acl.Policy {
	return &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}
}

// recursiveLinksView follows `links` recursively from the entry.
func recursiveLinksView() ViewConfig {
	return ViewConfig{
		Entry: ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{
			{From: "entry", Follow: "links", CollectAs: "chain", Recursive: true},
		},
	}
}

// TestACLViewTraversal_HiddenIntermediaryBlocksReachability is the canary for
// BUG-9Z20WH: on Entry -> H(hidden) -> V, a source-gated recursive traversal
// must stop at H, so V (reachable only through H) never surfaces. Before the
// fix, the traversal loaded every collected id off the raw store, admitted the
// hidden H into the working set, and the next pass then walked H's edges and
// returned V — leaking the existence of H and its edge to V.
//
// Note the out-gate (viewReader.Filter in executeViewRef) does NOT close this:
// it drops H from the returned collections, but V is readable in its own right,
// so V is emitted. Only source-gating in loadViewEntities stops H from acting
// as a stepping-stone.
func TestACLViewTraversal_HiddenIntermediaryBlocksReachability(t *testing.T) {
	app := traversalLeakApp()
	d := mustNewACL(t, ticketReaderPolicy(), app.store)
	app.acl = d

	ctx := gateCtxFor(aliceCtx(), t, d)
	result, err := app.views.executeView(ctx, recursiveLinksView(), "TKT-ENTRY", defaultViewWorld())
	if err != nil {
		t.Fatalf("executeView: %v", err)
	}

	for _, e := range result.Collections["chain"] {
		switch e.ID {
		case "SEC-HIDDEN":
			t.Errorf("LEAK: hidden intermediary SEC-HIDDEN surfaced in traversal")
		case "TKT-VISIBLE":
			t.Errorf("LEAK: TKT-VISIBLE reachable only via hidden SEC-HIDDEN surfaced: %v",
				collectIDs(result.Collections["chain"]))
		}
	}
}

// TestACLViewTraversal_NopACLReturnsFullChain is the byte-identical regression
// guard: a full-read principal (NopACL, the default) must see the entire chain
// unchanged. The gate permits every read, so the fix is a no-op for full-read
// callers — this fails if source-gating ever over-filters.
func TestACLViewTraversal_NopACLReturnsFullChain(t *testing.T) {
	app := traversalLeakApp() // app.acl defaults to NopACL (no policy set)

	result, err := app.views.executeView(context.Background(), recursiveLinksView(), "TKT-ENTRY", defaultViewWorld())
	if err != nil {
		t.Fatalf("executeView: %v", err)
	}

	got := map[string]bool{}
	for _, e := range result.Collections["chain"] {
		got[e.ID] = true
	}
	for _, want := range []string{"SEC-HIDDEN", "TKT-VISIBLE"} {
		if !got[want] {
			t.Errorf("NopACL traversal dropped %s; got %v", want,
				collectIDs(result.Collections["chain"]))
		}
	}
}

// TestACLViewTraversal_WhereCannotProbeHiddenProperty pins the secondary leak:
// applyViewTraverse runs `where:` over traversed entities. If a hidden entity
// reaches the filter, a predicate over its property becomes a one-bit oracle on
// that hidden value. Source-gating keeps hidden entities out of the working set
// entirely, so the where: never sees SEC-HIDDEN regardless of the predicate.
func TestACLViewTraversal_WhereCannotProbeHiddenProperty(t *testing.T) {
	app := traversalLeakApp()
	d := mustNewACL(t, ticketReaderPolicy(), app.store)
	app.acl = d

	view := ViewConfig{
		Entry: ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{
			// A predicate matching SEC-HIDDEN's classified status. If the hidden
			// entity reached the filter, matching/not-matching would disclose
			// the value; gated out, it can never appear.
			{From: "entry", Follow: "links", CollectAs: "probe",
				Recursive: true, Where: "status = classified"},
		},
	}

	ctx := gateCtxFor(aliceCtx(), t, d)
	result, err := app.views.executeView(ctx, view, "TKT-ENTRY", defaultViewWorld())
	if err != nil {
		t.Fatalf("executeView: %v", err)
	}
	for _, e := range result.Collections["probe"] {
		if e.ID == "SEC-HIDDEN" {
			t.Errorf("LEAK: where: predicate surfaced hidden SEC-HIDDEN via its property value")
		}
	}
}

// TestACLViewTraversal_EntryGateBlocksHiddenEntry pins executeViewRef's defensive
// entry gate: a principal who cannot read the entry gets a not-found error, not
// a partially-built view. (All production callers gate the entry first; this
// covers the synthetic-ViewConfig path in sections.go and any future caller.)
func TestACLViewTraversal_EntryGateBlocksHiddenEntry(t *testing.T) {
	app := traversalLeakApp()
	// viewer reads `secret` but NOT `ticket`, so the ticket entry is hidden.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"secret"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	ctx := gateCtxFor(aliceCtx(), t, d)
	// executeView entry type must match; use a secret-typed view whose entry is
	// the ticket to isolate the gate. Simpler: a ticket-entry view over a hidden
	// ticket entry.
	view := ViewConfig{Entry: ViewEntry{Type: "ticket"}}
	if _, err := app.views.executeView(ctx, view, "TKT-ENTRY", defaultViewWorld()); err == nil {
		t.Errorf("executeView returned a view for an entry the principal cannot read")
	}
}

// TestACLViewTraversal_ViewsAPIDoesNotLeakReachableDescendant drives the leak
// through the real _views wire handler (handleV1Views), not executeView
// directly, proving the API consumer inherits the fix. The registered view
// renders the recursive `chain` collection into a section, so a leaked
// TKT-VISIBLE would appear as a section entity in the JSON response.
func TestACLViewTraversal_ViewsAPIDoesNotLeakReachableDescendant(t *testing.T) {
	meta := traversalLeakMeta()
	cfg := &Config{
		App: AppConfig{Name: "Test"},
		Views: map[string]ViewConfig{
			"chain": {
				Entry:    ViewEntry{Type: "ticket"},
				Traverse: []ViewTraverse{{From: "entry", Follow: "links", CollectAs: "chain", Recursive: true}},
				Sections: []ViewSection{{Heading: "Chain", Source: "chain", Display: "list"}},
			},
		},
	}
	app := newAppFromParts(cfg, meta, traversalLeakFixture(meta))

	d := mustNewACL(t, ticketReaderPolicy(), app.store)
	app.acl = d

	rec := viewsAs(aliceCtx(), t, app, d, "ticket", "TKT-ENTRY")
	if rec.Code != http.StatusOK {
		t.Fatalf("_views: got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if body := rec.Body.String(); leaksVisibleDescendant(body) {
		t.Errorf("LEAK: _views response exposed descendant reachable only via hidden node: %s", body)
	}
}

// TestACLViewTraversal_SidePanelDoesNotLeakReachableDescendant drives the leak
// through the entity-detail side panel (handleV1SidePanel -> executeSidePanel ->
// executeView via a synthetic ViewConfig), the second affected consumer. A
// leaked TKT-VISIBLE would appear as a section entity in the panel.
func TestACLViewTraversal_SidePanelDoesNotLeakReachableDescendant(t *testing.T) {
	meta := traversalLeakMeta()
	cfg := &Config{App: AppConfig{Name: "Test"}, Forms: map[string]Form{}}
	app := newAppFromParts(cfg, meta, traversalLeakFixture(meta))
	app.Cfg().Forms[sidePanelFormID] = Form{
		EntityType: "ticket",
		SidePanel: &SidePanelConfig{
			Traverse: []ViewTraverse{{From: "entry", Follow: "links", CollectAs: "chain", Recursive: true}},
			Sections: []ViewSection{{Heading: "Chain", Source: "chain", Display: "list"}},
		},
	}

	d := mustNewACL(t, ticketReaderPolicy(), app.store)
	app.acl = d

	rec := sidePanelAs(aliceCtx(), t, app, d, "TKT-ENTRY")
	if rec.Code != http.StatusOK {
		t.Fatalf("_sidepanel: got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if body := rec.Body.String(); leaksVisibleDescendant(body) {
		t.Errorf("LEAK: _sidepanel exposed descendant reachable only via hidden node: %s", body)
	}
}

// leaksVisibleDescendant reports whether a wire response exposed TKT-VISIBLE —
// the descendant reachable only through the hidden intermediary SEC-HIDDEN —
// either by id or by its display title.
func leaksVisibleDescendant(body string) bool {
	return strings.Contains(body, "TKT-VISIBLE") ||
		strings.Contains(body, "reachable only via the hidden node")
}

// facedTraversalMeta is the faced-type fixture for the world/face half of the
// source gate: `policy` declares faces, `note` does not.
func facedTraversalMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m, err := metamodel.Parse([]byte(`
entities:
  policy:
    label: Policy
    id_prefix: POL
    faces:
      draft: {}
      published: { label: Published }
    properties:
      title: { type: string }
  note:
    label: Note
    id_prefix: NOTE
    properties:
      title: { type: string }
relations:
  links:
    from: [policy, note]
    to: [policy, note]
`))
	if err != nil {
		t.Fatalf("parse faced meta: %v", err)
	}
	return m
}

// TestACLViewTraversal_SourceGateMatchesLoaderResolution pins that the frontier
// source gate resolves rows the SAME way loadViewEntities does.
//
// This is not tidiness. A type declaring `faces:` stores NO row at the zero
// coordinate (BUG-HC6I2T), so a default-world header scan sees nothing for a
// faced entity. A gate built on such a scan would drop every faced entity from
// the frontier even under permit-all — a silent functional regression wearing a
// denial's clothes, and a gate deciding about a different graph than the one
// being walked. Asserting the two agree is what keeps them from drifting.
func TestACLViewTraversal_SourceGateMatchesLoaderResolution(t *testing.T) {
	meta := facedTraversalMeta(t)
	app := newAppFromParts(&Config{App: AppConfig{Name: "Test"}}, meta, newFixture())
	seedEntity(app, &entity.Entity{ID: "NOTE-1", Type: "note",
		Properties: map[string]any{"title": "note"}})
	seedEntity(app, &entity.Entity{ID: "POL-PUB", Type: "policy", Face: "draft",
		Properties: map[string]any{"title": "draft"}})
	seedEntity(app, &entity.Entity{ID: "POL-PUB", Type: "policy", Face: "published",
		Properties: map[string]any{"title": "published"}})
	seedEntity(app, &entity.Entity{ID: "POL-DRAFT", Type: "policy", Face: "draft",
		Properties: map[string]any{"title": "draft only"}})

	ids := []string{"NOTE-1", "POL-PUB", "POL-DRAFT"}
	for _, tc := range []struct {
		name  string
		world viewWorld
	}{
		{"default world", defaultViewWorld()},
		{"published world", publishedViewWorld("policy", "note")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			gated := app.views.readableViewIDs(ctx, ids, tc.world)
			loaded := collectIDs(app.views.loadViewEntities(ctx, ids, tc.world))
			if !slices.Equal(gated, loaded) {
				t.Errorf("source gate and loader disagree about which rows exist:\n"+
					"  gate   = %v\n  loader = %v\n"+
					"The gate must resolve the same rows the loader will, or it is "+
					"gating a different graph than the one being walked (BUG-9Z20WH).",
					gated, loaded)
			}
		})
	}
}

// TestACLViewTraversal_FrontierAppliesFaceGate pins the second half of the
// frontier verdict. PermitsReadMany is face-BLIND, so without faceReadable a
// principal granted only `policy@published` could walk THROUGH a draft-only
// entity to reach its descendants — the same reachability leak one coordinate
// down (TKT-O7R2A1).
//
// The world here SELECTS the draft face deliberately. Under the default world
// a faced entity has no row at all (BUG-HC6I2T), so the id is dropped for
// absence before the face gate is ever consulted — which would make this test
// pass with the face gate deleted, proving nothing. Resolving the draft row is
// what puts the face check on the critical path.
func TestACLViewTraversal_FrontierAppliesFaceGate(t *testing.T) {
	meta := facedTraversalMeta(t)
	app := newAppFromParts(&Config{App: AppConfig{Name: "Test"}}, meta, newFixture())
	seedEntity(app, &entity.Entity{ID: "POL-DRAFT", Type: "policy", Face: "draft",
		Properties: map[string]any{"title": "draft only"}})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"policy@published", "note"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	ctx := gateCtxFor(aliceCtx(), t, d)

	draftWorld := viewWorld{name: "draftworld", scope: store.NewWorldScope(
		map[string]store.TypeResolution{
			"policy": {Chain: []entity.Face{entity.Face("draft")}, Fallback: store.FallbackExclude},
		})}
	ids := []string{"POL-DRAFT"}

	// Preconditions. If either is lost the test no longer proves the face gate
	// does the work, so they are assertions rather than assumptions.
	rowVerdicts, err := readGateFromContext(ctx).PermitsReadMany(ctx, "policy", ids)
	if err != nil {
		t.Fatalf("row gate probe: %v", err)
	}
	if !rowVerdicts["POL-DRAFT"] {
		t.Fatal("precondition lost: the row gate no longer permits the draft-only " +
			"entity, so the face gate is not what would be doing the work")
	}
	var resolved bool
	for hdr, herr := range store.ListEntityHeaders(ctx, app.store,
		store.EntityQuery{IDs: ids, World: draftWorld.scope}) {
		if herr != nil {
			t.Fatalf("header scan: %v", herr)
		}
		if hdr.ID == "POL-DRAFT" {
			resolved = true
		}
	}
	if !resolved {
		t.Fatal("precondition lost: the draft row no longer resolves in this world, " +
			"so the id would be dropped for absence before the face gate is reached")
	}

	if got := app.views.readableViewIDs(ctx, ids, draftWorld); len(got) != 0 {
		t.Errorf("frontier expanded a face-denied node: %v; a principal granted only "+
			"policy@published must not traverse THROUGH a draft-only entity", got)
	}
	// The loader agrees, which is the invariant the gate must preserve.
	if got := collectIDs(app.views.loadViewEntities(ctx, ids, draftWorld)); len(got) != 0 {
		t.Errorf("loader admitted a face-denied node: %v", got)
	}
}
