package workspace

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/rename"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// renameTestMetamodelYAML uses the shared rename test metamodel.
var renameTestMetamodelYAML = testutil.RenameTestMetamodelYAML()

// renameTestEnv bundles everything a rename test typically needs. The
// struct return shape avoids the awkward five-value tuple unpack and
// the dogsled lint that goes with it.
type renameTestEnv struct {
	ws   *Workspace
	repo *repository.Repository
	meta *metamodel.Metamodel
	g    *graph.Graph
	fs   storage.FS
}

// setupRenameTestEnv builds a workspace + repo + graph + in-memory fs
// for rename tests. It mirrors the setupTestEnv that previously lived
// in internal/rename/rename_test.go, but constructs a real Workspace so
// the tests can call ws.Rename instead of the deleted free function.
func setupRenameTestEnv(t *testing.T) renameTestEnv {
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
	_ = fs.MkdirAll(ctx.RelationsDir, 0o755)
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	_ = fs.WriteFile(ctx.MetamodelPath, []byte(renameTestMetamodelYAML), 0o644)

	meta, err := metamodel.Parse([]byte(renameTestMetamodelYAML))
	if err != nil {
		t.Fatalf("failed to parse test metamodel: %v", err)
	}

	repo := repository.New(fs, ctx)
	g := graph.New()
	ws := NewWithGraph(repo, meta, g)

	return renameTestEnv{ws: ws, repo: repo, meta: meta, g: g, fs: fs}
}

func TestRename_NoRelations(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws, repo, meta, g := env.ws, env.repo, env.meta, env.g

	oldID := "REQ-001"
	newID := "REQ-100"
	entity := testutil.NewEntity(oldID, "requirement").With("title", "Test Requirement").Build()
	if err := repo.WriteEntity(entity, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}
	g.AddNode(entity)

	result, err := ws.Rename("requirement", oldID, newID, rename.Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	if result.OldID != oldID {
		t.Errorf("OldID = %q, want %q", result.OldID, oldID)
	}
	if result.NewID != newID {
		t.Errorf("NewID = %q, want %q", result.NewID, newID)
	}
	if len(result.RelationsUpdated) != 0 {
		t.Errorf("RelationsUpdated = %d, want 0", len(result.RelationsUpdated))
	}

	if _, ok := g.GetNode(oldID); ok {
		t.Error("old ID should not exist in graph")
	}
	if _, ok := g.GetNode(newID); !ok {
		t.Error("new ID should exist in graph")
	}

	if _, err := repo.ReadEntity("requirement", newID, meta); err != nil {
		t.Errorf("new entity file should exist: %v", err)
	}
	if _, err := repo.ReadEntity("requirement", oldID, meta); err == nil {
		t.Error("old entity file should not exist")
	}
}

func TestRename_OutgoingRelations(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws, repo, meta, g := env.ws, env.repo, env.meta, env.g

	oldDecID := "DEC-001"
	newDecID := "DEC-100"
	req := testutil.NewEntity("REQ-001", "requirement").With("title", "Requirement").Build()
	dec := testutil.NewEntity(oldDecID, "decision").With("title", "Decision").Build()

	if err := repo.WriteEntity(req, meta); err != nil {
		t.Fatalf("WriteEntity(req) error = %v", err)
	}
	if err := repo.WriteEntity(dec, meta); err != nil {
		t.Fatalf("WriteEntity(dec) error = %v", err)
	}
	g.AddNode(req)
	g.AddNode(dec)

	rel := testutil.NewRelation(dec.ID, "addresses", req.ID).Build()
	if err := repo.WriteRelation(rel); err != nil {
		t.Fatalf("WriteRelation() error = %v", err)
	}
	g.AddEdge(rel)

	result, err := ws.Rename("decision", oldDecID, newDecID, rename.Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	if len(result.RelationsUpdated) != 1 {
		t.Errorf("RelationsUpdated = %d, want 1", len(result.RelationsUpdated))
	}

	outgoing := g.OutgoingEdges(newDecID)
	if len(outgoing) != 1 {
		t.Fatalf("new entity should have 1 outgoing edge, got %d", len(outgoing))
	}
	if outgoing[0].From != newDecID || outgoing[0].To != req.ID {
		t.Errorf("outgoing edge = %s -> %s, want %s -> %s", outgoing[0].From, outgoing[0].To, newDecID, req.ID)
	}

	if _, err := repo.ReadRelation(newDecID, "addresses", req.ID); err != nil {
		t.Errorf("new relation file should exist: %v", err)
	}
	if _, err := repo.ReadRelation(oldDecID, "addresses", req.ID); err == nil {
		t.Error("old relation file should not exist")
	}
}

func TestRename_IncomingRelations(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws, repo, meta, g := env.ws, env.repo, env.meta, env.g

	oldReqID := "REQ-001"
	newReqID := "REQ-100"
	req := testutil.NewEntity(oldReqID, "requirement").With("title", "Requirement").Build()
	dec := testutil.NewEntity("DEC-001", "decision").With("title", "Decision").Build()

	if err := repo.WriteEntity(req, meta); err != nil {
		t.Fatalf("WriteEntity(req) error = %v", err)
	}
	if err := repo.WriteEntity(dec, meta); err != nil {
		t.Fatalf("WriteEntity(dec) error = %v", err)
	}
	g.AddNode(req)
	g.AddNode(dec)

	rel := testutil.NewRelation(dec.ID, "addresses", req.ID).Build()
	if err := repo.WriteRelation(rel); err != nil {
		t.Fatalf("WriteRelation() error = %v", err)
	}
	g.AddEdge(rel)

	result, err := ws.Rename("requirement", oldReqID, newReqID, rename.Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	if len(result.RelationsUpdated) != 1 {
		t.Errorf("RelationsUpdated = %d, want 1", len(result.RelationsUpdated))
	}

	incoming := g.IncomingEdges(newReqID)
	if len(incoming) != 1 {
		t.Fatalf("new entity should have 1 incoming edge, got %d", len(incoming))
	}
	if incoming[0].From != dec.ID || incoming[0].To != newReqID {
		t.Errorf("incoming edge = %s -> %s, want %s -> %s", incoming[0].From, incoming[0].To, dec.ID, newReqID)
	}

	if _, err := repo.ReadRelation(dec.ID, "addresses", newReqID); err != nil {
		t.Errorf("new relation file should exist: %v", err)
	}
	if _, err := repo.ReadRelation(dec.ID, "addresses", oldReqID); err == nil {
		t.Error("old relation file should not exist")
	}
}

func TestRename_BothIncomingAndOutgoing(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws, repo, meta, g := env.ws, env.repo, env.meta, env.g

	oldID := "REQ-001"
	newID := "REQ-100"
	req2ID := "REQ-002"
	req3ID := "REQ-003"

	for _, id := range []string{oldID, req2ID, req3ID} {
		e := testutil.NewEntity(id, "requirement").With("title", "Requirement "+id).Build()
		if err := repo.WriteEntity(e, meta); err != nil {
			t.Fatalf("WriteEntity(%s) error = %v", id, err)
		}
		g.AddNode(e)
	}

	rel1 := testutil.NewRelation(oldID, "depends-on", req2ID).Build()
	if err := repo.WriteRelation(rel1); err != nil {
		t.Fatalf("WriteRelation(rel1) error = %v", err)
	}
	g.AddEdge(rel1)

	rel2 := testutil.NewRelation(req3ID, "depends-on", oldID).Build()
	if err := repo.WriteRelation(rel2); err != nil {
		t.Fatalf("WriteRelation(rel2) error = %v", err)
	}
	g.AddEdge(rel2)

	result, err := ws.Rename("requirement", oldID, newID, rename.Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	if len(result.RelationsUpdated) != 2 {
		t.Errorf("RelationsUpdated = %d, want 2", len(result.RelationsUpdated))
	}

	outgoing := g.OutgoingEdges(newID)
	if len(outgoing) != 1 || outgoing[0].To != req2ID {
		t.Errorf("%s should have outgoing edge to %s", newID, req2ID)
	}

	incoming := g.IncomingEdges(newID)
	if len(incoming) != 1 || incoming[0].From != req3ID {
		t.Errorf("%s should have incoming edge from %s", newID, req3ID)
	}
}

func TestRename_SelfReferential(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws, repo, meta, g := env.ws, env.repo, env.meta, env.g

	oldID := "REQ-001"
	newID := "REQ-100"
	req := testutil.NewEntity(oldID, "requirement").With("title", "Self-referential").Build()
	if err := repo.WriteEntity(req, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}
	g.AddNode(req)

	rel := testutil.NewRelation(req.ID, "depends-on", req.ID).Build()
	if err := repo.WriteRelation(rel); err != nil {
		t.Fatalf("WriteRelation() error = %v", err)
	}
	g.AddEdge(rel)

	result, err := ws.Rename("requirement", oldID, newID, rename.Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	if len(result.RelationsUpdated) != 2 {
		t.Errorf("RelationsUpdated = %d, want 2", len(result.RelationsUpdated))
	}

	outgoing := g.OutgoingEdges(newID)
	if len(outgoing) != 1 {
		t.Fatalf("should have 1 outgoing edge, got %d", len(outgoing))
	}
	if outgoing[0].From != newID || outgoing[0].To != newID {
		t.Errorf("edge = %s -> %s, want %s -> %s", outgoing[0].From, outgoing[0].To, newID, newID)
	}
}

func TestRename_DryRun(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws, repo, meta, g := env.ws, env.repo, env.meta, env.g

	oldReqID := "REQ-001"
	newReqID := "REQ-100"
	req := testutil.NewEntity(oldReqID, "requirement").With("title", "Requirement").Build()
	dec := testutil.NewEntity("DEC-001", "decision").With("title", "Decision").Build()

	if err := repo.WriteEntity(req, meta); err != nil {
		t.Fatalf("WriteEntity(req) error = %v", err)
	}
	if err := repo.WriteEntity(dec, meta); err != nil {
		t.Fatalf("WriteEntity(dec) error = %v", err)
	}
	g.AddNode(req)
	g.AddNode(dec)

	rel := testutil.NewRelation(dec.ID, "addresses", req.ID).Build()
	if err := repo.WriteRelation(rel); err != nil {
		t.Fatalf("WriteRelation() error = %v", err)
	}
	g.AddEdge(rel)

	result, err := ws.Rename("requirement", oldReqID, newReqID, rename.Options{DryRun: true})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	if result.NewID != newReqID {
		t.Errorf("NewID = %q, want %q", result.NewID, newReqID)
	}
	if len(result.RelationsUpdated) != 1 {
		t.Errorf("RelationsUpdated = %d, want 1", len(result.RelationsUpdated))
	}

	if _, ok := g.GetNode(oldReqID); !ok {
		t.Error("old ID should still exist in graph (dry run)")
	}
	if _, ok := g.GetNode(newReqID); ok {
		t.Error("new ID should not exist in graph (dry run)")
	}

	if _, readErr := repo.ReadEntity("requirement", oldReqID, meta); readErr != nil {
		t.Error("old entity file should still exist (dry run)")
	}
}

func TestRename_ErrorNewIDExists(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws, repo, meta, g := env.ws, env.repo, env.meta, env.g

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

	_, err := ws.Rename("requirement", req1.ID, req2.ID, rename.Options{})
	if err == nil {
		t.Fatal("Rename() should fail when new ID already exists")
	}
	expectedErr := "entity with ID " + req2.ID + " already exists"
	if err.Error() != expectedErr {
		t.Errorf("error = %q, want %q", err.Error(), expectedErr)
	}
}

func TestRename_ErrorOldIDNotFound(t *testing.T) {
	ws := setupRenameTestEnv(t).ws

	_, err := ws.Rename("requirement", "REQ-999", "REQ-100", rename.Options{})
	if err == nil {
		t.Fatal("Rename() should fail when old ID doesn't exist")
	}
}

func TestRename_ErrorInvalidNewID(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws, repo, meta, g := env.ws, env.repo, env.meta, env.g

	req := testutil.NewEntity("REQ-001", "requirement").With("title", "Test").Build()
	if err := repo.WriteEntity(req, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}
	g.AddNode(req)

	_, err := ws.Rename("requirement", "REQ-001", "../evil", rename.Options{})
	if err == nil {
		t.Fatal("Rename() should fail for invalid new ID")
	}
}

func TestRename_ErrorTypeMismatch(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws, repo, meta, g := env.ws, env.repo, env.meta, env.g

	req := testutil.NewEntity("REQ-001", "requirement").With("title", "Test").Build()
	if err := repo.WriteEntity(req, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}
	g.AddNode(req)

	_, err := ws.Rename("decision", "REQ-001", "REQ-100", rename.Options{})
	if err == nil {
		t.Fatal("Rename() should fail when entity type doesn't match")
	}
}

func TestRename_PreservesContent(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws, repo, meta, g := env.ws, env.repo, env.meta, env.g

	oldID := "REQ-001"
	newID := "REQ-100"
	expectedTitle := "With Content"
	expectedStatus := "approved"
	req := testutil.NewEntity(oldID, "requirement").
		With("title", expectedTitle).
		With("status", expectedStatus).
		WithContent("# Description\n\nThis is the detailed description.\n").
		Build()
	if err := repo.WriteEntity(req, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}
	g.AddNode(req)

	_, err := ws.Rename("requirement", oldID, newID, rename.Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	newEntity, err := repo.ReadEntity("requirement", newID, meta)
	if err != nil {
		t.Fatalf("ReadEntity() error = %v", err)
	}

	if newEntity.GetString("title") != expectedTitle {
		t.Errorf("title = %q, want %q", newEntity.GetString("title"), expectedTitle)
	}
	if newEntity.GetString("status") != expectedStatus {
		t.Errorf("status = %q, want %q", newEntity.GetString("status"), expectedStatus)
	}
	if newEntity.Content == "" {
		t.Error("content should be preserved")
	}
}

func TestRename_NoTempFilesLeft(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws, repo, meta, g, fs := env.ws, env.repo, env.meta, env.g, env.fs

	oldID := "REQ-001"
	newID := "REQ-100"
	req := testutil.NewEntity(oldID, "requirement").With("title", "Test").Build()
	if err := repo.WriteEntity(req, meta); err != nil {
		t.Fatalf("WriteEntity() error = %v", err)
	}
	g.AddNode(req)

	_, err := ws.Rename("requirement", oldID, newID, rename.Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	entries, _ := fs.ReadDir("/project/entities/requirements")
	for _, entry := range entries {
		if name := entry.Name(); len(name) > 4 && name[len(name)-4:] == ".new" {
			t.Errorf("temp file should not exist: %s", name)
		}
	}
}
