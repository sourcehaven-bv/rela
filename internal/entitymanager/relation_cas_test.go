package entitymanager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func TestUpdateRelation_ExpectedVersion(t *testing.T) {
	t.Parallel()
	mgr, _ := newManager(t, nil)
	ctx := context.Background()
	req := createReq(t, mgr, "r")
	dec := createDec(t, mgr, "d")
	key := entity.RelationKey{From: dec.ID, Type: "addresses", To: req.ID}
	rel, err := mgr.CreateRelation(ctx, key, entity.RelationOptions{Properties: map[string]any{"weight": "high"}})
	if err != nil {
		t.Fatalf("create relation: %v", err)
	}
	stale := string(store.RelationVersionOf(rel))

	if _, err = mgr.UpdateRelation(ctx, key, entity.RelationOptions{
		Properties: map[string]any{"weight": "low"}, ExpectedVersion: stale,
	}); err != nil {
		t.Fatalf("update at the current version: %v", err)
	}
	_, err = mgr.UpdateRelation(ctx, key, entity.RelationOptions{
		Properties: map[string]any{"weight": "mid"}, ExpectedVersion: stale,
	})
	var conflict *store.VersionConflictError
	if !errors.As(err, &conflict) || !errors.Is(err, store.ErrConflict) {
		t.Fatalf("update at a stale version: err = %v, want a version conflict", err)
	}
}
