package workspace

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/rename"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// renameTestMetamodelYAML uses the shared rename test metamodel.
var renameTestMetamodelYAML = testutil.RenameTestMetamodelYAML()

// renameTestEnv bundles everything a rename test typically needs. The
// struct return shape avoids the awkward tuple unpack and the dogsled
// lint that goes with it.
type renameTestEnv struct {
	ws   *Workspace
	meta *metamodel.Metamodel
	fs   storage.FS
	ctx  *project.Context
}

// addEntity persists an entity into the workspace (the fsstore writes
// it to disk).
func (e renameTestEnv) addEntity(t *testing.T, ent *entity.Entity) {
	t.Helper()
	e.ws.SeedEntityForTest(ent)
}

// addRelation persists a relation into the workspace (the fsstore
// writes it to disk).
func (e renameTestEnv) addRelation(t *testing.T, rel *entity.Relation) {
	t.Helper()
	e.ws.SeedRelationForTest(rel)
}

// entityExists reports whether an entity file exists on disk.
func (e renameTestEnv) entityExists(plural, id string) bool {
	path := e.ctx.EntitiesDir + "/" + plural + "/" + id + ".md"
	_, err := e.fs.Stat(path)
	return err == nil
}

// relationExists reports whether a relation file exists on disk.
func (e renameTestEnv) relationExists(from, to string) bool {
	path := e.ctx.RelationsDir + "/" + from + "--addresses--" + to + ".md"
	_, err := e.fs.Stat(path)
	return err == nil
}

// setupRenameTestEnv builds a workspace + repo + in-memory fs for
// rename tests. It mirrors the setupTestEnv that previously lived in
// internal/rename/rename_test.go, but constructs a real Workspace so
// the tests can call ws.Rename instead of the deleted free function.
func setupRenameTestEnv(t *testing.T) renameTestEnv {
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

	_ = fs.MkdirAll(ctx.EntitiesDir+"/requirements", 0o755)
	_ = fs.MkdirAll(ctx.EntitiesDir+"/decisions", 0o755)
	_ = fs.MkdirAll(ctx.RelationsDir, 0o755)
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	_ = fs.WriteFile(ctx.MetamodelPath, []byte(renameTestMetamodelYAML), 0o644)

	meta, err := metamodel.Parse([]byte(renameTestMetamodelYAML))
	if err != nil {
		t.Fatalf("failed to parse test metamodel: %v", err)
	}

	ws, err := New(fs, ctx, NopScriptExecutor)
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	return renameTestEnv{ws: ws, meta: meta, fs: fs, ctx: ctx}
}

func TestRename_NoRelations(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws := env.ws

	oldID := "REQ-001"
	newID := "REQ-100"
	entity := testutil.NewEntity(oldID, "requirement").With("title", "Test Requirement").Build()
	env.addEntity(t, entity)

	result, err := ws.rename("requirement", oldID, newID, rename.Options{})
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

	if _, ok := ws.lookupEntity(oldID); ok {
		t.Error("old ID should not exist in store")
	}
	if _, ok := ws.lookupEntity(newID); !ok {
		t.Error("new ID should exist in store")
	}

	if !env.entityExists("requirements", newID) {
		t.Errorf("new entity file should exist: %v", err)
	}
	if env.entityExists("requirements", oldID) {
		t.Error("old entity file should not exist")
	}
}

func TestRename_OutgoingRelations(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws := env.ws

	oldDecID := "DEC-001"
	newDecID := "DEC-100"
	req := testutil.NewEntity("REQ-001", "requirement").With("title", "Requirement").Build()
	dec := testutil.NewEntity(oldDecID, "decision").With("title", "Decision").Build()

	env.addEntity(t, req)
	env.addEntity(t, dec)

	rel := testutil.NewRelation(dec.ID, "addresses", req.ID).Build()
	env.addRelation(t, rel)

	result, err := ws.rename("decision", oldDecID, newDecID, rename.Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	if len(result.RelationsUpdated) != 1 {
		t.Errorf("RelationsUpdated = %d, want 1", len(result.RelationsUpdated))
	}

	outgoing := ws.OutgoingRelations(newDecID)
	if len(outgoing) != 1 {
		t.Fatalf("new entity should have 1 outgoing edge, got %d", len(outgoing))
	}
	if outgoing[0].From != newDecID || outgoing[0].To != req.ID {
		t.Errorf("outgoing edge = %s -> %s, want %s -> %s", outgoing[0].From, outgoing[0].To, newDecID, req.ID)
	}

	if !env.relationExists(newDecID, req.ID) {
		t.Errorf("new relation file should exist: %v", err)
	}
	if env.relationExists(oldDecID, req.ID) {
		t.Error("old relation file should not exist")
	}
}

func TestRename_IncomingRelations(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws := env.ws

	oldReqID := "REQ-001"
	newReqID := "REQ-100"
	req := testutil.NewEntity(oldReqID, "requirement").With("title", "Requirement").Build()
	dec := testutil.NewEntity("DEC-001", "decision").With("title", "Decision").Build()

	env.addEntity(t, req)
	env.addEntity(t, dec)

	rel := testutil.NewRelation(dec.ID, "addresses", req.ID).Build()
	env.addRelation(t, rel)

	result, err := ws.rename("requirement", oldReqID, newReqID, rename.Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	if len(result.RelationsUpdated) != 1 {
		t.Errorf("RelationsUpdated = %d, want 1", len(result.RelationsUpdated))
	}

	incoming := ws.IncomingRelations(newReqID)
	if len(incoming) != 1 {
		t.Fatalf("new entity should have 1 incoming edge, got %d", len(incoming))
	}
	if incoming[0].From != dec.ID || incoming[0].To != newReqID {
		t.Errorf("incoming edge = %s -> %s, want %s -> %s", incoming[0].From, incoming[0].To, dec.ID, newReqID)
	}

	if !env.relationExists(dec.ID, newReqID) {
		t.Error("new relation file should exist")
	}
	if env.relationExists(dec.ID, oldReqID) {
		t.Error("old relation file should not exist")
	}
}

func TestRename_BothIncomingAndOutgoing(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws := env.ws

	oldID := "REQ-001"
	newID := "REQ-100"
	req2ID := "REQ-002"
	req3ID := "REQ-003"

	for _, id := range []string{oldID, req2ID, req3ID} {
		e := testutil.NewEntity(id, "requirement").With("title", "Requirement "+id).Build()
		env.addEntity(t, e)
	}

	rel1 := testutil.NewRelation(oldID, "depends-on", req2ID).Build()
	env.addRelation(t, rel1)

	rel2 := testutil.NewRelation(req3ID, "depends-on", oldID).Build()
	env.addRelation(t, rel2)

	result, err := ws.rename("requirement", oldID, newID, rename.Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	if len(result.RelationsUpdated) != 2 {
		t.Errorf("RelationsUpdated = %d, want 2", len(result.RelationsUpdated))
	}

	outgoing := ws.OutgoingRelations(newID)
	if len(outgoing) != 1 || outgoing[0].To != req2ID {
		t.Errorf("%s should have outgoing edge to %s", newID, req2ID)
	}

	incoming := ws.IncomingRelations(newID)
	if len(incoming) != 1 || incoming[0].From != req3ID {
		t.Errorf("%s should have incoming edge from %s", newID, req3ID)
	}
}

func TestRename_SelfReferential(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws := env.ws

	oldID := "REQ-001"
	newID := "REQ-100"
	req := testutil.NewEntity(oldID, "requirement").With("title", "Self-referential").Build()
	env.addEntity(t, req)

	rel := testutil.NewRelation(req.ID, "depends-on", req.ID).Build()
	env.addRelation(t, rel)

	result, err := ws.rename("requirement", oldID, newID, rename.Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	if len(result.RelationsUpdated) != 2 {
		t.Errorf("RelationsUpdated = %d, want 2", len(result.RelationsUpdated))
	}

	outgoing := ws.OutgoingRelations(newID)
	if len(outgoing) != 1 {
		t.Fatalf("should have 1 outgoing edge, got %d", len(outgoing))
	}
	if outgoing[0].From != newID || outgoing[0].To != newID {
		t.Errorf("edge = %s -> %s, want %s -> %s", outgoing[0].From, outgoing[0].To, newID, newID)
	}
}

func TestRename_DryRun(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws := env.ws

	oldReqID := "REQ-001"
	newReqID := "REQ-100"
	req := testutil.NewEntity(oldReqID, "requirement").With("title", "Requirement").Build()
	dec := testutil.NewEntity("DEC-001", "decision").With("title", "Decision").Build()

	env.addEntity(t, req)
	env.addEntity(t, dec)

	rel := testutil.NewRelation(dec.ID, "addresses", req.ID).Build()
	env.addRelation(t, rel)

	result, err := ws.rename("requirement", oldReqID, newReqID, rename.Options{DryRun: true})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	if result.NewID != newReqID {
		t.Errorf("NewID = %q, want %q", result.NewID, newReqID)
	}
	if len(result.RelationsUpdated) != 1 {
		t.Errorf("RelationsUpdated = %d, want 1", len(result.RelationsUpdated))
	}

	if _, ok := ws.lookupEntity(oldReqID); !ok {
		t.Error("old ID should still exist in store (dry run)")
	}
	if _, ok := ws.lookupEntity(newReqID); ok {
		t.Error("new ID should not exist in store (dry run)")
	}

	if !env.entityExists("requirements", oldReqID) {
		t.Error("old entity file should still exist (dry run)")
	}
}

func TestRename_ErrorNewIDExists(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws := env.ws

	req1 := testutil.NewEntity("REQ-001", "requirement").With("title", "First").Build()
	req2 := testutil.NewEntity("REQ-002", "requirement").With("title", "Second").Build()

	env.addEntity(t, req1)
	env.addEntity(t, req2)

	_, err := ws.rename("requirement", req1.ID, req2.ID, rename.Options{})
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

	_, err := ws.rename("requirement", "REQ-999", "REQ-100", rename.Options{})
	if err == nil {
		t.Fatal("Rename() should fail when old ID doesn't exist")
	}
}

func TestRename_ErrorInvalidNewID(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws := env.ws

	req := testutil.NewEntity("REQ-001", "requirement").With("title", "Test").Build()
	env.addEntity(t, req)

	_, err := ws.rename("requirement", "REQ-001", "../evil", rename.Options{})
	if err == nil {
		t.Fatal("Rename() should fail for invalid new ID")
	}
}

func TestRename_ErrorTypeMismatch(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws := env.ws

	req := testutil.NewEntity("REQ-001", "requirement").With("title", "Test").Build()
	env.addEntity(t, req)

	_, err := ws.rename("decision", "REQ-001", "REQ-100", rename.Options{})
	if err == nil {
		t.Fatal("Rename() should fail when entity type doesn't match")
	}
}

func TestRename_PreservesContent(t *testing.T) {
	env := setupRenameTestEnv(t)
	ws := env.ws

	oldID := "REQ-001"
	newID := "REQ-100"
	expectedTitle := "With Content"
	expectedStatus := "approved"
	req := testutil.NewEntity(oldID, "requirement").
		With("title", expectedTitle).
		With("status", expectedStatus).
		WithContent("# Description\n\nThis is the detailed description.\n").
		Build()
	env.addEntity(t, req)

	_, err := ws.rename("requirement", oldID, newID, rename.Options{})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	newEntity, ok := ws.lookupEntity(newID)
	if !ok {
		t.Fatalf("GetEntity(%q) not found in store", newID)
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
	ws, fs := env.ws, env.fs

	oldID := "REQ-001"
	newID := "REQ-100"
	req := testutil.NewEntity(oldID, "requirement").With("title", "Test").Build()
	env.addEntity(t, req)

	_, err := ws.rename("requirement", oldID, newID, rename.Options{})
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
