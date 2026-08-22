package worldreader_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/worldreader"
)

// fixedSearcher answers ServesWorld with a canned verdict.
type fixedSearcher struct{ serves bool }

func (f fixedSearcher) ServesWorld(store.WorldScope) bool { return f.serves }

func newParts(t *testing.T, scope store.WorldScope) (*worldreader.Resolver, *worldreader.RelationReader) {
	t.Helper()
	r, err := worldreader.NewResolver(statesFixture(t, "PAGE-1", ""), scope, identityCanon{})
	require.NoError(t, err)
	rr, err := worldreader.NewRelationReader(&recordingLister{}, contentTypes{})
	require.NoError(t, err)
	return r, rr
}

// TestNewSurface_SearcherConstructibility is RULING 3's criterion: a
// world-bound surface must not be CONSTRUCTIBLE over a searcher that
// cannot honor its world.
//
// It is enforced at construction rather than on the search path because
// internal/search is world-agnostic by construction — its Query carries
// no world, so there is nothing there to check. And the ACL row gate
// cannot compensate: guard rule 1 makes it world-independent, so a
// wrongly-scoped hit would reach the user unopposed.
func TestNewSurface_SearcherConstructibility(t *testing.T) {
	worldScope := store.NewWorldScope(map[string]store.TypeResolution{
		"page": {Chain: []entity.Pointer{"published"}, Fallback: store.FallbackExclude},
	})

	for _, tc := range []struct {
		name     string
		scope    store.WorldScope
		searcher worldreader.WorldAwareSearcher
		wantErr  bool
	}{
		{
			name:     "default world accepts any searcher",
			scope:    store.DefaultWorld(),
			searcher: fixedSearcher{serves: false},
		},
		{
			name:  "non-default world with NO searcher is fine",
			scope: worldScope,
		},
		{
			name:     "non-default world REFUSES a searcher that cannot serve it",
			scope:    worldScope,
			searcher: fixedSearcher{serves: false},
			wantErr:  true,
		},
		{
			name:     "non-default world accepts a searcher that affirms it",
			scope:    worldScope,
			searcher: fixedSearcher{serves: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, rr := newParts(t, tc.scope)
			got, err := worldreader.NewSurface(r, rr, tc.searcher)
			if tc.wantErr {
				require.Error(t, err, "a world-bound surface must not be constructible here")
				require.ErrorIs(t, err, worldreader.ErrSearcherCannotServeWorld)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
		})
	}
}

// TestNewSurface_TodaysSearchersCannotBackAWorldBoundSurface pins the
// intended outcome rather than only the mechanism: NOTHING implements
// WorldAwareSearcher today, so the nil-searcher case is the only way to
// build a world-bound surface until per-world indexing lands (Step 5,
// TKT-9KZGJO). If a future searcher starts affirming ServesWorld without
// actually scoping its index, this test is where that claim should be
// re-examined.
func TestNewSurface_TodaysSearchersCannotBackAWorldBoundSurface(t *testing.T) {
	worldScope := store.NewWorldScope(map[string]store.TypeResolution{
		"page": {Chain: []entity.Pointer{"published"}, Fallback: store.FallbackExclude},
	})
	r, rr := newParts(t, worldScope)

	// A stand-in for any of today's searchers: it does not implement
	// WorldAwareSearcher at all, so it cannot be passed here — the
	// compiler enforces that. Passing one that says "no" is the closest
	// representable equivalent.
	_, err := worldreader.NewSurface(r, rr, fixedSearcher{serves: false})
	require.ErrorIs(t, err, worldreader.ErrSearcherCannotServeWorld)
}

func TestNewSurface_RejectsNilCollaborators(t *testing.T) {
	r, rr := newParts(t, store.DefaultWorld())

	_, err := worldreader.NewSurface(nil, rr, nil)
	require.Error(t, err)

	_, err = worldreader.NewSurface(r, nil, nil)
	require.Error(t, err, "without the relation capability the scope dispatch could be bypassed")
}
