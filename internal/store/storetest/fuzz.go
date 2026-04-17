package storetest

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FuzzFactory returns a fresh store without needing *testing.T (for use inside f.Fuzz).
type FuzzFactory func() store.Store

// FuzzRelationKeyCollision verifies that relation key construction never
// causes collisions or round-trip failures.
func FuzzRelationKeyCollision(f *testing.F, factory FuzzFactory) {
	f.Add("A", "requires", "B")
	f.Add("A-B", "rel", "C")
	f.Add("X", "Y", "Z")
	f.Add("", "rel", "B")
	f.Add("A", "", "B")
	f.Add("A--B", "rel", "C")

	f.Fuzz(func(t *testing.T, from, relType, to string) {
		s := factory()
		bg := context.Background()

		err1 := s.CreateEntity(bg, entity.New(from, "t"))
		if from == "" || strings.Contains(from, "--") {
			assert.Error(t, err1)
			return
		}
		require.NoError(t, err1)

		if from != to {
			err2 := s.CreateEntity(bg, entity.New(to, "t"))
			if to == "" || strings.Contains(to, "--") {
				assert.Error(t, err2)
				return
			}
			require.NoError(t, err2)
		}

		r, err := s.CreateRelation(bg, from, relType, to, nil)
		if relType == "" || strings.Contains(relType, "--") {
			assert.Error(t, err)
			return
		}
		if err != nil {
			return
		}

		got, err := s.GetRelation(bg, from, relType, to)
		require.NoError(t, err)
		assert.Equal(t, r.Key(), got.Key())
	})
}

// FuzzAttachmentKeyCollision verifies that attachment key construction
// rejects invalid property names and round-trips valid ones.
func FuzzAttachmentKeyCollision(f *testing.F, factory FuzzFactory) {
	f.Add("entity", "prop")
	f.Add("E-1", "screenshot")
	f.Add("E-1", "some/path")
	f.Add("", "prop")
	f.Add("E-1", "")

	f.Fuzz(func(t *testing.T, entityID, prop string) {
		s := factory()
		bg := context.Background()

		err := s.CreateEntity(bg, entity.New(entityID, "t"))
		if entityID == "" || strings.Contains(entityID, "--") {
			assert.Error(t, err)
			return
		}
		require.NoError(t, err)

		err = s.AttachFile(bg, entityID, prop, "f.txt", strings.NewReader("data"))
		if prop == "" || strings.Contains(prop, "/") {
			assert.Error(t, err)
			return
		}
		if err != nil {
			return
		}

		rc, err := s.ReadAttachment(bg, entityID, prop)
		require.NoError(t, err)
		rc.Close()
	})
}

// FuzzRenameKeyCollapse verifies rename never loses relations.
func FuzzRenameKeyCollapse(f *testing.F, factory FuzzFactory) {
	f.Add("A", "B", "C", "rel")
	f.Add("", "B", "C", "rel")
	f.Add("A", "B", "C", "")

	f.Fuzz(func(t *testing.T, id1, id2, id3, relType string) {
		if id1 == id2 || id1 == id3 || id2 == id3 {
			return
		}

		s := factory()
		bg := context.Background()

		if s.CreateEntity(bg, entity.New(id1, "t")) != nil {
			return
		}
		if s.CreateEntity(bg, entity.New(id2, "t")) != nil {
			return
		}
		if s.CreateEntity(bg, entity.New(id3, "t")) != nil {
			return
		}

		if _, err := s.CreateRelation(bg, id1, relType, id2, nil); err != nil {
			return
		}
		if _, err := s.CreateRelation(bg, id1, relType, id3, nil); err != nil {
			return
		}

		before := countRelations(t, s)

		_, err := s.RenameEntity(bg, id2, id3)
		if err != nil {
			return
		}

		after := countRelations(t, s)
		if after < before {
			t.Errorf("rename %q→%q lost relations: had %d, now %d",
				id2, id3, before, after)
		}
	})
}

// FuzzConcurrentOps verifies the store is safe under concurrent access.
func FuzzConcurrentOps(f *testing.F, factory FuzzFactory) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 0, 1, 2, 3})
	f.Add([]byte{0, 0, 0, 3, 3, 3, 5, 5, 5})       // heavy create/delete
	f.Add([]byte{0, 6, 7, 6, 7, 6, 7})              // subscribe/cancel churn
	f.Add([]byte{8, 9, 10, 8, 9, 10, 0, 3, 8, 9})   // relation ops

	f.Fuzz(func(t *testing.T, ops []byte) {
		if len(ops) < 2 || len(ops) > 100 {
			return
		}

		s := factory()
		bg := context.Background()

		for _, id := range []string{"E-1", "E-2", "E-3"} {
			_ = s.CreateEntity(bg, entity.New(id, "ticket"))
		}
		_, _ = s.CreateRelation(bg, "E-1", "blocks", "E-2", nil)

		var wg sync.WaitGroup
		wg.Add(len(ops))

		for _, op := range ops {
			op := op
			go func() {
				defer wg.Done()

				switch op % 11 {
				case 0: // CreateEntity
					_ = s.CreateEntity(bg, entity.New("E-new", "ticket"))
				case 1: // GetEntity
					_, _ = s.GetEntity(bg, "E-1")
				case 2: // UpdateEntity
					e := entity.New("E-1", "ticket")
					e.SetString("title", "updated")
					_ = s.UpdateEntity(bg, e)
				case 3: // DeleteEntity
					_, _ = s.DeleteEntity(bg, "E-3", true)
				case 4: // RenameEntity
					_, _ = s.RenameEntity(bg, "E-2", "E-renamed")
				case 5: // ListEntities
					for _, err := range s.ListEntities(bg, store.EntityQuery{}) {
						_ = err
					}
				case 6: // Subscribe + use
					events, cancel := s.Subscribe(1)
					select {
					case <-events:
					default:
					}
					cancel()
				case 7: // Close (tests double-close safety)
					_ = s.Close()
				case 8: // CreateRelation
					_, _ = s.CreateRelation(bg, "E-1", "needs", "E-3", nil)
				case 9: // ListRelations
					for _, err := range s.ListRelations(bg, store.RelationQuery{}) {
						_ = err
					}
				case 10: // Search
					for _, err := range s.Search(bg, store.SearchQuery{Text: "E-1"}) {
						_ = err
					}
				}
			}()
		}
		wg.Wait()
	})
}

// FuzzCloneNestedValues verifies deep clone semantics for nested property values.
func FuzzCloneNestedValues(f *testing.F, factory FuzzFactory) {
	f.Add("tags", 0)  // []string
	f.Add("meta", 1)  // map[string]interface{}
	f.Add("items", 2) // []interface{}

	f.Fuzz(func(t *testing.T, propName string, valueType int) {
		if propName == "id" || propName == "type" {
			return
		}

		s := factory()
		bg := context.Background()

		e := entity.New("T-1", "ticket")

		switch valueType % 3 {
		case 0:
			e.Properties[propName] = []string{"a", "b", "c"}
		case 1:
			e.Properties[propName] = map[string]interface{}{"key": "original"}
		case 2:
			e.Properties[propName] = []interface{}{"x", "y"}
		}

		require.NoError(t, s.CreateEntity(bg, e))

		clone, err := s.GetEntity(bg, "T-1")
		require.NoError(t, err)

		switch v := clone.Properties[propName].(type) {
		case []string:
			if len(v) > 0 {
				v[0] = "MUTATED"
			}
		case map[string]interface{}:
			v["key"] = "MUTATED"
		case []interface{}:
			if len(v) > 0 {
				v[0] = "MUTATED"
			}
		}

		original, err := s.GetEntity(bg, "T-1")
		require.NoError(t, err)

		switch v := original.Properties[propName].(type) {
		case []string:
			assert.NotContains(t, v, "MUTATED",
				"clone mutation leaked into stored entity ([]string)")
		case map[string]interface{}:
			assert.NotEqual(t, "MUTATED", v["key"],
				"clone mutation leaked into stored entity (map)")
		case []interface{}:
			for _, item := range v {
				assert.NotEqual(t, "MUTATED", item,
					"clone mutation leaked into stored entity ([]interface{})")
			}
		}
	})
}

// FuzzPropertyValuesTypeZoo verifies PropertyValues and Search filters
// handle all property value types without panicking.
func FuzzPropertyValuesTypeZoo(f *testing.F, factory FuzzFactory) {
	f.Add("prop", 0, "hello")
	f.Add("prop", 1, "42")
	f.Add("prop", 2, "true")
	f.Add("prop", 3, "")
	f.Add("prop", 4, "a,b,c")

	f.Fuzz(func(t *testing.T, propName string, valueType int, raw string) {
		if propName == "id" || propName == "type" {
			return
		}

		s := factory()
		bg := context.Background()

		e := entity.New("T-1", "ticket")

		switch valueType % 6 {
		case 0:
			e.Properties[propName] = raw
		case 1:
			e.Properties[propName] = len(raw)
		case 2:
			e.Properties[propName] = raw == "true"
		case 3:
			e.Properties[propName] = nil
		case 4:
			e.Properties[propName] = strings.Split(raw, ",")
		case 5:
			e.Properties[propName] = map[string]interface{}{"v": raw}
		}

		require.NoError(t, s.CreateEntity(bg, e))

		vals, err := s.PropertyValues(bg, propName, 10)
		require.NoError(t, err)
		_ = vals

		for _, op := range []store.FilterOp{
			store.FilterEq, store.FilterNe, store.FilterContains,
			store.FilterGt, store.FilterLt, store.FilterGte, store.FilterLte,
			store.FilterIn, store.FilterExists, store.FilterNotExists,
		} {
			for _, err := range s.Search(bg, store.SearchQuery{
				Filters: []store.PropertyFilter{{Property: propName, Value: raw, Op: op}},
			}) {
				require.NoError(t, err)
			}
		}
	})
}
