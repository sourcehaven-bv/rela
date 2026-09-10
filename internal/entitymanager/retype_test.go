package entitymanager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
)

// TestUpdateEntity_TypeIsImmutable: the store checks a row's type against its
// family only on create, so an update that retyped a row split the family (on
// fsstore, two files under two type directories). The manager now refuses a
// retype on EVERY update path, not only the sync upsert.
//
// The subject is the FACELESS `ticket` type. Manager.UpdateEntity authorizes
// on the body's face but reads the pre-image with store.GetEntity, which
// resolves the ZERO coordinate — and since BUG-HC6I2T a faced type stores no
// row there, so an update naming a face fails not-found before it reaches the
// guard. On `page` this test would assert a not-found, not the guard. The
// guard itself compares the body's type against the stored row's and is
// face-independent, so `ticket` pins it honestly.
func TestUpdateEntity_TypeIsImmutable(t *testing.T) {
	ctx := context.Background()
	mgr, st := newCopyAuthzManager(t, acl.NopACL{})
	seedRaw(ctx, t, st, &entity.Entity{ID: "TKT-1", Type: "ticket",
		Properties: map[string]any{"title": "p"}})

	// `note` declares no id prefix, so the body passes per-type validation
	// and the refusal below is the manager's guard, not a prefix mismatch.
	_, err := mgr.UpdateEntity(ctx, &entity.Entity{ID: "TKT-1", Type: "note",
		Properties: map[string]any{"title": "retyped"}})
	if !errors.Is(err, entitymanager.ErrTypeImmutable) {
		t.Fatalf("want ErrTypeImmutable, got %v", err)
	}
	got, gerr := st.GetEntity(ctx, "TKT-1")
	if gerr != nil || got.Type != "ticket" || got.Properties["title"] != "p" {
		t.Errorf("the row must be untouched; got %+v (err %v)", got, gerr)
	}
}
