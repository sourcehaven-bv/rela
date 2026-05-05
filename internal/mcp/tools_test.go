package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// makeTestServer creates a Server with a populated store for handler testing.
func makeTestServer(t *testing.T) *Server {
	t.Helper()

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:    "Requirement",
				IDPrefix: "REQ",
				Properties: map[string]metamodel.PropertyDef{
					"title":    {Type: "string", Required: true},
					"status":   {Type: "string"},
					"priority": {Type: "string"},
				},
			},
			"decision": {
				Label:    "Decision",
				IDPrefix: "DEC",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "string"},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"addresses": {
				Label: "addresses",
				From:  []string{"decision"},
				To:    []string{"requirement"},
			},
		},
	}

	st := memstore.New()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		testutil.EntityFor(meta, "requirement").ID("REQ-001").With("status", "accepted").Build(),
		testutil.EntityFor(meta, "requirement").ID("REQ-002").With("status", "draft").Build(),
		testutil.EntityFor(meta, "requirement").ID("REQ-003").With("status", "accepted").Build(),
		testutil.EntityFor(meta, "decision").ID("DEC-001").With("status", "accepted").Build(),
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed entity %s: %v", e.ID, err)
		}
	}
	if _, err := st.CreateRelation(ctx, "DEC-001", "addresses", "REQ-001", nil); err != nil {
		t.Fatalf("seed relation: %v", err)
	}

	ws := workspace.NewForTest(meta, workspace.WithTestStore(st))

	return &Server{
		ws:     ws,
		logger: slog.New(slog.DiscardHandler),
	}
}

func getResultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	return result.Content[0].(mcp.TextContent).Text
}

func isErrorResult(result *mcp.CallToolResult) bool {
	return result.IsError
}

// --- Entity handler tests ---

func TestHandleListEntities_All(t *testing.T) {
	s := makeTestServer(t)
	result, err := s.handleListEntities(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	var entities []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &entities); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(entities) != 4 {
		t.Errorf("expected 4 entities, got %d", len(entities))
	}
}

func TestHandleListEntities_ByType(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{"type": "requirement"})
	result, err := s.handleListEntities(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	var entities []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &entities); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(entities) != 3 {
		t.Errorf("expected 3 requirements, got %d", len(entities))
	}
}

func TestHandleListEntities_WithFilter(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{
		"type":  "requirement",
		"where": "status=accepted",
	})
	result, err := s.handleListEntities(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	var entities []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &entities); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(entities) != 2 {
		t.Errorf("expected 2 accepted requirements, got %d", len(entities))
	}
}

func TestHandleListEntities_WithPagination(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{
		"limit":  float64(2),
		"offset": float64(1),
	})
	result, err := s.handleListEntities(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	var entities []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &entities); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(entities) != 2 {
		t.Errorf("expected 2 entities with limit=2 offset=1, got %d", len(entities))
	}
}

func TestHandleShowEntity(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{"id": "REQ-001"})
	result, err := s.handleShowEntity(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "REQ-001") {
		t.Error("expected result to contain entity ID")
	}
	if !strings.Contains(text, "title") {
		t.Error("expected result to contain title property")
	}
}

func TestHandleShowEntity_NotFound(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{"id": "NONEXISTENT"})
	result, err := s.handleShowEntity(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("expected error result for nonexistent entity")
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "entity not found") {
		t.Errorf("expected 'entity not found' error, got %s", text)
	}
}

func TestHandleSearchEntities(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{"query": "accepted"})
	result, err := s.handleSearchEntities(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	var entities []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &entities); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	// Should match REQ-001, REQ-003, and DEC-001 (all have status=accepted)
	if len(entities) < 1 {
		t.Errorf("expected at least 1 match, got %d", len(entities))
	}
}

func TestHandleSearchEntities_ByType(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{
		"query": "accepted",
		"type":  "decision",
	})
	result, err := s.handleSearchEntities(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	var entities []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &entities); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	// Only DEC-001 is a decision with "accepted"
	if len(entities) != 1 {
		t.Errorf("expected 1 decision matching 'accepted', got %d", len(entities))
	}
}

func TestHandleUpdateEntity_NoUpdates(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{"id": "REQ-001"})
	result, err := s.handleUpdateEntity(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("expected error when no updates specified")
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "no updates specified") {
		t.Errorf("expected 'no updates specified' error, got %s", text)
	}
}

func TestHandleUpdateEntity_NotFound(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{
		"id":         "NONEXISTENT",
		"properties": map[string]interface{}{"title": "new"},
	})
	result, err := s.handleUpdateEntity(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("expected error for nonexistent entity")
	}
}

func TestHandleUpdateEntity_DeletesPropertyOnNil(t *testing.T) {
	s := makeTestServer(t)
	// REQ-001 starts with status=accepted; null should remove it.
	req := makeToolRequest(map[string]interface{}{
		"id":         "REQ-001",
		"properties": map[string]interface{}{"status": nil},
	})
	result, err := s.handleUpdateEntity(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(t, result))
	}
	updated, getErr := s.ws.Store().GetEntity(context.Background(), "REQ-001")
	if getErr != nil {
		t.Fatalf("get entity: %v", getErr)
	}
	if _, present := updated.Properties["status"]; present {
		t.Errorf("expected status to be removed, but it is still present: %v", updated.Properties["status"])
	}
}

func TestHandleUpdateEntity_DeleteOnlyCallSurvivesGuard(t *testing.T) {
	// AC 7: a delete-only call must NOT trigger the "no updates specified" guard.
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{
		"id":         "REQ-001",
		"properties": map[string]interface{}{"status": nil},
	})
	result, err := s.handleUpdateEntity(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isErrorResult(result) {
		text := getResultText(t, result)
		if strings.Contains(text, "no updates specified") {
			t.Fatalf("delete-only call wrongly hit the no-updates-specified guard: %s", text)
		}
		t.Fatalf("unexpected error: %s", text)
	}
}

func TestHandleUpdateEntity_DeleteAbsentPropertyIsNoOp(t *testing.T) {
	// `priority` is in the metamodel but not set on REQ-001; deleting it should be a no-op.
	s := makeTestServer(t)
	before, _ := s.ws.Store().GetEntity(context.Background(), "REQ-001")
	if _, present := before.Properties["priority"]; present {
		t.Fatalf("test setup: REQ-001 should not have a priority property")
	}

	req := makeToolRequest(map[string]interface{}{
		"id":         "REQ-001",
		"properties": map[string]interface{}{"priority": nil},
	})
	result, err := s.handleUpdateEntity(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success on no-op delete, got error: %s", getResultText(t, result))
	}
	after, _ := s.ws.Store().GetEntity(context.Background(), "REQ-001")
	if _, present := after.Properties["priority"]; present {
		t.Errorf("priority should remain absent")
	}
	if after.GetString("status") != before.GetString("status") {
		t.Errorf("status changed unexpectedly: was %q, now %q", before.GetString("status"), after.GetString("status"))
	}
}

func TestHandleUpdateEntity_DeleteRequiredPropertyRejected(t *testing.T) {
	// `title` is required; attempting to delete it must surface an actionable error
	// rather than silently producing a now-invalid entity.
	s := makeTestServer(t)
	before, _ := s.ws.Store().GetEntity(context.Background(), "REQ-001")
	beforeTitle := before.GetString("title")

	req := makeToolRequest(map[string]interface{}{
		"id":         "REQ-001",
		"properties": map[string]interface{}{"title": nil},
	})
	result, err := s.handleUpdateEntity(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isErrorResult(result) {
		t.Fatalf("expected error when deleting required property, got success")
	}
	if !strings.Contains(getResultText(t, result), "required") {
		t.Errorf("expected 'required' in error message, got %s", getResultText(t, result))
	}
	// Entity must be unchanged.
	after, _ := s.ws.Store().GetEntity(context.Background(), "REQ-001")
	if after.GetString("title") != beforeTitle {
		t.Errorf("entity must be unchanged after rejected delete: title was %q, now %q", beforeTitle, after.GetString("title"))
	}
}

func TestHandleUpdateEntity_JSONStringPropertiesNullDeletes(t *testing.T) {
	// End-to-end check that the JSON-string `properties` fallback also supports null-as-delete.
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{
		"id":         "REQ-001",
		"properties": `{"status": null}`,
	})
	result, err := s.handleUpdateEntity(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(t, result))
	}
	updated, _ := s.ws.Store().GetEntity(context.Background(), "REQ-001")
	if _, present := updated.Properties["status"]; present {
		t.Errorf("status should be removed when sent as JSON string with null")
	}
}

func TestHandleUpdateEntity_DeleteUnknownPropertyRejected(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{
		"id":         "REQ-001",
		"properties": map[string]interface{}{"unknown_prop": nil},
	})
	result, err := s.handleUpdateEntity(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("expected error for unknown property name")
	}
	if !strings.Contains(getResultText(t, result), "unknown properties") {
		t.Errorf("expected 'unknown properties' error, got %s", getResultText(t, result))
	}
}

func TestHandleUpdateEntity_MixedSetAndUnset(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{
		"id": "REQ-001",
		"properties": map[string]interface{}{
			"status": nil,           // delete
			"title":  "Renamed Req", // set
		},
	})
	result, err := s.handleUpdateEntity(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(t, result))
	}
	updated, _ := s.ws.Store().GetEntity(context.Background(), "REQ-001")
	if _, present := updated.Properties["status"]; present {
		t.Errorf("status should be removed")
	}
	if got := updated.GetString("title"); got != "Renamed Req" {
		t.Errorf("expected title 'Renamed Req', got %q", got)
	}
}

func TestHandleUpdateEntity_EmptyStringIsNoOp(t *testing.T) {
	// AC 8: empty string is silently filtered, so it must NOT delete an existing value
	// AND it must NOT itself satisfy the "no updates specified" guard alone.
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{
		"id":         "REQ-001",
		"properties": map[string]interface{}{"status": ""},
	})
	result, err := s.handleUpdateEntity(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty-string-only properties get filtered to an empty set, so the guard fires.
	if !isErrorResult(result) {
		t.Fatalf("expected 'no updates specified' since empty string is filtered, got success")
	}
	if !strings.Contains(getResultText(t, result), "no updates specified") {
		t.Errorf("expected 'no updates specified', got %s", getResultText(t, result))
	}
	// And the existing status is untouched.
	updated, _ := s.ws.Store().GetEntity(context.Background(), "REQ-001")
	if got := updated.GetString("status"); got != "accepted" {
		t.Errorf("status should remain 'accepted', got %q", got)
	}
}

func TestHandleUpdateEntity_SetAndOverwriteStillWorks(t *testing.T) {
	// AC 3: regression guard for the existing positive set/overwrite path.
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{
		"id":         "REQ-001",
		"properties": map[string]interface{}{"status": "rejected"},
	})
	result, err := s.handleUpdateEntity(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(t, result))
	}
	updated, _ := s.ws.Store().GetEntity(context.Background(), "REQ-001")
	if got := updated.GetString("status"); got != "rejected" {
		t.Errorf("expected status 'rejected', got %q", got)
	}
}

func TestUpdateEntityToolDescriptionMentionsNullDelete(t *testing.T) {
	const phrase = "set a property to null"
	tool := toolUpdateEntity()
	if !strings.Contains(strings.ToLower(tool.Description), phrase) {
		t.Errorf("tool description should mention %q, got: %q", phrase, tool.Description)
	}
	propsSchema, ok := tool.InputSchema.Properties["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("properties schema not found or wrong type: %#v", tool.InputSchema.Properties["properties"])
	}
	desc, _ := propsSchema["description"].(string)
	if !strings.Contains(strings.ToLower(desc), phrase) {
		t.Errorf("properties arg description should mention %q, got: %q", phrase, desc)
	}
}

func TestHandleDeleteEntity_NotFound(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{"id": "NONEXISTENT"})
	result, err := s.handleDeleteEntity(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("expected error for nonexistent entity")
	}
}

func TestHandleDeleteEntity_NoCascade(t *testing.T) {
	s := makeTestServer(t)
	// DEC-001 has a relation, so delete without cascade should fail
	req := makeToolRequest(map[string]interface{}{"id": "DEC-001", "cascade": false})
	result, err := s.handleDeleteEntity(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("expected error when deleting entity with relations and cascade=false")
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "relation(s)") {
		t.Errorf("expected relation count in error, got %s", text)
	}
}

// --- Relation handler tests ---

func TestHandleListRelations_All(t *testing.T) {
	s := makeTestServer(t)
	result, err := s.handleListRelations(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	var rels []relationJSON
	if err := json.Unmarshal([]byte(text), &rels); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(rels) != 1 {
		t.Errorf("expected 1 relation, got %d", len(rels))
	}
}

func TestHandleListRelations_ByType(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{"type": "addresses"})
	result, err := s.handleListRelations(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	var rels []relationJSON
	if err := json.Unmarshal([]byte(text), &rels); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(rels) != 1 {
		t.Errorf("expected 1 addresses relation, got %d", len(rels))
	}
}

func TestHandleListRelations_ByFrom(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{"from": "DEC-001"})
	result, err := s.handleListRelations(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	var rels []relationJSON
	if err := json.Unmarshal([]byte(text), &rels); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(rels) != 1 {
		t.Errorf("expected 1 relation from DEC-001, got %d", len(rels))
	}
}

func TestHandleListRelations_NoMatch(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{"type": "implements"})
	result, err := s.handleListRelations(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "[]") {
		t.Errorf("expected empty array, got %s", text)
	}
}

func TestHandleListRelations_Pagination(t *testing.T) {
	s := makeTestServer(t)
	// Add another relation for pagination testing
	if _, err := s.ws.Store().CreateRelation(context.Background(), "DEC-001", "addresses", "REQ-002", nil); err != nil {
		t.Fatalf("seed relation: %v", err)
	}

	req := makeToolRequest(map[string]interface{}{"limit": float64(1)})
	result, err := s.handleListRelations(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	var rels []relationJSON
	if err := json.Unmarshal([]byte(text), &rels); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(rels) != 1 {
		t.Errorf("expected 1 relation with limit=1, got %d", len(rels))
	}
}

func TestHandleCreateRelation_MissingFields(t *testing.T) {
	s := makeTestServer(t)
	// Missing "type".
	req := makeToolRequest(map[string]interface{}{
		"from": "DEC-001",
		"to":   "REQ-001",
	})
	result, err := s.handleCreateRelation(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("expected error for missing type")
	}

	// Missing "from".
	req = makeToolRequest(map[string]interface{}{
		"type": "addresses",
		"to":   "REQ-001",
	})
	result, err = s.handleCreateRelation(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("expected error for missing from")
	}

	// Missing "to".
	req = makeToolRequest(map[string]interface{}{
		"from": "DEC-001",
		"type": "addresses",
	})
	result, err = s.handleCreateRelation(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("expected error for missing to")
	}
}

func TestHandleCreateEntity_RejectsCustomIDForShortType(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{
		"type":       "requirement",
		"id":         "my-custom-id",
		"properties": map[string]interface{}{"title": "Nope"},
	})
	result, err := s.handleCreateEntity(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isErrorResult(result) {
		t.Fatal("expected error result for custom ID on short-ID type")
	}
	text := getResultText(t, result)
	// Pin on "custom ID" so the test fails if the message stops naming the
	// caller's input, rather than just mentioning "short" for unrelated reasons.
	for _, want := range []string{"requirement", "short", "my-custom-id", "custom ID"} {
		if !strings.Contains(text, want) {
			t.Errorf("error text %q missing %q", text, want)
		}
	}
}

func TestHandleDeleteRelation_MissingFields(t *testing.T) {
	s := makeTestServer(t)
	// Missing "type".
	req := makeToolRequest(map[string]interface{}{
		"from": "DEC-001",
		"to":   "REQ-001",
	})
	result, err := s.handleDeleteRelation(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("expected error for missing type")
	}
}

func TestHandleDeleteRelation_NotFound(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{
		"from": "REQ-001",
		"type": "nonexistent",
		"to":   "REQ-002",
	})
	result, err := s.handleDeleteRelation(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("expected error for nonexistent relation")
	}
}

// --- Trace handler tests ---

func TestHandleTraceFrom(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{"id": "REQ-001"})
	result, err := s.handleTraceFrom(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "REQ-001") {
		t.Error("expected trace result to contain root ID")
	}
}

func TestHandleTraceFrom_NotFound(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{"id": "NONEXISTENT"})
	result, err := s.handleTraceFrom(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("expected error for nonexistent entity")
	}
}

func TestHandleTraceTo(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{"id": "REQ-001"})
	result, err := s.handleTraceTo(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "REQ-001") {
		t.Error("expected trace result to contain root ID")
	}
}

func TestHandleFindPath(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{"from": "DEC-001", "to": "REQ-001"})
	result, err := s.handleFindPath(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "DEC-001") || !strings.Contains(text, "REQ-001") {
		t.Error("expected path to contain both entities")
	}
}

func TestHandleFindPath_NoPath(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{"from": "REQ-002", "to": "REQ-003"})
	result, err := s.handleFindPath(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "No path found") {
		t.Errorf("expected 'No path found' message, got %s", text)
	}
}

func TestHandleFindPath_NotFound(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{"from": "NONEXISTENT", "to": "REQ-001"})
	result, err := s.handleFindPath(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("expected error for nonexistent entity")
	}
}

// --- Analysis handler tests ---

func TestHandleAnalyzeOrphans(t *testing.T) {
	s := makeTestServer(t)
	result, err := s.handleAnalyzeOrphans(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	// REQ-002, REQ-003 are orphans (no relations)
	if !strings.Contains(text, "orphan") {
		t.Errorf("expected orphan entities, got %s", text)
	}
}

func TestHandleAnalyzeOrphans_ByType(t *testing.T) {
	s := makeTestServer(t)
	req := makeToolRequest(map[string]interface{}{"type": "decision"})
	result, err := s.handleAnalyzeOrphans(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	// DEC-001 has a relation, so no orphan decisions
	if !strings.Contains(text, "No orphan entities found") {
		t.Errorf("expected no orphan decisions, got %s", text)
	}
}

func TestHandleAnalyzeCardinality_NoViolations(t *testing.T) {
	s := makeTestServer(t)
	result, err := s.handleAnalyzeCardinality(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "All cardinality constraints satisfied") {
		t.Errorf("expected no violations, got %s", text)
	}
}

func TestHandleAnalyzeCardinality_WithViolation(t *testing.T) {
	s := makeTestServer(t)
	// Set a minimum cardinality that won't be met
	minVal := 5
	meta := s.ws.Meta()
	meta.Relations["addresses"] = metamodel.RelationDef{
		From:        []string{"decision"},
		To:          []string{"requirement"},
		MinOutgoing: &minVal,
	}
	result, err := s.handleAnalyzeCardinality(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "violation") {
		t.Errorf("expected violations, got %s", text)
	}
}

func TestHandleAnalyzeProperties(t *testing.T) {
	s := makeTestServer(t)
	result, err := s.handleAnalyzeProperties(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All entities should be valid
	text := getResultText(t, result)
	if isErrorResult(result) {
		t.Errorf("unexpected error result: %s", text)
	}
}

func TestHandleAnalyzeValidations_NoRules(t *testing.T) {
	s := makeTestServer(t)
	result, err := s.handleAnalyzeValidations(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "No custom validation rules") {
		t.Errorf("expected 'No custom validation rules' message, got %s", text)
	}
}

// --- Schema handler tests ---

func TestHandleGetMetamodel(t *testing.T) {
	s := makeTestServer(t)
	result, err := s.handleGetMetamodel(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if parsed["entities"] == nil {
		t.Error("expected entities in metamodel output")
	}
	if parsed["relations"] == nil {
		t.Error("expected relations in metamodel output")
	}
}

func TestHandleListEntityTypes(t *testing.T) {
	s := makeTestServer(t)
	result, err := s.handleListEntityTypes(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	var types []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &types); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(types) != 2 {
		t.Errorf("expected 2 entity types, got %d", len(types))
	}
}

func TestHandleListRelationTypes(t *testing.T) {
	s := makeTestServer(t)
	result, err := s.handleListRelationTypes(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)
	var types []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &types); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(types) != 1 {
		t.Errorf("expected 1 relation type, got %d", len(types))
	}
}

// --- Resource handler tests ---

func TestHandleReadEntity(t *testing.T) {
	s := makeTestServer(t)
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "rela://entity/requirement/REQ-001"
	contents, err := s.handleReadEntity(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}
	text := contents[0].(mcp.TextResourceContents).Text
	if !strings.Contains(text, "REQ-001") {
		t.Error("expected entity ID in response")
	}
}

func TestHandleReadEntity_TypeMismatch(t *testing.T) {
	s := makeTestServer(t)
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "rela://entity/decision/REQ-001"
	_, err := s.handleReadEntity(context.Background(), req)
	if err == nil {
		t.Error("expected error for type mismatch")
	}
	if !strings.Contains(err.Error(), "not decision") {
		t.Errorf("expected type mismatch error, got %v", err)
	}
}

func TestHandleReadEntity_NotFound(t *testing.T) {
	s := makeTestServer(t)
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "rela://entity/requirement/REQ-999"
	_, err := s.handleReadEntity(context.Background(), req)
	if err == nil {
		t.Error("expected error for nonexistent entity")
	}
}

func TestHandleReadEntity_InvalidURI(t *testing.T) {
	s := makeTestServer(t)
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "rela://entity/onlyone"
	_, err := s.handleReadEntity(context.Background(), req)
	if err == nil {
		t.Error("expected error for invalid URI")
	}
}

func TestHandleReadRelation(t *testing.T) {
	s := makeTestServer(t)
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "rela://relation/DEC-001/addresses/REQ-001"
	contents, err := s.handleReadRelation(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}
	text := contents[0].(mcp.TextResourceContents).Text
	if !strings.Contains(text, "DEC-001") {
		t.Error("expected relation from ID in response")
	}
}

func TestHandleReadRelation_NotFound(t *testing.T) {
	s := makeTestServer(t)
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "rela://relation/REQ-001/nonexistent/REQ-002"
	_, err := s.handleReadRelation(context.Background(), req)
	if err == nil {
		t.Error("expected error for nonexistent relation")
	}
}

// --- Helper function tests ---

func TestResolveType(t *testing.T) {
	s := makeTestServer(t)
	tests := []struct {
		input    string
		expected string
	}{
		{"requirement", "requirement"},
		{"requirements", "requirement"},
		{"decision", "decision"},
		{"decisions", "decision"},
		{"unknown", "unknown"}, // falls through
	}
	for _, tt := range tests {
		got := s.resolveType(tt.input)
		if got != tt.expected {
			t.Errorf("resolveType(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestResolveEntityType(t *testing.T) {
	s := makeTestServer(t)
	resolved, def, err := s.resolveEntityType("requirement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != "requirement" {
		t.Errorf("expected 'requirement', got %s", resolved)
	}
	if def == nil {
		t.Error("expected non-nil entity def")
	}
}

func TestResolveEntityType_Unknown(t *testing.T) {
	s := makeTestServer(t)
	_, _, err := s.resolveEntityType("nonexistent")
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestApplyPagination(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}

	// No pagination
	result := applyPagination(items, 0, 0)
	if len(result) != 5 {
		t.Errorf("expected 5 items, got %d", len(result))
	}

	// Limit only
	result = applyPagination(items, 0, 3)
	if len(result) != 3 {
		t.Errorf("expected 3 items with limit=3, got %d", len(result))
	}

	// Offset only
	result = applyPagination(items, 2, 0)
	if len(result) != 3 {
		t.Errorf("expected 3 items with offset=2, got %d", len(result))
	}

	// Both
	result = applyPagination(items, 1, 2)
	if len(result) != 2 {
		t.Errorf("expected 2 items with offset=1 limit=2, got %d", len(result))
	}

	// Offset beyond length
	result = applyPagination(items, 10, 0)
	if result != nil {
		t.Errorf("expected nil for offset beyond length, got %v", result)
	}
}
