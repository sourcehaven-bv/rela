package worldreader_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/worldreader"
)

// stateReaderFunc adapts a function to worldreader.StateReader.
type stateReaderFunc func(context.Context, string, entity.Pointer) (*entity.Entity, error)

func (f stateReaderFunc) GetEntityState(
	ctx context.Context, id string, p entity.Pointer,
) (*entity.Entity, error) {
	return f(ctx, id, p)
}

// identityCanon is a canonicalizer with no aliases: every name is already
// canonical.
type identityCanon struct{}

func (identityCanon) CanonicalType(name string) (string, bool) { return name, true }

// aliasCanon maps one alias to its canonical type.
type aliasCanon map[string]string

func (a aliasCanon) CanonicalType(name string) (string, bool) {
	if canonical, ok := a[name]; ok {
		return canonical, true
	}
	return name, true
}

// statesFixture builds a reader over a fixed set of stored states. The
// entity type is fixed: these tests vary the WORLD and the stored
// pointers, and a second type would only exercise the scope map, which
// TestResolve_Rules already covers via its unscoped-type case.
func statesFixture(t *testing.T, id string, pointers ...entity.Pointer) worldreader.StateReader {
	t.Helper()
	const typ = "page"
	have := map[entity.Pointer]bool{}
	for _, p := range pointers {
		have[p] = true
	}
	return stateReaderFunc(func(_ context.Context, gotID string, p entity.Pointer) (*entity.Entity, error) {
		if gotID != id || !have[p] {
			return nil, store.ErrNotFound
		}
		e := entity.New(id, typ)
		e.Pointer = p
		e.SetString("title", id+"@"+string(p))
		return e, nil
	})
}

func pageScope(fb store.Fallback, chain ...entity.Pointer) store.WorldScope {
	return store.NewWorldScope(map[string]store.TypeResolution{
		"page": {Chain: chain, Fallback: fb},
	})
}

// TestResolve_Rules walks the three resolution rules. They must agree
// with storeutil.WorldPrimes and the SQL pushdown — the whole point of
// having one contract is that the decorator cannot drift from them.
func TestResolve_Rules(t *testing.T) {
	for _, tc := range []struct {
		name     string
		stored   []entity.Pointer
		scope    store.WorldScope
		wantPtr  entity.Pointer
		wantVia  worldreader.Rule
		wantMiss bool
	}{
		{
			name:    "rule 2: first existing coordinate wins",
			stored:  []entity.Pointer{"", "review", "published"},
			scope:   pageScope(store.FallbackDefaultState, "review", "published"),
			wantPtr: "review",
			wantVia: worldreader.RuleChain,
		},
		{
			name:    "rule 2: chain order is the whole semantic content",
			stored:  []entity.Pointer{"", "review", "published"},
			scope:   pageScope(store.FallbackDefaultState, "published", "review"),
			wantPtr: "published",
			wantVia: worldreader.RuleChain,
		},
		{
			name:    "rule 2: skips a coordinate that does not exist",
			stored:  []entity.Pointer{"", "published"},
			scope:   pageScope(store.FallbackExclude, "review", "published"),
			wantPtr: "published",
			wantVia: worldreader.RuleChain,
		},
		{
			name:    "rule 3: otherwise-default serves the default face",
			stored:  []entity.Pointer{""},
			scope:   pageScope(store.FallbackDefaultState, "published"),
			wantPtr: "",
			wantVia: worldreader.RuleFallbackDefault,
		},
		{
			name:     "rule 3: otherwise-exclude yields nothing",
			stored:   []entity.Pointer{""},
			scope:    pageScope(store.FallbackExclude, "published"),
			wantMiss: true,
			wantVia:  worldreader.RuleExcluded,
		},
		{
			name:    "rule 1: unscoped type keeps its default state",
			stored:  []entity.Pointer{"", "published"},
			scope:   store.NewWorldScope(map[string]store.TypeResolution{"note": {}}),
			wantPtr: "",
			wantVia: worldreader.RuleUnscoped,
		},
		{
			name:     "otherwise-default still yields nothing when no state exists at all",
			stored:   nil,
			scope:    pageScope(store.FallbackDefaultState, "published"),
			wantMiss: true,
			wantVia:  worldreader.RuleExcluded,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := worldreader.NewResolver(
				statesFixture(t, "PAGE-1", tc.stored...), tc.scope, identityCanon{})
			require.NoError(t, err)

			got, err := r.Resolve(context.Background(), "page", "PAGE-1")
			require.NoError(t, err)

			assert.Equal(t, tc.wantVia, got.Via, "provenance")
			if tc.wantMiss {
				assert.False(t, got.Found, "the world must exclude this entity")
				assert.Nil(t, got.Entity)
				return
			}
			require.True(t, got.Found)
			require.NotNil(t, got.Entity)
			assert.Equal(t, tc.wantPtr, got.Pointer)
			assert.Equal(t, "PAGE-1@"+string(tc.wantPtr), got.Entity.GetString("title"))
		})
	}
}

// TestResolve_AliasCanonicalizesAtTheBoundary is the alias trap, named
// for its failure.
//
// store.WorldScope is keyed on CANONICAL type names — a store holds no
// metamodel and cannot resolve an alias. So an alias reaching
// WorldScope.For arrives as an unknown type, which is ok=false, which is
// rule 1: the DEFAULT STATE SERVED IN A WORLD THAT MEANT TO EXCLUDE IT.
//
// That is fail-OPEN, which is why canonicalization happens at this
// boundary rather than being left to callers to remember.
func TestResolve_AliasCanonicalizesAtTheBoundary(t *testing.T) {
	// Only the default state exists, and the world excludes anything
	// without a published state. Correct answer: NOT FOUND.
	reader := statesFixture(t, "PAGE-1", "")
	scope := pageScope(store.FallbackExclude, "published")

	canon := aliasCanon{"pages": "page"} // "pages" is an alias for "page"
	r, err := worldreader.NewResolver(reader, scope, canon)
	require.NoError(t, err)

	got, err := r.Resolve(context.Background(), "pages", "PAGE-1")
	require.NoError(t, err)

	assert.False(t, got.Found,
		"an ALIASED type must resolve through its canonical name: treating "+
			"the alias as an unknown type is rule 1, which serves the default "+
			"state in a world that meant to exclude it — fail-open")
	assert.Equal(t, worldreader.RuleExcluded, got.Via)
}

// TestResolve_StoreFaultIsNotAMiss: only the visibility Reader may
// collapse a fault into a clean miss (its oracle-free contract requires
// it). Doing that here would make a backend outage look like an
// exclusion at the wrong layer, and the operator would debug the world
// instead of the database.
func TestResolve_StoreFaultIsNotAMiss(t *testing.T) {
	boom := errors.New("connection refused")
	reader := stateReaderFunc(func(context.Context, string, entity.Pointer) (*entity.Entity, error) {
		return nil, boom
	})
	r, err := worldreader.NewResolver(reader, pageScope(store.FallbackExclude, "published"), identityCanon{})
	require.NoError(t, err)

	_, err = r.Resolve(context.Background(), "page", "PAGE-1")
	assert.ErrorIs(t, err, boom, "a store fault must surface, not read as an exclusion")
}

// TestResolve_ChainShortCircuits pins that the walk stops at the first
// hit. Chains are 1-3 in practice and the common case is a single point
// read; walking the whole chain regardless would multiply every Get.
func TestResolve_ChainShortCircuits(t *testing.T) {
	var asked []entity.Pointer
	reader := stateReaderFunc(func(_ context.Context, id string, p entity.Pointer) (*entity.Entity, error) {
		asked = append(asked, p)
		if p != "review" {
			return nil, store.ErrNotFound
		}
		e := entity.New(id, "page")
		e.Pointer = p
		return e, nil
	})
	r, err := worldreader.NewResolver(reader,
		pageScope(store.FallbackDefaultState, "review", "published"), identityCanon{})
	require.NoError(t, err)

	_, err = r.Resolve(context.Background(), "page", "PAGE-1")
	require.NoError(t, err)

	assert.Equal(t, []entity.Pointer{"review"}, asked,
		"the first hit wins; later coordinates and the fallback are never fetched")
}

func TestNewResolver_RejectsNilCollaborators(t *testing.T) {
	_, err := worldreader.NewResolver(nil, store.DefaultWorld(), identityCanon{})
	require.Error(t, err, "a nil reader must be rejected")

	_, err = worldreader.NewResolver(statesFixture(t, "X"), store.DefaultWorld(), nil)
	require.Error(t, err,
		"a nil canonicalizer would make every aliased type rule 1 — the fail-open direction")
}
