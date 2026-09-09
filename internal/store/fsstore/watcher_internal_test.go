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
	// New installs the self-echo recorder on the MemFS itself
	// (BUG-S24X52) — no manual OnPostWrite here, so these tests
	// exercise the same wiring production gets.
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
		Properties: map[string]any{"status": "open"},
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

// TestNewInstallsSelfEchoRecorder pins that New itself wires the
// echo tracker to the FS's post-write observer (BUG-S24X52): a write
// made through the store, replayed to the reconciler as if fsnotify
// had reported it, is recognized as a self-echo and produces no
// second event. newTestStore deliberately does NOT call OnPostWrite.
func TestNewInstallsSelfEchoRecorder(t *testing.T) {
	s, fs := newTestStore(t)
	ch, cancel := s.Subscribe(16)
	defer cancel()

	require.NoError(t, s.CreateEntity(context.Background(), &entity.Entity{
		ID: "REQ-1", Type: "requirement",
		Properties: map[string]any{"title": "one"},
	}))
	require.Len(t, drainEvents(ch), 1, "the store's own Created event")

	path := "/entities/requirements/REQ-1.md"
	_, err := fs.ReadFile(path)
	require.NoError(t, err, "the write must have landed on the FS")

	s.handleExternalEvents([]storage.ChangeEvent{{Path: path, Op: storage.OpModify}})
	assert.Empty(t, drainEvents(ch), "a self-write replayed by the watcher must be suppressed")
	require.NoError(t, s.echoWiringErr)
}

// TestTwoStoresOnOneFSKeepTheirOwnEchoes pins the fan-out contract
// (BUG-S24X52 review C1): opening a second store on the same FS must
// not evict the first store's recorder, and each store records only
// writes under its own root.
func TestTwoStoresOnOneFSKeepTheirOwnEchoes(t *testing.T) {
	fs := storage.NewMemFS()
	open := func(root string) *FSStore {
		t.Helper()
		require.NoError(t, fs.MkdirAll(root, 0o755))
		rooted, err := storage.NewRootedFS(fs, root)
		require.NoError(t, err)
		s, err := New(Config{
			FS: fs, Rooted: rooted,
			EntitiesKey: "entities", RelationsKey: "relations", CacheKey: ".rela",
			Schemas: map[string]store.EntityTypeSchema{"requirement": {Plural: "requirements"}},
		})
		require.NoError(t, err)
		return s
	}
	s1 := open("/a")
	s2 := open("/b")
	defer s2.Close()

	ch1, cancel1 := s1.Subscribe(16)
	defer cancel1()
	require.NoError(t, s1.CreateEntity(context.Background(), &entity.Entity{ID: "REQ-1", Type: "requirement"}))
	require.Len(t, drainEvents(ch1), 1)

	p1 := "/a/entities/requirements/REQ-1.md"
	data, err := fs.ReadFile(p1)
	require.NoError(t, err)
	assert.True(t, s1.echoes.IsEcho(p1, data), "s1 must still be recording after s2 opened")
	assert.False(t, s2.echoes.IsEcho(p1, data), "s2 must not record a write under s1's root")

	s1.handleExternalEvents([]storage.ChangeEvent{{Path: p1, Op: storage.OpModify}})
	assert.Empty(t, drainEvents(ch1), "s1's own write replayed must be suppressed even with s2 alive")

	// Close uninstalls: a later write at s1's path is no longer recorded.
	require.NoError(t, s1.Close())
	require.NoError(t, fs.WriteFile(p1, []byte("---\nid: REQ-1\ntype: requirement\n---\nlater\n"), 0o644))
	assert.False(t, s1.echoes.IsEcho(p1, []byte("---\nid: REQ-1\ntype: requirement\n---\nlater\n")),
		"a closed store must not keep recording")
}

// opaqueFS hides MemFS's OnPostWrite so the store sees an FS that
// cannot be observed — the shape a bare OsFS has.
type opaqueFS struct{ storage.FS }

func TestStartWatchingRefusesUnobservableFS(t *testing.T) {
	fs := opaqueFS{storage.NewMemFS()}
	rooted, err := storage.NewRootedFS(fs, "/")
	require.NoError(t, err)
	s, err := New(Config{
		FS:           fs,
		Rooted:       rooted,
		EntitiesKey:  "entities",
		RelationsKey: "relations",
		CacheKey:     ".rela",
		Schemas: map[string]store.EntityTypeSchema{
			"requirement": {Plural: "requirements"},
		},
	})
	require.NoError(t, err, "opening without an observable FS is allowed (CLI paths never watch)")
	require.ErrorIs(t, s.echoWiringErr, ErrWatchNeedsObservableFS)

	err = s.StartWatching()
	require.ErrorIs(t, err, ErrWatchNeedsObservableFS)
}
