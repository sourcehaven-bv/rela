package sync

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestLocalType_FacedEntity pins that the type a relation route and a forced
// record need is found for a faced entity, which has no zero-face row
// (DEC-NPZICR). A zero-face read reported it missing.
func TestLocalType_FacedEntity(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.createLocalEntity(t, "TKT-1", nil)
	faced := &entity.Entity{ID: "TKT-F", Type: "ticket", Face: "draft"}
	if err := h.st.CreateEntity(ctx, faced); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"TKT-1", "TKT-F"} {
		typ, err := h.engine.localType(ctx, id)
		if err != nil || typ != "ticket" {
			t.Errorf("localType(%s) = %q, %v; want ticket", id, typ, err)
		}
	}
	if _, err := h.engine.localType(ctx, "NOPE-1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("localType of a missing id = %v, want ErrNotFound", err)
	}
}
