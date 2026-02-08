package dataentry

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// newGraphTestApp builds a full App with graph templates for graph handler tests.
func newGraphTestApp(t *testing.T) *App {
	t.Helper()
	meta := testMeta()
	cfg := testConfig()
	g := testGraph()

	// Add a relation edge for testing
	g.AddEdge(model.NewRelation("TKT-001", "depends_on", "TKT-002"))

	styleMap, styledTypes := buildStyleMap(cfg, meta)
	tmpl, err := template.New("").Funcs(templateFuncs(styleMap, styledTypes)).Parse(allTemplates())
	if err != nil {
		t.Fatalf("parsing templates: %v", err)
	}
	tmpl, err = tmpl.Parse(graphTemplates)
	if err != nil {
		t.Fatalf("parsing graph templates: %v", err)
	}

	return &App{
		Cfg:         cfg,
		meta:        meta,
		g:           g,
		tmpl:        tmpl,
		styleMap:    styleMap,
		styledTypes: styledTypes,
	}
}

func TestHandleGraph(t *testing.T) {
	t.Run("renders graph page", func(t *testing.T) {
		app := newGraphTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/graph", http.NoBody)
		w := httptest.NewRecorder()
		app.handleGraph(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "Graph Explorer") {
			t.Error("expected Graph Explorer heading in page")
		}
		if !strings.Contains(body, "cytoscape.min.js") {
			t.Error("expected cytoscape.min.js script tag")
		}
	})

	t.Run("includes app name in title", func(t *testing.T) {
		app := newGraphTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/graph", http.NoBody)
		w := httptest.NewRecorder()
		app.handleGraph(w, r)
		body := w.Body.String()
		if !strings.Contains(body, "Test App - Graph Explorer") {
			t.Error("expected app name in page title")
		}
	})
}

func TestHandleGraphData(t *testing.T) {
	t.Run("default mode returns content data", func(t *testing.T) {
		app := newGraphTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/api/graph-data", http.NoBody)
		w := httptest.NewRecorder()
		app.handleGraphData(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}

		var resp graphDataResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Should have 3 nodes (TKT-001, TKT-002, CMP-001)
		if len(resp.Nodes) != 3 {
			t.Errorf("expected 3 nodes, got %d", len(resp.Nodes))
		}
		// Should have 1 edge (TKT-001 depends_on TKT-002)
		if len(resp.Edges) != 1 {
			t.Errorf("expected 1 edge, got %d", len(resp.Edges))
		}
		// Should have 2 entity types (component, ticket — sorted)
		if len(resp.EntityTypes) != 2 {
			t.Errorf("expected 2 entity types, got %d", len(resp.EntityTypes))
		}
		// Should have 2 relation types
		if len(resp.RelationTypes) != 2 {
			t.Errorf("expected 2 relation types, got %d", len(resp.RelationTypes))
		}
	})

	t.Run("content mode explicit", func(t *testing.T) {
		app := newGraphTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/api/graph-data?mode=content", http.NoBody)
		w := httptest.NewRecorder()
		app.handleGraphData(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp graphDataResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Nodes) != 3 {
			t.Errorf("expected 3 nodes, got %d", len(resp.Nodes))
		}
	})

	t.Run("metamodel mode returns type graph", func(t *testing.T) {
		app := newGraphTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/api/graph-data?mode=metamodel", http.NoBody)
		w := httptest.NewRecorder()
		app.handleGraphData(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp graphDataResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Metamodel nodes = entity types (component, ticket)
		if len(resp.Nodes) != 2 {
			t.Errorf("expected 2 metamodel nodes, got %d", len(resp.Nodes))
		}
		// Metamodel edges = relation from/to combos:
		// depends_on: ticket -> ticket (1 edge)
		// belongs_to: ticket -> component (1 edge)
		if len(resp.Edges) != 2 {
			t.Errorf("expected 2 metamodel edges, got %d", len(resp.Edges))
		}
	})

	t.Run("unknown mode defaults to content", func(t *testing.T) {
		app := newGraphTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/api/graph-data?mode=bogus", http.NoBody)
		w := httptest.NewRecorder()
		app.handleGraphData(w, r)

		var resp graphDataResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		// Unknown mode falls back to content — should have 3 nodes
		if len(resp.Nodes) != 3 {
			t.Errorf("expected 3 nodes (content fallback), got %d", len(resp.Nodes))
		}
	})
}

func TestBuildContentGraphData(t *testing.T) {
	app := newGraphTestApp(t)
	resp := app.buildContentGraphData()

	t.Run("nodes have correct structure", func(t *testing.T) {
		nodeMap := make(map[string]graphNode)
		for _, n := range resp.Nodes {
			nodeMap[n.ID] = n
		}

		tkt1, ok := nodeMap["TKT-001"]
		if !ok {
			t.Fatal("expected TKT-001 in nodes")
		}
		if tkt1.Type != "ticket" {
			t.Errorf("expected type 'ticket', got %q", tkt1.Type)
		}
		if tkt1.Title != "First Ticket" {
			t.Errorf("expected title 'First Ticket', got %q", tkt1.Title)
		}
		if tkt1.Properties["status"] != "open" {
			t.Errorf("expected status 'open', got %q", tkt1.Properties["status"])
		}
	})

	t.Run("edges have correct structure", func(t *testing.T) {
		if len(resp.Edges) != 1 {
			t.Fatalf("expected 1 edge, got %d", len(resp.Edges))
		}
		e := resp.Edges[0]
		if e.Source != "TKT-001" {
			t.Errorf("expected source TKT-001, got %q", e.Source)
		}
		if e.Target != "TKT-002" {
			t.Errorf("expected target TKT-002, got %q", e.Target)
		}
		if e.Type != "depends_on" {
			t.Errorf("expected type depends_on, got %q", e.Type)
		}
	})

	t.Run("entity types are sorted with labels", func(t *testing.T) {
		if len(resp.EntityTypes) < 2 {
			t.Fatalf("expected at least 2 entity types, got %d", len(resp.EntityTypes))
		}
		// Sorted alphabetically: component, ticket
		if resp.EntityTypes[0].Type != "component" {
			t.Errorf("expected first entity type 'component', got %q", resp.EntityTypes[0].Type)
		}
		if resp.EntityTypes[0].Label != "Component" {
			t.Errorf("expected label 'Component', got %q", resp.EntityTypes[0].Label)
		}
		if resp.EntityTypes[1].Type != "ticket" {
			t.Errorf("expected second entity type 'ticket', got %q", resp.EntityTypes[1].Type)
		}
	})

	t.Run("entity type counts are correct", func(t *testing.T) {
		etMap := make(map[string]graphEntityType)
		for _, et := range resp.EntityTypes {
			etMap[et.Type] = et
		}
		if etMap["ticket"].Count != 2 {
			t.Errorf("expected ticket count 2, got %d", etMap["ticket"].Count)
		}
		if etMap["component"].Count != 1 {
			t.Errorf("expected component count 1, got %d", etMap["component"].Count)
		}
	})

	t.Run("relation type counts are correct", func(t *testing.T) {
		rtMap := make(map[string]graphRelationType)
		for _, rt := range resp.RelationTypes {
			rtMap[rt.Type] = rt
		}
		if rtMap["depends_on"].Count != 1 {
			t.Errorf("expected depends_on count 1, got %d", rtMap["depends_on"].Count)
		}
		if rtMap["depends_on"].Label != "depends on" {
			t.Errorf("expected label 'depends on', got %q", rtMap["depends_on"].Label)
		}
		if rtMap["belongs_to"].Count != 0 {
			t.Errorf("expected belongs_to count 0, got %d", rtMap["belongs_to"].Count)
		}
	})

	t.Run("entity type colors are assigned", func(t *testing.T) {
		for _, et := range resp.EntityTypes {
			if et.Color == "" {
				t.Errorf("expected color for entity type %q", et.Type)
			}
		}
	})
}

func TestBuildMetamodelGraphData(t *testing.T) {
	app := newGraphTestApp(t)
	resp := app.buildMetamodelGraphData()

	t.Run("nodes are entity types", func(t *testing.T) {
		if len(resp.Nodes) != 2 {
			t.Fatalf("expected 2 nodes, got %d", len(resp.Nodes))
		}
		nodeMap := make(map[string]graphNode)
		for _, n := range resp.Nodes {
			nodeMap[n.ID] = n
		}

		ticket, ok := nodeMap["ticket"]
		if !ok {
			t.Fatal("expected 'ticket' node")
		}
		if ticket.Title != "Ticket" {
			t.Errorf("expected title 'Ticket', got %q", ticket.Title)
		}
		// Properties should be type definitions, not instance values
		if ticket.Properties["title"] != "string" {
			t.Errorf("expected property 'title' type 'string', got %q", ticket.Properties["title"])
		}
	})

	t.Run("edges represent relation definitions", func(t *testing.T) {
		if len(resp.Edges) != 2 {
			t.Fatalf("expected 2 edges, got %d", len(resp.Edges))
		}
		edgeMap := make(map[string]graphEdge)
		for _, e := range resp.Edges {
			edgeMap[e.Source+"--"+e.Type+"--"+e.Target] = e
		}
		if _, ok := edgeMap["ticket--depends_on--ticket"]; !ok {
			t.Error("expected edge ticket--depends_on--ticket")
		}
		if _, ok := edgeMap["ticket--belongs_to--component"]; !ok {
			t.Error("expected edge ticket--belongs_to--component")
		}
	})

	t.Run("relation counts reflect edge combinations", func(t *testing.T) {
		rtMap := make(map[string]graphRelationType)
		for _, rt := range resp.RelationTypes {
			rtMap[rt.Type] = rt
		}
		// depends_on: ticket -> ticket = 1 combo
		if rtMap["depends_on"].Count != 1 {
			t.Errorf("expected depends_on count 1, got %d", rtMap["depends_on"].Count)
		}
		// belongs_to: ticket -> component = 1 combo
		if rtMap["belongs_to"].Count != 1 {
			t.Errorf("expected belongs_to count 1, got %d", rtMap["belongs_to"].Count)
		}
	})
}

func TestBuildMetaInfo(t *testing.T) {
	app := newGraphTestApp(t)
	metaData := app.buildMetaInfo([]string{"component", "ticket"}, []string{"belongs_to", "depends_on"})

	t.Run("meta entities have properties", func(t *testing.T) {
		if len(metaData.Entities) != 2 {
			t.Fatalf("expected 2 meta entities, got %d", len(metaData.Entities))
		}

		meMap := make(map[string]graphMetaEntity)
		for _, me := range metaData.Entities {
			meMap[me.Type] = me
		}

		ticket := meMap["ticket"]
		if ticket.Label != "Ticket" {
			t.Errorf("expected label 'Ticket', got %q", ticket.Label)
		}
		if len(ticket.Properties) != 3 {
			t.Errorf("expected 3 ticket properties, got %d", len(ticket.Properties))
		}

		// Properties should be sorted
		propNames := make([]string, 0, len(ticket.Properties))
		for _, p := range ticket.Properties {
			propNames = append(propNames, p.Name)
		}
		for i := 1; i < len(propNames); i++ {
			if propNames[i] < propNames[i-1] {
				t.Errorf("properties not sorted: %v", propNames)
				break
			}
		}
	})

	t.Run("meta relations have from/to", func(t *testing.T) {
		if len(metaData.Relations) != 2 {
			t.Fatalf("expected 2 meta relations, got %d", len(metaData.Relations))
		}

		mrMap := make(map[string]graphMetaRelation)
		for _, mr := range metaData.Relations {
			mrMap[mr.Type] = mr
		}

		dep := mrMap["depends_on"]
		if len(dep.From) != 1 || dep.From[0] != "ticket" {
			t.Errorf("expected depends_on from [ticket], got %v", dep.From)
		}
		if len(dep.To) != 1 || dep.To[0] != "ticket" {
			t.Errorf("expected depends_on to [ticket], got %v", dep.To)
		}
	})
}

func TestHandleIndexGraphRedirect(t *testing.T) {
	t.Run("graph as first nav item redirects", func(t *testing.T) {
		app := newGraphTestApp(t)
		app.Cfg.Navigation = []NavigationEntry{
			{Label: "Graph", Graph: true},
			{Label: "Tickets", List: "tickets"},
		}
		r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		w := httptest.NewRecorder()
		app.handleIndex(w, r)
		if w.Code != http.StatusFound {
			t.Errorf("expected 302 redirect, got %d", w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "/graph" {
			t.Errorf("expected redirect to /graph, got %q", loc)
		}
	})
}

func TestNavElementsGraphSkipsCount(t *testing.T) {
	app := newGraphTestApp(t)
	app.Cfg.Navigation = []NavigationEntry{
		{Label: "Graph", Graph: true},
		{Label: "Tickets", List: "tickets"},
	}
	elements := app.navElements("")
	if len(elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(elements))
	}
	if elements[0].Item == nil || !elements[0].Item.Graph {
		t.Error("expected first element to be a graph item")
	}
	if elements[0].Item.EntityType != "" {
		t.Errorf("expected empty entity type for graph, got %q", elements[0].Item.EntityType)
	}
	if elements[0].Item.Count != 0 {
		t.Errorf("expected count 0 for graph, got %d", elements[0].Item.Count)
	}
}

func TestBuildContentGraphDataEmptyGraph(t *testing.T) {
	meta := testMeta()
	cfg := testConfig()
	g := testGraph()

	// Remove all nodes
	for _, n := range g.AllNodes() {
		g.RemoveNode(n.ID)
	}

	styleMap, styledTypes := buildStyleMap(cfg, meta)
	app := &App{Cfg: cfg, meta: meta, g: g, styleMap: styleMap, styledTypes: styledTypes}
	resp := app.buildContentGraphData()

	if len(resp.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(resp.Nodes))
	}
	if len(resp.Edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(resp.Edges))
	}
	// Entity types still exist in metamodel
	if len(resp.EntityTypes) != 2 {
		t.Errorf("expected 2 entity types, got %d", len(resp.EntityTypes))
	}
	// All counts should be 0
	for _, et := range resp.EntityTypes {
		if et.Count != 0 {
			t.Errorf("expected count 0 for %s, got %d", et.Type, et.Count)
		}
	}
}

func TestGraphDataResponseJSON(t *testing.T) {
	app := newGraphTestApp(t)
	r := httptest.NewRequest(http.MethodGet, "/api/graph-data", http.NoBody)
	w := httptest.NewRecorder()
	app.handleGraphData(w, r)

	var raw map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	// Verify all expected top-level keys are present
	for _, key := range []string{"nodes", "edges", "entityTypes", "relationTypes", "meta"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing key %q in response", key)
		}
	}

	// Verify meta has expected sub-keys
	meta, ok := raw["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("expected meta to be an object")
	}
	for _, key := range []string{"entities", "relations"} {
		if _, ok := meta[key]; !ok {
			t.Errorf("missing key %q in meta", key)
		}
	}
}
