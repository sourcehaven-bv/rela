package fsstore_test

import (
	"context"
	"fmt"
	"io/fs"
	"maps"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// The tests in this file pin TKT-C2YVX1: fsstore refuses a write of an
// entity type its schemas do not declare. The file of such a write would
// land in a directory the scan on the next open skips, so the entity would
// be lost without an error.

// fileSnapshot returns every file under the MemFS root with its content.
func fileSnapshot(t *testing.T, mfs *storage.MemFS) map[string]string {
	t.Helper()
	out := map[string]string{}
	require.NoError(t, mfs.Walk("/", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, readErr := mfs.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		out[p] = string(data)
		return nil
	}))
	return out
}

// writeVia runs fn against the store directly, or against the view a Tx
// hands its callback.
type writeVia struct {
	name string
	run  func(t *testing.T, s *fsstore.FSStore, fn func(store.Store) error) error
}

var writeVias = []writeVia{
	{name: "direct", run: func(_ *testing.T, s *fsstore.FSStore, fn func(store.Store) error) error {
		return fn(s)
	}},
	{name: "tx", run: func(_ *testing.T, s *fsstore.FSStore, fn func(store.Store) error) error {
		return s.Tx(context.Background(), fn)
	}},
}

// TestUndeclaredType_WritesRefused covers AC1, AC2 and AC4: every write
// entry point that takes a caller-supplied type refuses an undeclared one,
// directly and through Tx, before any file is written or removed. The
// update cases change a declared entity's type, so a refusal that came
// after the write would leave a file behind or remove the old one.
func TestUndeclaredType_WritesRefused(t *testing.T) {
	ctx := context.Background()
	writes := []struct {
		name  string
		write func(st store.Store) error
	}{
		{name: "CreateEntity", write: func(st store.Store) error {
			return st.CreateEntity(ctx, entity.New("WID-1", "widget"))
		}},
		{name: "UpdateEntity", write: func(st store.Store) error {
			return st.UpdateEntity(ctx, entity.New("REQ-1", "widget"))
		}},
		{name: "UpdateEntityIf", write: func(st store.Store) error {
			cur, err := st.GetEntity(ctx, entity.Ref{ID: "REQ-1"})
			if err != nil {
				return err
			}
			_, err = st.UpdateEntityIf(ctx, entity.New("REQ-1", "widget"),
				store.UpdateCondition{ExpectedVersion: store.VersionOf(cur)})
			return err
		}},
	}
	for _, via := range writeVias {
		for _, w := range writes {
			t.Run(via.name+"/"+w.name, func(t *testing.T) {
				mfs := storage.NewMemFS()
				s := openStore(t, mfs)
				t.Cleanup(func() { _ = s.Close() })
				req := entity.New("REQ-1", "requirement")
				req.Properties["title"] = "Kept"
				require.NoError(t, s.CreateEntity(ctx, req))
				before := fileSnapshot(t, mfs)

				err := via.run(t, s, w.write)
				require.Error(t, err)
				assert.Contains(t, err.Error(), `unknown entity type "widget"`)

				assert.Equal(t, before, fileSnapshot(t, mfs), "a refused write must write and remove nothing")
				_, err = s.GetEntity(ctx, entity.Ref{ID: "WID-1"})
				require.ErrorIs(t, err, store.ErrNotFound)
				got, err := s.GetEntity(ctx, entity.Ref{ID: "REQ-1"})
				require.NoError(t, err)
				assert.Equal(t, "requirement", got.Type, "the entity keeps its declared type")
				n, err := s.CountEntities(ctx, store.EntityQuery{Type: "widget", Faces: store.AllFaces()})
				require.NoError(t, err)
				assert.Zero(t, n, "the refused entity must not be listed")
			})
		}
	}
}

// TestUndeclaredType_ReplaceFamilyRefused covers AC3 and its Tx half of
// AC4: a family replacement with a row of an undeclared type keeps
// returning store.ErrInvalidFamilyReplacement and leaves the old family as
// it was.
func TestUndeclaredType_ReplaceFamilyRefused(t *testing.T) {
	ctx := context.Background()
	for _, via := range writeVias {
		t.Run(via.name, func(t *testing.T) {
			mfs := storage.NewMemFS()
			s := openStore(t, mfs)
			t.Cleanup(func() { _ = s.Close() })
			require.NoError(t, s.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
			before := fileSnapshot(t, mfs)

			err := via.run(t, s, func(st store.Store) error {
				replacer, ok := st.(store.FamilyReplacer)
				require.True(t, ok, "fsstore views replace families")
				return replacer.ReplaceEntityFamily(ctx, "REQ-1", []store.FamilyReplacement{
					{Entity: entity.New("REQ-1", "widget")},
				})
			})
			require.ErrorIs(t, err, store.ErrInvalidFamilyReplacement)

			assert.Equal(t, before, fileSnapshot(t, mfs))
			got, err := s.GetEntity(ctx, entity.Ref{ID: "REQ-1"})
			require.NoError(t, err)
			assert.Equal(t, "requirement", got.Type)
		})
	}
}

// TestUndeclaredType_PathLikeTypeRefused covers AC6: a type string that
// looks like a path is not declared, so it cannot choose where a file goes.
func TestUndeclaredType_PathLikeTypeRefused(t *testing.T) {
	ctx := context.Background()
	for _, typ := range []string{"../relations/x", "", "requirements/../x"} {
		t.Run(typ, func(t *testing.T) {
			mfs := storage.NewMemFS()
			s := openStore(t, mfs)
			t.Cleanup(func() { _ = s.Close() })
			before := fileSnapshot(t, mfs)

			err := s.CreateEntity(ctx, entity.New("X-1", typ))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unknown entity type")
			assert.Equal(t, before, fileSnapshot(t, mfs), "nothing may be written anywhere")
		})
	}
}

// TestSchemas_EmptyPluralRoundTrips covers AC5: a declared type with no
// Plural is written to "<type>s" and found there on the next open. Before
// TKT-C2YVX1 the write used "<type>s" while the scan ignored the type, so
// the entity was lost on reopen.
func TestSchemas_EmptyPluralRoundTrips(t *testing.T) {
	ctx := context.Background()
	mfs := storage.NewMemFS()
	cfg := newConfig(mfs)
	cfg.Schemas = map[string]store.EntityTypeSchema{"gadget": {}}

	s1, err := fsstore.New(cfg)
	require.NoError(t, err)
	require.NoError(t, s1.CreateEntity(ctx, entity.New("GAD-1", "gadget")))
	require.NoError(t, s1.Close())
	_, err = mfs.Stat("/entities/gadgets/GAD-1.md")
	require.NoError(t, err, "an empty Plural means <type>s")

	s2, err := fsstore.New(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s2.Close() })
	got, err := s2.GetEntity(ctx, entity.Ref{ID: "GAD-1"})
	require.NoError(t, err, "the entity must survive a reopen")
	assert.Equal(t, "gadget", got.Type)
}

// TestSetSchemas_AddsWritableType covers AC7: after SetSchemas declares a
// type with a custom plural, a write of it lands in that directory and
// survives a reopen, and files already in the directory join the index.
func TestSetSchemas_AddsWritableType(t *testing.T) {
	ctx := context.Background()
	mfs := storage.NewMemFS()
	require.NoError(t, mfs.MkdirAll("/entities/gizmo-shelf", 0o755))
	require.NoError(t, mfs.WriteFile("/entities/gizmo-shelf/GZ-9.md",
		[]byte("---\nid: GZ-9\ntype: gizmo\n---\n"), 0o644))
	s := openStore(t, mfs)

	require.Error(t, s.CreateEntity(ctx, entity.New("GZ-1", "gizmo")), "precondition: gizmo is not declared yet")
	_, err := s.GetEntity(ctx, entity.Ref{ID: "GZ-9"})
	require.ErrorIs(t, err, store.ErrNotFound, "precondition: the directory is not scanned yet")

	withGizmo := maps.Clone(defaultTestSchemas)
	withGizmo["gizmo"] = store.EntityTypeSchema{Plural: "gizmo-shelf"}
	require.NoError(t, s.SetSchemas(withGizmo))

	got, err := s.GetEntity(ctx, entity.Ref{ID: "GZ-9"})
	require.NoError(t, err, "a file already in the new type's directory joins the index")
	assert.Equal(t, "gizmo", got.Type)
	require.NoError(t, s.CreateEntity(ctx, entity.New("GZ-1", "gizmo")))
	_, err = mfs.Stat("/entities/gizmo-shelf/GZ-1.md")
	require.NoError(t, err, "the write uses the declared plural")
	require.NoError(t, s.Close())

	cfg := newConfig(mfs)
	cfg.Schemas = withGizmo
	reopened, err := fsstore.New(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reopened.Close() })
	for _, id := range []string{"GZ-1", "GZ-9"} {
		_, err := reopened.GetEntity(ctx, entity.Ref{ID: id})
		require.NoError(t, err, "%s must survive a reopen", id)
	}
}

// recordingObserver records the observer calls a store makes.
type recordingObserver struct {
	mu      sync.Mutex
	put     []string
	deleted []string
}

func (o *recordingObserver) EntityPut(e *entity.Entity) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.put = append(o.put, e.ID)
	return nil
}

func (o *recordingObserver) EntityDelete(id string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.deleted = append(o.deleted, id)
	return nil
}

func (o *recordingObserver) EntityRenamed(string, *entity.Entity) error { return nil }

// drainEvents returns the events already buffered on ch.
func drainEvents(ch <-chan store.Event) []store.Event {
	var out []store.Event
	for {
		select {
		case ev := <-ch:
			out = append(out, ev)
		default:
			return out
		}
	}
}

// TestSetSchemas_AnnouncesIndexedFiles pins that a file joining the index
// through SetSchemas reaches observers (search), subscribers and the
// mention index, as a file the watcher finds does. It also pins the two
// files that must stay out: one whose id belongs to a soft-deleted family,
// and one whose id is already indexed under another type.
func TestSetSchemas_AnnouncesIndexedFiles(t *testing.T) {
	ctx := context.Background()
	mfs := storage.NewMemFS()
	obs := &recordingObserver{}
	cfg := newConfig(mfs)
	cfg.Observers = []store.EntityObserver{obs}
	s, err := fsstore.New(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	require.NoError(t, s.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
	require.NoError(t, s.CreateEntity(ctx, entity.New("REQ-7", "requirement")))
	_, err = s.SoftDelete().MarkDeleted(ctx, "REQ-7", "tester")
	require.NoError(t, err)
	require.NoError(t, mfs.MkdirAll("/entities/gizmo-shelf", 0o755))
	for id, body := range map[string]string{"GZ-9": "See `REQ-1`.\n", "REQ-1": "", "REQ-7": ""} {
		require.NoError(t, mfs.WriteFile("/entities/gizmo-shelf/"+id+".md",
			[]byte("---\nid: "+id+"\ntype: gizmo\n---\n"+body), 0o644))
	}
	// Build the mention index first, so the reload has to update it.
	_, err = s.MentionSources(ctx, "REQ-1", entity.Ref{}, 10)
	require.NoError(t, err)
	events, cancel := s.Subscribe(16)
	t.Cleanup(cancel)
	obs.put = nil

	withGizmo := maps.Clone(defaultTestSchemas)
	withGizmo["gizmo"] = store.EntityTypeSchema{Plural: "gizmo-shelf"}
	require.NoError(t, s.SetSchemas(withGizmo))

	assert.Equal(t, []string{"GZ-9"}, obs.put, "observers hear the new file and only that one")
	assert.Equal(t, []store.Event{{Op: store.EventEntityCreated, EntityType: "gizmo", EntityID: "GZ-9"}},
		drainEvents(events))
	refs, err := s.MentionSources(ctx, "REQ-1", entity.Ref{}, 10)
	require.NoError(t, err)
	assert.Equal(t, []entity.Ref{{ID: "GZ-9"}}, refs, "the mention index records the new file's links")

	got, err := s.GetEntity(ctx, entity.Ref{ID: "REQ-1"})
	require.NoError(t, err)
	assert.Equal(t, "requirement", got.Type, "an id collision keeps the indexed entity")
	_, err = s.GetEntity(ctx, entity.Ref{ID: "REQ-7"})
	require.ErrorIs(t, err, store.ErrNotFound, "a soft-deleted id stays hidden")
}

// TestSetSchemas_RemovedTypeLeavesIndex pins that a type the new schemas
// drop leaves the index as it would on reopen, announced as deletes, with
// its files left on disk. Declaring it again brings the entities back.
func TestSetSchemas_RemovedTypeLeavesIndex(t *testing.T) {
	ctx := context.Background()
	mfs := storage.NewMemFS()
	schemas := map[string]store.EntityTypeSchema{
		"requirement": {Plural: "reqs"},
		"solution":    {Plural: "solutions"},
	}
	obs := &recordingObserver{}
	cfg := newConfig(mfs)
	cfg.Schemas = schemas
	cfg.Observers = []store.EntityObserver{obs}
	s, err := fsstore.New(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	require.NoError(t, s.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
	require.NoError(t, s.CreateEntity(ctx, entity.New("SOL-1", "solution")))
	events, cancel := s.Subscribe(16)
	t.Cleanup(cancel)

	require.NoError(t, s.SetSchemas(map[string]store.EntityTypeSchema{"solution": {Plural: "solutions"}}))

	_, err = s.GetEntity(ctx, entity.Ref{ID: "REQ-1"})
	require.ErrorIs(t, err, store.ErrNotFound, "a removed type's entities leave the index")
	assert.Equal(t, []string{"REQ-1"}, obs.deleted)
	assert.Equal(t, []store.Event{{Op: store.EventEntityDeleted, EntityType: "requirement", EntityID: "REQ-1"}},
		drainEvents(events))
	_, err = mfs.Stat("/entities/reqs/REQ-1.md")
	require.NoError(t, err, "the file stays on disk")
	require.ErrorContains(t, s.CreateEntity(ctx, entity.New("REQ-2", "requirement")),
		`unknown entity type "requirement"`)
	_, err = s.GetEntity(ctx, entity.Ref{ID: "SOL-1"})
	require.NoError(t, err, "a type that stays declared is untouched")

	require.NoError(t, s.SetSchemas(schemas))
	_, err = s.GetEntity(ctx, entity.Ref{ID: "REQ-1"})
	require.NoError(t, err, "declaring the type again brings its entities back")
}

// TestSetSchemas_RemovedTypeKeepsSoftDeletes pins that a soft-deleted
// entity of a type a reload removes stays soft-deleted: declaring the type
// again must not bring it back as live. While the type is undeclared the
// soft-delete operations skip it, because its file cannot be located.
func TestSetSchemas_RemovedTypeKeepsSoftDeletes(t *testing.T) {
	ctx := context.Background()
	schemas := map[string]store.EntityTypeSchema{
		"requirement": {Plural: "reqs"},
		"solution":    {Plural: "solutions"},
	}
	cfg := newConfig(storage.NewMemFS())
	cfg.Schemas = schemas
	s, err := fsstore.New(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	require.NoError(t, s.CreateEntity(ctx, entity.New("REQ-7", "requirement")))
	_, err = s.SoftDelete().MarkDeleted(ctx, "REQ-7", "tester")
	require.NoError(t, err)

	require.NoError(t, s.SetSchemas(map[string]store.EntityTypeSchema{"solution": {Plural: "solutions"}}))
	marked, err := s.SoftDelete().ListMarked(ctx)
	require.NoError(t, err)
	assert.Empty(t, marked, "a family of an undeclared type is skipped")
	_, err = s.SoftDelete().Unmark(ctx, "REQ-7")
	require.ErrorIs(t, err, store.ErrNotFound)

	require.NoError(t, s.SetSchemas(schemas))
	_, err = s.GetEntity(ctx, entity.Ref{ID: "REQ-7"})
	require.ErrorIs(t, err, store.ErrNotFound, "re-declaring the type must not bring a soft-deleted entity back")
	marked, err = s.SoftDelete().ListMarked(ctx)
	require.NoError(t, err)
	require.Len(t, marked, 1)
	assert.Equal(t, "REQ-7", marked[0].ID)
	_, err = s.SoftDelete().Unmark(ctx, "REQ-7")
	require.NoError(t, err)
	_, err = s.GetEntity(ctx, entity.Ref{ID: "REQ-7"})
	require.NoError(t, err, "the mark still restores once the type is back")
}

// TestSetSchemas_PluralChange pins when a type may move to another
// directory on reload: at once when the index holds none of its entities,
// and only after a restart when it does.
func TestSetSchemas_PluralChange(t *testing.T) {
	ctx := context.Background()
	moved := maps.Clone(defaultTestSchemas)
	moved["requirement"] = store.EntityTypeSchema{Plural: "needs"}

	t.Run("no entities", func(t *testing.T) {
		mfs := storage.NewMemFS()
		s := openStore(t, mfs)
		t.Cleanup(func() { _ = s.Close() })
		require.NoError(t, s.SetSchemas(moved))
		require.NoError(t, s.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
		_, err := mfs.Stat("/entities/needs/REQ-1.md")
		require.NoError(t, err, "the write uses the new plural")
	})
	t.Run("entities in the old directory", func(t *testing.T) {
		mfs := storage.NewMemFS()
		require.NoError(t, mfs.MkdirAll("/entities/gizmos", 0o755))
		require.NoError(t, mfs.WriteFile("/entities/gizmos/GZ-9.md",
			[]byte("---\nid: GZ-9\ntype: gizmo\n---\n"), 0o644))
		s := openStore(t, mfs)
		t.Cleanup(func() { _ = s.Close() })
		require.NoError(t, s.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
		movedAndAdded := maps.Clone(moved)
		movedAndAdded["gizmo"] = store.EntityTypeSchema{Plural: "gizmos"}
		delete(movedAndAdded, "solution")
		require.NoError(t, s.CreateEntity(ctx, entity.New("SOL-1", "solution")))

		require.EqualError(t, s.SetSchemas(movedAndAdded),
			`fsstore: type "requirement": plural changed from "requirements" to "needs" while entities exist; `+
				`rename entities/requirements to entities/needs and restart`)

		require.NoError(t, s.CreateEntity(ctx, entity.New("REQ-2", "requirement")))
		_, err := mfs.Stat("/entities/requirements/REQ-2.md")
		require.NoError(t, err, "the refused type keeps its old directory")
		_, err = s.GetEntity(ctx, entity.Ref{ID: "GZ-9"})
		require.NoError(t, err, "an added type is applied despite the refusal")
		_, err = s.GetEntity(ctx, entity.Ref{ID: "SOL-1"})
		require.ErrorIs(t, err, store.ErrNotFound, "a removed type is applied despite the refusal")
	})
}

// TestSetSchemas_Refusals pins the inputs SetSchemas and New reject, each
// leaving the previous schemas in force. A plural must be one plain
// directory name: path.Join cleans "../relations" before RootedFS checks
// the key, so it would put entity files in relations/.
func TestSetSchemas_Refusals(t *testing.T) {
	tests := []struct {
		name    string
		schemas map[string]store.EntityTypeSchema
		want    string
	}{
		{name: "empty", schemas: map[string]store.EntityTypeSchema{}, want: "must be populated"},
		{name: "empty type name", schemas: map[string]store.EntityTypeSchema{"": {}}, want: "empty name"},
		{
			name:    "parent escape",
			schemas: map[string]store.EntityTypeSchema{"x": {Plural: "../relations"}},
			want:    `entity type "x": plural "../relations" is not a usable directory name`,
		},
		{
			name:    "dot dot",
			schemas: map[string]store.EntityTypeSchema{"x": {Plural: ".."}},
			want:    `entity type "x": plural ".." is not a usable directory name`,
		},
		{
			name:    "unsafe type name in the default plural",
			schemas: map[string]store.EntityTypeSchema{"a/b": {}},
			want:    `entity type "a/b": plural "a/bs" is not a usable directory name`,
		},
		{
			name: "duplicate plural",
			schemas: map[string]store.EntityTypeSchema{
				"item":  {},
				"thing": {Plural: "items"},
			},
			want: `entity types "item" and "thing" share the directory "items"`,
		},
		{
			name: "duplicate plural differing in case",
			schemas: map[string]store.EntityTypeSchema{
				"item":  {Plural: "Items"},
				"thing": {Plural: "items"},
			},
			want: `entity types "item" and "thing" share the directory "items"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			mfs := storage.NewMemFS()
			s := openStore(t, mfs)
			t.Cleanup(func() { _ = s.Close() })

			require.ErrorContains(t, s.SetSchemas(tc.schemas), tc.want)
			require.NoError(t, s.CreateEntity(ctx, entity.New("REQ-1", "requirement")),
				"the previous schemas stay in force")

			cfg := newConfig(mfs)
			cfg.Schemas = tc.schemas
			_, err := fsstore.New(cfg)
			require.ErrorContains(t, err, tc.want, "New refuses the same schemas")
		})
	}
}

// TestSetSchemas_ConcurrentWithReadsAndWrites runs SetSchemas beside
// writes and reads of a declared type, for the race detector: the schemas
// are published atomically and SetSchemas takes the write locks.
func TestSetSchemas_ConcurrentWithReadsAndWrites(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, storage.NewMemFS())
	t.Cleanup(func() { _ = s.Close() })
	withGizmo := maps.Clone(defaultTestSchemas)
	withGizmo["gizmo"] = store.EntityTypeSchema{Plural: "gizmos"}

	var wg sync.WaitGroup
	wg.Go(func() {
		for i := range 20 {
			next := defaultTestSchemas
			if i%2 == 0 {
				next = withGizmo
			}
			assert.NoError(t, s.SetSchemas(next))
		}
	})
	wg.Go(func() {
		for i := range 20 {
			assert.NoError(t, s.CreateEntity(ctx, entity.New(fmt.Sprintf("REQ-%d", i), "requirement")))
		}
	})
	wg.Go(func() {
		for range 20 {
			for _, err := range s.ListEntities(ctx, store.EntityQuery{Faces: store.AllFaces()}) {
				assert.NoError(t, err)
			}
		}
	})
	wg.Wait()
	n, err := s.CountEntities(ctx, store.EntityQuery{Type: "requirement", Faces: store.AllFaces()})
	require.NoError(t, err)
	assert.Equal(t, 20, n)
}
