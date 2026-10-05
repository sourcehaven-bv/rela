package visibility_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// TestResolver_FamilyInDeclarationOrder pins that Family.Faces follows the
// families scope's chain when the wiring supplies one (TKT-7IZHP0 §3.1, G18):
// the implicit face first, then chain order, then faces the chain does not
// name by token. Without the option the order is by token.
func TestResolver_FamilyInDeclarationOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := memstore.New()
	for _, f := range []entity.Face{"", "archived", "draft", "published", "review", "zombie"} {
		if err := st.CreateEntity(ctx, &entity.Entity{ID: "POL-1", Type: "policy", Face: f}); err != nil {
			t.Fatal(err)
		}
	}
	families := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"review", "published", "draft"}, Fallback: store.FallbackExclude},
	})
	order := func() store.WorldScope { return families }

	cases := []struct {
		name string
		opts []visibility.ResolverOption
		want []entity.Face
	}{
		{name: "declaration order", opts: []visibility.ResolverOption{visibility.WithFamilies(order)},
			want: []entity.Face{"", "review", "published", "draft", "archived", "zombie"}},
		{name: "token order without the option",
			want: []entity.Face{"", "archived", "draft", "published", "review", "zombie"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := visibility.NewResolver(visibility.NopGate{}, visibility.NopRedactor{}, st, tc.opts...)
			if err != nil {
				t.Fatal(err)
			}
			fam, ok, err := r.Family(ctx, "policy", "POL-1")
			if err != nil || !ok {
				t.Fatalf("Family = (%v, %v)", ok, err)
			}
			if !slices.Equal(fam.Faces, tc.want) {
				t.Errorf("Faces = %q, want %q", fam.Faces, tc.want)
			}
		})
	}
}

func TestWithFamilies_RejectsNil(t *testing.T) {
	t.Parallel()
	_, err := visibility.NewResolver(visibility.NopGate{}, visibility.NopRedactor{}, memstore.New(),
		visibility.WithFamilies(nil))
	if err == nil {
		t.Fatal("NewResolver accepted WithFamilies(nil)")
	}
}
