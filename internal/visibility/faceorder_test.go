package visibility_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// TestResolver_FamilyInDeclarationOrder pins that Family.Faces follows the
// schema's face order when the wiring supplies one (TKT-7IZHP0 §3.1): the
// implicit face first, then declaration order, then undeclared faces by
// token. Without the option the order is by token.
func TestResolver_FamilyInDeclarationOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := memstore.New()
	for _, f := range []entity.Face{"", "archived", "draft", "published", "review", "zombie"} {
		if err := st.CreateEntity(ctx, &entity.Entity{ID: "POL-1", Type: "policy", Face: f}); err != nil {
			t.Fatal(err)
		}
	}
	order := func(entityType string) []string {
		if entityType != "policy" {
			return nil
		}
		return []string{"review", "published", "draft"}
	}

	cases := []struct {
		name string
		opts []visibility.ResolverOption
		want []entity.Face
	}{
		{name: "declaration order", opts: []visibility.ResolverOption{visibility.WithFaceOrder(order)},
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

func TestWithFaceOrder_RejectsNil(t *testing.T) {
	t.Parallel()
	_, err := visibility.NewResolver(visibility.NopGate{}, visibility.NopRedactor{}, memstore.New(),
		visibility.WithFaceOrder(nil))
	if err == nil {
		t.Fatal("NewResolver accepted WithFaceOrder(nil)")
	}
}
