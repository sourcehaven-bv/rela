package storetest

import (
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// familyReplacerOf returns the store's conversion capability or fails the
// test: a backend that declares Capabilities.FamilyReplace must have it.
func familyReplacerOf(t *testing.T, s store.Store) store.FamilyReplacer {
	t.Helper()
	r, ok := s.(store.FamilyReplacer)
	require.True(t, ok, "store declared Capabilities.FamilyReplace but does not implement store.FamilyReplacer")
	return r
}

// seedConvertible stores FEAT-050 at the given faces, titled by face.
func seedConvertible(t *testing.T, s store.Store, faces ...entity.Face) {
	t.Helper()
	for _, face := range faces {
		e := entity.New("FEAT-050", "feature")
		e.Face = face
		e.SetString("title", "at "+face.String())
		e.Content = "body " + face.String()
		require.NoError(t, s.CreateEntity(ctx(), e))
	}
}

// replacementRow builds the converted requirement row for one target face,
// carrying the title of the source face it came from.
func replacementRow(source, target entity.Face) store.FamilyReplacement {
	e := entity.New("FEAT-050", "requirement")
	e.Face = target
	e.SetString("title", "at "+source.String())
	e.Content = "body " + source.String()
	return store.FamilyReplacement{SourceFace: source, Entity: e}
}

// familyFaces returns "type|face|title" for every stored row of FEAT-050.
func familyFaces(t *testing.T, s store.Store) []string {
	t.Helper()
	var out []string
	for e, err := range s.ListEntities(ctx(), store.EntityQuery{IDs: []string{"FEAT-050"}, Faces: store.AllFaces()}) {
		require.NoError(t, err)
		out = append(out, e.Type+"|"+e.Face.String()+"|"+e.GetString("title"))
	}
	sort.Strings(out)
	return out
}

// drainEvents collects the events delivered within eventTimeout of the
// last one.
func drainEvents(events <-chan store.Event) []store.Event {
	var out []store.Event
	for {
		select {
		case ev := <-events:
			out = append(out, ev)
		case <-time.After(eventTimeout):
			return out
		}
	}
}

// faceIndex is a face-aware observer that keeps the live (id|face) → type
// set, the way a search index would.
type faceIndex struct{ rows map[string]string }

func (x *faceIndex) EntityPut(e *entity.Entity) error {
	x.rows[e.ID+"|"+e.Face.String()] = e.Type
	return nil
}

func (x *faceIndex) EntityDelete(string) error { return nil }

func (x *faceIndex) EntityRenamed(string, *entity.Entity) error { return nil }

func (x *faceIndex) EntityFaceDelete(id string, p entity.Face) error {
	delete(x.rows, id+"|"+p.String())
	return nil
}

// RunFamilyReplaceTests pins [store.FamilyReplacer]: one validation rule set
// for every backend, a face swap that never trips the family key, and a
// delete event for every coordinate the conversion empties.
func RunFamilyReplaceTests(t *testing.T, f Factory, caps Capabilities) {
	t.Run("ChangesType", func(t *testing.T) {
		s := f(t)
		seedConvertible(t, s, "draft")
		require.NoError(t, familyReplacerOf(t, s).ReplaceEntityFamily(ctx(), "FEAT-050",
			[]store.FamilyReplacement{replacementRow("draft", "draft")}))
		assert.Equal(t, []string{"requirement|draft|at draft"}, familyFaces(t, s))
		got, err := s.GetEntity(ctx(), entity.Ref{ID: "FEAT-050", Face: "draft"})
		require.NoError(t, err)
		assert.Equal(t, "body draft", got.Content)
	})

	t.Run("SwapsFaces", func(t *testing.T) {
		s := f(t)
		seedConvertible(t, s, "draft", "approved")
		require.NoError(t, familyReplacerOf(t, s).ReplaceEntityFamily(ctx(), "FEAT-050", []store.FamilyReplacement{
			replacementRow("draft", "approved"),
			replacementRow("approved", "draft"),
		}))
		assert.Equal(t, []string{"requirement|approved|at draft", "requirement|draft|at approved"}, familyFaces(t, s))
	})

	t.Run("RejectsInvalidPlans", func(t *testing.T) {
		mixed := replacementRow("approved", "approved")
		mixed.Entity.Type = "solution"
		cases := []struct {
			name string
			plan []store.FamilyReplacement
		}{
			{"shared target face", []store.FamilyReplacement{
				replacementRow("draft", "approved"), replacementRow("approved", "approved"),
			}},
			{"mixed types", []store.FamilyReplacement{replacementRow("draft", "draft"), mixed}},
			{"unaccounted face", []store.FamilyReplacement{replacementRow("draft", "draft")}},
			{"repeated source", []store.FamilyReplacement{
				replacementRow("draft", "draft"), replacementRow("draft", "approved"),
			}},
			{"unknown source", []store.FamilyReplacement{
				replacementRow("draft", "draft"), replacementRow("approved", "approved"), replacementRow("other", "other"),
			}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				s := f(t)
				seedConvertible(t, s, "draft", "approved")
				before := familyFaces(t, s)
				err := familyReplacerOf(t, s).ReplaceEntityFamily(ctx(), "FEAT-050", tc.plan)
				require.ErrorIs(t, err, store.ErrInvalidFamilyReplacement)
				assert.Equal(t, before, familyFaces(t, s), "a rejected plan must not change any row")
			})
		}
	})

	t.Run("MissingFamilyIsNotFound", func(t *testing.T) {
		s := f(t)
		err := familyReplacerOf(t, s).ReplaceEntityFamily(ctx(), "FEAT-050",
			[]store.FamilyReplacement{replacementRow("draft", "draft")})
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("EmptiedFaceEmitsDelete", func(t *testing.T) {
		s := f(t)
		seedConvertible(t, s, "draft")
		events, cancel := s.Subscribe(10)
		defer cancel()
		require.NoError(t, familyReplacerOf(t, s).ReplaceEntityFamily(ctx(), "FEAT-050",
			[]store.FamilyReplacement{replacementRow("draft", "approved")}))
		var ops []string
		for _, ev := range drainEvents(events) {
			ops = append(ops, fmt.Sprint(ev.Op)+"|"+ev.EntityType+"|"+ev.Face.String())
		}
		assert.ElementsMatch(t, []string{
			fmt.Sprint(store.EventEntityUpdated) + "|requirement|approved",
			fmt.Sprint(store.EventEntityDeleted) + "|feature|draft",
		}, ops)
	})

	if caps.Observers != nil {
		t.Run("BareIDObserverDropsOldType", func(t *testing.T) {
			bare := &faceRecorder{}
			aware := &faceAwareRecorder{}
			s := caps.Observers(t, bare, aware)
			seedConvertible(t, s, "draft", "approved")
			*bare, *aware = faceRecorder{}, faceAwareRecorder{}
			require.NoError(t, familyReplacerOf(t, s).ReplaceEntityFamily(ctx(), "FEAT-050", []store.FamilyReplacement{
				replacementRow("draft", "approved"),
				replacementRow("approved", "draft"),
			}))
			assert.Equal(t, []string{"FEAT-050"}, bare.deletes, "bare-id observer must drop the old document once")
			assert.ElementsMatch(t, []string{"FEAT-050|approved", "FEAT-050|draft"}, bare.puts)
			assert.ElementsMatch(t, []string{"FEAT-050|approved", "FEAT-050|draft"}, aware.faceDeletes)
			assert.ElementsMatch(t, []string{"FEAT-050|approved", "FEAT-050|draft"}, aware.puts)
		})

		t.Run("FaceObserverKeepsSwappedFaces", func(t *testing.T) {
			idx := &faceIndex{rows: map[string]string{}}
			s := caps.Observers(t, idx)
			seedConvertible(t, s, "draft", "approved")
			require.NoError(t, familyReplacerOf(t, s).ReplaceEntityFamily(ctx(), "FEAT-050", []store.FamilyReplacement{
				replacementRow("draft", "approved"),
				replacementRow("approved", "draft"),
			}))
			assert.Equal(t, map[string]string{
				"FEAT-050|approved": "requirement",
				"FEAT-050|draft":    "requirement",
			}, idx.rows, "a face delete must never follow the put that refilled the face")
		})
	}

	if caps.TxRollback {
		t.Run("RolledBack", func(t *testing.T) {
			s := f(t)
			seedConvertible(t, s, "draft", "approved")
			before := familyFaces(t, s)
			sentinel := errors.New("boom")
			err := s.Tx(ctx(), func(tx store.Store) error {
				if err := familyReplacerOf(t, tx).ReplaceEntityFamily(ctx(), "FEAT-050", []store.FamilyReplacement{
					replacementRow("draft", "approved"),
					replacementRow("approved", "draft"),
				}); err != nil {
					return err
				}
				return sentinel
			})
			require.ErrorIs(t, err, sentinel)
			assert.Equal(t, before, familyFaces(t, s))
		})
	}
}
