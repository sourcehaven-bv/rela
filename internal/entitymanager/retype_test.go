package entitymanager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
)

// TestUpdateEntity_TypeIsImmutable: the store checks a sibling face's type
// against its family but not the bare row's, so an update that retyped the
// bare row split the family (on fsstore, two files under two type
// directories). The manager now refuses a retype on EVERY update path, not
// only the sync upsert.
func TestUpdateEntity_TypeIsImmutable(t *testing.T) {
	ctx := context.Background()
	mgr, st := newCopyAuthzManager(t, acl.NopACL{})
	seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-1", Type: "page", Properties: map[string]any{"title": "p"}})
	seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-1", Type: "page", Face: "published",
		Properties: map[string]any{"title": "p"}})

	// `note` declares no id prefix, so the body passes per-type validation
	// and the refusal below is the manager's guard, not a prefix mismatch.
	_, err := mgr.UpdateEntity(ctx, &entity.Entity{ID: "PAGE-1", Type: "note",
		Properties: map[string]any{"title": "retyped"}})
	if !errors.Is(err, entitymanager.ErrTypeImmutable) {
		t.Fatalf("want ErrTypeImmutable, got %v", err)
	}
	got, _ := st.GetEntity(ctx, "PAGE-1")
	if got.Type != "page" || got.Properties["title"] != "p" {
		t.Errorf("the bare row must be untouched; got %+v", got)
	}
}
