package workspace

import (
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// TestWithTxCommit verifies that on a successful WithTx callback the
// staged disk writes are committed AND the staged graph mutations are
// applied to the live graph.
func TestWithTxCommit(t *testing.T) {
	ws := setupTestWorkspace(t)

	wantTitle := "tx commit test"
	entity := testutil.EntityFor(ws.Meta(), "requirement").
		ID("REQ-001").
		With("title", wantTitle).
		Build()

	err := ws.WithTx(func(tx *Tx) error {
		return tx.WriteEntity(entity)
	})
	if err != nil {
		t.Fatalf("WithTx returned error: %v", err)
	}

	// Graph mutation applied.
	got, ok := ws.Graph().GetNode("REQ-001")
	if !ok {
		t.Fatal("entity not in graph after successful tx")
	}
	if got.GetString("title") != wantTitle {
		t.Errorf("graph entity title = %q, want %q", got.GetString("title"), wantTitle)
	}

	// On-disk file present (verified by re-reading via the repo).
	read, err := ws.Repo().ReadEntity("requirement", "REQ-001", ws.Meta())
	if err != nil {
		t.Fatalf("ReadEntity failed: %v", err)
	}
	if read == nil {
		t.Fatal("entity not found on disk after successful tx")
	}
}

// TestWithTxRollback verifies that when the WithTx callback returns an
// error, the disk writes are rolled back AND no graph mutations are
// applied.
func TestWithTxRollback(t *testing.T) {
	ws := setupTestWorkspace(t)

	entity := testutil.EntityFor(ws.Meta(), "requirement").
		ID("REQ-002").
		With("title", "rollback test").
		Build()

	wantErr := errors.New("forced rollback")
	err := ws.WithTx(func(tx *Tx) error {
		if writeErr := tx.WriteEntity(entity); writeErr != nil {
			t.Fatalf("WriteEntity failed: %v", writeErr)
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WithTx returned %v, want %v", err, wantErr)
	}

	// Graph must NOT contain the entity.
	if _, ok := ws.Graph().GetNode("REQ-002"); ok {
		t.Error("entity present in graph after rolled-back tx")
	}

	// On-disk file must NOT exist. ReadEntity returns an error or nil
	// when the entity doesn't exist; either is acceptable here.
	read, err := ws.Repo().ReadEntity("requirement", "REQ-002", ws.Meta())
	if err == nil && read != nil {
		t.Error("entity present on disk after rolled-back tx")
	}
}

// TestWithTxMixedOps verifies that a single tx can mix entity writes,
// relation writes, and deletes, all of which apply atomically on commit.
// In particular it asserts that deleting an entity removes its incident
// edges from the graph (a property of graph.RemoveNode that the Tx
// primitive relies on).
func TestWithTxMixedOps(t *testing.T) {
	ws := setupTestWorkspace(t)

	// Seed: REQ-A and REQ-B with an edge REQ-A --depends-on--> REQ-B.
	if _, _, err := ws.createEntity("requirement", CreateOptions{
		ID:         "REQ-A",
		Properties: map[string]any{"title": "to be deleted"},
	}); err != nil {
		t.Fatalf("seed REQ-A: %v", err)
	}
	if _, _, err := ws.createEntity("requirement", CreateOptions{
		ID:         "REQ-B",
		Properties: map[string]any{"title": "survivor"},
	}); err != nil {
		t.Fatalf("seed REQ-B: %v", err)
	}
	if _, err := ws.createRelation("REQ-A", "depends-on", "REQ-B"); err != nil {
		t.Fatalf("seed relation: %v", err)
	}

	// In one tx: create REQ-C, create REQ-C --depends-on--> REQ-B,
	// delete REQ-A (which has an outgoing edge to REQ-B).
	// Expected: REQ-A gone, REQ-A's edge gone, REQ-B and REQ-C present,
	// REQ-C --depends-on--> REQ-B present.
	thirdEntity := testutil.EntityFor(ws.Meta(), "requirement").
		ID("REQ-C").
		With("title", "third").
		Build()
	newRel := model.NewRelation("REQ-C", "depends-on", "REQ-B")

	err := ws.WithTx(func(tx *Tx) error {
		if err := tx.WriteEntity(thirdEntity); err != nil {
			return err
		}
		if err := tx.WriteRelation(newRel); err != nil {
			return err
		}
		// Tx.DeleteEntity stages the disk delete and a node-removal
		// graph op. graph.RemoveNode will also clean up incident
		// edges as a side effect — verified below.
		return tx.DeleteEntity("requirement", "REQ-A")
	})
	if err != nil {
		t.Fatalf("WithTx returned error: %v", err)
	}

	g := ws.Graph()
	if _, ok := g.GetNode("REQ-A"); ok {
		t.Error("REQ-A should be deleted")
	}
	if _, ok := g.GetEdge("REQ-A", "depends-on", "REQ-B"); ok {
		t.Error("REQ-A's outgoing edge should have been removed by RemoveNode")
	}
	if _, ok := g.GetNode("REQ-B"); !ok {
		t.Error("REQ-B should still exist")
	}
	if _, ok := g.GetNode("REQ-C"); !ok {
		t.Error("REQ-C should be created")
	}
	if _, ok := g.GetEdge("REQ-C", "depends-on", "REQ-B"); !ok {
		t.Error("relation REQ-C --depends-on--> REQ-B should be created")
	}
}

// TestWithTxRollbackMultiOp verifies that a tx that stages multiple
// operations and then errors leaves NONE of them applied. This is
// the case the rename migration depends on.
func TestWithTxRollbackMultiOp(t *testing.T) {
	ws := setupTestWorkspace(t)

	// Seed: REQ-A exists.
	if _, _, err := ws.createEntity("requirement", CreateOptions{
		ID:         "REQ-A",
		Properties: map[string]any{"title": "pre-existing"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// In one tx: write two new entities and a relation, then return
	// an error. Expected: nothing committed, REQ-A unchanged.
	e1 := testutil.EntityFor(ws.Meta(), "requirement").ID("REQ-X").With("title", "x").Build()
	e2 := testutil.EntityFor(ws.Meta(), "requirement").ID("REQ-Y").With("title", "y").Build()
	rel := model.NewRelation("REQ-X", "depends-on", "REQ-Y")

	wantErr := errors.New("forced rollback in multi-op tx")
	err := ws.WithTx(func(tx *Tx) error {
		if err := tx.WriteEntity(e1); err != nil {
			return err
		}
		if err := tx.WriteEntity(e2); err != nil {
			return err
		}
		if err := tx.WriteRelation(rel); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WithTx returned %v, want %v", err, wantErr)
	}

	g := ws.Graph()
	if _, ok := g.GetNode("REQ-X"); ok {
		t.Error("REQ-X must not be in graph after rolled-back tx")
	}
	if _, ok := g.GetNode("REQ-Y"); ok {
		t.Error("REQ-Y must not be in graph after rolled-back tx")
	}
	if _, ok := g.GetEdge("REQ-X", "depends-on", "REQ-Y"); ok {
		t.Error("relation must not be in graph after rolled-back tx")
	}
	// Pre-existing entity is untouched.
	if _, ok := g.GetNode("REQ-A"); !ok {
		t.Error("REQ-A must still exist after rolled-back tx")
	}
}

// TestWithTxClosedWorkspace verifies that WithTx returns an error after
// the workspace is closed and never opens a repository transaction.
func TestWithTxClosedWorkspace(t *testing.T) {
	ws := setupTestWorkspace(t)
	if err := ws.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	called := false
	err := ws.WithTx(func(_ *Tx) error {
		called = true
		return nil
	})
	if err == nil {
		t.Error("WithTx on closed workspace should return an error")
	}
	if called {
		t.Error("WithTx callback should not run on a closed workspace")
	}
}
