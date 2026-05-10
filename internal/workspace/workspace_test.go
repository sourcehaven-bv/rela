package workspace

import (
	"context"
	"errors"
	"strings"
	"testing"

	entitypkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// testMetamodelYAML is the shared workspace test metamodel.
var testMetamodelYAML = testutil.WorkspaceMetamodelYAML()

// testContext creates a standard project.Context for testing.
func testContext() *project.Context {
	return &project.Context{
		Root:                 "/project",
		MetamodelPath:        "/project/metamodel.yaml",
		CacheDir:             "/project/.rela",
		EntitiesDir:          "/project/entities",
		RelationsDir:         "/project/relations",
		TemplatesDir:         "/project/templates",
		EntityTemplatesDir:   "/project/templates/entities",
		RelationTemplatesDir: "/project/templates/relations",
	}
}

// setupWorkspaceFS creates an in-memory filesystem with directories for the given metamodel.
func setupWorkspaceFS(ctx *project.Context, meta *metamodel.Metamodel, metamodelYAML string) storage.FS {
	fs := storage.NewMemFS()
	for _, entityDef := range meta.Entities {
		_ = fs.MkdirAll(ctx.EntitiesDir+"/"+entityDef.Plural, 0o755)
	}
	_ = fs.MkdirAll(ctx.RelationsDir, 0o755)
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	_ = fs.MkdirAll(ctx.EntityTemplatesDir, 0o755)
	_ = fs.MkdirAll(ctx.RelationTemplatesDir, 0o755)
	_ = fs.WriteFile(ctx.MetamodelPath, []byte(metamodelYAML), 0o644)
	return fs
}

// setupWorkspaceWithMetamodel creates a workspace with the given metamodel YAML.
// Uses a real script engine for Lua automation tests.
func setupWorkspaceWithMetamodel(t *testing.T, metamodelYAML string) *Workspace {
	t.Helper()
	meta, err := metamodel.Parse([]byte(metamodelYAML))
	if err != nil {
		t.Fatalf("failed to parse test metamodel: %v", err)
	}

	ctx := testContext()
	fs := setupWorkspaceFS(ctx, meta, metamodelYAML)
	return NewForTest(meta, WithFS(fs, ctx), WithScript(script.NewEngine()))
}

// setupTestWorkspace creates a workspace with the standard test metamodel.
func setupTestWorkspace(t *testing.T) *Workspace {
	t.Helper()
	meta, err := metamodel.Parse([]byte(testMetamodelYAML))
	if err != nil {
		t.Fatalf("failed to parse test metamodel: %v", err)
	}

	ctx := testContext()
	fs := setupWorkspaceFS(ctx, meta, testMetamodelYAML)
	return NewForTest(meta, WithFS(fs, ctx))
}

// mustCreate is a test helper that creates an entity, fatally failing on error.
// Returns the created entity so tests can reference its generated ID without
// hardcoding it.
func mustCreate(t *testing.T, ws *Workspace, entityType string, opts CreateOptions) *entitypkg.Entity {
	t.Helper()
	e, _, err := ws.createEntity(entityType, opts)
	if err != nil {
		t.Fatalf("mustCreate(%s): %v", entityType, err)
	}
	return e
}

// --- Constructor tests ---

func TestNew(t *testing.T) {
	meta, _ := metamodel.Parse([]byte(testMetamodelYAML))
	ctx := testContext()
	fs := setupWorkspaceFS(ctx, meta, testMetamodelYAML)

	ws, err := New(fs, ctx, NopScriptExecutor)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if ws.Store() == nil {
		t.Error("expected store to be initialized")
	}
	if ws.Meta() == nil {
		t.Error("expected meta to be initialized")
	}
	if ws.FS() == nil {
		t.Error("expected fs to be initialized")
	}
	if ws.Paths() == nil {
		t.Error("expected paths to be initialized")
	}
}

func TestNewWithGraph(t *testing.T) {
	ws := setupTestWorkspace(t)
	if ws.Store() == nil {
		t.Error("expected store")
	}
	if ws.Meta() == nil {
		t.Error("expected meta")
	}
}

// --- Type resolution ---

func TestResolveEntityType(t *testing.T) {
	ws := setupTestWorkspace(t)

	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"requirement", "requirement", false},
		{"decision", "decision", false},
		{"requirements", "requirement", false},
		{"decisions", "decision", false},
		{"unknown", "", true},
	}

	for _, tt := range tests {
		resolved, _, err := ws.ResolveEntityType(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ResolveEntityType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if resolved != tt.want {
			t.Errorf("ResolveEntityType(%q) = %q, want %q", tt.input, resolved, tt.want)
		}
	}
}

// --- ID generation ---

func TestGenerateID(t *testing.T) {
	ws := setupTestWorkspace(t)

	id, err := ws.GenerateID("requirement", "")
	if err != nil {
		t.Fatalf("GenerateID() error = %v", err)
	}
	if id != "REQ-001" {
		t.Errorf("GenerateID() = %q, want REQ-001", id)
	}
}

func TestGenerateID_ManualType(t *testing.T) {
	ws := setupTestWorkspace(t)

	_, err := ws.GenerateID("stakeholder", "")
	if err == nil {
		t.Error("expected error for manual ID type")
	}
}

func TestGenerateID_UnknownType(t *testing.T) {
	ws := setupTestWorkspace(t)

	_, err := ws.GenerateID("nonexistent", "")
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestGenerateID_Sequential(t *testing.T) {
	ws := setupTestWorkspace(t)

	// Create an existing entity so the next ID is REQ-002.
	mustCreate(t, ws, "requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "Existing"},
	})

	id, err := ws.GenerateID("requirement", "")
	if err != nil {
		t.Fatalf("GenerateID() error = %v", err)
	}
	if id != "REQ-002" {
		t.Errorf("GenerateID() = %q, want REQ-002", id)
	}
}

func TestGenerateID_ShortWithIDCaps(t *testing.T) {
	shortIDMetamodel := `version: "1.0"
entities:
  ticket-upper:
    label: Ticket (Uppercase)
    plural: tickets-upper
    id_prefix: "TKT-"
    id_type: short
    id_caps: upper
    properties:
      title:
        type: string
        required: true
  ticket-lower:
    label: Ticket (Lowercase)
    plural: tickets-lower
    id_prefix: "TKT-"
    id_type: short
    id_caps: lower
    properties:
      title:
        type: string
        required: true
  ticket-default:
    label: Ticket (Default)
    plural: tickets-default
    id_prefix: "TKT-"
    id_type: short
    properties:
      title:
        type: string
        required: true
relations: {}
`
	ws := setupWorkspaceWithMetamodel(t, shortIDMetamodel)

	tests := []struct {
		entityType  string
		expectUpper bool
	}{
		{"ticket-upper", true},
		{"ticket-lower", false},
		{"ticket-default", true}, // default is uppercase
	}

	for _, tt := range tests {
		t.Run(tt.entityType, func(t *testing.T) {
			id, err := ws.GenerateID(tt.entityType, "")
			if err != nil {
				t.Fatalf("GenerateID() error = %v", err)
			}

			// ID should start with TKT-
			if !hasPrefix(id, "TKT-") {
				t.Errorf("GenerateID() = %q, expected prefix TKT-", id)
			}

			// Get the random suffix (everything after TKT-)
			suffix := id[4:]
			if suffix == "" {
				t.Fatalf("GenerateID() = %q, no suffix generated", id)
			}

			// Check case of the random suffix
			for _, c := range suffix {
				if c >= 'A' && c <= 'Z' {
					if !tt.expectUpper {
						t.Errorf("GenerateID() = %q, expected lowercase suffix but found uppercase char %q", id, c)
					}
				} else if c >= 'a' && c <= 'z' {
					if tt.expectUpper {
						t.Errorf("GenerateID() = %q, expected uppercase suffix but found lowercase char %q", id, c)
					}
				}
				// digits 0-9 are case-neutral, ignore them
			}
		})
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// --- CreateEntity ---

func TestCreateEntity(t *testing.T) {
	ws := setupTestWorkspace(t)

	entity, result, err := ws.createEntity("requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "My Requirement"},
	})
	if err != nil {
		t.Fatalf("CreateEntity() error = %v", err)
	}
	if entity.ID != "REQ-001" {
		t.Errorf("entity.ID = %q, want REQ-001", entity.ID)
	}
	if entity.Type != "requirement" {
		t.Errorf("entity.Type = %q, want requirement", entity.Type)
	}
	if entity.GetString("title") != "My Requirement" {
		t.Errorf("title = %q, want My Requirement", entity.GetString("title"))
	}
	// Automation should have set status to draft.
	if entity.GetString("status") != "draft" {
		t.Errorf("status = %q, want draft", entity.GetString("status"))
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if _, ok := ws.GetEntity("REQ-001"); !ok {
		t.Error("entity not found after create")
	}
}

func TestCreateEntity_WithCustomID(t *testing.T) {
	ws := setupTestWorkspace(t)

	entity, _, err := ws.createEntity("stakeholder", CreateOptions{
		ID:         "alice",
		Properties: map[string]interface{}{"name": "Alice"},
	})
	if err != nil {
		t.Fatalf("CreateEntity() error = %v", err)
	}
	if entity.ID != "alice" {
		t.Errorf("entity.ID = %q, want alice", entity.ID)
	}
}

func TestCreateEntity_DuplicateID(t *testing.T) {
	ws := setupTestWorkspace(t)

	_, _, err := ws.createEntity("stakeholder", CreateOptions{
		ID:         "alice",
		Properties: map[string]interface{}{"name": "Alice"},
	})
	if err != nil {
		t.Fatalf("first create error = %v", err)
	}

	_, _, err = ws.createEntity("stakeholder", CreateOptions{
		ID:         "alice",
		Properties: map[string]interface{}{"name": "Duplicate"},
	})
	if err == nil {
		t.Error("expected error for duplicate ID")
	}
}

func TestCreateEntity_CustomIDRejectedForSequential(t *testing.T) {
	ws := setupTestWorkspace(t)

	_, _, err := ws.createEntity("requirement", CreateOptions{
		ID:         "REQ-042",
		Properties: map[string]interface{}{"title": "Should Not Be Allowed"},
	})
	if err == nil {
		t.Fatal("expected error for custom ID on sequential type")
	}
	msg := err.Error()
	// Assert the message names the type, the id_type, the offending id, and
	// "custom ID" so any future refactor that loses one of these fails here.
	for _, want := range []string{"requirement", "sequential", "REQ-042", "custom ID"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}

	// Verify no entity of any kind was persisted — a regression that silently
	// substituted a generated ID would not surface if we only checked REQ-042.
	if _, ok := ws.GetEntity("REQ-042"); ok {
		t.Error("entity was persisted despite rejection")
	}
	if n := countEntities(t, ws); n != 0 {
		t.Errorf("store has %d entities after rejection, want 0", n)
	}
}

func TestCreateEntity_CustomIDRejectedForShort(t *testing.T) {
	shortMetamodel := `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "TKT-"
    id_type: short
    properties:
      title:
        type: string
        required: true
relations: {}
`
	ws := setupWorkspaceWithMetamodel(t, shortMetamodel)

	_, _, err := ws.createEntity("ticket", CreateOptions{
		ID:         "my-custom-id",
		Properties: map[string]interface{}{"title": "Should Not Be Allowed"},
	})
	if err == nil {
		t.Fatal("expected error for custom ID on short type")
	}
	msg := err.Error()
	for _, want := range []string{"ticket", "short", "my-custom-id", "custom ID"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}

	// Verify no entity was persisted (symmetric with the sequential case).
	if _, ok := ws.GetEntity("my-custom-id"); ok {
		t.Error("entity was persisted despite rejection")
	}
	if n := countEntities(t, ws); n != 0 {
		t.Errorf("store has %d entities after rejection, want 0", n)
	}
}

// countEntities returns the number of entities in the workspace's store.
func countEntities(t *testing.T, ws *Workspace) int {
	t.Helper()
	n := 0
	for _, err := range ws.Store().ListEntities(context.Background(), store.EntityQuery{}) {
		if err != nil {
			t.Fatalf("ListEntities: %v", err)
		}
		n++
	}
	return n
}

// TestCreateEntity_RequiredMissingSurfacesWarning verifies AC8/AC1
// from TKT-QETTR: a required-property-missing condition no longer
// hard-rejects the create — it succeeds with a warning per DEC-HWZHA.
func TestCreateEntity_RequiredMissingSurfacesWarning(t *testing.T) {
	ws := setupTestWorkspace(t)

	// title is required but not provided — soft condition per DEC-HWZHA.
	created, result, err := ws.createEntity("requirement", CreateOptions{})
	if err != nil {
		t.Fatalf("createEntity should succeed with warning, got error: %v", err)
	}
	if created == nil {
		t.Fatal("expected created entity")
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected at least one warning for required-field-missing")
	}
	found := false
	for _, w := range result.Warnings {
		if w.Code == "required_property_unset" && w.Path == "/properties/title" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected required_property_unset warning at /properties/title, got %v", result.Warnings)
	}
}

func TestCreateEntity_UnknownType(t *testing.T) {
	ws := setupTestWorkspace(t)

	_, _, err := ws.createEntity("nonexistent", CreateOptions{})
	if err == nil {
		t.Error("expected error for unknown entity type")
	}
}

func TestCreateEntity_WithContent(t *testing.T) {
	ws := setupTestWorkspace(t)

	entity, _, err := ws.createEntity("requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "With Body"},
		Content:    "# Description\n\nSome content.",
	})
	if err != nil {
		t.Fatalf("CreateEntity() error = %v", err)
	}
	if entity.Content != "# Description\n\nSome content." {
		t.Errorf("content = %q", entity.Content)
	}
}

// --- UpdateEntity ---

func TestUpdateEntity(t *testing.T) {
	ws := setupTestWorkspace(t)

	entity, _, err := ws.createEntity("requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "Original"},
	})
	if err != nil {
		t.Fatalf("create error = %v", err)
	}

	// Clone for old entity.
	oldEntity := &entitypkg.Entity{
		ID:         entity.ID,
		Type:       entity.Type,
		Properties: map[string]interface{}{"title": "Original", "status": entity.GetString("status")},
	}

	entity.SetString("title", "Updated")

	result, err := ws.updateEntity(entity, oldEntity)
	if err != nil {
		t.Fatalf("UpdateEntity() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	updated, ok := ws.GetEntity(entity.ID)
	if !ok {
		t.Fatal("entity not found after update")
	}
	if updated.GetString("title") != "Updated" {
		t.Errorf("title = %q, want Updated", updated.GetString("title"))
	}
}

// --- DeleteEntity ---

func TestDeleteEntity_NoCascade_NoRelations(t *testing.T) {
	ws := setupTestWorkspace(t)

	req, _, err := ws.createEntity("requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "To Delete"},
	})
	if err != nil {
		t.Fatalf("create error = %v", err)
	}

	result, err := ws.deleteEntity("requirement", req.ID, false)
	if err != nil {
		t.Fatalf("DeleteEntity() error = %v", err)
	}
	if result.RelationsDeleted != 0 {
		t.Errorf("relations deleted = %d, want 0", result.RelationsDeleted)
	}
	if _, ok := ws.GetEntity(req.ID); ok {
		t.Error("entity still present after delete")
	}
}

func TestDeleteEntity_CascadeRelations(t *testing.T) {
	ws := setupTestWorkspace(t)

	req := mustCreate(t, ws, "requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "Req"},
	})
	dec := mustCreate(t, ws, "decision", CreateOptions{
		Properties: map[string]interface{}{"title": "Dec"},
	})

	_, err := ws.createRelation(dec.ID, "addresses", req.ID)
	if err != nil {
		t.Fatalf("CreateRelation error = %v", err)
	}

	// Delete without cascade should fail.
	_, err = ws.deleteEntity("requirement", req.ID, false)
	if !errors.Is(err, ErrHasRelations) {
		t.Errorf("expected ErrHasRelations, got %v", err)
	}

	// Delete with cascade should work.
	result, err := ws.deleteEntity("requirement", req.ID, true)
	if err != nil {
		t.Fatalf("cascade delete error = %v", err)
	}
	if result.RelationsDeleted != 1 {
		t.Errorf("relations deleted = %d, want 1", result.RelationsDeleted)
	}
}

func TestDeleteEntity_NotFound(t *testing.T) {
	ws := setupTestWorkspace(t)

	_, err := ws.deleteEntity("requirement", "NONEXISTENT", false)
	if err == nil {
		t.Error("expected error for missing entity")
	}
}

// --- CreateRelation ---

func TestCreateRelation(t *testing.T) {
	ws := setupTestWorkspace(t)

	req := mustCreate(t, ws, "requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "Req"},
	})
	dec := mustCreate(t, ws, "decision", CreateOptions{
		Properties: map[string]interface{}{"title": "Dec"},
	})

	rel, err := ws.createRelation(dec.ID, "addresses", req.ID)
	if err != nil {
		t.Fatalf("CreateRelation() error = %v", err)
	}
	if rel.From != dec.ID || rel.Type != "addresses" || rel.To != req.ID {
		t.Errorf("unexpected relation: %+v", rel)
	}

	if _, ok := ws.GetRelation(dec.ID, "addresses", req.ID); !ok {
		t.Error("relation not found after create")
	}
}

func TestCreateRelation_Duplicate(t *testing.T) {
	ws := setupTestWorkspace(t)

	req := mustCreate(t, ws, "requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "Req"},
	})
	dec := mustCreate(t, ws, "decision", CreateOptions{
		Properties: map[string]interface{}{"title": "Dec"},
	})

	_, _ = ws.createRelation(dec.ID, "addresses", req.ID)
	_, err := ws.createRelation(dec.ID, "addresses", req.ID)
	if err == nil {
		t.Error("expected error for duplicate relation")
	}
}

func TestCreateRelation_MissingEndpoint(t *testing.T) {
	ws := setupTestWorkspace(t)

	_, err := ws.createRelation("MISSING", "addresses", "ALSO-MISSING")
	if err == nil {
		t.Error("expected error for missing endpoints")
	}
}

// --- DeleteRelation ---

func TestDeleteRelation(t *testing.T) {
	ws := setupTestWorkspace(t)

	req := mustCreate(t, ws, "requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "Req"},
	})
	dec := mustCreate(t, ws, "decision", CreateOptions{
		Properties: map[string]interface{}{"title": "Dec"},
	})
	_, _ = ws.createRelation(dec.ID, "addresses", req.ID)

	err := ws.deleteRelation(dec.ID, "addresses", req.ID)
	if err != nil {
		t.Fatalf("DeleteRelation() error = %v", err)
	}

	if _, ok := ws.GetRelation(dec.ID, "addresses", req.ID); ok {
		t.Error("relation still present after delete")
	}
}

// --- Errors ---

func TestIsValidationError(t *testing.T) {
	err := newValidationError(nil)
	if !IsValidationError(err) {
		t.Error("expected IsValidationError to return true")
	}
	if IsValidationError(nil) {
		t.Error("expected IsValidationError(nil) to return false")
	}
}

// --- Automation create_entity integration tests ---

func TestCreateEntity_AutomationWithIfExistsSkip(t *testing.T) {
	ws := setupTestWorkspaceWithCreateEntityAutomation(t, "skip")

	// Create a requirement - this triggers automation to create checklist.
	req, result, err := ws.createEntity("requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "Test Req"},
	})
	if err != nil {
		t.Fatalf("CreateEntity error = %v", err)
	}

	// Automation should have created a checklist.
	if len(result.EntitiesCreated) != 1 {
		t.Fatalf("expected 1 entity created by automation, got %d", len(result.EntitiesCreated))
	}
	checklist1 := result.EntitiesCreated[0]
	if checklist1.Type != "checklist" {
		t.Errorf("expected checklist type, got %s", checklist1.Type)
	}

	// Relation should exist.
	if len(result.RelationsCreated) != 1 {
		t.Fatalf("expected 1 relation created, got %d", len(result.RelationsCreated))
	}

	// Make a copy of the old state for property change detection.
	oldReq := entitypkg.New(req.ID, req.Type)
	for k, v := range req.Properties {
		oldReq.Properties[k] = v
	}

	// Now update the requirement to trigger automation again.
	req.SetString("status", "approved")
	updateResult, err := ws.updateEntity(req, oldReq)
	if err != nil {
		t.Fatalf("UpdateEntity error = %v", err)
	}

	// With if_exists:skip, should return existing checklist, not create new one.
	if len(updateResult.EntitiesCreated) != 1 {
		t.Fatalf("expected 1 entity (existing), got %d", len(updateResult.EntitiesCreated))
	}
	checklist2 := updateResult.EntitiesCreated[0]
	if checklist2.ID != checklist1.ID {
		t.Errorf("expected same checklist ID %s, got %s", checklist1.ID, checklist2.ID)
	}

	// No new relation should be created.
	if len(updateResult.RelationsCreated) != 0 {
		t.Errorf("expected 0 new relations, got %d", len(updateResult.RelationsCreated))
	}
}

func TestCreateEntity_AutomationWithIfExistsError(t *testing.T) {
	ws := setupTestWorkspaceWithCreateEntityAutomation(t, "error")

	// Create a requirement - this triggers automation to create checklist.
	req, result, err := ws.createEntity("requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "Test Req"},
	})
	if err != nil {
		t.Fatalf("CreateEntity error = %v", err)
	}

	// First creation should succeed.
	if len(result.EntitiesCreated) != 1 {
		t.Fatalf("expected 1 entity created, got %d", len(result.EntitiesCreated))
	}
	checklist1 := result.EntitiesCreated[0]
	if checklist1.Type != "checklist" {
		t.Errorf("expected checklist type, got %s", checklist1.Type)
	}

	// Make a copy of the old state for property change detection.
	oldReq := entitypkg.New(req.ID, req.Type)
	for k, v := range req.Properties {
		oldReq.Properties[k] = v
	}

	// Update the same requirement to trigger automation again.
	// With if_exists:error, this should produce an error.
	req.SetString("status", "approved")
	updateResult, err := ws.updateEntity(req, oldReq)
	if err != nil {
		t.Fatalf("UpdateEntity error = %v", err)
	}

	// With if_exists:error, should get an error about existing entity.
	if len(updateResult.AutomationErrors) == 0 {
		t.Fatal("expected automation error for if_exists:error, got none")
	}

	// Verify the error message mentions the existing relation.
	foundExpectedError := false
	for _, errMsg := range updateResult.AutomationErrors {
		if strings.Contains(errMsg, "already exists via") && strings.Contains(errMsg, "has-checklist") {
			foundExpectedError = true
			break
		}
	}

	if !foundExpectedError {
		t.Errorf("expected error about existing relation, got: %v", updateResult.AutomationErrors)
	}

	// No new entity should be created.
	if len(updateResult.EntitiesCreated) != 0 {
		t.Errorf("expected 0 entities created with if_exists:error, got %d",
			len(updateResult.EntitiesCreated))
	}
}

func TestCreateEntity_AutomationWithIfExistsReplace(t *testing.T) {
	ws := setupTestWorkspaceWithCreateEntityAutomation(t, "replace")

	// Create a requirement - this triggers automation to create checklist.
	req, result, err := ws.createEntity("requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "Test Req"},
	})
	if err != nil {
		t.Fatalf("CreateEntity error = %v", err)
	}

	// Automation should have created a checklist.
	if len(result.EntitiesCreated) != 1 {
		t.Fatalf("expected 1 entity created, got %d", len(result.EntitiesCreated))
	}
	checklist1 := result.EntitiesCreated[0]

	// Make a copy of the old state for property change detection.
	oldReq := entitypkg.New(req.ID, req.Type)
	for k, v := range req.Properties {
		oldReq.Properties[k] = v
	}

	// Now update the requirement to trigger automation again.
	req.SetString("status", "approved")
	updateResult, err := ws.updateEntity(req, oldReq)
	if err != nil {
		t.Fatalf("UpdateEntity error = %v", err)
	}

	// Check if automation triggered by verifying the title was updated.
	if req.GetString("title") != "Updated requirement" {
		t.Fatalf("automation did not trigger - title not updated, got %q", req.GetString("title"))
	}

	// Check for automation errors.
	if len(updateResult.AutomationErrors) > 0 {
		t.Fatalf("automation errors: %v", updateResult.AutomationErrors)
	}

	// With if_exists:replace, should delete old and create new checklist.
	if len(updateResult.EntitiesCreated) != 1 {
		t.Fatalf("expected 1 new entity, got %d; errors: %v",
			len(updateResult.EntitiesCreated), updateResult.AutomationErrors)
	}
	checklist2 := updateResult.EntitiesCreated[0]
	if checklist2.ID == checklist1.ID {
		t.Errorf("expected different checklist ID after replace, got same: %s", checklist2.ID)
	}

	if _, ok := ws.GetEntity(checklist1.ID); ok {
		t.Errorf("old checklist %s should be deleted", checklist1.ID)
	}
}

func TestCreateEntity_AutomationWithIfExistsUnknown(t *testing.T) {
	// Test that unknown if_exists values produce an error when the relation exists.
	ws := setupTestWorkspaceWithCreateEntityAutomation(t, "invalid_value")

	// Create a requirement - this triggers automation to create checklist.
	req, result, err := ws.createEntity("requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "Test Req"},
	})
	if err != nil {
		t.Fatalf("CreateEntity error = %v", err)
	}

	// First creation should succeed (no existing relation to check).
	if len(result.EntitiesCreated) != 1 {
		t.Fatalf("expected 1 entity created, got %d", len(result.EntitiesCreated))
	}

	// Make a copy of the old state for property change detection.
	oldReq := entitypkg.New(req.ID, req.Type)
	for k, v := range req.Properties {
		oldReq.Properties[k] = v
	}

	// Update the same requirement to trigger automation again.
	// With unknown if_exists value, this should produce an error.
	req.SetString("status", "approved")
	updateResult, err := ws.updateEntity(req, oldReq)
	if err != nil {
		t.Fatalf("UpdateEntity error = %v", err)
	}

	// Should get an error about unknown if_exists value.
	if len(updateResult.AutomationErrors) == 0 {
		t.Fatal("expected automation error for unknown if_exists value, got none")
	}

	foundExpectedError := false
	for _, errMsg := range updateResult.AutomationErrors {
		if strings.Contains(errMsg, "unknown if_exists value") {
			foundExpectedError = true
			break
		}
	}
	if !foundExpectedError {
		t.Errorf("expected error about unknown if_exists value, got: %v", updateResult.AutomationErrors)
	}

	// No new entity should be created.
	if len(updateResult.EntitiesCreated) != 0 {
		t.Errorf("expected 0 entities created with unknown if_exists, got %d",
			len(updateResult.EntitiesCreated))
	}
}

func setupTestWorkspaceWithCreateEntityAutomation(t *testing.T, ifExists string) *Workspace {
	t.Helper()
	metamodelYAML := `version: "1.0"
entities:
  requirement:
    label: Requirement
    plural: requirements
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      status:
        type: string
  checklist:
    label: Checklist
    plural: checklists
    id_prefix: "CHK-"
    id_type: short
    properties:
      title:
        type: string
        required: true
      status:
        type: string
relations:
  has-checklist:
    label: has checklist
    from: [requirement]
    to: [checklist]
automations:
  - name: create-checklist
    on:
      entity: [requirement]
      created: true
    do:
      - create_entity:
          type: checklist
          relation: has-checklist
          if_exists: ` + ifExists + `
          properties:
            title: "Checklist for requirement"
  - name: mark-updated
    on:
      entity: [requirement]
      property: status
    do:
      - set: title
        value: "Updated requirement"
      - create_entity:
          type: checklist
          relation: has-checklist
          if_exists: ` + ifExists + `
          properties:
            title: "Checklist for requirement"
`
	return setupWorkspaceWithMetamodel(t, metamodelYAML)
}

// --- Template variant integration tests ---

func TestCreateEntity_AutomationWithTemplate(t *testing.T) {
	ws, fs, ctx := setupTestWorkspaceWithTemplateAutomation(t)

	// Create the template variant file.
	enhancementTemplate := `---
title: Enhancement Checklist
status: pending
---
## Enhancement Tasks
`
	_ = fs.WriteFile(ctx.EntityTemplatesDir+"/checklist--enhancement.md",
		[]byte(enhancementTemplate), 0o644)

	// Create a requirement with kind=enhancement - triggers automation with template.
	_, result, err := ws.createEntity("requirement", CreateOptions{
		Properties: map[string]interface{}{
			"title": "Test Req",
			"kind":  "enhancement",
		},
	})
	if err != nil {
		t.Fatalf("CreateEntity error = %v", err)
	}

	// Automation should have created a checklist with template defaults.
	if len(result.EntitiesCreated) != 1 {
		t.Fatalf("expected 1 entity created by automation, got %d", len(result.EntitiesCreated))
	}
	checklist := result.EntitiesCreated[0]
	if checklist.Type != "checklist" {
		t.Errorf("expected checklist type, got %s", checklist.Type)
	}

	// Verify template was applied - should have title from template.
	if checklist.GetString("title") != "Enhancement Checklist" {
		t.Errorf("expected title from template, got %q", checklist.GetString("title"))
	}
	if checklist.GetString("status") != "pending" {
		t.Errorf("expected status from template, got %q", checklist.GetString("status"))
	}
}

func TestCreateEntity_AutomationWithMissingTemplate(t *testing.T) {
	ws, _, _ := setupTestWorkspaceWithTemplateAutomation(t)

	// Create a requirement with kind=nonexistent - template doesn't exist.
	_, result, err := ws.createEntity("requirement", CreateOptions{
		Properties: map[string]interface{}{
			"title": "Test Req",
			"kind":  "nonexistent",
		},
	})
	if err != nil {
		t.Fatalf("CreateEntity error = %v", err)
	}

	// Automation should report error for missing template variant.
	if len(result.AutomationErrors) == 0 {
		t.Error("expected error for missing template variant")
	}

	// No entity should be created when template is missing.
	if len(result.EntitiesCreated) != 0 {
		t.Errorf("expected 0 entities created, got %d", len(result.EntitiesCreated))
	}
}

func TestCreateEntity_AutomationWithEmptyTemplate(t *testing.T) {
	ws, fs, ctx := setupTestWorkspaceWithTemplateAutomation(t)

	// Create the default template file (no variant).
	defaultTemplate := `---
title: Default Checklist
status: open
---
## Default Tasks
`
	_ = fs.WriteFile(ctx.EntityTemplatesDir+"/checklist.md",
		[]byte(defaultTemplate), 0o644)

	// Create a requirement with empty kind - should use default template.
	_, result, err := ws.createEntity("requirement", CreateOptions{
		Properties: map[string]interface{}{
			"title": "Test Req",
			// kind not set - empty string
		},
	})
	if err != nil {
		t.Fatalf("CreateEntity error = %v", err)
	}

	// Should succeed with default template.
	if len(result.AutomationErrors) != 0 {
		t.Errorf("unexpected errors: %v", result.AutomationErrors)
	}

	// Entity should be created with default template.
	if len(result.EntitiesCreated) != 1 {
		t.Fatalf("expected 1 entity created, got %d", len(result.EntitiesCreated))
	}
	checklist := result.EntitiesCreated[0]
	if checklist.GetString("title") != "Default Checklist" {
		t.Errorf("expected title from default template, got %q", checklist.GetString("title"))
	}
}

func setupTestWorkspaceWithTemplateAutomation(t *testing.T) (*Workspace, storage.FS, *project.Context) {
	t.Helper()
	metamodelYAML := `version: "1.0"
entities:
  requirement:
    label: Requirement
    plural: requirements
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      kind:
        type: string
  checklist:
    label: Checklist
    plural: checklists
    id_prefix: "CHK-"
    id_type: short
    properties:
      title:
        type: string
        required: true
      status:
        type: string
relations:
  has-checklist:
    label: has checklist
    from: [requirement]
    to: [checklist]
automations:
  - name: create-checklist-with-template
    on:
      entity: [requirement]
      created: true
    do:
      - create_entity:
          type: checklist
          template: "{{new.kind}}"
          relation: has-checklist
`
	meta, err := metamodel.Parse([]byte(metamodelYAML))
	if err != nil {
		t.Fatalf("failed to parse test metamodel: %v", err)
	}

	ctx := testContext()
	fs := setupWorkspaceFS(ctx, meta, metamodelYAML)
	ws := NewForTest(meta, WithFS(fs, ctx))

	return ws, fs, ctx
}

// --- Automation recursion depth limit tests ---

func TestCreateEntity_AutomationDepthLimit(t *testing.T) {
	// This test verifies that recursive automations are limited to maxAutomationDepth.
	// Creating a "starter" triggers creation of "chain" entities recursively.
	metamodelYAML := `version: "1.0"
entities:
  starter:
    label: Starter
    plural: starters
    id_prefix: "START-"
    id_type: short
    properties:
      title:
        type: string
  chain:
    label: Chain
    plural: chains
    id_prefix: "CHAIN-"
    id_type: short
    properties:
      title:
        type: string
      depth:
        type: string
relations:
  triggers:
    label: triggers
    from: [starter, chain]
    to: [chain]
automations:
  - name: starter-creates-chain
    on:
      entity: [starter]
      created: true
    do:
      - create_entity:
          type: chain
          relation: triggers
          properties:
            title: "Chain from starter"
            depth: "1"
  - name: chain-creates-chain
    on:
      entity: [chain]
      created: true
    do:
      - create_entity:
          type: chain
          relation: triggers
          properties:
            title: "Chain from chain"
`
	ws := setupWorkspaceWithMetamodel(t, metamodelYAML)

	// Create starter entity - this should trigger a chain of automations.
	_, result, err := ws.createEntity("starter", CreateOptions{
		Properties: map[string]interface{}{"title": "Test Starter"},
	})
	if err != nil {
		t.Fatalf("CreateEntity error = %v", err)
	}

	// Should have created multiple chain entities (limited by maxAutomationDepth).
	// At depth 0: starter created, automation creates chain at depth 1
	// At depth 1: chain created, automation creates chain at depth 2
	// ... up to maxAutomationDepth
	chainCount := 0
	for _, e := range result.EntitiesCreated {
		if e.Type == "chain" {
			chainCount++
		}
	}

	// We should have exactly maxAutomationDepth chain entities.
	// Depth 0 creates at depth 1, depth 1 creates at depth 2, etc.
	// So entities are created at depths 1 through maxAutomationDepth.
	if chainCount != maxAutomationDepth {
		t.Errorf("expected %d chain entities (depth limit), got %d", maxAutomationDepth, chainCount)
	}

	// Should have a warning about iteration limit being reached.
	hasLimitWarning := false
	for _, w := range result.AutomationWarnings {
		if strings.Contains(w, "iteration limit") {
			hasLimitWarning = true
			break
		}
	}
	if !hasLimitWarning {
		t.Errorf("expected warning about iteration limit, got warnings: %v", result.AutomationWarnings)
	}

	// Verify store is consistent - all entities should be present.
	allNodes := collectEntities(ws.Store(), store.EntityQuery{})
	// 1 starter + maxAutomationDepth chains
	expectedTotal := 1 + maxAutomationDepth
	if len(allNodes) != expectedTotal {
		t.Errorf("expected %d total entities, got %d", expectedTotal, len(allNodes))
	}
}

func TestCreateEntity_AutomationChainWithoutLoop(t *testing.T) {
	// This test verifies that non-looping chains work correctly: A → B → C
	metamodelYAML := `version: "1.0"
entities:
  alpha:
    label: Alpha
    plural: alphas
    id_prefix: "A-"
    id_type: short
    properties:
      title:
        type: string
  beta:
    label: Beta
    plural: betas
    id_prefix: "B-"
    id_type: short
    properties:
      title:
        type: string
  gamma:
    label: Gamma
    plural: gammas
    id_prefix: "G-"
    id_type: short
    properties:
      title:
        type: string
relations:
  creates:
    label: creates
    from: [alpha, beta]
    to: [beta, gamma]
automations:
  - name: alpha-creates-beta
    on:
      entity: [alpha]
      created: true
    do:
      - create_entity:
          type: beta
          relation: creates
          properties:
            title: "Beta from Alpha"
  - name: beta-creates-gamma
    on:
      entity: [beta]
      created: true
    do:
      - create_entity:
          type: gamma
          relation: creates
          properties:
            title: "Gamma from Beta"
`
	ws := setupWorkspaceWithMetamodel(t, metamodelYAML)

	// Create alpha - should trigger beta creation, which triggers gamma creation.
	_, result, err := ws.createEntity("alpha", CreateOptions{
		Properties: map[string]interface{}{"title": "Test Alpha"},
	})
	if err != nil {
		t.Fatalf("CreateEntity error = %v", err)
	}

	// Should have created 1 beta and 1 gamma.
	betaCount := 0
	gammaCount := 0
	for _, e := range result.EntitiesCreated {
		switch e.Type {
		case "beta":
			betaCount++
		case "gamma":
			gammaCount++
		}
	}

	if betaCount != 1 {
		t.Errorf("expected 1 beta entity, got %d", betaCount)
	}
	if gammaCount != 1 {
		t.Errorf("expected 1 gamma entity, got %d", gammaCount)
	}

	// Should have no warnings (well under depth limit).
	if len(result.AutomationWarnings) > 0 {
		t.Errorf("expected no warnings, got: %v", result.AutomationWarnings)
	}

	// Verify relations: alpha → beta → gamma.
	allNodes := collectEntities(ws.Store(), store.EntityQuery{})
	if len(allNodes) != 3 {
		t.Errorf("expected 3 entities, got %d", len(allNodes))
	}
}

// --- Lua automation tests ---
// Inline lua: actions work with in-memory FS. Only lua_file: actions need real
// filesystem (os.OpenRoot doesn't work with in-memory FS), so those tests verify
// validation errors rather than actual script execution.

func TestLuaAutomation_InlineCode(t *testing.T) {
	ws := setupTestWorkspaceWithLuaAutomation(t)

	// Create entity to trigger Lua automation.
	entity, result, err := ws.createEntity("requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "Test Req"},
	})
	if err != nil {
		t.Fatalf("CreateEntity error = %v", err)
	}

	// Should have no errors.
	if len(result.AutomationErrors) > 0 {
		t.Errorf("unexpected automation errors: %v", result.AutomationErrors)
	}

	// Lua automation updates the entity via rela.update_entity.
	updated, _ := ws.GetEntity(entity.ID)
	if updated.GetString("status") != "processed" {
		t.Errorf("expected status 'processed' from Lua, got %q", updated.GetString("status"))
	}
}

func TestLuaAutomation_EntityGlobals(t *testing.T) {
	// Test that entity global is available in Lua on create.
	metamodelYAML := `version: "1.0"
entities:
  item:
    label: Item
    plural: items
    id_prefix: "ITEM-"
    id_type: short
    properties:
      title:
        type: string
        required: true
      lua_result:
        type: string
relations: {}
automations:
  - name: check-entity-globals
    on:
      entity: [item]
      created: true
    do:
      - lua: |
          -- Access entity global and verify it exists
          if entity and entity.id then
            rela.update_entity(entity.id, {lua_result = "entity_id:" .. entity.id})
          end
`
	ws := setupWorkspaceWithMetamodel(t, metamodelYAML)

	// Create entity - automation triggers on created.
	entity, result, err := ws.createEntity("item", CreateOptions{
		Properties: map[string]interface{}{"title": "Test Item"},
	})
	if err != nil {
		t.Fatalf("CreateEntity error = %v", err)
	}

	if len(result.AutomationErrors) > 0 {
		t.Errorf("automation errors: %v", result.AutomationErrors)
	}

	// Verify lua_result was set by Lua code using entity global.
	updated, _ := ws.GetEntity(entity.ID)
	expectedResult := "entity_id:" + entity.ID
	if updated.GetString("lua_result") != expectedResult {
		t.Errorf("expected lua_result %q, got %q", expectedResult, updated.GetString("lua_result"))
	}
}

func TestLuaAutomation_OldEntityGlobal(t *testing.T) {
	// Test that old_entity global is available during updates.
	// Use a simple inline Lua code that just sets old_status.
	metamodelYAML := `version: "1.0"
entities:
  item:
    label: Item
    plural: items
    id_prefix: "ITEM-"
    id_type: short
    properties:
      title:
        type: string
        required: true
      status:
        type: string
      old_status:
        type: string
relations: {}
automations:
  - name: capture-old-status
    on:
      entity: [item]
      property: status
    do:
      - lua: "if old_entity then rela.update_entity(entity.id, {old_status = old_entity.properties.status or 'nil'}) else rela.update_entity(entity.id, {old_status = 'no_old'}) end"
`
	ws := setupWorkspaceWithMetamodel(t, metamodelYAML)

	// Create entity with initial status.
	entity, _, err := ws.createEntity("item", CreateOptions{
		Properties: map[string]interface{}{"title": "Test Item", "status": "draft"},
	})
	if err != nil {
		t.Fatalf("CreateEntity error = %v", err)
	}

	entityID := entity.ID

	// Get fresh entity from the store (may have been modified by creation automations).
	fresh, _ := ws.GetEntity(entityID)
	entity = fresh

	// Update to trigger automation.
	oldEntity := entity.Clone()
	updated := entity.Clone()
	updated.SetString("status", "active")

	result, err := ws.updateEntity(updated, oldEntity)
	if err != nil {
		t.Fatalf("UpdateEntity error = %v", err)
	}

	if len(result.AutomationErrors) > 0 {
		t.Errorf("automation errors: %v", result.AutomationErrors)
	}

	// Verify old_status was captured from old_entity.
	finalEntity, _ := ws.GetEntity(entityID)
	oldStatusVal := finalEntity.GetString("old_status")
	switch oldStatusVal {
	case "":
		t.Errorf("old_status is empty - Lua code did not run")
	case "no_old":
		t.Errorf("old_entity global was nil in Lua")
	case "nil":
		t.Errorf("old_entity.properties.status was nil")
	case "draft":
		// Success - expected value
	default:
		t.Errorf("expected old_status 'draft', got %q", oldStatusVal)
	}
}

func TestLuaAutomation_LuaFilePathTraversal(t *testing.T) {
	// Test that path traversal attempts are blocked.
	metamodelYAML := `version: "1.0"
entities:
  item:
    label: Item
    plural: items
    id_prefix: "ITEM-"
    id_type: short
    properties:
      title:
        type: string
        required: true
relations: {}
automations:
  - name: path-traversal-attempt
    on:
      entity: [item]
      created: true
    do:
      - lua_file: "../../../etc/passwd"
`
	ws := setupWorkspaceWithMetamodel(t, metamodelYAML)

	// Create entity - should trigger automation with path traversal attempt.
	_, result, err := ws.createEntity("item", CreateOptions{
		Properties: map[string]interface{}{"title": "Test Item"},
	})
	if err != nil {
		t.Fatalf("CreateEntity error = %v", err)
	}

	// Should have an error about path traversal.
	if len(result.AutomationErrors) == 0 {
		t.Fatal("expected automation error for path traversal, got none")
	}

	foundTraversalError := false
	for _, errMsg := range result.AutomationErrors {
		if strings.Contains(errMsg, "local path") || strings.Contains(errMsg, "..") {
			foundTraversalError = true
			break
		}
	}
	if !foundTraversalError {
		t.Errorf("expected path traversal error, got: %v", result.AutomationErrors)
	}
}

func TestLuaAutomation_LuaFileMissingExtension(t *testing.T) {
	// Test that files without .lua extension are rejected.
	metamodelYAML := `version: "1.0"
entities:
  item:
    label: Item
    plural: items
    id_prefix: "ITEM-"
    id_type: short
    properties:
      title:
        type: string
        required: true
relations: {}
automations:
  - name: wrong-extension
    on:
      entity: [item]
      created: true
    do:
      - lua_file: "script.txt"
`
	ws := setupWorkspaceWithMetamodel(t, metamodelYAML)

	// Create entity - should trigger automation with wrong extension.
	_, result, err := ws.createEntity("item", CreateOptions{
		Properties: map[string]interface{}{"title": "Test Item"},
	})
	if err != nil {
		t.Fatalf("CreateEntity error = %v", err)
	}

	// Should have an error about extension.
	if len(result.AutomationErrors) == 0 {
		t.Fatal("expected automation error for wrong extension, got none")
	}

	foundExtensionError := false
	for _, errMsg := range result.AutomationErrors {
		if strings.Contains(errMsg, ".lua extension") {
			foundExtensionError = true
			break
		}
	}
	if !foundExtensionError {
		t.Errorf("expected .lua extension error, got: %v", result.AutomationErrors)
	}
}

func TestLuaAutomation_LuaExecutionError(t *testing.T) {
	// Test that Lua errors are captured and reported.
	metamodelYAML := `version: "1.0"
entities:
  item:
    label: Item
    plural: items
    id_prefix: "ITEM-"
    id_type: short
    properties:
      title:
        type: string
        required: true
relations: {}
automations:
  - name: lua-error
    on:
      entity: [item]
      created: true
    do:
      - lua: |
          -- This will cause a Lua error
          error("intentional test error")
`
	ws := setupWorkspaceWithMetamodel(t, metamodelYAML)

	// Create entity - should trigger automation with Lua error.
	_, result, err := ws.createEntity("item", CreateOptions{
		Properties: map[string]interface{}{"title": "Test Item"},
	})
	if err != nil {
		t.Fatalf("CreateEntity error = %v", err)
	}

	// Should have a Lua execution error.
	if len(result.AutomationErrors) == 0 {
		t.Fatal("expected Lua execution error, got none")
	}

	foundLuaError := false
	for _, errMsg := range result.AutomationErrors {
		// New envelope shape: "<path>:<line>: <message>" where path is
		// "automation:<automation-name>" for inline `lua: |` blocks.
		if strings.Contains(errMsg, "automation:lua-error") &&
			strings.Contains(errMsg, "intentional test error") {

			foundLuaError = true
			break
		}
	}
	if !foundLuaError {
		t.Errorf("expected automation script error, got: %v", result.AutomationErrors)
	}
}

func setupTestWorkspaceWithLuaAutomation(t *testing.T) *Workspace {
	t.Helper()
	metamodelYAML := `version: "1.0"
entities:
  requirement:
    label: Requirement
    plural: requirements
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      status:
        type: string
relations: {}
automations:
  - name: lua-on-create
    on:
      entity: [requirement]
      created: true
    do:
      - lua: |
          -- Set status to processed via Lua
          rela.update_entity(entity.id, {status = "processed"})
`
	return setupWorkspaceWithMetamodel(t, metamodelYAML)
}
