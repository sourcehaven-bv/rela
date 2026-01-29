package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// makeToolRequest creates a CallToolRequest with the given arguments.
func makeToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

func TestConvertEntity_WithoutRelations(t *testing.T) {
	g := graph.New()
	e := model.NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "Test requirement"
	e.Properties["status"] = "draft"
	e.Content = "Some content"
	g.AddNode(e)

	result, err := convertEntity(e, g, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed entityJSON
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if parsed.ID != "REQ-001" {
		t.Errorf("expected ID REQ-001, got %s", parsed.ID)
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
	g := graph.New()
	e1 := model.NewEntity("REQ-001", "requirement")
	e1.Properties["title"] = "Requirement 1"
	e2 := model.NewEntity("SOL-001", "solution")
	e2.Properties["title"] = "Solution 1"
	g.AddNode(e1)
	g.AddNode(e2)

	rel := model.NewRelation("SOL-001", "addresses", "REQ-001")
	g.AddEdge(rel)

	result, err := convertEntity(e1, g, true)
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
	if parsed.Relations.Incoming["addresses"][0].ID != "SOL-001" {
		t.Errorf("expected incoming from SOL-001, got %s",
			parsed.Relations.Incoming["addresses"][0].ID)
	}
}

func TestConvertEntity_NoRelationsPresent(t *testing.T) {
	g := graph.New()
	e := model.NewEntity("REQ-001", "requirement")
	g.AddNode(e)

	result, err := convertEntity(e, g, true)
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
	e := model.NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "My Title"
	e.Properties["status"] = "accepted"

	result := convertEntitySummary(e)

	if result["id"] != "REQ-001" {
		t.Errorf("expected id REQ-001, got %v", result["id"])
	}
	if result["type"] != "requirement" {
		t.Errorf("expected type requirement, got %v", result["type"])
	}
	if result["title"] != "My Title" {
		t.Errorf("expected title 'My Title', got %v", result["title"])
	}
	if result["status"] != "accepted" {
		t.Errorf("expected status 'accepted', got %v", result["status"])
	}
}

func TestConvertEntitySummary_NoTitleNoStatus(t *testing.T) {
	e := model.NewEntity("REQ-002", "requirement")

	result := convertEntitySummary(e)

	if result["id"] != "REQ-002" {
		t.Errorf("expected id REQ-002, got %v", result["id"])
	}
	if _, ok := result["title"]; ok {
		t.Error("expected no title key when title is empty")
	}
	if _, ok := result["status"]; ok {
		t.Error("expected no status key when status is empty")
	}
}

func TestConvertRelation(t *testing.T) {
	r := model.NewRelation("SOL-001", "addresses", "REQ-001")
	r.Properties = map[string]interface{}{"rationale": "because"}
	r.Content = "Relation content"

	result, err := convertRelation(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed relationJSON
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if parsed.From != "SOL-001" {
		t.Errorf("expected from SOL-001, got %s", parsed.From)
	}
	if parsed.Type != "addresses" {
		t.Errorf("expected type addresses, got %s", parsed.Type)
	}
	if parsed.To != "REQ-001" {
		t.Errorf("expected to REQ-001, got %s", parsed.To)
	}
	if parsed.Content != "Relation content" {
		t.Errorf("expected content 'Relation content', got %s", parsed.Content)
	}
	if parsed.Properties["rationale"] != "because" {
		t.Errorf("expected property rationale=because, got %v", parsed.Properties["rationale"])
	}
}

func TestConvertTraceResult(t *testing.T) {
	tr := &graph.TraceResult{
		ID:    "REQ-001",
		Type:  "requirement",
		Title: "Root Req",
		Depth: 0,
		Children: []*graph.TraceResult{
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
	steps := []graph.PathStep{
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
	result, err := convertPathSteps([]graph.PathStep{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "[]") {
		t.Errorf("expected empty array, got %s", result)
	}
}

func TestBuildRelations_NoEdges(t *testing.T) {
	g := graph.New()
	e := model.NewEntity("REQ-001", "requirement")
	g.AddNode(e)

	rels := buildRelations("REQ-001", g)
	if rels != nil {
		t.Error("expected nil relations for entity with no edges")
	}
}

func TestBuildRelations_OutgoingOnly(t *testing.T) {
	g := graph.New()
	e1 := model.NewEntity("SOL-001", "solution")
	e1.Properties["title"] = "Solution"
	e2 := model.NewEntity("REQ-001", "requirement")
	e2.Properties["title"] = "Requirement"
	g.AddNode(e1)
	g.AddNode(e2)

	rel := model.NewRelation("SOL-001", "addresses", "REQ-001")
	g.AddEdge(rel)

	rels := buildRelations("SOL-001", g)
	if rels == nil {
		t.Fatal("expected non-nil relations")
	}
	if len(rels.Outgoing["addresses"]) != 1 {
		t.Errorf("expected 1 outgoing addresses relation, got %d", len(rels.Outgoing["addresses"]))
	}
	if rels.Outgoing["addresses"][0].ID != "REQ-001" {
		t.Errorf("expected target REQ-001, got %s", rels.Outgoing["addresses"][0].ID)
	}
	if rels.Outgoing["addresses"][0].Title != "Requirement" {
		t.Errorf("expected title 'Requirement', got %s", rels.Outgoing["addresses"][0].Title)
	}
	if rels.Incoming != nil {
		t.Error("expected no incoming relations")
	}
}

func TestBuildRelations_IncomingOnly(t *testing.T) {
	g := graph.New()
	e1 := model.NewEntity("REQ-001", "requirement")
	e1.Properties["title"] = "Requirement"
	e2 := model.NewEntity("SOL-001", "solution")
	e2.Properties["title"] = "Solution"
	g.AddNode(e1)
	g.AddNode(e2)

	rel := model.NewRelation("SOL-001", "addresses", "REQ-001")
	g.AddEdge(rel)

	rels := buildRelations("REQ-001", g)
	if rels == nil {
		t.Fatal("expected non-nil relations")
	}
	if rels.Outgoing != nil {
		t.Error("expected no outgoing relations")
	}
	if len(rels.Incoming["addresses"]) != 1 {
		t.Errorf("expected 1 incoming addresses relation, got %d", len(rels.Incoming["addresses"]))
	}
	if rels.Incoming["addresses"][0].ID != "SOL-001" {
		t.Errorf("expected source SOL-001, got %s", rels.Incoming["addresses"][0].ID)
	}
}

func TestBuildRelations_BothDirections(t *testing.T) {
	g := graph.New()
	e1 := model.NewEntity("REQ-001", "requirement")
	e1.Properties["title"] = "Req"
	e2 := model.NewEntity("SOL-001", "solution")
	e2.Properties["title"] = "Sol"
	e3 := model.NewEntity("DEC-001", "decision")
	e3.Properties["title"] = "Dec"
	g.AddNode(e1)
	g.AddNode(e2)
	g.AddNode(e3)

	g.AddEdge(model.NewRelation("SOL-001", "addresses", "REQ-001"))
	g.AddEdge(model.NewRelation("REQ-001", "motivates", "DEC-001"))

	rels := buildRelations("REQ-001", g)
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
	entities := []*model.Entity{
		func() *model.Entity {
			e := model.NewEntity("REQ-001", "requirement")
			e.Properties["title"] = "First"
			e.Properties["status"] = "draft"
			return e
		}(),
		func() *model.Entity {
			e := model.NewEntity("REQ-002", "requirement")
			e.Properties["title"] = "Second"
			return e
		}(),
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
		func() *model.Relation {
			r := model.NewRelation("SOL-001", "addresses", "REQ-001")
			r.Properties = map[string]interface{}{"weight": "high"}
			return r
		}(),
		model.NewRelation("CMP-001", "implements", "SOL-001"),
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
	entities := []*model.Entity{
		model.NewEntity("REQ-003", "requirement"),
		model.NewEntity("REQ-001", "requirement"),
		model.NewEntity("REQ-002", "requirement"),
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
		model.NewRelation("SOL-001", "implements", "REQ-001"),
		model.NewRelation("SOL-001", "addresses", "REQ-001"),
		model.NewRelation("CMP-001", "implements", "SOL-001"),
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
		model.NewRelation("SOL-001", "addresses", "REQ-002"),
		model.NewRelation("SOL-001", "addresses", "REQ-001"),
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

func TestMatchesSearch(t *testing.T) {
	e := model.NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "Authentication Feature"
	e.Properties["status"] = "draft"
	e.Content = "Users should be able to log in"

	// Match by ID
	if !matchesSearch(e, "req-001") {
		t.Error("expected match by ID")
	}
	// Match by property
	if !matchesSearch(e, "authentication") {
		t.Error("expected match by property value")
	}
	// Match by content
	if !matchesSearch(e, "log in") {
		t.Error("expected match by content")
	}
	// No match
	if matchesSearch(e, "nonexistent") {
		t.Error("expected no match for nonexistent query")
	}
	// Non-string property should not match
	e.Properties["priority"] = 5
}

func TestCountEdgesByType(t *testing.T) {
	edges := []*model.Relation{
		model.NewRelation("A", "addresses", "B"),
		model.NewRelation("A", "implements", "C"),
		model.NewRelation("A", "addresses", "D"),
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
	r := model.NewRelation("SOL-001", "addresses", "REQ-001")

	result, err := convertRelation(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed relationJSON
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if parsed.From != "SOL-001" {
		t.Errorf("expected from SOL-001, got %s", parsed.From)
	}
	if parsed.Content != "" {
		t.Errorf("expected empty content, got %s", parsed.Content)
	}
}

func TestConvertTraceResult_DeepNesting(t *testing.T) {
	tr := &graph.TraceResult{
		ID:    "REQ-001",
		Type:  "requirement",
		Title: "Root",
		Depth: 0,
		Children: []*graph.TraceResult{
			{
				ID:       "SOL-001",
				Type:     "solution",
				Title:    "Level 1",
				Depth:    1,
				Relation: "addresses",
				Children: []*graph.TraceResult{
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

func TestMatchesSearch_ByIDCaseInsensitive(t *testing.T) {
	e := model.NewEntity("REQ-001", "requirement")

	// matchesSearch expects queryLower to already be lowercase
	if !matchesSearch(e, "req") {
		t.Error("expected case-insensitive ID match")
	}
	if !matchesSearch(e, "req-001") {
		t.Error("expected full ID match")
	}
}

func TestMatchesSearch_ByContent(t *testing.T) {
	e := model.NewEntity("REQ-001", "requirement")
	e.Content = "This is about Authentication"

	if !matchesSearch(e, "authentication") {
		t.Error("expected case-insensitive content match")
	}
}

func TestMatchesSearch_NoMatch(t *testing.T) {
	e := model.NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "Something"
	e.Content = "Other content"

	if matchesSearch(e, "zzznomatch") {
		t.Error("expected no match")
	}
}

func TestMatchesSearch_NonStringProperty(t *testing.T) {
	e := model.NewEntity("REQ-001", "requirement")
	e.Properties["count"] = 42

	// Non-string properties should not be searched
	if matchesSearch(e, "42") {
		t.Error("expected non-string property to not match")
	}
}

func TestConvertEntity_WithProperties(t *testing.T) {
	g := graph.New()
	e := model.NewEntity("DEC-001", "decision")
	e.Properties["title"] = "Use Go"
	e.Properties["status"] = "accepted"
	e.Properties["priority"] = "high"
	g.AddNode(e)

	result, err := convertEntity(e, g, false)
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
	entities := []*model.Entity{
		model.NewEntity("A-001", "test"),
		model.NewEntity("B-001", "test"),
		model.NewEntity("C-001", "test"),
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
