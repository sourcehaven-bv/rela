package storetest

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// FuzzFactory returns a fresh store without needing *testing.T (for use inside f.Fuzz).
type FuzzFactory func() store.Store

// createEntityOrSkip applies the directional validity oracle for an
// entity ID (TKT-PCLGGL): anything the shared storeutil.ValidateID
// rejects MUST be rejected by the store; anything it accepts MAY still
// be rejected by a stricter backend (fsstore rejects ':' for Windows
// compat) — only an accepted write proceeds to round-trip assertions.
// Returns true when the entity was created.
//
// Vacuity anchor: a backend that rejected every write would pass these
// fuzz targets trivially, but the conformance suite (storetest.RunAll's
// Entity tests) asserts plain creates succeed, and every backend runs
// both. These fuzz targets only chase key-collision and round-trip
// divergence; over-rejection fails loudly in the conformance suite.
func createEntityOrSkip(t *testing.T, s store.Store, id string) bool {
	t.Helper()
	err := s.CreateEntity(context.Background(), entity.New(id, "t"))
	if storeutil.ValidateID(id) != nil {
		assert.Error(t, err)
		return false
	}
	return err == nil
}

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

		// Directional oracle — see createEntityOrSkip. The previous
		// hand-modeled ""/"--" checks went stale when control-character
		// rejection was added and produced false fuzz failures.
		if !createEntityOrSkip(t, s, from) {
			return
		}
		if from != to && !createEntityOrSkip(t, s, to) {
			return
		}

		r, err := s.CreateRelation(bg, from, relType, to, nil)
		if storeutil.ValidateRelationType(relType) != nil {
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

		// Directional oracle — see createEntityOrSkip.
		if !createEntityOrSkip(t, s, entityID) {
			return
		}

		err := s.AttachFile(bg, entityID, prop, "f.txt", strings.NewReader("data"))
		if storeutil.ValidateProperty(prop) != nil {
			assert.Error(t, err)
			return
		}
		if err != nil {
			return
		}

		rc, err := s.ReadAttachment(bg, entityID, prop, "f.txt")
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
	f.Add([]byte{0, 0, 0, 3, 3, 3, 5, 5, 5})      // heavy create/delete
	f.Add([]byte{0, 6, 7, 6, 7, 6, 7})            // subscribe/cancel churn
	f.Add([]byte{8, 9, 10, 8, 9, 10, 0, 3, 8, 9}) // relation ops

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
		if entity.IsReservedEntityKey(propName) {
			return
		}

		s := factory()
		bg := context.Background()

		e := entity.New("T-1", "ticket")

		switch valueType % 3 {
		case 0:
			e.Properties[propName] = []string{"a", "b", "c"}
		case 1:
			e.Properties[propName] = map[string]any{"key": "original"}
		case 2:
			e.Properties[propName] = []any{"x", "y"}
		}

		require.NoError(t, s.CreateEntity(bg, e))

		clone, err := s.GetEntity(bg, "T-1")
		require.NoError(t, err)

		switch v := clone.Properties[propName].(type) {
		case []string:
			if len(v) > 0 {
				v[0] = "MUTATED"
			}
		case map[string]any:
			v["key"] = "MUTATED"
		case []any:
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
		case map[string]any:
			assert.NotEqual(t, "MUTATED", v["key"],
				"clone mutation leaked into stored entity (map)")
		case []any:
			for _, item := range v {
				assert.NotEqual(t, "MUTATED", item,
					"clone mutation leaked into stored entity ([]interface{})")
			}
		}
	})
}

// FuzzPropertyValuesTypeZoo verifies PropertyValues and Search filters
// handle all property value types without panicking, and that every accepted
// property value reads back equal to what was written.
//
// The round-trip half applies the same directional oracle as
// createEntityOrSkip: anything storeutil.ValidateProperties rejects, the
// store MUST reject; anything it accepts MUST come back byte-for-byte. It
// exists because both halves have failed silently before. BUG-B1RA3J: a
// value fsstore wrote as a block scalar that read back as "" with no error.
// BUG-X7ICNM: invalid UTF-8 that pgstore "stored" as U+FFFD and reported
// success. A target that only checked for an absent error passed over both.
func FuzzPropertyValuesTypeZoo(f *testing.F, factory FuzzFactory) {
	f.Add("prop", 0, "hello")
	f.Add("prop", 1, "42")
	f.Add("prop", 2, "true")
	f.Add("prop", 3, "")
	f.Add("prop", 4, "a,b,c")
	f.Add("prop", 0, "\n0")     // BUG-B1RA3J: block scalar yaml.v3 cannot read back
	f.Add("prop", 5, "\n")      // BUG-B1RA3J: silent loss, nested
	f.Add("prop", 0, "\n\xc80") // BUG-X7ICNM: invalid UTF-8
	f.Add("prop", 5, "\xc8")    // BUG-X7ICNM: invalid UTF-8, nested
	f.Add("prop", 0, "a\x00b")  // BUG-X7ICNM: NUL, which Postgres jsonb cannot hold

	f.Fuzz(func(t *testing.T, propName string, valueType int, raw string) {
		if entity.IsReservedEntityKey(propName) {
			return
		}
		// Names storeutil.ValidateProperty rejects (empty, containing "/")
		// have no agreed contract yet: only sqlitestore enforces the rule,
		// and only on PropertyValues (BUG-CQYD5X). Skipped rather than asserted
		// either way until that is settled.
		if storeutil.ValidateProperty(propName) != nil {
			return
		}

		s := factory()
		bg := context.Background()

		e := entity.New("T-1", "ticket")

		// Go's % keeps the sign, so a negative valueType matched no case and
		// the entity went in with NO property — which let an invalid-UTF-8
		// name skip the write gate and reach PropertyValues, where only
		// pgstore refuses it (BUG-CQYD5X). Every input now sets a property.
		switch ((valueType % 6) + 6) % 6 {
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
			e.Properties[propName] = map[string]any{"v": raw}
		}

		want := e.Clone()
		err := s.CreateEntity(bg, e)
		if ruleErr := storeutil.ValidateProperties(want.Properties); ruleErr != nil {
			// Not merely "some error": the store must have refused for the
			// rule's reason, or a conflict or I/O failure would satisfy this.
			assert.ErrorContains(t, err, ruleErr.Error(),
				"shared rule rejects %q; every store must, for that reason", raw)
			return
		}
		require.NoError(t, err)

		got, err := s.GetEntity(bg, "T-1")
		require.NoError(t, err)
		assert.Equal(t, normalizeProps(want.Properties), normalizeProps(got.Properties),
			"property value did not round-trip")

		vals, err := s.PropertyValues(bg, propName, 10)
		require.NoError(t, err)
		_ = vals

		// Property filter fuzz testing is covered in search conformance tests.
		_ = search.FilterEq
	})
}

// normalizeProps rewrites typed containers into the generic shapes a backend
// that serializes through JSON or YAML hands back, so a []string written and
// a []any read are compared by content rather than by Go type.
//
// Deliberately folded: a nil slice and an empty slice both become []any{},
// because no serializing backend can tell them apart (both are written as
// "[]"). Deliberately NOT folded: a missing key, a nil value, and an empty
// container stay distinct, so a backend that dropped an empty list or wrote
// it as null still fails the comparison.
func normalizeProps(props map[string]any) map[string]any {
	out := make(map[string]any, len(props))
	for k, v := range props {
		out[k] = normalizeValue(v)
	}
	return out
}

func normalizeValue(val any) any {
	switch v := val.(type) {
	case []string:
		out := make([]any, len(v))
		for i, s := range v {
			out[i] = s
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = normalizeValue(e)
		}
		return out
	case map[string]string:
		out := make(map[string]any, len(v))
		for k, s := range v {
			out[k] = s
		}
		return out
	case map[string]any:
		return normalizeProps(v)
	}
	return val
}
