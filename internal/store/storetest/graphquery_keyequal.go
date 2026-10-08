package storetest

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// runKeyEqualTests pins [store.PropKeyEqual] (TKT-SM20FG): an object
// property's entry equals a string, every other shape does not match, and
// the malformed shapes are [store.ErrInvalidQuery] on every backend.
func runKeyEqualTests(t *testing.T, f Factory) {
	t.Helper()

	seed := func(t *testing.T) store.Store {
		t.Helper()
		s := f(t)
		seedEntityWithProps(t, s, "ticket", "T-1", map[string]any{"ref": map[string]any{"id": "42", "url": "https://x.test/42"}})
		seedEntityWithProps(t, s, "ticket", "T-2", map[string]any{"ref": map[string]any{"id": "43"}})
		seedEntityWithProps(t, s, "ticket", "T-num", map[string]any{"ref": map[string]any{"id": 42}})
		seedEntityWithProps(t, s, "ticket", "T-str", map[string]any{"ref": "42"})
		seedEntityWithProps(t, s, "ticket", "T-list", map[string]any{"ref": []any{"42"}})
		seedEntityWithProps(t, s, "ticket", "T-none", nil)
		seedEntityWithProps(t, s, "ticket", "T-other", map[string]any{"other": map[string]any{"id": "42"}})
		seedEntityWithProps(t, s, "story", "S-1", map[string]any{"ref": map[string]any{"id": "42"}})
		return s
	}
	key := func(value string) store.PropPredicate {
		return store.PropPredicate{Property: "ref", Op: store.PropKeyEqual, Key: "id", Value: value}
	}

	t.Run("matches the string entry only", func(t *testing.T) {
		s := seed(t)
		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			Props:      []store.PropPredicate{key("42")},
			Faces:      store.InWorld(store.TrivialScope()),
		})
		require.Equal(t, []string{"T-1"}, got)
	})

	// AC2: the stored object reads back as written.
	t.Run("round-trips the object", func(t *testing.T) {
		s := seed(t)
		got, err := s.GetEntity(context.Background(), entity.Ref{ID: "T-1"})
		require.NoError(t, err)
		require.Equal(t, map[string]any{"id": "42", "url": "https://x.test/42"}, got.Properties["ref"])
	})

	t.Run("headers agree", func(t *testing.T) {
		s := seed(t)
		var ids []string
		for _, typ := range []string{"story", "ticket"} {
			for h, err := range store.GraphQueryHeaders(context.Background(), s, store.GraphQuery{
				EntityType: typ,
				Props:      []store.PropPredicate{key("42")},
				Faces:      store.AllFaces(),
			}) {
				require.NoError(t, err)
				ids = append(ids, h.ID)
			}
		}
		slices.Sort(ids)
		require.Equal(t, []string{"S-1", "T-1"}, ids)
	})

	t.Run("inside a narrowing and a negated branch", func(t *testing.T) {
		s := seed(t)
		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			Narrowing:  []store.NarrowBranch{{Props: []store.PropPredicate{key("43")}}, {Props: []store.PropPredicate{key("42")}}},
			Faces:      store.InWorld(store.TrivialScope()),
		})
		require.Equal(t, []string{"T-1", "T-2"}, got)
	})

	invalid := []struct {
		name string
		p    store.PropPredicate
	}{
		{"key on PropEqual", store.PropPredicate{Property: "ref", Op: store.PropEqual, Key: "id", Value: "42"}},
		{"empty key", store.PropPredicate{Property: "ref", Op: store.PropKeyEqual, Value: "42"}},
		{"empty value", store.PropPredicate{Property: "ref", Op: store.PropKeyEqual, Key: "id"}},
		{"unsafe key", store.PropPredicate{Property: "ref", Op: store.PropKeyEqual, Key: `i"d`, Value: "42"}},
		{"unknown operator", store.PropPredicate{Property: "ref", Op: store.PropOp(99), Value: "42"}},
	}
	for _, tc := range invalid {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			s := f(t)
			for _, q := range []store.GraphQuery{
				{EntityType: "ticket", Props: []store.PropPredicate{tc.p}, Faces: store.AllFaces()},
				{EntityType: "ticket", Narrowing: []store.NarrowBranch{{Props: []store.PropPredicate{tc.p}}}, Faces: store.AllFaces()},
			} {
				var gotErr error
				for _, err := range s.GraphQuery(context.Background(), q) {
					if err != nil {
						gotErr = err
						break
					}
				}
				require.True(t, errors.Is(gotErr, store.ErrInvalidQuery), "got %v", gotErr)
			}
		})
	}
}
