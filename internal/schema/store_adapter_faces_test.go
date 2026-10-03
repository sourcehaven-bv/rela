package schema

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TestStoreCounter_CountsFamilies pins that a faced type is counted in
// entities, not face rows: an entity stored at two faces counts once, and
// one stored only at its later face still counts.
func TestStoreCounter_CountsFamilies(t *testing.T) {
	st := memstore.New()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Face: "draft"},
		{ID: "POL-1", Type: "policy", Face: "published"},
		{ID: "POL-2", Type: "policy", Face: "published"},
		{ID: "REQ-1", Type: "requirement"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	families := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"draft", "published"}, Fallback: store.FallbackExclude},
	})

	counter := mustCounter(t, st, families)
	if got := counter.CountByEntityType("policy"); got != 2 {
		t.Errorf("policy count = %d, want 2 entities (3 face rows)", got)
	}
	if got := counter.CountByEntityType("requirement"); got != 1 {
		t.Errorf("requirement count = %d, want 1", got)
	}
}
