package fsstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func newTestStore(t *testing.T) (*FSStore, *storage.MemFS) {
	t.Helper()
	fs := storage.NewMemFS()
	rooted, err := storage.NewRootedFS(fs, "/")
	require.NoError(t, err)
	s, err := New(Config{
		FS:           fs,
		Rooted:       rooted,
		EntitiesKey:  "entities",
		RelationsKey: "relations",
		CacheKey:     ".rela",
		Schemas: map[string]store.EntityTypeSchema{
			"requirement": {Plural: "requirements", PropertyOrder: []string{"title", "status"}},
			"solution":    {Plural: "solutions", PropertyOrder: []string{"title"}},
		},
	})
	require.NoError(t, err)
	// Mirror production wiring: subscribe the store's RecordWrite to
	// the filesystem's post-write hook so the watcher's self-echo
	// LRU sees the bytes that actually landed.
	fs.OnPostWrite(s.RecordWrite)
	return s, fs
}

func drainEvents(ch <-chan store.Event) []store.Event {
	var out []store.Event
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, ev)
		default:
			return out
		}
	}
}

func TestExternalCreateEmitsCreated(t *testing.T) {
	s, fs := newTestStore(t)
	ch, cancel := s.Subscribe(16)
	defer cancel()

	path := "/entities/tickets/T-1.md"
	body := "---\nid: T-1\ntype: ticket\n---\n\nBody\n"
	require.NoError(t, fs.MkdirAll("/entities/tickets", 0o755))
	require.NoError(t, fs.WriteFileExternal(path, []byte(body), 0o644))

	s.handleExternalEvents([]storage.ChangeEvent{{Path: path, Op: storage.OpCreate}})

	events := drainEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, store.EventEntityCreated, events[0].Op)
	assert.Equal(t, "T-1", events[0].EntityID)
	assert.Equal(t, "ticket", events[0].EntityType)

	e, err := s.GetEntity(context.Background(), "T-1")
	require.NoError(t, err)
	assert.Equal(t, "ticket", e.Type)
}

func TestExternalUpdateEmitsUpdated(t *testing.T) {
	s, fs := newTestStore(t)

	require.NoError(t, s.CreateEntity(context.Background(), &entity.Entity{
		ID:         "T-1",
		Type:       "ticket",
		Properties: map[string]interface{}{"status": "open"},
	}))

	ch, cancel := s.Subscribe(16)
	defer cancel()

	path := "/entities/tickets/T-1.md"
	body := "---\nid: T-1\ntype: ticket\nstatus: closed\n---\n\nUpdated\n"
	require.NoError(t, fs.WriteFileExternal(path, []byte(body), 0o644))

	s.handleExternalEvents([]storage.ChangeEvent{{Path: path, Op: storage.OpModify}})

	events := drainEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, store.EventEntityUpdated, events[0].Op)
	assert.Equal(t, "T-1", events[0].EntityID)

	e, err := s.GetEntity(context.Background(), "T-1")
	require.NoError(t, err)
	assert.Equal(t, "closed", e.Properties["status"])
}

func TestExternalDeleteEmitsDeleted(t *testing.T) {
	s, fs := newTestStore(t)

	require.NoError(t, s.CreateEntity(context.Background(), &entity.Entity{
		ID:   "T-1",
		Type: "ticket",
	}))

	ch, cancel := s.Subscribe(16)
	defer cancel()

	path := "/entities/tickets/T-1.md"
	require.NoError(t, fs.Remove(path))

	s.handleExternalEvents([]storage.ChangeEvent{{Path: path, Op: storage.OpDelete}})

	events := drainEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, store.EventEntityDeleted, events[0].Op)
	assert.Equal(t, "T-1", events[0].EntityID)

	_, err := s.GetEntity(context.Background(), "T-1")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestSelfWriteIsSuppressed(t *testing.T) {
	s, _ := newTestStore(t)

	// Create via the store API — this records the hash and emits one event.
	require.NoError(t, s.CreateEntity(context.Background(), &entity.Entity{
		ID:   "T-1",
		Type: "ticket",
	}))

	ch, cancel := s.Subscribe(16)
	defer cancel()

	// Simulate the fsnotify echo of the write we just did.
	path := "/entities/tickets/T-1.md"
	s.handleExternalEvents([]storage.ChangeEvent{{Path: path, Op: storage.OpCreate}})

	events := drainEvents(ch)
	assert.Empty(t, events, "self-write should not emit a duplicate event")
}

func TestExternalRelationChange(t *testing.T) {
	s, fs := newTestStore(t)

	require.NoError(t, s.CreateEntity(context.Background(), &entity.Entity{ID: "A", Type: "ticket"}))
	require.NoError(t, s.CreateEntity(context.Background(), &entity.Entity{ID: "B", Type: "ticket"}))

	ch, cancel := s.Subscribe(16)
	defer cancel()

	path := "/relations/A--blocks--B.md"
	body := "---\nfrom: A\nrelation: blocks\nto: B\n---\n"
	require.NoError(t, fs.MkdirAll("/relations", 0o755))
	require.NoError(t, fs.WriteFileExternal(path, []byte(body), 0o644))

	s.handleExternalEvents([]storage.ChangeEvent{{Path: path, Op: storage.OpCreate}})

	events := drainEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, store.EventRelationCreated, events[0].Op)
	assert.Equal(t, "A", events[0].From)
	assert.Equal(t, "B", events[0].To)
	assert.Equal(t, "blocks", events[0].RelationType)
}

func TestExternalRelationDelete(t *testing.T) {
	s, fs := newTestStore(t)

	require.NoError(t, s.CreateEntity(context.Background(), &entity.Entity{ID: "A", Type: "ticket"}))
	require.NoError(t, s.CreateEntity(context.Background(), &entity.Entity{ID: "B", Type: "ticket"}))
	_, err := s.CreateRelation(context.Background(), "A", "blocks", "B", nil)
	require.NoError(t, err)

	ch, cancel := s.Subscribe(16)
	defer cancel()

	path := "/relations/A--blocks--B.md"
	require.NoError(t, fs.Remove(path))

	s.handleExternalEvents([]storage.ChangeEvent{{Path: path, Op: storage.OpDelete}})

	events := drainEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, store.EventRelationDeleted, events[0].Op)
}

func TestNonMarkdownPathIgnored(t *testing.T) {
	s, fs := newTestStore(t)

	ch, cancel := s.Subscribe(16)
	defer cancel()

	require.NoError(t, fs.MkdirAll("/entities/tickets", 0o755))
	require.NoError(t, fs.WriteFileExternal("/entities/tickets/note.txt", []byte("hi"), 0o644))

	s.handleExternalEvents([]storage.ChangeEvent{
		{Path: "/entities/tickets/note.txt", Op: storage.OpCreate},
	})

	assert.Empty(t, drainEvents(ch))
}

func TestOutsideWatchedDirsIgnored(t *testing.T) {
	s, fs := newTestStore(t)

	ch, cancel := s.Subscribe(16)
	defer cancel()

	require.NoError(t, fs.MkdirAll("/other", 0o755))
	require.NoError(t, fs.WriteFileExternal("/other/stray.md", []byte("---\nid: X\ntype: y\n---\n"), 0o644))

	s.handleExternalEvents([]storage.ChangeEvent{
		{Path: "/other/stray.md", Op: storage.OpCreate},
	})

	assert.Empty(t, drainEvents(ch))
}

func TestHasPathPrefix(t *testing.T) {
	cases := []struct {
		path, dir string
		want      bool
	}{
		{"/entities/tickets/T-1.md", "/entities", true},
		{"/entities/tickets/T-1.md", "/entities/", true},
		{"/entities", "/entities", false},
		{"/entities-other/x.md", "/entities", false},
		{"/other/x.md", "/entities", false},
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, hasPathPrefix(c.path, c.dir),
			"hasPathPrefix(%q, %q)", c.path, c.dir)
	}
}
