package repository

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const testMetamodelYAML = `version: "1.0"
entities:
  requirement:
    label: Requirement
    plural: requirements
    id_prefix: "REQ-"
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
    properties:
      title:
        type: string
        required: true
relations:
  addresses:
    label: Addresses
    from: [decision]
    to: [requirement]
`

func setupTestRepo(t *testing.T) (*Repository, *metamodel.Metamodel) {
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

	// Create directory structure
	_ = fs.MkdirAll(ctx.EntitiesDir+"/requirements", 0o755)
	_ = fs.MkdirAll(ctx.EntitiesDir+"/decisions", 0o755)
	_ = fs.MkdirAll(ctx.RelationsDir, 0o755)
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	_ = fs.MkdirAll(ctx.EntityTemplatesDir, 0o755)
	_ = fs.MkdirAll(ctx.RelationTemplatesDir, 0o755)

	// Write metamodel file (for LoadMetamodel test)
	_ = fs.WriteFile(ctx.MetamodelPath, []byte(testMetamodelYAML), 0o644)

	meta, err := metamodel.Parse([]byte(testMetamodelYAML))
	if err != nil {
		t.Fatalf("failed to parse test metamodel: %v", err)
	}

	repo := New(fs, ctx)
	return repo, meta
}

// --- Entity CRUD ---

func TestRepository_WriteAndReadEntity(t *testing.T) {
	repo, meta := setupTestRepo(t)

	entity := model.NewEntity("REQ-001", "requirement")
	entity.SetString("title", "Test Requirement")
	entity.SetString("status", "draft")

	if err := repo.WriteEntity(entity, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}

	if entity.FilePath == "" {
		t.Error("WriteEntity() should set entity.FilePath")
	}

	got, err := repo.ReadEntity("requirement", "REQ-001", meta)
	if err != nil {
		t.Fatalf("ReadEntity() error = %v", err)
	}

	if got.ID != "REQ-001" {
		t.Errorf("ReadEntity() ID = %q, want %q", got.ID, "REQ-001")
	}
	if got.Type != "requirement" {
		t.Errorf("ReadEntity() Type = %q, want %q", got.Type, "requirement")
	}
	if got.GetString("title") != "Test Requirement" {
		t.Errorf("ReadEntity() title = %q, want %q", got.GetString("title"), "Test Requirement")
	}
}

func TestRepository_WriteEntitySetsFilePath(t *testing.T) {
	repo, meta := setupTestRepo(t)

	entity := model.NewEntity("REQ-002", "requirement")
	entity.SetString("title", "Path Check")

	if err := repo.WriteEntity(entity, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}

	want := "/project/entities/requirements/REQ-002.md"
	if entity.FilePath != want {
		t.Errorf("entity.FilePath = %q, want %q", entity.FilePath, want)
	}
}

func TestRepository_WriteEntityUnknownType(t *testing.T) {
	repo, meta := setupTestRepo(t)

	entity := model.NewEntity("FOO-001", "unknown_type")
	entity.SetString("title", "Bad Type")

	err := repo.WriteEntity(entity, meta)
	if err == nil {
		t.Fatal("WriteEntity() should fail for unknown entity type")
	}
}

func TestRepository_ReadEntityNotFound(t *testing.T) {
	repo, meta := setupTestRepo(t)

	_, err := repo.ReadEntity("requirement", "NONEXISTENT", meta)
	if err == nil {
		t.Fatal("ReadEntity() should fail for non-existent entity")
	}
}

func TestRepository_ReadEntityUnknownType(t *testing.T) {
	repo, meta := setupTestRepo(t)

	_, err := repo.ReadEntity("nonexistent", "FOO-001", meta)
	if err == nil {
		t.Fatal("ReadEntity() should fail for unknown entity type")
	}
}

func TestRepository_DeleteEntity(t *testing.T) {
	repo, meta := setupTestRepo(t)

	entity := model.NewEntity("REQ-003", "requirement")
	entity.SetString("title", "To Delete")

	if err := repo.WriteEntity(entity, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}

	// Verify it exists
	if _, err := repo.ReadEntity("requirement", "REQ-003", meta); err != nil {
		t.Fatalf("entity should exist before delete: %v", err)
	}

	if err := repo.DeleteEntity("requirement", "REQ-003", meta); err != nil {
		t.Fatalf("DeleteEntity() error = %v", err)
	}

	// Verify it's gone
	_, err := repo.ReadEntity("requirement", "REQ-003", meta)
	if err == nil {
		t.Error("entity should not exist after delete")
	}
}

func TestRepository_DeleteEntityUnknownType(t *testing.T) {
	repo, meta := setupTestRepo(t)

	err := repo.DeleteEntity("nonexistent", "FOO-001", meta)
	if err == nil {
		t.Fatal("DeleteEntity() should fail for unknown entity type")
	}
}

func TestRepository_ListEntities(t *testing.T) {
	repo, meta := setupTestRepo(t)

	// Write multiple entities
	for _, id := range []string{"REQ-001", "REQ-002"} {
		e := model.NewEntity(id, "requirement")
		e.SetString("title", "Entity "+id)
		if err := repo.WriteEntity(e, meta); err != nil {
			t.Fatalf("WriteEntity(%s) error = %v", id, err)
		}
	}
	dec := model.NewEntity("DEC-001", "decision")
	dec.SetString("title", "Decision 1")
	if err := repo.WriteEntity(dec, meta); err != nil {
		t.Fatalf("WriteEntity(DEC-001) error = %v", err)
	}

	entities, err := repo.ListEntities(meta)
	if err != nil {
		t.Fatalf("ListEntities() error = %v", err)
	}

	if len(entities) != 3 {
		t.Errorf("ListEntities() returned %d entities, want 3", len(entities))
	}
}

// --- Relation CRUD ---

func TestRepository_WriteAndReadRelation(t *testing.T) {
	repo, _ := setupTestRepo(t)

	rel := model.NewRelation("DEC-001", "addresses", "REQ-001")

	if err := repo.WriteRelation(rel); err != nil {
		t.Fatalf("WriteRelation() error = %v", err)
	}

	if rel.FilePath == "" {
		t.Error("WriteRelation() should set relation.FilePath")
	}

	got, err := repo.ReadRelation("DEC-001", "addresses", "REQ-001")
	if err != nil {
		t.Fatalf("ReadRelation() error = %v", err)
	}

	if got.From != "DEC-001" {
		t.Errorf("ReadRelation() From = %q, want %q", got.From, "DEC-001")
	}
	if got.Type != "addresses" {
		t.Errorf("ReadRelation() Type = %q, want %q", got.Type, "addresses")
	}
	if got.To != "REQ-001" {
		t.Errorf("ReadRelation() To = %q, want %q", got.To, "REQ-001")
	}
}

func TestRepository_DeleteRelation(t *testing.T) {
	repo, _ := setupTestRepo(t)

	rel := model.NewRelation("DEC-001", "addresses", "REQ-001")
	if err := repo.WriteRelation(rel); err != nil {
		t.Fatalf("WriteRelation() error = %v", err)
	}

	if err := repo.DeleteRelation("DEC-001", "addresses", "REQ-001"); err != nil {
		t.Fatalf("DeleteRelation() error = %v", err)
	}

	_, err := repo.ReadRelation("DEC-001", "addresses", "REQ-001")
	if err == nil {
		t.Error("relation should not exist after delete")
	}
}

func TestRepository_ListRelations(t *testing.T) {
	repo, _ := setupTestRepo(t)

	r1 := model.NewRelation("DEC-001", "addresses", "REQ-001")
	r2 := model.NewRelation("DEC-001", "addresses", "REQ-002")

	if err := repo.WriteRelation(r1); err != nil {
		t.Fatalf("WriteRelation(r1) error = %v", err)
	}
	if err := repo.WriteRelation(r2); err != nil {
		t.Fatalf("WriteRelation(r2) error = %v", err)
	}

	relations, err := repo.ListRelations()
	if err != nil {
		t.Fatalf("ListRelations() error = %v", err)
	}

	if len(relations) != 2 {
		t.Errorf("ListRelations() returned %d relations, want 2", len(relations))
	}
}

// --- Sync ---

func TestRepository_Sync(t *testing.T) {
	repo, meta := setupTestRepo(t)

	// Write entities and a relation
	e1 := model.NewEntity("REQ-001", "requirement")
	e1.SetString("title", "Req 1")
	e2 := model.NewEntity("DEC-001", "decision")
	e2.SetString("title", "Dec 1")
	_ = repo.WriteEntity(e1, meta)
	_ = repo.WriteEntity(e2, meta)

	rel := model.NewRelation("DEC-001", "addresses", "REQ-001")
	_ = repo.WriteRelation(rel)

	// Sync into a fresh graph
	g := graph.New()
	result, err := repo.Sync(meta, g)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if result.EntitiesLoaded != 2 {
		t.Errorf("Sync() EntitiesLoaded = %d, want 2", result.EntitiesLoaded)
	}
	if result.RelationsLoaded != 1 {
		t.Errorf("Sync() RelationsLoaded = %d, want 1", result.RelationsLoaded)
	}

	// Verify graph contents
	if _, ok := g.GetNode("REQ-001"); !ok {
		t.Error("graph should contain REQ-001 after sync")
	}
	if _, ok := g.GetNode("DEC-001"); !ok {
		t.Error("graph should contain DEC-001 after sync")
	}
}

// --- Cache ---

func TestRepository_CacheSaveAndLoad(t *testing.T) {
	repo, meta := setupTestRepo(t)

	// Build a graph with entities
	g := graph.New()
	e1 := model.NewEntity("REQ-001", "requirement")
	e1.SetString("title", "Cached Req")
	g.AddNode(e1)

	// Initially no cache
	if repo.CacheExists() {
		t.Error("CacheExists() should be false before save")
	}

	if err := repo.SaveCache(g); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	if !repo.CacheExists() {
		t.Error("CacheExists() should be true after save")
	}

	// Load into a fresh graph
	g2 := graph.New()
	if err := repo.LoadCache(g2); err != nil {
		t.Fatalf("LoadCache() error = %v", err)
	}

	node, ok := g2.GetNode("REQ-001")
	if !ok {
		t.Fatal("loaded graph should contain REQ-001")
	}
	if node.GetString("title") != "Cached Req" {
		t.Errorf("loaded entity title = %q, want %q", node.GetString("title"), "Cached Req")
	}
	_ = meta // used by setupTestRepo
}

// --- Metamodel ---

func TestRepository_LoadMetamodel(t *testing.T) {
	repo, _ := setupTestRepo(t)

	meta, err := repo.LoadMetamodel()
	if err != nil {
		t.Fatalf("LoadMetamodel() error = %v", err)
	}

	if _, ok := meta.GetEntityDef("requirement"); !ok {
		t.Error("loaded metamodel should contain 'requirement' entity type")
	}
	if _, ok := meta.GetEntityDef("decision"); !ok {
		t.Error("loaded metamodel should contain 'decision' entity type")
	}
	if _, ok := meta.GetRelationDef("addresses"); !ok {
		t.Error("loaded metamodel should contain 'addresses' relation type")
	}
}

// --- Path Helpers ---

func TestRepository_EntityFilePath(t *testing.T) {
	repo, meta := setupTestRepo(t)

	got := repo.EntityFilePath("requirement", "REQ-001", meta)
	want := "/project/entities/requirements/REQ-001.md"
	if got != want {
		t.Errorf("EntityFilePath() = %q, want %q", got, want)
	}
}

func TestRepository_EntityFilePath_UnknownType(t *testing.T) {
	repo, meta := setupTestRepo(t)

	got := repo.EntityFilePath("nonexistent", "FOO-001", meta)
	if got != "" {
		t.Errorf("EntityFilePath() = %q, want empty string", got)
	}
}

func TestRepository_EntityTypeDir(t *testing.T) {
	repo, meta := setupTestRepo(t)

	got := repo.EntityTypeDir("requirement", meta)
	want := "/project/entities/requirements"
	if got != want {
		t.Errorf("EntityTypeDir() = %q, want %q", got, want)
	}
}

func TestRepository_EntityTypeDir_UnknownType(t *testing.T) {
	repo, meta := setupTestRepo(t)

	got := repo.EntityTypeDir("nonexistent", meta)
	if got != "" {
		t.Errorf("EntityTypeDir() = %q, want empty string", got)
	}
}

// --- Accessors ---

func TestRepository_FS(t *testing.T) {
	repo, _ := setupTestRepo(t)

	if repo.FS() == nil {
		t.Error("FS() should not be nil")
	}
}

func TestRepository_Paths(t *testing.T) {
	repo, _ := setupTestRepo(t)

	if repo.Paths() == nil {
		t.Error("Paths() should not be nil")
	}
	if repo.Paths().Root != "/project" {
		t.Errorf("Paths().Root = %q, want %q", repo.Paths().Root, "/project")
	}
}

// --- Entity with Content ---

func TestRepository_EntityWithContent(t *testing.T) {
	repo, meta := setupTestRepo(t)

	entity := model.NewEntity("REQ-010", "requirement")
	entity.SetString("title", "With Content")
	entity.Content = "# Description\n\nThis is the body.\n"

	if err := repo.WriteEntity(entity, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}

	got, err := repo.ReadEntity("requirement", "REQ-010", meta)
	if err != nil {
		t.Fatalf("ReadEntity() error = %v", err)
	}

	if got.Content == "" {
		t.Error("entity content should not be empty")
	}
}

// --- Default Plural ---

func TestRepository_EntityFilePath_DefaultPlural(t *testing.T) {
	// Test entity type without explicit plural → uses type+"s"
	fs := storage.NewMemFS()
	ctx := &project.Context{
		Root:        "/project",
		EntitiesDir: "/project/entities",
	}

	metaYAML := `version: "1.0"
entities:
  component:
    label: Component
    id_type: manual
    properties:
      title:
        type: string
`
	meta, err := metamodel.Parse([]byte(metaYAML))
	if err != nil {
		t.Fatalf("failed to parse metamodel: %v", err)
	}

	repo := New(fs, ctx)
	got := repo.EntityFilePath("component", "COMP-001", meta)
	want := "/project/entities/components/COMP-001.md"
	if got != want {
		t.Errorf("EntityFilePath() = %q, want %q (default plural)", got, want)
	}
}

// --- Templates ---

func TestRepository_LoadEntityTemplate_NotFound(t *testing.T) {
	repo, _ := setupTestRepo(t)

	// No template file exists — should return nil, nil
	doc, err := repo.LoadEntityTemplate("requirement")
	if err != nil {
		t.Fatalf("LoadEntityTemplate() error = %v", err)
	}
	if doc != nil {
		t.Error("LoadEntityTemplate() should return nil when no template exists")
	}
}

func TestRepository_LoadEntityTemplate_Exists(t *testing.T) {
	repo, _ := setupTestRepo(t)

	// Write a template file
	tmplContent := "---\nstatus: draft\n---\n\n# Description\n\nTODO\n"
	tmplPath := "/project/templates/entities/requirement.md"
	if err := repo.FS().WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	doc, err := repo.LoadEntityTemplate("requirement")
	if err != nil {
		t.Fatalf("LoadEntityTemplate() error = %v", err)
	}
	if doc == nil {
		t.Fatal("LoadEntityTemplate() should return non-nil for existing template")
	}
}

// --- Multiple Entity Types ---

func TestRepository_MultipleEntityTypes(t *testing.T) {
	repo, meta := setupTestRepo(t)

	req := model.NewEntity("REQ-001", "requirement")
	req.SetString("title", "A Requirement")
	dec := model.NewEntity("DEC-001", "decision")
	dec.SetString("title", "A Decision")

	if err := repo.WriteEntity(req, meta); err != nil {
		t.Fatalf("WriteEntity(req) error = %v", err)
	}
	if err := repo.WriteEntity(dec, meta); err != nil {
		t.Fatalf("WriteEntity(dec) error = %v", err)
	}

	// Read back each type
	gotReq, err := repo.ReadEntity("requirement", "REQ-001", meta)
	if err != nil {
		t.Fatalf("ReadEntity(requirement) error = %v", err)
	}
	if gotReq.GetString("title") != "A Requirement" {
		t.Errorf("requirement title = %q", gotReq.GetString("title"))
	}

	gotDec, err := repo.ReadEntity("decision", "DEC-001", meta)
	if err != nil {
		t.Fatalf("ReadEntity(decision) error = %v", err)
	}
	if gotDec.GetString("title") != "A Decision" {
		t.Errorf("decision title = %q", gotDec.GetString("title"))
	}

	// List should return both
	entities, err := repo.ListEntities(meta)
	if err != nil {
		t.Fatalf("ListEntities() error = %v", err)
	}
	if len(entities) != 2 {
		t.Errorf("ListEntities() = %d, want 2", len(entities))
	}
}

// --- Overwrite ---

func TestRepository_OverwriteEntity(t *testing.T) {
	repo, meta := setupTestRepo(t)

	entity := model.NewEntity("REQ-001", "requirement")
	entity.SetString("title", "Original")
	_ = repo.WriteEntity(entity, meta)

	// Update and overwrite
	entity.SetString("title", "Updated")
	if err := repo.WriteEntity(entity, meta); err != nil {
		t.Fatalf("WriteEntity(update) error = %v", err)
	}

	got, err := repo.ReadEntity("requirement", "REQ-001", meta)
	if err != nil {
		t.Fatalf("ReadEntity() error = %v", err)
	}
	if got.GetString("title") != "Updated" {
		t.Errorf("title = %q, want %q", got.GetString("title"), "Updated")
	}
}

// --- Compile-time check ---

func TestRepository_New(_ *testing.T) {
	_ = New(storage.NewMemFS(), &project.Context{})
}
