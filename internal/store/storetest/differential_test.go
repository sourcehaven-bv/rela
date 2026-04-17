package storetest_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStores() (store.Store, store.Store) {
	mem := memstore.New()
	fs := storage.NewMemFS()
	fss, err := fsstore.New(fsstore.Config{
		FS:             fs,
		EntitiesDir:    "/entities",
		RelationsDir:   "/relations",
		AttachmentsDir: "/attachments",
		CacheDir:       "/.rela",
	})
	if err != nil {
		panic(err)
	}
	return mem, fss
}

// collectEntities drains an entity iterator into a sorted slice.
func collectEntities(it func(func(*entity.Entity, error) bool)) ([]*entity.Entity, error) {
	var out []*entity.Entity
	var retErr error
	for e, err := range it {
		if err != nil {
			retErr = err
			break
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, retErr
}

// collectRelations drains a relation iterator into a sorted slice.
func collectRelations(it func(func(*entity.Relation, error) bool)) ([]*entity.Relation, error) {
	var out []*entity.Relation
	var retErr error
	for r, err := range it {
		if err != nil {
			retErr = err
			break
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out, retErr
}

// FuzzDifferential runs identical fuzzed operation sequences against memstore
// and fsstore, asserting that all observable state matches after each operation.
func FuzzDifferential(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})
	f.Add([]byte{0, 0, 0, 5, 5, 5, 8, 8, 8})       // heavy create/delete
	f.Add([]byte{0, 3, 7, 0, 3, 7})                  // create/update/rename cycles
	f.Add([]byte{0, 1, 4, 6, 9, 10, 11})             // relation + search paths
	f.Add([]byte{0, 1, 2, 0, 1, 2, 5, 5, 8, 8})     // create many, delete many

	f.Fuzz(func(t *testing.T, ops []byte) {
		if len(ops) < 2 || len(ops) > 50 {
			return
		}

		mem, fss := newStores()
		bg := context.Background()

		// Subscribe to both stores to compare events.
		memEvents, memCancel := mem.Subscribe(100)
		defer memCancel()
		fssEvents, fssCancel := fss.Subscribe(100)
		defer fssCancel()

		// Seed both stores identically.
		for _, s := range []store.Store{mem, fss} {
			e1 := entity.New("E-1", "ticket")
			e1.SetString("status", "open")
			e1.SetString("title", "First")
			require.NoError(t, s.CreateEntity(bg, e1))

			e2 := entity.New("E-2", "ticket")
			e2.SetString("status", "closed")
			require.NoError(t, s.CreateEntity(bg, e2))

			e3 := entity.New("E-3", "feature")
			e3.SetString("status", "open")
			require.NoError(t, s.CreateEntity(bg, e3))

			_, err := s.CreateRelation(bg, "E-1", "blocks", "E-2", nil)
			require.NoError(t, err)
		}

		nextID := 4

		for _, op := range ops {
			switch op % 12 {
			case 0: // CreateEntity
				id := fmt.Sprintf("E-%d", nextID)
				nextID++
				e := entity.New(id, "ticket")
				e.SetString("status", "open")
				err1 := mem.CreateEntity(bg, e.Clone())
				err2 := fss.CreateEntity(bg, e.Clone())
				assertSameError(t, err1, err2, "CreateEntity %s", id)

			case 1: // GetEntity
				got1, err1 := mem.GetEntity(bg, "E-1")
				got2, err2 := fss.GetEntity(bg, "E-1")
				assertSameError(t, err1, err2, "GetEntity E-1")
				if err1 == nil {
					assertSameEntity(t, got1, got2)
				}

			case 2: // CountEntities
				c1, err1 := mem.CountEntities(bg, store.EntityQuery{})
				c2, err2 := fss.CountEntities(bg, store.EntityQuery{})
				assertSameError(t, err1, err2, "CountEntities")
				if err1 == nil {
					assert.Equal(t, c1, c2, "CountEntities mismatch")
				}

			case 3: // UpdateEntity
				e := entity.New("E-1", "ticket")
				e.SetString("status", "updated")
				e.SetString("title", "Modified")
				err1 := mem.UpdateEntity(bg, e.Clone())
				err2 := fss.UpdateEntity(bg, e.Clone())
				assertSameError(t, err1, err2, "UpdateEntity E-1")

			case 4: // CreateRelation
				_, err1 := mem.CreateRelation(bg, "E-1", "needs", "E-3", nil)
				_, err2 := fss.CreateRelation(bg, "E-1", "needs", "E-3", nil)
				assertSameError(t, err1, err2, "CreateRelation E-1→E-3")

			case 5: // DeleteEntity (cascade)
				_, err1 := mem.DeleteEntity(bg, "E-2", true)
				_, err2 := fss.DeleteEntity(bg, "E-2", true)
				assertSameError(t, err1, err2, "DeleteEntity E-2")

			case 6: // ListRelations
				rels1, err1 := collectRelations(mem.ListRelations(bg, store.RelationQuery{}))
				rels2, err2 := collectRelations(fss.ListRelations(bg, store.RelationQuery{}))
				assertSameError(t, err1, err2, "ListRelations")
				if err1 == nil {
					assert.Equal(t, len(rels1), len(rels2), "ListRelations count mismatch")
					for i := range rels1 {
						if i < len(rels2) {
							assert.Equal(t, rels1[i].Key(), rels2[i].Key(), "relation key mismatch at %d", i)
						}
					}
				}

			case 7: // RenameEntity
				_, err1 := mem.RenameEntity(bg, "E-3", "E-RENAMED")
				_, err2 := fss.RenameEntity(bg, "E-3", "E-RENAMED")
				assertSameError(t, err1, err2, "RenameEntity E-3→E-RENAMED")

			case 8: // DeleteRelation
				err1 := mem.DeleteRelation(bg, "E-1", "blocks", "E-2")
				err2 := fss.DeleteRelation(bg, "E-1", "blocks", "E-2")
				assertSameError(t, err1, err2, "DeleteRelation")

			case 9: // PropertyValues
				vals1, err1 := mem.PropertyValues(bg, "status", 0)
				vals2, err2 := fss.PropertyValues(bg, "status", 0)
				assertSameError(t, err1, err2, "PropertyValues status")
				if err1 == nil {
					sort.Strings(vals1)
					sort.Strings(vals2)
					assert.Equal(t, vals1, vals2, "PropertyValues mismatch")
				}

			case 10: // AttachFile + ReadAttachment
				data := "attachment-content"
				err1 := mem.AttachFile(bg, "E-1", "diagram", "pic.png", strings.NewReader(data))
				err2 := fss.AttachFile(bg, "E-1", "diagram", "pic.png", strings.NewReader(data))
				assertSameError(t, err1, err2, "AttachFile")
				if err1 == nil {
					rc1, e1 := mem.ReadAttachment(bg, "E-1", "diagram")
					rc2, e2 := fss.ReadAttachment(bg, "E-1", "diagram")
					assertSameError(t, e1, e2, "ReadAttachment")
					if e1 == nil {
						d1, _ := io.ReadAll(rc1)
						d2, _ := io.ReadAll(rc2)
						rc1.Close()
						rc2.Close()
						assert.Equal(t, string(d1), string(d2), "attachment content mismatch")
					}
				}
			}
		}

		// Final consistency check: compare full entity and relation listings.
		ents1, err1 := collectEntities(mem.ListEntities(bg, store.EntityQuery{}))
		ents2, err2 := collectEntities(fss.ListEntities(bg, store.EntityQuery{}))
		assertSameError(t, err1, err2, "final ListEntities")
		if err1 == nil {
			assertSameEntityList(t, ents1, ents2, "final entities")
		}

		rels1, err1 := collectRelations(mem.ListRelations(bg, store.RelationQuery{}))
		rels2, err2 := collectRelations(fss.ListRelations(bg, store.RelationQuery{}))
		assertSameError(t, err1, err2, "final ListRelations")
		if err1 == nil {
			assert.Equal(t, len(rels1), len(rels2), "final relation count mismatch")
		}

		// Compare events emitted by both stores.
		var memEvts, fssEvts []store.Event
		for {
			select {
			case e := <-memEvents:
				memEvts = append(memEvts, e)
			default:
				goto doneMemEvents
			}
		}
	doneMemEvents:
		for {
			select {
			case e := <-fssEvents:
				fssEvts = append(fssEvts, e)
			default:
				goto doneFssEvents
			}
		}
	doneFssEvents:
		if assert.Equal(t, len(memEvts), len(fssEvts), "event count mismatch: mem=%d fs=%d", len(memEvts), len(fssEvts)) {
			for i := range memEvts {
				assert.Equal(t, memEvts[i].Op, fssEvts[i].Op, "event[%d] Op mismatch", i)
				assert.Equal(t, memEvts[i].EntityID, fssEvts[i].EntityID, "event[%d] EntityID mismatch", i)
				assert.Equal(t, memEvts[i].EntityType, fssEvts[i].EntityType, "event[%d] EntityType mismatch", i)
				assert.Equal(t, memEvts[i].RelationType, fssEvts[i].RelationType, "event[%d] RelationType mismatch", i)
				assert.Equal(t, memEvts[i].From, fssEvts[i].From, "event[%d] From mismatch", i)
				assert.Equal(t, memEvts[i].To, fssEvts[i].To, "event[%d] To mismatch", i)
			}
		}
	})
}

// assertSameError checks that both errors are either both nil or both non-nil
// with the same sentinel.
func assertSameError(t *testing.T, err1, err2 error, msg string, args ...interface{}) {
	t.Helper()
	prefix := fmt.Sprintf(msg, args...)
	if err1 == nil && err2 == nil {
		return
	}
	if (err1 == nil) != (err2 == nil) {
		t.Errorf("%s: error mismatch: mem=%v fs=%v", prefix, err1, err2)
		return
	}
	// Both non-nil — check same sentinel.
	for _, sentinel := range []error{store.ErrNotFound, store.ErrConflict, store.ErrHasRelations} {
		mem := errors.Is(err1, sentinel)
		fs := errors.Is(err2, sentinel)
		if mem != fs {
			t.Errorf("%s: sentinel mismatch for %v: mem=%v fs=%v", prefix, sentinel, mem, fs)
		}
	}
}

func assertSameEntity(t *testing.T, e1, e2 *entity.Entity) {
	t.Helper()
	assert.Equal(t, e1.ID, e2.ID, "entity ID")
	assert.Equal(t, e1.Type, e2.Type, "entity type")
	assert.Equal(t, e1.Properties, e2.Properties, "entity properties for %s", e1.ID)
}

func assertSameEntityList(t *testing.T, list1, list2 []*entity.Entity, label string) {
	t.Helper()
	if !assert.Equal(t, len(list1), len(list2), "%s: count mismatch", label) {
		return
	}
	for i := range list1 {
		assertSameEntity(t, list1[i], list2[i])
	}
}
