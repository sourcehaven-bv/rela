package rename_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/rename"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

func seedEntity(t *testing.T, st store.Store, id, entityType string) {
	t.Helper()
	e := entity.New(id, entityType)
	if err := st.CreateEntity(context.Background(), e); err != nil {
		t.Fatalf("seed entity %s: %v", id, err)
	}
}

func seedRelation(t *testing.T, st store.Store, from, relType, to string) {
	t.Helper()
	if _, err := st.CreateRelation(context.Background(), from, relType, to, nil); err != nil {
		t.Fatalf("seed relation %s--%s-->%s: %v", from, relType, to, err)
	}
}

func TestRename_NotFound(t *testing.T) {
	st := memstore.New()
	_, err := rename.Rename(context.Background(), st, "REQ-1", "REQ-2", rename.Options{})
	if !errors.Is(err, rename.ErrEntityNotFound) {
		t.Fatalf("expected ErrEntityNotFound, got %v", err)
	}
}

func TestRename_TargetExists(t *testing.T) {
	st := memstore.New()
	seedEntity(t, st, "REQ-1", "requirement")
	seedEntity(t, st, "REQ-2", "requirement")
	_, err := rename.Rename(context.Background(), st, "REQ-1", "REQ-2", rename.Options{})
	if !errors.Is(err, rename.ErrEntityAlreadyExists) {
		t.Fatalf("expected ErrEntityAlreadyExists, got %v", err)
	}
}

func TestRename_EntityTypeMismatch(t *testing.T) {
	st := memstore.New()
	seedEntity(t, st, "REQ-1", "requirement")
	_, err := rename.Rename(context.Background(), st, "REQ-1", "REQ-2", rename.Options{EntityType: "decision"})
	if !errors.Is(err, rename.ErrEntityTypeMismatch) {
		t.Fatalf("expected ErrEntityTypeMismatch, got %v", err)
	}
	if _, err := st.GetEntity(context.Background(), "REQ-1"); err != nil {
		t.Errorf("old entity removed despite type-mismatch failure: %v", err)
	}
}

func TestRename_DryRunPreservesStore(t *testing.T) {
	st := memstore.New()
	seedEntity(t, st, "REQ-1", "requirement")
	seedEntity(t, st, "REQ-2", "requirement")
	seedRelation(t, st, "REQ-1", "addresses", "REQ-2")

	res, err := rename.Rename(context.Background(), st, "REQ-1", "REQ-3", rename.Options{DryRun: true})
	if err != nil {
		t.Fatalf("Rename dry: %v", err)
	}
	if res.NewID != "REQ-3" || res.OldID != "REQ-1" {
		t.Errorf("result = %+v, want OldID=REQ-1 NewID=REQ-3", res)
	}
	if _, err := st.GetEntity(context.Background(), "REQ-1"); err != nil {
		t.Errorf("entity gone after dry-run: %v", err)
	}
}

func TestRename_RewritesRelations(t *testing.T) {
	st := memstore.New()
	seedEntity(t, st, "REQ-1", "requirement")
	seedEntity(t, st, "DEC-1", "decision")
	seedEntity(t, st, "DEC-2", "decision")
	// REQ-1 → DEC-1 (outgoing), DEC-2 → REQ-1 (incoming).
	seedRelation(t, st, "REQ-1", "addresses", "DEC-1")
	seedRelation(t, st, "DEC-2", "references", "REQ-1")

	res, err := rename.Rename(context.Background(), st, "REQ-1", "REQ-9", rename.Options{})
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if len(res.RelationsUpdated) != 2 {
		t.Errorf("RelationsUpdated = %d, want 2", len(res.RelationsUpdated))
	}
	// Old ID gone.
	if _, err := st.GetEntity(context.Background(), "REQ-1"); err == nil {
		t.Error("old entity still present")
	}
	// New ID has the outgoing relation.
	if _, err := st.GetRelation(context.Background(), "REQ-9", "addresses", "DEC-1"); err != nil {
		t.Errorf("rewritten outgoing relation missing: %v", err)
	}
	if _, err := st.GetRelation(context.Background(), "DEC-2", "references", "REQ-9"); err != nil {
		t.Errorf("rewritten incoming relation missing: %v", err)
	}
}

func TestRename_SelfReferentialCountsTwice(t *testing.T) {
	// Pins workspace's pre-extraction semantic that self-ref edges
	// contribute two RelationRef entries (one per direction).
	st := memstore.New()
	seedEntity(t, st, "REQ-1", "requirement")
	seedRelation(t, st, "REQ-1", "depends-on", "REQ-1")

	res, err := rename.Rename(context.Background(), st, "REQ-1", "REQ-9", rename.Options{})
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if len(res.RelationsUpdated) != 2 {
		t.Errorf("RelationsUpdated = %d, want 2", len(res.RelationsUpdated))
	}
}
