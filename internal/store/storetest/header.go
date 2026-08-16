package storetest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunHeaderTests runs conformance tests for content-free header listing
// (TKT-1ESTYJ).
//
// Every backend must pass these, whether it implements store.HeaderReader
// natively or is served by the generic fallback: store.ListEntityHeaders
// picks the path, and the OBSERVABLE contract is identical either way. That
// equivalence is the point — a caller must never have to know which it got.
func RunHeaderTests(t *testing.T, f Factory) {
	t.Run("HeadersMatchEntitiesMinusContent", func(t *testing.T) {
		s := f(t)
		for _, id := range []string{"A-001", "A-002", "B-001"} {
			e := entity.New(id, typeOf(id))
			e.SetString("title", "title of "+id)
			e.Content = "# body of " + id + "\n\n" + strings.Repeat("filler ", 100)
			require.NoError(t, s.CreateEntity(ctx(), e))
		}

		// The header set must agree with the entity set on identity and
		// properties, so a caller can swap one for the other without
		// re-deriving what it may rely on.
		wantIDs := make([]string, 0)
		wantProps := map[string]string{}
		for e, err := range s.ListEntities(ctx(), store.EntityQuery{}) {
			require.NoError(t, err)
			wantIDs = append(wantIDs, e.ID)
			wantProps[e.ID] = e.GetString("title")
		}

		gotIDs := make([]string, 0)
		for h, err := range store.ListEntityHeaders(ctx(), s, store.EntityQuery{}) {
			require.NoError(t, err)
			gotIDs = append(gotIDs, h.ID)
			assert.Equal(t, typeOf(h.ID), h.Type, "header type for %s", h.ID)
			assert.Equal(t, wantProps[h.ID], asString(h.Properties["title"]),
				"header properties must match the entity's for %s", h.ID)
			assert.False(t, h.UpdatedAt.IsZero(), "header UpdatedAt must be populated for %s", h.ID)
		}

		// Same members AND same order: ListEntities promises a stable
		// ascending-by-id order, and headers are the same listing with a
		// column removed — a caller that sorts by neither must still see
		// both paths agree.
		assert.Equal(t, wantIDs, gotIDs)
	})

	t.Run("FiltersByType", func(t *testing.T) {
		s := f(t)
		for _, id := range []string{"A-001", "A-002", "B-001"} {
			e := entity.New(id, typeOf(id))
			e.Content = "body"
			require.NoError(t, s.CreateEntity(ctx(), e))
		}

		got := make([]string, 0)
		for h, err := range store.ListEntityHeaders(ctx(), s, store.EntityQuery{Type: "atype"}) {
			require.NoError(t, err)
			got = append(got, h.ID)
		}
		assert.Equal(t, []string{"A-001", "A-002"}, got)
	})

	t.Run("FiltersByIDs", func(t *testing.T) {
		s := f(t)
		for _, id := range []string{"A-001", "A-002", "B-001"} {
			e := entity.New(id, typeOf(id))
			require.NoError(t, s.CreateEntity(ctx(), e))
		}

		got := make([]string, 0)
		for h, err := range store.ListEntityHeaders(ctx(), s, store.EntityQuery{IDs: []string{"A-002", "B-001"}}) {
			require.NoError(t, err)
			got = append(got, h.ID)
		}
		assert.Equal(t, []string{"A-002", "B-001"}, got)
	})

	t.Run("EmptyStoreYieldsNothing", func(t *testing.T) {
		s := f(t)
		n := 0
		for _, err := range store.ListEntityHeaders(ctx(), s, store.EntityQuery{}) {
			require.NoError(t, err)
			n++
		}
		assert.Zero(t, n)
	})

	// A header must not expose stored state by reference: a caller that
	// mutates what it reads would corrupt the store through a read-only API.
	// (The in-package HeaderOf projection shares the map deliberately and is
	// documented read-only; a STORE listing has no such caller contract.)
	t.Run("MutatingHeaderPropsDoesNotAffectStore", func(t *testing.T) {
		s := f(t)
		e := entity.New("A-001", "atype")
		e.SetString("title", "original")
		require.NoError(t, s.CreateEntity(ctx(), e))

		for h, err := range store.ListEntityHeaders(ctx(), s, store.EntityQuery{}) {
			require.NoError(t, err)
			h.Properties["title"] = "mutated"
		}

		got, err := s.GetEntity(ctx(), "A-001")
		require.NoError(t, err)
		assert.Equal(t, "original", got.GetString("title"),
			"mutating a header's Properties must not write through to the store")
	})

	// Early return from the caller's loop must stop the iteration cleanly:
	// analyze caps a section at N issues and abandons the rest of the scan,
	// so a backend that ignores a false yield would keep working after the
	// consumer stopped caring.
	t.Run("StopsOnEarlyReturn", func(t *testing.T) {
		s := f(t)
		for _, id := range []string{"A-001", "A-002", "B-001"} {
			require.NoError(t, s.CreateEntity(ctx(), entity.New(id, typeOf(id))))
		}

		seen := 0
		for _, err := range store.ListEntityHeaders(ctx(), s, store.EntityQuery{}) {
			require.NoError(t, err)
			seen++
			break
		}
		assert.Equal(t, 1, seen)
	})
}

// typeOf maps the fixture id prefix to a type, so the type filter has
// something to discriminate on.
func typeOf(id string) string {
	if strings.HasPrefix(id, "A-") {
		return "atype"
	}
	return "btype"
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
