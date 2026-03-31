package rename

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// testMetamodelYAML uses the shared rename test metamodel
var testMetamodelYAML = testutil.RenameTestMetamodelYAML()

func setupTestEnv(t *testing.T) (*repository.Repository, *metamodel.Metamodel, *graph.Graph, storage.FS) {
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

	// Write metamodel file
	_ = fs.WriteFile(ctx.MetamodelPath, []byte(testMetamodelYAML), 0o644)

	meta, err := metamodel.Parse([]byte(testMetamodelYAML))
	if err != nil {
		t.Fatalf("failed to parse test metamodel: %v", err)
	}

	repo := repository.New(fs, ctx)
	g := graph.New()

	return repo, meta, g, fs
}

func TestRename_NoRelations(t *testing.T) {
	repo, meta, g, _ := setupTestEnv(t)

	// Create entity
	entity := testutil.NewEntity("REQ-001", "requirement").With("title", "Test Requirement").Build()
	if err := repo.WriteEntity(entity, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}
	g.AddNode(entity)

	// Rename
	result, err := Rename(repo, meta, g, "requirement", "REQ-001", "REQ-100", Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	// Check result
	if result.OldID != "REQ-001" {
		t.Errorf("OldID = %q, want %q", result.OldID, "REQ-001")
	}
	if result.NewID != "REQ-100" {
		t.Errorf("NewID = %q, want %q", result.NewID, "REQ-100")
	}
	if len(result.RelationsUpdated) != 0 {
		t.Errorf("RelationsUpdated = %d, want 0", len(result.RelationsUpdated))
	}

	// Verify graph updated
	if _, ok := g.GetNode("REQ-001"); ok {
		t.Error("old ID should not exist in graph")
	}
	if _, ok := g.GetNode("REQ-100"); !ok {
		t.Error("new ID should exist in graph")
	}

	// Verify files
	if _, err := repo.ReadEntity("requirement", "REQ-100", meta); err != nil {
		t.Errorf("new entity file should exist: %v", err)
	}
	if _, err := repo.ReadEntity("requirement", "REQ-001", meta); err == nil {
		t.Error("old entity file should not exist")
	}
}

func TestRename_OutgoingRelations(t *testing.T) {
	repo, meta, g, _ := setupTestEnv(t)

	// Create entities
	req := testutil.NewEntity("REQ-001", "requirement").With("title", "Requirement").Build()
	dec := testutil.NewEntity("DEC-001", "decision").With("title", "Decision").Build()

	if err := repo.WriteEntity(req, meta); err != nil {
		t.Fatalf("WriteEntity(req) error = %v", err)
	}
	if err := repo.WriteEntity(dec, meta); err != nil {
		t.Fatalf("WriteEntity(dec) error = %v", err)
	}
	g.AddNode(req)
	g.AddNode(dec)

	// Create outgoing relation from DEC-001
	rel := testutil.NewRelation("DEC-001", "addresses", "REQ-001").Build()
	if err := repo.WriteRelation(rel); err != nil {
		t.Fatalf("WriteRelation() error = %v", err)
	}
	g.AddEdge(rel)

	// Rename DEC-001 -> DEC-100 (entity with outgoing relation)
	result, err := Rename(repo, meta, g, "decision", "DEC-001", "DEC-100", Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	// Check relations updated
	if len(result.RelationsUpdated) != 1 {
		t.Errorf("RelationsUpdated = %d, want 1", len(result.RelationsUpdated))
	}

	// Verify graph
	outgoing := g.OutgoingEdges("DEC-100")
	if len(outgoing) != 1 {
		t.Fatalf("new entity should have 1 outgoing edge, got %d", len(outgoing))
	}
	if outgoing[0].From != "DEC-100" || outgoing[0].To != "REQ-001" {
		t.Errorf("outgoing edge = %s -> %s, want DEC-100 -> REQ-001", outgoing[0].From, outgoing[0].To)
	}

	// Verify files
	if _, err := repo.ReadRelation("DEC-100", "addresses", "REQ-001"); err != nil {
		t.Errorf("new relation file should exist: %v", err)
	}
	if _, err := repo.ReadRelation("DEC-001", "addresses", "REQ-001"); err == nil {
		t.Error("old relation file should not exist")
	}
}

func TestRename_IncomingRelations(t *testing.T) {
	repo, meta, g, _ := setupTestEnv(t)

	// Create entities
	req := testutil.NewEntity("REQ-001", "requirement").With("title", "Requirement").Build()
	dec := testutil.NewEntity("DEC-001", "decision").With("title", "Decision").Build()

	if err := repo.WriteEntity(req, meta); err != nil {
		t.Fatalf("WriteEntity(req) error = %v", err)
	}
	if err := repo.WriteEntity(dec, meta); err != nil {
		t.Fatalf("WriteEntity(dec) error = %v", err)
	}
	g.AddNode(req)
	g.AddNode(dec)

	// Create relation to REQ-001
	rel := testutil.NewRelation("DEC-001", "addresses", "REQ-001").Build()
	if err := repo.WriteRelation(rel); err != nil {
		t.Fatalf("WriteRelation() error = %v", err)
	}
	g.AddEdge(rel)

	// Rename REQ-001 -> REQ-100 (entity with incoming relation)
	result, err := Rename(repo, meta, g, "requirement", "REQ-001", "REQ-100", Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	// Check relations updated
	if len(result.RelationsUpdated) != 1 {
		t.Errorf("RelationsUpdated = %d, want 1", len(result.RelationsUpdated))
	}

	// Verify graph
	incoming := g.IncomingEdges("REQ-100")
	if len(incoming) != 1 {
		t.Fatalf("new entity should have 1 incoming edge, got %d", len(incoming))
	}
	if incoming[0].From != "DEC-001" || incoming[0].To != "REQ-100" {
		t.Errorf("incoming edge = %s -> %s, want DEC-001 -> REQ-100", incoming[0].From, incoming[0].To)
	}

	// Verify files
	if _, err := repo.ReadRelation("DEC-001", "addresses", "REQ-100"); err != nil {
		t.Errorf("new relation file should exist: %v", err)
	}
	if _, err := repo.ReadRelation("DEC-001", "addresses", "REQ-001"); err == nil {
		t.Error("old relation file should not exist")
	}
}

func TestRename_BothIncomingAndOutgoing(t *testing.T) {
	repo, meta, g, _ := setupTestEnv(t)

	// Create entities: REQ-001, REQ-002, REQ-003
	for _, id := range []string{"REQ-001", "REQ-002", "REQ-003"} {
		e := testutil.NewEntity(id, "requirement").With("title", "Requirement "+id).Build()
		if err := repo.WriteEntity(e, meta); err != nil {
			t.Fatalf("WriteEntity(%s) error = %v", id, err)
		}
		g.AddNode(e)
	}

	// REQ-001 depends-on REQ-002 (outgoing from REQ-001)
	rel1 := testutil.NewRelation("REQ-001", "depends-on", "REQ-002").Build()
	if err := repo.WriteRelation(rel1); err != nil {
		t.Fatalf("WriteRelation(rel1) error = %v", err)
	}
	g.AddEdge(rel1)

	// REQ-003 depends-on REQ-001 (incoming to REQ-001)
	rel2 := testutil.NewRelation("REQ-003", "depends-on", "REQ-001").Build()
	if err := repo.WriteRelation(rel2); err != nil {
		t.Fatalf("WriteRelation(rel2) error = %v", err)
	}
	g.AddEdge(rel2)

	// Rename REQ-001 -> REQ-100
	result, err := Rename(repo, meta, g, "requirement", "REQ-001", "REQ-100", Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	// Should have 2 relations updated
	if len(result.RelationsUpdated) != 2 {
		t.Errorf("RelationsUpdated = %d, want 2", len(result.RelationsUpdated))
	}

	// Verify graph edges
	outgoing := g.OutgoingEdges("REQ-100")
	if len(outgoing) != 1 || outgoing[0].To != "REQ-002" {
		t.Error("REQ-100 should have outgoing edge to REQ-002")
	}

	incoming := g.IncomingEdges("REQ-100")
	if len(incoming) != 1 || incoming[0].From != "REQ-003" {
		t.Error("REQ-100 should have incoming edge from REQ-003")
	}
}

func TestRename_SelfReferential(t *testing.T) {
	repo, meta, g, _ := setupTestEnv(t)

	// Create entity
	req := testutil.NewEntity("REQ-001", "requirement").With("title", "Self-referential").Build()
	if err := repo.WriteEntity(req, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}
	g.AddNode(req)

	// Create self-referential relation
	rel := testutil.NewRelation("REQ-001", "depends-on", "REQ-001").Build()
	if err := repo.WriteRelation(rel); err != nil {
		t.Fatalf("WriteRelation() error = %v", err)
	}
	g.AddEdge(rel)

	// Rename REQ-001 -> REQ-100
	result, err := Rename(repo, meta, g, "requirement", "REQ-001", "REQ-100", Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	// Self-referential counts as 2 (incoming + outgoing)
	if len(result.RelationsUpdated) != 2 {
		t.Errorf("RelationsUpdated = %d, want 2", len(result.RelationsUpdated))
	}

	// Verify graph - self-referential edge should now be REQ-100 -> REQ-100
	outgoing := g.OutgoingEdges("REQ-100")
	if len(outgoing) != 1 {
		t.Fatalf("should have 1 outgoing edge, got %d", len(outgoing))
	}
	if outgoing[0].From != "REQ-100" || outgoing[0].To != "REQ-100" {
		t.Errorf("edge = %s -> %s, want REQ-100 -> REQ-100", outgoing[0].From, outgoing[0].To)
	}
}

func TestRename_DryRun(t *testing.T) {
	repo, meta, g, _ := setupTestEnv(t)

	// Create entity with relation
	req := testutil.NewEntity("REQ-001", "requirement").With("title", "Requirement").Build()
	dec := testutil.NewEntity("DEC-001", "decision").With("title", "Decision").Build()

	if err := repo.WriteEntity(req, meta); err != nil {
		t.Fatalf("WriteEntity(req) error = %v", err)
	}
	if err := repo.WriteEntity(dec, meta); err != nil {
		t.Fatalf("WriteEntity(dec) error = %v", err)
	}
	g.AddNode(req)
	g.AddNode(dec)

	rel := testutil.NewRelation("DEC-001", "addresses", "REQ-001").Build()
	if err := repo.WriteRelation(rel); err != nil {
		t.Fatalf("WriteRelation() error = %v", err)
	}
	g.AddEdge(rel)

	// Dry run
	result, err := Rename(repo, meta, g, "requirement", "REQ-001", "REQ-100", Options{DryRun: true})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	// Result should show what would change
	if result.NewID != "REQ-100" {
		t.Errorf("NewID = %q, want %q", result.NewID, "REQ-100")
	}
	if len(result.RelationsUpdated) != 1 {
		t.Errorf("RelationsUpdated = %d, want 1", len(result.RelationsUpdated))
	}

	// But nothing should have changed
	if _, ok := g.GetNode("REQ-001"); !ok {
		t.Error("old ID should still exist in graph (dry run)")
	}
	if _, ok := g.GetNode("REQ-100"); ok {
		t.Error("new ID should not exist in graph (dry run)")
	}

	// Files unchanged
	if _, readErr := repo.ReadEntity("requirement", "REQ-001", meta); readErr != nil {
		t.Error("old entity file should still exist (dry run)")
	}
}

func TestRename_ErrorNewIDExists(t *testing.T) {
	repo, meta, g, _ := setupTestEnv(t)

	// Create two entities
	req1 := testutil.NewEntity("REQ-001", "requirement").With("title", "First").Build()
	req2 := testutil.NewEntity("REQ-002", "requirement").With("title", "Second").Build()

	if err := repo.WriteEntity(req1, meta); err != nil {
		t.Fatalf("WriteEntity(req1) error = %v", err)
	}
	if err := repo.WriteEntity(req2, meta); err != nil {
		t.Fatalf("WriteEntity(req2) error = %v", err)
	}
	g.AddNode(req1)
	g.AddNode(req2)

	// Try to rename REQ-001 to REQ-002 (already exists)
	_, err := Rename(repo, meta, g, "requirement", "REQ-001", "REQ-002", Options{})
	if err == nil {
		t.Fatal("Rename() should fail when new ID already exists")
	}
	if err.Error() != "entity with ID REQ-002 already exists" {
		t.Errorf("error = %q, want 'entity with ID REQ-002 already exists'", err.Error())
	}
}

func TestRename_ErrorOldIDNotFound(t *testing.T) {
	repo, meta, g, _ := setupTestEnv(t)

	// Try to rename non-existent entity
	_, err := Rename(repo, meta, g, "requirement", "REQ-999", "REQ-100", Options{})
	if err == nil {
		t.Fatal("Rename() should fail when old ID doesn't exist")
	}
}

func TestRename_ErrorInvalidNewID(t *testing.T) {
	repo, meta, g, _ := setupTestEnv(t)

	// Create entity
	req := testutil.NewEntity("REQ-001", "requirement").With("title", "Test").Build()
	if err := repo.WriteEntity(req, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}
	g.AddNode(req)

	// Try to rename with invalid ID (path traversal)
	_, err := Rename(repo, meta, g, "requirement", "REQ-001", "../evil", Options{})
	if err == nil {
		t.Fatal("Rename() should fail for invalid new ID")
	}
}

func TestRename_ErrorTypeMismatch(t *testing.T) {
	repo, meta, g, _ := setupTestEnv(t)

	// Create entity
	req := testutil.NewEntity("REQ-001", "requirement").With("title", "Test").Build()
	if err := repo.WriteEntity(req, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}
	g.AddNode(req)

	// Try to rename with wrong type
	_, err := Rename(repo, meta, g, "decision", "REQ-001", "REQ-100", Options{})
	if err == nil {
		t.Fatal("Rename() should fail when entity type doesn't match")
	}
}

func TestRename_PreservesContent(t *testing.T) {
	repo, meta, g, _ := setupTestEnv(t)

	// Create entity with content
	req := testutil.NewEntity("REQ-001", "requirement").
		With("title", "With Content").
		With("status", "approved").
		WithContent("# Description\n\nThis is the detailed description.\n").
		Build()
	if err := repo.WriteEntity(req, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}
	g.AddNode(req)

	// Rename
	_, err := Rename(repo, meta, g, "requirement", "REQ-001", "REQ-100", Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	// Read back and verify content preserved
	newEntity, err := repo.ReadEntity("requirement", "REQ-100", meta)
	if err != nil {
		t.Fatalf("ReadEntity() error = %v", err)
	}

	if newEntity.GetString("title") != "With Content" {
		t.Errorf("title = %q, want %q", newEntity.GetString("title"), "With Content")
	}
	if newEntity.GetString("status") != "approved" {
		t.Errorf("status = %q, want %q", newEntity.GetString("status"), "approved")
	}
	if newEntity.Content == "" {
		t.Error("content should be preserved")
	}
}

func TestRename_NoTempFilesLeft(t *testing.T) {
	repo, meta, g, fs := setupTestEnv(t)

	// Create entity
	req := testutil.NewEntity("REQ-001", "requirement").With("title", "Test").Build()
	if err := repo.WriteEntity(req, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}
	g.AddNode(req)

	// Rename
	_, err := Rename(repo, meta, g, "requirement", "REQ-001", "REQ-100", Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	// No .new temp files should be left behind
	entries, _ := fs.ReadDir("/project/entities/requirements")
	for _, entry := range entries {
		if name := entry.Name(); len(name) > 4 && name[len(name)-4:] == ".new" {
			t.Errorf("temp file should not exist: %s", name)
		}
	}
}
