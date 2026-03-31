package workspace

import (
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// testMetamodelYAML is the shared workspace test metamodel - use testutil.WorkspaceMetamodelYAML()
var testMetamodelYAML = testutil.WorkspaceMetamodelYAML()

func setupTestWorkspace(t *testing.T) *Workspace {
	t.Helper()
	fs := storage.NewMemFS()

	root := "/project"
	ctx := &project.Context{
		Root:                 root,
		MetamodelPath:        root + "/metamodel.yaml",
		CacheDir:             root + "/.rela",
		CachePath:            root + "/.rela/cache.json",
		EntitiesDir:          root + "/entities",
		RelationsDir:         root + "/relations",
		TemplatesDir:         root + "/templates",
		EntityTemplatesDir:   root + "/templates/entities",
		RelationTemplatesDir: root + "/templates/relations",
	}

	_ = fs.MkdirAll(ctx.EntitiesDir+"/requirements", 0o755)
	_ = fs.MkdirAll(ctx.EntitiesDir+"/decisions", 0o755)
	_ = fs.MkdirAll(ctx.EntitiesDir+"/stakeholders", 0o755)
	_ = fs.MkdirAll(ctx.EntitiesDir+"/checklists", 0o755)
	_ = fs.MkdirAll(ctx.RelationsDir, 0o755)
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	_ = fs.MkdirAll(ctx.EntityTemplatesDir, 0o755)
	_ = fs.MkdirAll(ctx.RelationTemplatesDir, 0o755)
	_ = fs.WriteFile(ctx.MetamodelPath, []byte(testMetamodelYAML), 0o644)

	meta, err := metamodel.Parse([]byte(testMetamodelYAML))
	if err != nil {
		t.Fatalf("failed to parse test metamodel: %v", err)
	}

	repo := repository.New(fs, ctx)
	g := graph.New()
	ws := NewWithGraph(repo, meta, g)

	return ws
}

// mustCreate is a test helper that creates an entity, fatally failing on error.
func mustCreate(t *testing.T, ws *Workspace, entityType string, opts CreateOptions) {
	t.Helper()
	if _, _, err := ws.CreateEntity(entityType, opts); err != nil {
		t.Fatalf("mustCreate(%s): %v", entityType, err)
	}
}

// --- Constructor tests ---

func TestNew(t *testing.T) {
	fs := storage.NewMemFS()
	root := "/project"
	ctx := &project.Context{
		Root:                 root,
		MetamodelPath:        root + "/metamodel.yaml",
		CacheDir:             root + "/.rela",
		CachePath:            root + "/.rela/cache.json",
		EntitiesDir:          root + "/entities",
		RelationsDir:         root + "/relations",
		TemplatesDir:         root + "/templates",
		EntityTemplatesDir:   root + "/templates/entities",
		RelationTemplatesDir: root + "/templates/relations",
	}
	_ = fs.MkdirAll(ctx.EntitiesDir+"/requirements", 0o755)
	_ = fs.MkdirAll(ctx.EntitiesDir+"/decisions", 0o755)
	_ = fs.MkdirAll(ctx.RelationsDir, 0o755)
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	_ = fs.WriteFile(ctx.MetamodelPath, []byte(testMetamodelYAML), 0o644)

	repo := repository.New(fs, ctx)
	ws, err := New(repo)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if ws.Graph() == nil {
		t.Error("expected graph to be initialized")
	}
	if ws.Meta() == nil {
		t.Error("expected meta to be initialized")
	}
	if ws.Repo() == nil {
		t.Error("expected repo to be initialized")
	}
}

func TestNewWithGraph(t *testing.T) {
	ws := setupTestWorkspace(t)
	if ws.Graph() == nil {
		t.Error("expected graph")
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

	// Add an existing entity so the next ID is REQ-002.
	ws.graph.AddNode(testutil.Entity("requirement").ID("REQ-001").With("title", "first").Build())

	id, err := ws.GenerateID("requirement", "")
	if err != nil {
		t.Fatalf("GenerateID() error = %v", err)
	}
	if id != "REQ-002" {
		t.Errorf("GenerateID() = %q, want REQ-002", id)
	}
}

func TestGenerateID_ShortWithIDCaps(t *testing.T) {
	// Test that id_caps configuration affects short ID generation
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
	fs := storage.NewMemFS()
	root := "/project"
	ctx := &project.Context{
		Root:                 root,
		MetamodelPath:        root + "/metamodel.yaml",
		CacheDir:             root + "/.rela",
		CachePath:            root + "/.rela/cache.json",
		EntitiesDir:          root + "/entities",
		RelationsDir:         root + "/relations",
		TemplatesDir:         root + "/templates",
		EntityTemplatesDir:   root + "/templates/entities",
		RelationTemplatesDir: root + "/templates/relations",
	}

	_ = fs.MkdirAll(ctx.EntitiesDir+"/tickets-upper", 0o755)
	_ = fs.MkdirAll(ctx.EntitiesDir+"/tickets-lower", 0o755)
	_ = fs.MkdirAll(ctx.EntitiesDir+"/tickets-default", 0o755)
	_ = fs.MkdirAll(ctx.RelationsDir, 0o755)
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	_ = fs.WriteFile(ctx.MetamodelPath, []byte(shortIDMetamodel), 0o644)

	meta, err := metamodel.Parse([]byte(shortIDMetamodel))
	if err != nil {
		t.Fatalf("failed to parse metamodel: %v", err)
	}

	repo := repository.New(fs, ctx)
	g := graph.New()
	ws := NewWithGraph(repo, meta, g)

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

	entity, result, err := ws.CreateEntity("requirement", CreateOptions{
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

	// Verify entity is in graph.
	if _, ok := ws.graph.GetNode("REQ-001"); !ok {
		t.Error("entity not found in graph after create")
	}
}

func TestCreateEntity_WithCustomID(t *testing.T) {
	ws := setupTestWorkspace(t)

	entity, _, err := ws.CreateEntity("requirement", CreateOptions{
		ID:         "REQ-042",
		Properties: map[string]interface{}{"title": "Custom ID"},
	})
	if err != nil {
		t.Fatalf("CreateEntity() error = %v", err)
	}
	if entity.ID != "REQ-042" {
		t.Errorf("entity.ID = %q, want REQ-042", entity.ID)
	}
}

func TestCreateEntity_DuplicateID(t *testing.T) {
	ws := setupTestWorkspace(t)

	_, _, err := ws.CreateEntity("requirement", CreateOptions{
		ID:         "REQ-001",
		Properties: map[string]interface{}{"title": "First"},
	})
	if err != nil {
		t.Fatalf("first create error = %v", err)
	}

	_, _, err = ws.CreateEntity("requirement", CreateOptions{
		ID:         "REQ-001",
		Properties: map[string]interface{}{"title": "Duplicate"},
	})
	if err == nil {
		t.Error("expected error for duplicate ID")
	}
}

func TestCreateEntity_ValidationError(t *testing.T) {
	ws := setupTestWorkspace(t)

	// title is required but not provided
	_, _, err := ws.CreateEntity("requirement", CreateOptions{})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !IsValidationError(err) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestCreateEntity_UnknownType(t *testing.T) {
	ws := setupTestWorkspace(t)

	_, _, err := ws.CreateEntity("nonexistent", CreateOptions{})
	if err == nil {
		t.Error("expected error for unknown entity type")
	}
}

func TestCreateEntity_WithContent(t *testing.T) {
	ws := setupTestWorkspace(t)

	entity, _, err := ws.CreateEntity("requirement", CreateOptions{
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

	entity, _, err := ws.CreateEntity("requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "Original"},
	})
	if err != nil {
		t.Fatalf("create error = %v", err)
	}

	// Clone for old entity.
	oldEntity := &model.Entity{
		ID:         entity.ID,
		Type:       entity.Type,
		Properties: map[string]interface{}{"title": "Original", "status": entity.GetString("status")},
	}

	entity.SetString("title", "Updated")

	result, err := ws.UpdateEntity(entity, oldEntity)
	if err != nil {
		t.Fatalf("UpdateEntity() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Verify update in graph.
	updated, ok := ws.graph.GetNode(entity.ID)
	if !ok {
		t.Fatal("entity not found in graph")
	}
	if updated.GetString("title") != "Updated" {
		t.Errorf("title = %q, want Updated", updated.GetString("title"))
	}
}

// --- DeleteEntity ---

func TestDeleteEntity_NoCascade_NoRelations(t *testing.T) {
	ws := setupTestWorkspace(t)

	_, _, err := ws.CreateEntity("requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "To Delete"},
	})
	if err != nil {
		t.Fatalf("create error = %v", err)
	}

	result, err := ws.DeleteEntity("requirement", "REQ-001", false)
	if err != nil {
		t.Fatalf("DeleteEntity() error = %v", err)
	}
	if result.RelationsDeleted != 0 {
		t.Errorf("relations deleted = %d, want 0", result.RelationsDeleted)
	}
	if _, ok := ws.graph.GetNode("REQ-001"); ok {
		t.Error("entity still in graph after delete")
	}
}

func TestDeleteEntity_CascadeRelations(t *testing.T) {
	ws := setupTestWorkspace(t)

	mustCreate(t, ws, "requirement", CreateOptions{
		ID:         "REQ-001",
		Properties: map[string]interface{}{"title": "Req"},
	})
	mustCreate(t, ws, "decision", CreateOptions{
		ID:         "DEC-001",
		Properties: map[string]interface{}{"title": "Dec"},
	})

	_, err := ws.CreateRelation("DEC-001", "addresses", "REQ-001")
	if err != nil {
		t.Fatalf("CreateRelation error = %v", err)
	}

	// Delete without cascade should fail.
	_, err = ws.DeleteEntity("requirement", "REQ-001", false)
	if !errors.Is(err, ErrHasRelations) {
		t.Errorf("expected ErrHasRelations, got %v", err)
	}

	// Delete with cascade should work.
	result, err := ws.DeleteEntity("requirement", "REQ-001", true)
	if err != nil {
		t.Fatalf("cascade delete error = %v", err)
	}
	if result.RelationsDeleted != 1 {
		t.Errorf("relations deleted = %d, want 1", result.RelationsDeleted)
	}
}

func TestDeleteEntity_NotFound(t *testing.T) {
	ws := setupTestWorkspace(t)

	_, err := ws.DeleteEntity("requirement", "NONEXISTENT", false)
	if err == nil {
		t.Error("expected error for missing entity")
	}
}

// --- CreateRelation ---

func TestCreateRelation(t *testing.T) {
	ws := setupTestWorkspace(t)

	reqID := "REQ-001"
	decID := "DEC-001"
	mustCreate(t, ws, "requirement", CreateOptions{
		ID:         reqID,
		Properties: map[string]interface{}{"title": "Req"},
	})
	mustCreate(t, ws, "decision", CreateOptions{
		ID:         decID,
		Properties: map[string]interface{}{"title": "Dec"},
	})

	rel, err := ws.CreateRelation(decID, "addresses", reqID)
	if err != nil {
		t.Fatalf("CreateRelation() error = %v", err)
	}
	if rel.From != decID || rel.Type != "addresses" || rel.To != reqID {
		t.Errorf("unexpected relation: %+v", rel)
	}

	// Verify in graph.
	if _, ok := ws.graph.GetEdge("DEC-001", "addresses", "REQ-001"); !ok {
		t.Error("relation not found in graph")
	}
}

func TestCreateRelation_Duplicate(t *testing.T) {
	ws := setupTestWorkspace(t)

	mustCreate(t, ws, "requirement", CreateOptions{
		ID:         "REQ-001",
		Properties: map[string]interface{}{"title": "Req"},
	})
	mustCreate(t, ws, "decision", CreateOptions{
		ID:         "DEC-001",
		Properties: map[string]interface{}{"title": "Dec"},
	})

	_, _ = ws.CreateRelation("DEC-001", "addresses", "REQ-001")
	_, err := ws.CreateRelation("DEC-001", "addresses", "REQ-001")
	if err == nil {
		t.Error("expected error for duplicate relation")
	}
}

func TestCreateRelation_MissingEndpoint(t *testing.T) {
	ws := setupTestWorkspace(t)

	_, err := ws.CreateRelation("MISSING", "addresses", "ALSO-MISSING")
	if err == nil {
		t.Error("expected error for missing endpoints")
	}
}

// --- DeleteRelation ---

func TestDeleteRelation(t *testing.T) {
	ws := setupTestWorkspace(t)

	mustCreate(t, ws, "requirement", CreateOptions{
		ID:         "REQ-001",
		Properties: map[string]interface{}{"title": "Req"},
	})
	mustCreate(t, ws, "decision", CreateOptions{
		ID:         "DEC-001",
		Properties: map[string]interface{}{"title": "Dec"},
	})
	_, _ = ws.CreateRelation("DEC-001", "addresses", "REQ-001")

	err := ws.DeleteRelation("DEC-001", "addresses", "REQ-001")
	if err != nil {
		t.Fatalf("DeleteRelation() error = %v", err)
	}

	if _, ok := ws.graph.GetEdge("DEC-001", "addresses", "REQ-001"); ok {
		t.Error("relation still in graph after delete")
	}
}

// --- Sync / Reload ---

func TestSync(t *testing.T) {
	ws := setupTestWorkspace(t)

	// Create an entity via workspace.
	mustCreate(t, ws, "requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "Synced"},
	})

	// Sync should reload from disk.
	result, err := ws.Sync()
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.EntitiesLoaded != 1 {
		t.Errorf("entities loaded = %d, want 1", result.EntitiesLoaded)
	}
}

func TestReload(t *testing.T) {
	ws := setupTestWorkspace(t)

	mustCreate(t, ws, "requirement", CreateOptions{
		Properties: map[string]interface{}{"title": "Before Reload"},
	})

	result, err := ws.Reload()
	if err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if result.EntitiesLoaded != 1 {
		t.Errorf("entities loaded = %d, want 1", result.EntitiesLoaded)
	}
}

// --- Locking ---

func TestRLock(t *testing.T) {
	ws := setupTestWorkspace(t)

	// Just verify it doesn't deadlock.
	ws.RLock()
	_ = ws.Meta()
	ws.RUnlock()
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
	req, result, err := ws.CreateEntity("requirement", CreateOptions{
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
	oldReq := model.NewEntity(req.ID, req.Type)
	for k, v := range req.Properties {
		oldReq.Properties[k] = v
	}

	// Now update the requirement to trigger automation again.
	req.SetString("status", "approved")
	updateResult, err := ws.UpdateEntity(req, oldReq)
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
	req, result, err := ws.CreateEntity("requirement", CreateOptions{
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
	oldReq := model.NewEntity(req.ID, req.Type)
	for k, v := range req.Properties {
		oldReq.Properties[k] = v
	}

	// Update the same requirement to trigger automation again.
	// With if_exists:error, this should produce an error.
	req.SetString("status", "approved")
	updateResult, err := ws.UpdateEntity(req, oldReq)
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
	req, result, err := ws.CreateEntity("requirement", CreateOptions{
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
	oldReq := model.NewEntity(req.ID, req.Type)
	for k, v := range req.Properties {
		oldReq.Properties[k] = v
	}

	// Now update the requirement to trigger automation again.
	req.SetString("status", "approved")
	updateResult, err := ws.UpdateEntity(req, oldReq)
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

	// Old checklist should be gone from graph.
	if _, ok := ws.graph.GetNode(checklist1.ID); ok {
		t.Errorf("old checklist %s should be deleted from graph", checklist1.ID)
	}
}

func TestCreateEntity_AutomationWithIfExistsUnknown(t *testing.T) {
	// Test that unknown if_exists values produce an error when the relation exists.
	ws := setupTestWorkspaceWithCreateEntityAutomation(t, "invalid_value")

	// Create a requirement - this triggers automation to create checklist.
	req, result, err := ws.CreateEntity("requirement", CreateOptions{
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
	oldReq := model.NewEntity(req.ID, req.Type)
	for k, v := range req.Properties {
		oldReq.Properties[k] = v
	}

	// Update the same requirement to trigger automation again.
	// With unknown if_exists value, this should produce an error.
	req.SetString("status", "approved")
	updateResult, err := ws.UpdateEntity(req, oldReq)
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

	fs := storage.NewMemFS()
	root := "/project"
	ctx := &project.Context{
		Root:                 root,
		MetamodelPath:        root + "/metamodel.yaml",
		CacheDir:             root + "/.rela",
		CachePath:            root + "/.rela/cache.json",
		EntitiesDir:          root + "/entities",
		RelationsDir:         root + "/relations",
		TemplatesDir:         root + "/templates",
		EntityTemplatesDir:   root + "/templates/entities",
		RelationTemplatesDir: root + "/templates/relations",
	}

	_ = fs.MkdirAll(ctx.EntitiesDir+"/requirements", 0o755)
	_ = fs.MkdirAll(ctx.EntitiesDir+"/checklists", 0o755)
	_ = fs.MkdirAll(ctx.RelationsDir, 0o755)
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	_ = fs.MkdirAll(ctx.EntityTemplatesDir, 0o755)
	_ = fs.MkdirAll(ctx.RelationTemplatesDir, 0o755)
	_ = fs.WriteFile(ctx.MetamodelPath, []byte(metamodelYAML), 0o644)

	meta, err := metamodel.Parse([]byte(metamodelYAML))
	if err != nil {
		t.Fatalf("failed to parse test metamodel: %v", err)
	}

	repo := repository.New(fs, ctx)
	g := graph.New()
	ws := NewWithGraph(repo, meta, g)

	return ws
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
	_, result, err := ws.CreateEntity("requirement", CreateOptions{
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
	_, result, err := ws.CreateEntity("requirement", CreateOptions{
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
	_, result, err := ws.CreateEntity("requirement", CreateOptions{
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

	fs := storage.NewMemFS()
	root := "/project"
	ctx := &project.Context{
		Root:                 root,
		MetamodelPath:        root + "/metamodel.yaml",
		CacheDir:             root + "/.rela",
		CachePath:            root + "/.rela/cache.json",
		EntitiesDir:          root + "/entities",
		RelationsDir:         root + "/relations",
		TemplatesDir:         root + "/templates",
		EntityTemplatesDir:   root + "/templates/entities",
		RelationTemplatesDir: root + "/templates/relations",
	}

	_ = fs.MkdirAll(ctx.EntitiesDir+"/requirements", 0o755)
	_ = fs.MkdirAll(ctx.EntitiesDir+"/checklists", 0o755)
	_ = fs.MkdirAll(ctx.RelationsDir, 0o755)
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	_ = fs.MkdirAll(ctx.EntityTemplatesDir, 0o755)
	_ = fs.MkdirAll(ctx.RelationTemplatesDir, 0o755)
	_ = fs.WriteFile(ctx.MetamodelPath, []byte(metamodelYAML), 0o644)

	meta, err := metamodel.Parse([]byte(metamodelYAML))
	if err != nil {
		t.Fatalf("failed to parse test metamodel: %v", err)
	}

	repo := repository.New(fs, ctx)
	g := graph.New()
	ws := NewWithGraph(repo, meta, g)

	return ws, fs, ctx
}

// --- Automation recursion depth limit tests ---

func TestCreateEntity_AutomationDepthLimit(t *testing.T) {
	// This test verifies that recursive automations are limited to maxAutomationDepth.
	// We set up a chain where:
	// - Creating a "starter" triggers creation of a "chain" entity
	// - Creating a "chain" triggers creation of another "chain" entity
	// This would be infinite without the depth limit.

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

	fs := storage.NewMemFS()
	root := "/project"
	ctx := &project.Context{
		Root:                 root,
		MetamodelPath:        root + "/metamodel.yaml",
		CacheDir:             root + "/.rela",
		CachePath:            root + "/.rela/cache.json",
		EntitiesDir:          root + "/entities",
		RelationsDir:         root + "/relations",
		TemplatesDir:         root + "/templates",
		EntityTemplatesDir:   root + "/templates/entities",
		RelationTemplatesDir: root + "/templates/relations",
	}

	_ = fs.MkdirAll(ctx.EntitiesDir+"/starters", 0o755)
	_ = fs.MkdirAll(ctx.EntitiesDir+"/chains", 0o755)
	_ = fs.MkdirAll(ctx.RelationsDir, 0o755)
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	_ = fs.WriteFile(ctx.MetamodelPath, []byte(metamodelYAML), 0o644)

	meta, err := metamodel.Parse([]byte(metamodelYAML))
	if err != nil {
		t.Fatalf("failed to parse test metamodel: %v", err)
	}

	repo := repository.New(fs, ctx)
	g := graph.New()
	ws := NewWithGraph(repo, meta, g)

	// Create starter entity - this should trigger a chain of automations.
	_, result, err := ws.CreateEntity("starter", CreateOptions{
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

	// Verify graph is consistent - all entities should be in the graph.
	allNodes := g.AllNodes()
	// 1 starter + maxAutomationDepth chains
	expectedTotal := 1 + maxAutomationDepth
	if len(allNodes) != expectedTotal {
		t.Errorf("expected %d total nodes in graph, got %d", expectedTotal, len(allNodes))
	}
}

func TestCreateEntity_AutomationChainWithoutLoop(t *testing.T) {
	// This test verifies that non-looping chains work correctly.
	// A → B → C (3 levels, well under the limit)

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

	fs := storage.NewMemFS()
	root := "/project"
	ctx := &project.Context{
		Root:                 root,
		MetamodelPath:        root + "/metamodel.yaml",
		CacheDir:             root + "/.rela",
		CachePath:            root + "/.rela/cache.json",
		EntitiesDir:          root + "/entities",
		RelationsDir:         root + "/relations",
		TemplatesDir:         root + "/templates",
		EntityTemplatesDir:   root + "/templates/entities",
		RelationTemplatesDir: root + "/templates/relations",
	}

	_ = fs.MkdirAll(ctx.EntitiesDir+"/alphas", 0o755)
	_ = fs.MkdirAll(ctx.EntitiesDir+"/betas", 0o755)
	_ = fs.MkdirAll(ctx.EntitiesDir+"/gammas", 0o755)
	_ = fs.MkdirAll(ctx.RelationsDir, 0o755)
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	_ = fs.WriteFile(ctx.MetamodelPath, []byte(metamodelYAML), 0o644)

	meta, err := metamodel.Parse([]byte(metamodelYAML))
	if err != nil {
		t.Fatalf("failed to parse test metamodel: %v", err)
	}

	repo := repository.New(fs, ctx)
	g := graph.New()
	ws := NewWithGraph(repo, meta, g)

	// Create alpha - should trigger beta creation, which triggers gamma creation.
	_, result, err := ws.CreateEntity("alpha", CreateOptions{
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
	allNodes := g.AllNodes()
	if len(allNodes) != 3 {
		t.Errorf("expected 3 nodes in graph, got %d", len(allNodes))
	}
}
