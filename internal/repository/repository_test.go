package repository

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// testMetamodelYAML uses the shared rename test metamodel (req/dec with depends-on)
var testMetamodelYAML = testutil.RenameTestMetamodelYAML()

func setupTestRepo(t *testing.T) (*Repository, *metamodel.Metamodel, storage.FS) {
	t.Helper()
	fs := storage.NewMemFS()

	root := "/project"
	ctx := &project.Context{
		Root:                 root,
		MetamodelPath:        root + "/metamodel.yaml",
		CacheDir:             root + "/.rela",
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
	return repo, meta, fs
}

// --- Entity CRUD ---

func TestRepository_WriteAndReadEntity(t *testing.T) {
	repo, meta, _ := setupTestRepo(t)

	entity := testutil.NewEntity("REQ-001", "requirement").
		With("title", "Test Requirement").
		With("status", "draft").
		Build()

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
	repo, meta, _ := setupTestRepo(t)

	entity := testutil.NewEntity("REQ-002", "requirement").
		With("title", "Path Check").
		Build()

	if err := repo.WriteEntity(entity, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}

	want := "/project/entities/requirements/REQ-002.md"
	if entity.FilePath != want {
		t.Errorf("entity.FilePath = %q, want %q", entity.FilePath, want)
	}
}

func TestRepository_WriteEntityUnknownType(t *testing.T) {
	repo, meta, _ := setupTestRepo(t)

	entity := testutil.NewEntity("FOO-001", "unknown_type").
		With("title", "Bad Type").
		Build()

	err := repo.WriteEntity(entity, meta)
	if err == nil {
		t.Fatal("WriteEntity() should fail for unknown entity type")
	}
}

func TestRepository_ReadEntityNotFound(t *testing.T) {
	repo, meta, _ := setupTestRepo(t)

	_, err := repo.ReadEntity("requirement", "NONEXISTENT", meta)
	if err == nil {
		t.Fatal("ReadEntity() should fail for non-existent entity")
	}
}

func TestRepository_ReadEntityUnknownType(t *testing.T) {
	repo, meta, _ := setupTestRepo(t)

	_, err := repo.ReadEntity("nonexistent", "FOO-001", meta)
	if err == nil {
		t.Fatal("ReadEntity() should fail for unknown entity type")
	}
}

func TestRepository_DeleteEntity(t *testing.T) {
	repo, meta, _ := setupTestRepo(t)

	entity := testutil.NewEntity("REQ-003", "requirement").
		With("title", "To Delete").
		Build()

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
	repo, meta, _ := setupTestRepo(t)

	err := repo.DeleteEntity("nonexistent", "FOO-001", meta)
	if err == nil {
		t.Fatal("DeleteEntity() should fail for unknown entity type")
	}
}

func TestRepository_ListEntities(t *testing.T) {
	repo, meta, _ := setupTestRepo(t)

	// Write multiple entities
	for _, id := range []string{"REQ-001", "REQ-002"} {
		e := testutil.NewEntity(id, "requirement").
			With("title", "Entity "+id).
			Build()
		if err := repo.WriteEntity(e, meta); err != nil {
			t.Fatalf("WriteEntity(%s) error = %v", id, err)
		}
	}
	dec := testutil.NewEntity("DEC-001", "decision").
		With("title", "Decision 1").
		Build()
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
	repo, _, _ := setupTestRepo(t)

	rel := testutil.NewRelation("DEC-001", "addresses", "REQ-001").Build()

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
	repo, _, _ := setupTestRepo(t)

	rel := testutil.NewRelation("DEC-001", "addresses", "REQ-001").Build()
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
	repo, _, _ := setupTestRepo(t)

	r1 := testutil.NewRelation("DEC-001", "addresses", "REQ-001").Build()
	r2 := testutil.NewRelation("DEC-001", "addresses", "REQ-002").Build()

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
	repo, meta, _ := setupTestRepo(t)

	// Write entities and a relation
	e1 := testutil.NewEntity("REQ-001", "requirement").With("title", "Req 1").Build()
	e2 := testutil.NewEntity("DEC-001", "decision").With("title", "Dec 1").Build()
	_ = repo.WriteEntity(e1, meta)
	_ = repo.WriteEntity(e2, meta)

	rel := testutil.NewRelation("DEC-001", "addresses", "REQ-001").Build()
	_ = repo.WriteRelation(rel)

	// Sync returns a fresh graph populated from disk.
	g, result, err := repo.Sync(meta)
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

func TestRepository_Sync_MissingSourceEntity(t *testing.T) {
	repo, meta, fs := setupTestRepo(t)

	// Write only target entity
	e1 := testutil.NewEntity("REQ-001", "requirement").With("title", "Req 1").Build()
	_ = repo.WriteEntity(e1, meta)

	// Write relation with missing source directly as a file
	relContent := []byte("---\nfrom: DEC-999\nrelation: addresses\nto: REQ-001\n---\n")
	relPath := "/project/relations/DEC-999--addresses--REQ-001.md"
	_ = fs.WriteFile(relPath, relContent, 0o644)

	_, result, err := repo.Sync(meta)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if result.EntitiesLoaded != 1 {
		t.Errorf("EntitiesLoaded = %d, want 1", result.EntitiesLoaded)
	}
	if result.RelationsLoaded != 0 {
		t.Errorf("RelationsLoaded = %d, want 0 (missing source)", result.RelationsLoaded)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("got %d errors, want 1", len(result.Errors))
	}
}

func TestRepository_Sync_MissingTargetEntity(t *testing.T) {
	repo, meta, fs := setupTestRepo(t)

	// Write only source entity
	e1 := testutil.NewEntity("DEC-001", "decision").With("title", "Dec 1").Build()
	_ = repo.WriteEntity(e1, meta)

	// Write relation with missing target directly as a file
	relContent := []byte("---\nfrom: DEC-001\nrelation: addresses\nto: REQ-999\n---\n")
	relPath := "/project/relations/DEC-001--addresses--REQ-999.md"
	_ = fs.WriteFile(relPath, relContent, 0o644)

	_, result, err := repo.Sync(meta)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if result.EntitiesLoaded != 1 {
		t.Errorf("EntitiesLoaded = %d, want 1", result.EntitiesLoaded)
	}
	if result.RelationsLoaded != 0 {
		t.Errorf("RelationsLoaded = %d, want 0 (missing target)", result.RelationsLoaded)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("got %d errors, want 1", len(result.Errors))
	}
}

func TestRepository_Sync_ReturnsFreshGraph(t *testing.T) {
	repo, meta, _ := setupTestRepo(t)

	// Write one entity
	e1 := testutil.NewEntity("REQ-001", "requirement").With("title", "Req 1").Build()
	_ = repo.WriteEntity(e1, meta)

	// First sync — returns a graph populated from disk.
	first, _, err := repo.Sync(meta)
	if err != nil {
		t.Fatalf("first Sync() error = %v", err)
	}

	// Second sync — must return a different graph instance, not alias
	// the first. This is the contract that Workspace.Reload depends on
	// to leave readers' pre-reload snapshots intact.
	second, result, err := repo.Sync(meta)
	if err != nil {
		t.Fatalf("second Sync() error = %v", err)
	}
	if first == second {
		t.Error("Sync() returned the same graph instance on consecutive calls; must return a fresh graph")
	}

	if result.EntitiesLoaded != 1 {
		t.Errorf("EntitiesLoaded = %d, want 1", result.EntitiesLoaded)
	}
	if _, ok := second.GetNode("REQ-001"); !ok {
		t.Error("REQ-001 should exist in returned graph")
	}

	// Mutating the second graph must not affect the first — proves
	// they are not aliases.
	second.AddNode(testutil.NewEntity("AFTER-001", "after").Build())
	if _, ok := first.GetNode("AFTER-001"); ok {
		t.Error("mutation to second graph leaked into first; Sync must return independent graphs")
	}
}

// --- Metamodel ---

func TestRepository_LoadMetamodel(t *testing.T) {
	repo, _, _ := setupTestRepo(t)

	meta, _, err := repo.LoadMetamodel()
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
	repo, meta, _ := setupTestRepo(t)

	got := repo.EntityFilePath("requirement", "REQ-001", meta)
	want := "/project/entities/requirements/REQ-001.md"
	if got != want {
		t.Errorf("EntityFilePath() = %q, want %q", got, want)
	}
}

func TestRepository_EntityFilePath_UnknownType(t *testing.T) {
	repo, meta, _ := setupTestRepo(t)

	got := repo.EntityFilePath("nonexistent", "FOO-001", meta)
	if got != "" {
		t.Errorf("EntityFilePath() = %q, want empty string", got)
	}
}

func TestRepository_EntityTypeDir(t *testing.T) {
	repo, meta, _ := setupTestRepo(t)

	got := repo.EntityTypeDir("requirement", meta)
	want := "/project/entities/requirements"
	if got != want {
		t.Errorf("EntityTypeDir() = %q, want %q", got, want)
	}
}

func TestRepository_EntityTypeDir_UnknownType(t *testing.T) {
	repo, meta, _ := setupTestRepo(t)

	got := repo.EntityTypeDir("nonexistent", meta)
	if got != "" {
		t.Errorf("EntityTypeDir() = %q, want empty string", got)
	}
}

// --- Accessors ---

func TestRepository_Paths(t *testing.T) {
	repo, _, _ := setupTestRepo(t)

	if repo.Paths() == nil {
		t.Error("Paths() should not be nil")
	}
	if repo.Paths().Root != "/project" {
		t.Errorf("Paths().Root = %q, want %q", repo.Paths().Root, "/project")
	}
}

// --- Entity with Content ---

func TestRepository_EntityWithContent(t *testing.T) {
	repo, meta, _ := setupTestRepo(t)

	entity := testutil.NewEntity("REQ-010", "requirement").
		With("title", "With Content").
		WithContent("# Description\n\nThis is the body.\n").
		Build()

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
	repo, _, _ := setupTestRepo(t)

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
	repo, _, fs := setupTestRepo(t)

	// Write a template file
	tmplContent := "---\nstatus: draft\n---\n\n# Description\n\nTODO\n"
	tmplPath := "/project/templates/entities/requirement.md"
	if err := fs.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
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
	repo, meta, _ := setupTestRepo(t)

	req := testutil.NewEntity("REQ-001", "requirement").With("title", "A Requirement").Build()
	dec := testutil.NewEntity("DEC-001", "decision").With("title", "A Decision").Build()

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
	repo, meta, _ := setupTestRepo(t)

	entity := testutil.NewEntity("REQ-001", "requirement").With("title", "Original").Build()
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
