package workspace

import (
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const testMetamodelYAML = `version: "1.0"
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
  decision:
    label: Decision
    plural: decisions
    id_prefix: "DEC-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      status:
        type: string
  stakeholder:
    label: Stakeholder
    plural: stakeholders
    id_type: manual
    properties:
      name:
        type: string
        required: true
relations:
  addresses:
    label: Addresses
    from: [decision]
    to: [requirement]
automations:
  - name: auto-draft
    on:
      entity: [requirement]
      created: true
    do:
      - set: status
        value: draft
`

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
	if ws.Paths() == nil {
		t.Error("expected paths to be initialized")
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
	ws.graph.AddNode(&model.Entity{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{"title": "first"}})

	id, err := ws.GenerateID("requirement", "")
	if err != nil {
		t.Fatalf("GenerateID() error = %v", err)
	}
	if id != "REQ-002" {
		t.Errorf("GenerateID() = %q, want REQ-002", id)
	}
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

	mustCreate(t, ws, "requirement", CreateOptions{
		ID:         "REQ-001",
		Properties: map[string]interface{}{"title": "Req"},
	})
	mustCreate(t, ws, "decision", CreateOptions{
		ID:         "DEC-001",
		Properties: map[string]interface{}{"title": "Dec"},
	})

	rel, err := ws.CreateRelation("DEC-001", "addresses", "REQ-001")
	if err != nil {
		t.Fatalf("CreateRelation() error = %v", err)
	}
	if rel.From != "DEC-001" || rel.Type != "addresses" || rel.To != "REQ-001" {
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
