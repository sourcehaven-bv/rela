package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// testMeta returns a shared metamodel for convert tests with all needed entity types.
func testMeta() *metamodel.Metamodel {
	return testutil.NewMetamodel().
		DefineEntity("requirement").
		Label("Requirement").
		IDPrefix("REQ-").
		Prop("title", metamodel.PropertyTypeString, true).
		Prop("status", "status", false).
		End().
		DefineEntity("decision").
		Label("Decision").
		IDPrefix("DEC-").
		Prop("title", metamodel.PropertyTypeString, true).
		Prop("status", "status", false).
		Prop("priority", metamodel.PropertyTypeString, false).
		End().
		DefineEntity("solution").
		Label("Solution").
		IDPrefix("SOL-").
		Prop("title", metamodel.PropertyTypeString, true).
		End().
		DefineEntity("component").
		Label("Component").
		IDPrefix("CMP-").
		Prop("title", metamodel.PropertyTypeString, false).
		End().
		DefineEntity("test").
		Label("Test").
		Prop("title", metamodel.PropertyTypeString, false).
		End().
		WithRelation("addresses", "Addresses", []string{"solution"}, []string{"requirement"}).
		WithRelation("implements", "Implements", []string{"component"}, []string{"solution"}).
		WithRelation("motivates", "Motivates", []string{"requirement"}, []string{"decision"}).
		WithCustomType("status", []string{"draft", "proposed", "accepted", "rejected"}).
		Build()
}

// graphAdapter wraps *graph.Graph to implement relationQuerier for tests.
type graphAdapter struct {
	g *graph.Graph
}

func (a *graphAdapter) GetEntity(id string) (*model.Entity, bool) {
	return a.g.GetNode(id)
}

func (a *graphAdapter) OutgoingRelations(entityID string) []*model.Relation {
	return a.g.OutgoingEdges(entityID)
}

func (a *graphAdapter) IncomingRelations(entityID string) []*model.Relation {
	return a.g.IncomingEdges(entityID)
}

// makeToolRequest creates a CallToolRequest with the given arguments.
func makeToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

func TestConvertEntity_WithoutRelations(t *testing.T) {
	meta := testMeta()
	g := graph.New()
	e := testutil.EntityFor(meta, "requirement").ID("REQ-001").With("title", "Test requirement").WithContent("Some content").Build()
	g.AddNode(e)

	result, err := convertEntity(e, &graphAdapter{g}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed entityJSON
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if parsed.ID != e.ID {
		t.Errorf("expected ID %s, got %s", e.ID, parsed.ID)
	}
	if parsed.Type != "requirement" {
		t.Errorf("expected type requirement, got %s", parsed.Type)
	}
	if parsed.Content != "Some content" {
		t.Errorf("expected content 'Some content', got %s", parsed.Content)
	}
	if parsed.Relations != nil {
		t.Error("expected no relations when includeRelations=false")
	}
	if parsed.Properties["title"] != "Test requirement" {
		t.Errorf("expected title 'Test requirement', got %v", parsed.Properties["title"])
	}
}

func TestConvertEntity_WithRelations(t *testing.T) {
	meta := testMeta()
	g := graph.New()
	e1 := testutil.EntityFor(meta, "requirement").ID("REQ-001").Build()
	e2 := testutil.EntityFor(meta, "solution").ID("SOL-001").Build()
	g.AddNode(e1)
	g.AddNode(e2)

	g.AddEdge(testutil.NewRelation(e2.ID, "addresses", e1.ID).Build())

	result, err := convertEntity(e1, &graphAdapter{g}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed entityJSON
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if parsed.Relations == nil {
		t.Fatal("expected relations to be present")
	}
	if len(parsed.Relations.Incoming["addresses"]) != 1 {
		t.Errorf("expected 1 incoming 'addresses' relation, got %d",
			len(parsed.Relations.Incoming["addresses"]))
	}
	if parsed.Relations.Incoming["addresses"][0].ID != e2.ID {
		t.Errorf("expected incoming from %s, got %s",
			e2.ID, parsed.Relations.Incoming["addresses"][0].ID)
	}
}

func TestConvertEntity_NoRelationsPresent(t *testing.T) {
	meta := testMeta()
	g := graph.New()
	e := testutil.EntityFor(meta, "requirement").ID("REQ-001").Build()
	g.AddNode(e)

	result, err := convertEntity(e, &graphAdapter{g}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Even with includeRelations=true, if there are no relations, it should be nil
	var parsed entityJSON
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if parsed.Relations != nil {
		t.Error("expected nil relations when entity has no connections")
	}
}

func TestConvertEntitySummary(t *testing.T) {
	meta := testMeta()
	e := testutil.EntityFor(meta, "requirement").ID("REQ-001").With("title", "My Title").With("status", "accepted").Build()

	result := convertEntitySummary(e)

	if result["id"] != e.ID {
		t.Errorf("expected id %s, got %v", e.ID, result["id"])
	}
	if result["type"] != e.Type {
		t.Errorf("expected type %s, got %v", e.Type, result["type"])
	}
	if result["title"] != e.Properties["title"] {
		t.Errorf("expected title '%v', got %v", e.Properties["title"], result["title"])
	}
	if result["status"] != e.Properties["status"] {
		t.Errorf("expected status '%v', got %v", e.Properties["status"], result["status"])
	}
}

func TestConvertEntitySummary_NoTitleNoStatus(t *testing.T) {
	meta := testMeta()
	e := testutil.EntityFor(meta, "requirement").ID("REQ-002").Without("title").Without("status").Build()

	result := convertEntitySummary(e)

	if result["id"] != e.ID {
		t.Errorf("expected id %s, got %v", e.ID, result["id"])
	}
	if _, ok := result["title"]; ok {
		t.Error("expected no title key when title is empty")
	}
	if _, ok := result["status"]; ok {
		t.Error("expected no status key when status is empty")
	}
}

func TestConvertRelation(t *testing.T) {
	r := testutil.NewRelation("SOL-001", "addresses", "REQ-001").WithProperty("rationale", "because").WithContent("Relation content").Build()

	result, err := convertRelation(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed relationJSON
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if parsed.From != r.From {
		t.Errorf("expected from %s, got %s", r.From, parsed.From)
	}
	if parsed.Type != r.Type {
		t.Errorf("expected type %s, got %s", r.Type, parsed.Type)
	}
	if parsed.To != r.To {
		t.Errorf("expected to %s, got %s", r.To, parsed.To)
	}
	if parsed.Content != r.Content {
		t.Errorf("expected content '%s', got %s", r.Content, parsed.Content)
	}
	if parsed.Properties["rationale"] != r.Properties["rationale"] {
		t.Errorf("expected property rationale=%v, got %v", r.Properties["rationale"], parsed.Properties["rationale"])
	}
}

func TestConvertTraceResult(t *testing.T) {
	tr := &model.TraceResult{
		ID:    "REQ-001",
		Type:  "requirement",
		Title: "Root Req",
		Depth: 0,
		Children: []*model.TraceResult{
			{
				ID:       "SOL-001",
				Type:     "solution",
				Title:    "Child Sol",
				Depth:    1,
				Relation: "addresses",
				Incoming: true,
			},
		},
	}

	result, err := convertTraceResult(tr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed traceNodeJSON
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if parsed.ID != "REQ-001" {
		t.Errorf("expected ID REQ-001, got %s", parsed.ID)
	}
	if parsed.Depth != 0 {
		t.Errorf("expected depth 0, got %d", parsed.Depth)
	}
	if len(parsed.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(parsed.Children))
	}
	child := parsed.Children[0]
	if child.ID != "SOL-001" {
		t.Errorf("expected child ID SOL-001, got %s", child.ID)
	}
	if child.Relation != "addresses" {
		t.Errorf("expected child relation addresses, got %s", child.Relation)
	}
	if !child.Incoming {
		t.Error("expected child to be incoming")
	}
}

func TestConvertTraceResult_Nil(t *testing.T) {
	node := convertTraceNode(nil)
	if node != nil {
		t.Error("expected nil result for nil input")
	}
}

func TestConvertPathSteps(t *testing.T) {
	steps := []model.PathStep{
		{ID: "REQ-001", Type: "requirement", Title: "Start"},
		{ID: "SOL-001", Type: "solution", Title: "Middle", Relation: "addresses"},
		{ID: "CMP-001", Type: "component", Title: "End", Relation: "implements"},
	}

	result, err := convertPathSteps(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed []pathStepJSON
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(parsed) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(parsed))
	}
	if parsed[0].ID != "REQ-001" {
		t.Errorf("expected first step ID REQ-001, got %s", parsed[0].ID)
	}
	if parsed[1].Relation != "addresses" {
		t.Errorf("expected second step relation 'addresses', got %s", parsed[1].Relation)
	}
	if parsed[2].Title != "End" {
		t.Errorf("expected third step title 'End', got %s", parsed[2].Title)
	}
}

func TestConvertPathSteps_Empty(t *testing.T) {
	result, err := convertPathSteps([]model.PathStep{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "[]") {
		t.Errorf("expected empty array, got %s", result)
	}
}

func TestBuildRelations_NoEdges(t *testing.T) {
	meta := testMeta()
	g := graph.New()
	g.AddNode(testutil.EntityFor(meta, "requirement").ID("REQ-001").Build())

	rels := buildRelations("REQ-001", &graphAdapter{g})
	if rels != nil {
		t.Error("expected nil relations for entity with no edges")
	}
}

func TestBuildRelations_OutgoingOnly(t *testing.T) {
	meta := testMeta()
	g := graph.New()
	sol := testutil.EntityFor(meta, "solution").ID("SOL-001").With("title", "Solution").Build()
	req := testutil.EntityFor(meta, "requirement").ID("REQ-001").With("title", "Requirement").Build()
	g.AddNode(sol)
	g.AddNode(req)

	g.AddEdge(testutil.NewRelation(sol.ID, "addresses", req.ID).Build())

	rels := buildRelations(sol.ID, &graphAdapter{g})
	if rels == nil {
		t.Fatal("expected non-nil relations")
	}
	if len(rels.Outgoing["addresses"]) != 1 {
		t.Errorf("expected 1 outgoing addresses relation, got %d", len(rels.Outgoing["addresses"]))
	}
	if rels.Outgoing["addresses"][0].ID != req.ID {
		t.Errorf("expected target %s, got %s", req.ID, rels.Outgoing["addresses"][0].ID)
	}
	if rels.Outgoing["addresses"][0].Title != req.Properties["title"] {
		t.Errorf("expected title '%v', got %s", req.Properties["title"], rels.Outgoing["addresses"][0].Title)
	}
	if rels.Incoming != nil {
		t.Error("expected no incoming relations")
	}
}

func TestBuildRelations_IncomingOnly(t *testing.T) {
	meta := testMeta()
	g := graph.New()
	req := testutil.EntityFor(meta, "requirement").ID("REQ-001").With("title", "Requirement").Build()
	sol := testutil.EntityFor(meta, "solution").ID("SOL-001").With("title", "Solution").Build()
	g.AddNode(req)
	g.AddNode(sol)

	g.AddEdge(testutil.NewRelation(sol.ID, "addresses", req.ID).Build())

	rels := buildRelations(req.ID, &graphAdapter{g})
	if rels == nil {
		t.Fatal("expected non-nil relations")
	}
	if rels.Outgoing != nil {
		t.Error("expected no outgoing relations")
	}
	if len(rels.Incoming["addresses"]) != 1 {
		t.Errorf("expected 1 incoming addresses relation, got %d", len(rels.Incoming["addresses"]))
	}
	if rels.Incoming["addresses"][0].ID != sol.ID {
		t.Errorf("expected source %s, got %s", sol.ID, rels.Incoming["addresses"][0].ID)
	}
}

func TestBuildRelations_BothDirections(t *testing.T) {
	meta := testMeta()
	g := graph.New()
	g.AddNode(testutil.EntityFor(meta, "requirement").ID("REQ-001").With("title", "Req").Build())
	g.AddNode(testutil.EntityFor(meta, "solution").ID("SOL-001").With("title", "Sol").Build())
	g.AddNode(testutil.EntityFor(meta, "decision").ID("DEC-001").With("title", "Dec").Build())

	g.AddEdge(testutil.NewRelation("SOL-001", "addresses", "REQ-001").Build())
	g.AddEdge(testutil.NewRelation("REQ-001", "motivates", "DEC-001").Build())

	rels := buildRelations("REQ-001", &graphAdapter{g})
	if rels == nil {
		t.Fatal("expected non-nil relations")
	}
	if rels.Outgoing == nil {
		t.Fatal("expected outgoing relations")
	}
	if rels.Incoming == nil {
		t.Fatal("expected incoming relations")
	}
	if len(rels.Outgoing["motivates"]) != 1 {
		t.Errorf("expected 1 outgoing motivates, got %d", len(rels.Outgoing["motivates"]))
	}
	if len(rels.Incoming["addresses"]) != 1 {
		t.Errorf("expected 1 incoming addresses, got %d", len(rels.Incoming["addresses"]))
	}
}

func TestConvertEntitiesList(t *testing.T) {
	meta := testMeta()
	entities := []*model.Entity{
		testutil.EntityFor(meta, "requirement").ID("REQ-001").With("title", "First").With("status", "draft").Build(),
		testutil.EntityFor(meta, "requirement").ID("REQ-002").With("title", "Second").Build(),
	}

	result, err := convertEntitiesList(entities)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(parsed) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(parsed))
	}
	if parsed[0]["id"] != "REQ-001" {
		t.Errorf("expected first entity ID REQ-001, got %v", parsed[0]["id"])
	}
	if parsed[1]["id"] != "REQ-002" {
		t.Errorf("expected second entity ID REQ-002, got %v", parsed[1]["id"])
	}
}

func TestConvertEntitiesList_Empty(t *testing.T) {
	result, err := convertEntitiesList([]*model.Entity{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "[]") {
		t.Errorf("expected empty array, got %s", result)
	}
}

func TestConvertRelationsList(t *testing.T) {
	relations := []*model.Relation{
		testutil.NewRelation("SOL-001", "addresses", "REQ-001").WithProperty("weight", "high").Build(),
		testutil.NewRelation("CMP-001", "implements", "SOL-001").Build(),
	}

	result, err := convertRelationsList(relations)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed []relationJSON
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(parsed) != 2 {
		t.Fatalf("expected 2 relations, got %d", len(parsed))
	}
	if parsed[0].From != "SOL-001" {
		t.Errorf("expected first relation from SOL-001, got %s", parsed[0].From)
	}
	if parsed[0].Type != "addresses" {
		t.Errorf("expected first relation type addresses, got %s", parsed[0].Type)
	}
	if parsed[1].To != "SOL-001" {
		t.Errorf("expected second relation to SOL-001, got %s", parsed[1].To)
	}
}

func TestConvertRelationsList_Empty(t *testing.T) {
	result, err := convertRelationsList([]*model.Relation{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "[]") {
		t.Errorf("expected empty array, got %s", result)
	}
}

func TestSortEntitiesByID(t *testing.T) {
	meta := testMeta()
	entities := []*model.Entity{
		testutil.EntityFor(meta, "requirement").ID("REQ-003").Build(),
		testutil.EntityFor(meta, "requirement").ID("REQ-001").Build(),
		testutil.EntityFor(meta, "requirement").ID("REQ-002").Build(),
	}

	sortEntitiesByID(entities)

	if entities[0].ID != "REQ-001" {
		t.Errorf("expected first REQ-001, got %s", entities[0].ID)
	}
	if entities[1].ID != "REQ-002" {
		t.Errorf("expected second REQ-002, got %s", entities[1].ID)
	}
	if entities[2].ID != "REQ-003" {
		t.Errorf("expected third REQ-003, got %s", entities[2].ID)
	}
}

func TestSortRelations(t *testing.T) {
	relations := []*model.Relation{
		testutil.NewRelation("SOL-001", "implements", "REQ-001").Build(),
		testutil.NewRelation("SOL-001", "addresses", "REQ-001").Build(),
		testutil.NewRelation("CMP-001", "implements", "SOL-001").Build(),
	}

	sortRelations(relations)

	// CMP-001 < SOL-001 by From
	if relations[0].From != "CMP-001" {
		t.Errorf("expected first from CMP-001, got %s", relations[0].From)
	}
	// SOL-001/addresses < SOL-001/implements by Type
	if relations[1].Type != "addresses" {
		t.Errorf("expected second type addresses, got %s", relations[1].Type)
	}
	if relations[2].Type != "implements" {
		t.Errorf("expected third type implements, got %s", relations[2].Type)
	}
}

func TestSortRelations_ByTo(t *testing.T) {
	relations := []*model.Relation{
		testutil.NewRelation("SOL-001", "addresses", "REQ-002").Build(),
		testutil.NewRelation("SOL-001", "addresses", "REQ-001").Build(),
	}

	sortRelations(relations)

	if relations[0].To != "REQ-001" {
		t.Errorf("expected first to REQ-001, got %s", relations[0].To)
	}
	if relations[1].To != "REQ-002" {
		t.Errorf("expected second to REQ-002, got %s", relations[1].To)
	}
}

func TestMarshalJSON(t *testing.T) {
	data := map[string]string{"key": "value"}
	result, err := marshalJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, `"key": "value"`) {
		t.Errorf("expected JSON with key/value, got %s", result)
	}
}

func TestMarshalJSON_Indented(t *testing.T) {
	data := map[string]interface{}{
		"a": "b",
	}
	result, err := marshalJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should be indented with 2 spaces
	if !strings.Contains(result, "  ") {
		t.Errorf("expected indented JSON, got %s", result)
	}
}

func TestCountEdgesByType(t *testing.T) {
	edges := []*model.Relation{
		testutil.NewRelation("A", "addresses", "B").Build(),
		testutil.NewRelation("A", "implements", "C").Build(),
		testutil.NewRelation("A", "addresses", "D").Build(),
	}

	count := countEdgesByType(edges, "addresses")
	if count != 2 {
		t.Errorf("expected 2 addresses edges, got %d", count)
	}

	count = countEdgesByType(edges, "implements")
	if count != 1 {
		t.Errorf("expected 1 implements edge, got %d", count)
	}

	count = countEdgesByType(edges, "nonexistent")
	if count != 0 {
		t.Errorf("expected 0 nonexistent edges, got %d", count)
	}
}

func TestCountEdgesByType_Empty(t *testing.T) {
	count := countEdgesByType(nil, "addresses")
	if count != 0 {
		t.Errorf("expected 0 for nil edges, got %d", count)
	}
}

func TestConvertRelation_NoProperties(t *testing.T) {
	r := testutil.NewRelation("SOL-001", "addresses", "REQ-001").Build()

	result, err := convertRelation(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed relationJSON
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if parsed.From != r.From {
		t.Errorf("expected from %s, got %s", r.From, parsed.From)
	}
	if parsed.Content != "" {
		t.Errorf("expected empty content, got %s", parsed.Content)
	}
}

func TestConvertTraceResult_DeepNesting(t *testing.T) {
	tr := &model.TraceResult{
		ID:    "REQ-001",
		Type:  "requirement",
		Title: "Root",
		Depth: 0,
		Children: []*model.TraceResult{
			{
				ID:       "SOL-001",
				Type:     "solution",
				Title:    "Level 1",
				Depth:    1,
				Relation: "addresses",
				Children: []*model.TraceResult{
					{
						ID:       "CMP-001",
						Type:     "component",
						Title:    "Level 2",
						Depth:    2,
						Relation: "implements",
					},
				},
			},
		},
	}

	result, err := convertTraceResult(tr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed traceNodeJSON
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(parsed.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(parsed.Children))
	}
	if len(parsed.Children[0].Children) != 1 {
		t.Fatalf("expected 1 grandchild, got %d", len(parsed.Children[0].Children))
	}
	grandchild := parsed.Children[0].Children[0]
	if grandchild.ID != "CMP-001" {
		t.Errorf("expected grandchild ID CMP-001, got %s", grandchild.ID)
	}
	if grandchild.Depth != 2 {
		t.Errorf("expected grandchild depth 2, got %d", grandchild.Depth)
	}
}

func TestConvertEntity_WithProperties(t *testing.T) {
	meta := testMeta()
	g := graph.New()
	e := testutil.EntityFor(meta, "decision").ID("DEC-001").With("title", "Use Go").With("status", "accepted").With("priority", "high").Build()
	g.AddNode(e)

	result, err := convertEntity(e, &graphAdapter{g}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, `"DEC-001"`) {
		t.Error("expected JSON to contain entity ID")
	}
	if !strings.Contains(result, `"decision"`) {
		t.Error("expected JSON to contain entity type")
	}
	if !strings.Contains(result, `"Use Go"`) {
		t.Error("expected JSON to contain title")
	}
}

func TestSortEntitiesByID_AlreadySorted(t *testing.T) {
	meta := testMeta()
	entities := []*model.Entity{
		testutil.EntityFor(meta, "test").ID("A-001").Build(),
		testutil.EntityFor(meta, "test").ID("B-001").Build(),
		testutil.EntityFor(meta, "test").ID("C-001").Build(),
	}

	sortEntitiesByID(entities)

	if entities[0].ID != "A-001" {
		t.Errorf("expected first A-001, got %s", entities[0].ID)
	}
	if entities[2].ID != "C-001" {
		t.Errorf("expected last C-001, got %s", entities[2].ID)
	}
}

func TestSortEntitiesByID_Empty(_ *testing.T) {
	var entities []*model.Entity
	// Should not panic
	sortEntitiesByID(entities)
}

func TestSortRelations_Empty(_ *testing.T) {
	var relations []*model.Relation
	// Should not panic
	sortRelations(relations)
}

func TestExtractProperties_MapArgument(t *testing.T) {
	s := &Server{}
	req := makeToolRequest(map[string]interface{}{
		"properties": map[string]interface{}{
			"title":  "Test",
			"status": "draft",
		},
	})

	props := s.extractProperties(req)
	if props == nil {
		t.Fatal("expected non-nil properties")
	}
	if props["title"] != "Test" {
		t.Errorf("expected title 'Test', got %v", props["title"])
	}
	if props["status"] != "draft" {
		t.Errorf("expected status 'draft', got %v", props["status"])
	}
}

func TestExtractProperties_JSONString(t *testing.T) {
	s := &Server{}
	req := makeToolRequest(map[string]interface{}{
		"properties": `{"title":"From JSON","priority":"high"}`,
	})

	props := s.extractProperties(req)
	if props == nil {
		t.Fatal("expected non-nil properties from JSON string")
	}
	if props["title"] != "From JSON" {
		t.Errorf("expected title 'From JSON', got %v", props["title"])
	}
}

func TestExtractProperties_NoProperties(t *testing.T) {
	s := &Server{}
	req := makeToolRequest(map[string]interface{}{
		"id": "REQ-001",
	})

	props := s.extractProperties(req)
	if props != nil {
		t.Error("expected nil properties when key is missing")
	}
}

func TestExtractProperties_InvalidJSON(t *testing.T) {
	s := &Server{}
	req := makeToolRequest(map[string]interface{}{
		"properties": "not valid json",
	})

	props := s.extractProperties(req)
	if props != nil {
		t.Error("expected nil properties for invalid JSON string")
	}
}

func TestExtractProperties_UnsupportedType(t *testing.T) {
	s := &Server{}
	req := makeToolRequest(map[string]interface{}{
		"properties": 42,
	})

	props := s.extractProperties(req)
	if props != nil {
		t.Error("expected nil properties for unsupported type")
	}
}
