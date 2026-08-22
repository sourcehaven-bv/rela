package appbuild_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/worldreader"
)

// worldSchema declares one type with two content states and two worlds:
// `editorial` prefers the review face and falls back to the default;
// `public` serves only the published face and excludes everything else.
const worldSchema = `
entities:
  page:
    label: Page
    plural: pages
    id_prefix: "PAGE-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
    pointers:
      review: {}
      published: {}
relations: {}
worlds:
  editorial:
    select: [review]
    otherwise: default
  public:
    select: [published]
    otherwise: exclude
`

func newWorldServices(t *testing.T) *appbuild.Services {
	t.Helper()
	meta, err := metamodel.Parse([]byte(worldSchema))
	require.NoError(t, err, "the world schema must parse — worlds are declared in the metamodel")
	return appbuildtest.New(meta)
}

// TestWorldSurface_ResolvesThroughTheWiredStack is the end-to-end wiring
// check: a world named in schema.yaml is compiled at boot, looked up by
// name, and bound to a surface that resolves against the real store.
//
// It exercises the adapters that cannot live in worldreader itself —
// alias canonicalization and relation-scope classification both need the
// metamodel, which arch-lint forbids that package from importing.
func TestWorldSurface_ResolvesThroughTheWiredStack(t *testing.T) {
	svc := newWorldServices(t)
	defer func() { _ = svc.Close() }()
	ctx := context.Background()

	// PAGE-1 holds all three faces; PAGE-2 only its default.
	base := entity.New("PAGE-1", "page")
	base.SetString("title", "draft face")
	require.NoError(t, svc.Store().CreateEntity(ctx, base))

	review := entity.New("PAGE-1", "page")
	review.Pointer = entity.Pointer("review")
	review.SetString("title", "review face")
	require.NoError(t, svc.Store().CreateEntity(ctx, review))

	published := entity.New("PAGE-1", "page")
	published.Pointer = entity.Pointer("published")
	published.SetString("title", "published face")
	require.NoError(t, svc.Store().CreateEntity(ctx, published))

	lonely := entity.New("PAGE-2", "page")
	lonely.SetString("title", "only a draft")
	require.NoError(t, svc.Store().CreateEntity(ctx, lonely))

	for _, tc := range []struct {
		name      string
		world     string
		id        string
		wantTitle string
		wantVia   worldreader.Rule
		wantGone  bool
	}{
		{
			name:      "editorial prefers the review face",
			world:     "editorial",
			id:        "PAGE-1",
			wantTitle: "review face",
			wantVia:   worldreader.RuleChain,
		},
		{
			name:      "editorial falls back to the default face",
			world:     "editorial",
			id:        "PAGE-2",
			wantTitle: "only a draft",
			wantVia:   worldreader.RuleFallbackDefault,
		},
		{
			name:      "public serves the published face",
			world:     "public",
			id:        "PAGE-1",
			wantTitle: "published face",
			wantVia:   worldreader.RuleChain,
		},
		{
			name:     "public EXCLUDES an entity with no published face",
			world:    "public",
			id:       "PAGE-2",
			wantGone: true,
			wantVia:  worldreader.RuleExcluded,
		},
		{
			name:      "the empty name is the default world",
			world:     "",
			id:        "PAGE-1",
			wantTitle: "draft face",
			wantVia:   worldreader.RuleUnscoped,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			surface, err := appbuild.WorldSurface(svc, tc.world, nil)
			require.NoError(t, err)

			got, err := surface.Resolver().Resolve(ctx, "page", tc.id)
			require.NoError(t, err)

			assert.Equal(t, tc.wantVia, got.Via, "provenance")
			if tc.wantGone {
				assert.False(t, got.Found, "the world must exclude this entity entirely")
				return
			}
			require.True(t, got.Found)
			assert.Equal(t, tc.wantTitle, got.Entity.GetString("title"))
		})
	}
}

// TestWorldSurface_UnknownWorldIsAnError: a surface must not silently
// degrade to the default world when the name is wrong. That is the
// fail-open direction — a typo in a deployment config would serve drafts
// from a surface meant to be public.
func TestWorldSurface_UnknownWorldIsAnError(t *testing.T) {
	svc := newWorldServices(t)
	defer func() { _ = svc.Close() }()

	_, err := appbuild.WorldSurface(svc, "no-such-world", nil)
	require.Error(t, err, "an unknown world must be refused, never defaulted")
	assert.Contains(t, err.Error(), "no-such-world")
}

// TestWorldSurface_AliasResolvesToItsCanonicalType pins the adapter that
// makes alias canonicalization work end to end. WorldScope is keyed on
// CANONICAL names, so an alias reaching it unresolved is an unknown type
// → rule 1 → the default face served in a world that meant to exclude
// it. Fail-open, which is why the wiring supplies a real canonicalizer.
func TestWorldSurface_AliasResolvesToItsCanonicalType(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`
entities:
  page:
    label: Page
    plural: pages
    aliases: [pg]
    id_prefix: "PAGE-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
    pointers:
      published: {}
relations: {}
worlds:
  public:
    select: [published]
    otherwise: exclude
`))
	require.NoError(t, err)
	svc := appbuildtest.New(meta)
	defer func() { _ = svc.Close() }()
	ctx := context.Background()

	only := entity.New("PAGE-1", "page")
	only.SetString("title", "unpublished")
	require.NoError(t, svc.Store().CreateEntity(ctx, only))

	surface, err := appbuild.WorldSurface(svc, "public", nil)
	require.NoError(t, err)

	// Query by the ALIAS. The world excludes unpublished pages, so the
	// answer must be "not found" — not the default face.
	got, err := surface.Resolver().Resolve(ctx, "pg", "PAGE-1")
	require.NoError(t, err)
	assert.False(t, got.Found,
		"an aliased type must canonicalize before the scope is consulted: "+
			"otherwise it is an unknown type, which is rule 1, which serves "+
			"the default face in a world that meant to exclude it")
}
