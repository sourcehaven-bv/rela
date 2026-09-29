package docs

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TestSeedRowOf pins which row a doc id names: `ID@face` that face and no
// other, a bare id
// the zero face when stored, else a faced entity's first stored face, since a
// faced type has no zero-face row (DEC-NPZICR).
func TestSeedRowOf(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "R-1", Type: "risico", Properties: map[string]any{"titel": "zero"}},
		{ID: "R-1", Type: "risico", Face: "draft", Properties: map[string]any{"titel": "r1 draft"}},
		{ID: "P-1", Type: "page", Face: "draft", Properties: map[string]any{"titel": "p1 draft"}},
		{ID: "P-1", Type: "page", Face: "published", Properties: map[string]any{"titel": "p1 live"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		id, want string
	}{
		{"R-1", "zero"},
		{"R-1@draft", "r1 draft"},
		{"P-1@published", "p1 live"},
		{"P-1", "p1 draft"},
	}
	for _, tc := range tests {
		got, err := seedRowOf(ctx, st, tc.id)
		if err != nil {
			t.Errorf("seedRowOf(%s): %v", tc.id, err)
			continue
		}
		if got.Properties["titel"] != tc.want {
			t.Errorf("seedRowOf(%s) = %v, want %q", tc.id, got.Properties["titel"], tc.want)
		}
	}
	for _, id := range []string{"NOPE", "P-1@review"} {
		if _, err := seedRowOf(ctx, st, id); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("seedRowOf(%s) = %v, want ErrNotFound", id, err)
		}
	}
}
