package app_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// recordingObserver captures every EntityPut / EntityDelete /
// EntityRenamed it receives. Used by tests that assert the factory
// wires observers into the store correctly.
type recordingObserver struct {
	puts    []*entity.Entity
	deletes []string
	renames []renameRecord
}

type renameRecord struct {
	OldID   string
	Renamed *entity.Entity
}

func (r *recordingObserver) EntityPut(e *entity.Entity) error {
	r.puts = append(r.puts, e)
	return nil
}

func (r *recordingObserver) EntityDelete(id string) error {
	r.deletes = append(r.deletes, id)
	return nil
}

func (r *recordingObserver) EntityRenamed(oldID string, renamed *entity.Entity) error {
	r.renames = append(r.renames, renameRecord{OldID: oldID, Renamed: renamed})
	return nil
}

func (r *recordingObserver) putIDs() []string {
	ids := make([]string, 0, len(r.puts))
	for _, e := range r.puts {
		ids = append(ids, e.ID)
	}
	return ids
}

var _ store.EntityObserver = (*recordingObserver)(nil)

func TestFSFactoryOpensWorkingStore(t *testing.T) {
	root := t.TempDir()
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths := &project.Context{
		Root:         root,
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
		CacheDir:     filepath.Join(root, ".rela"),
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"policy": {Plural: "policies"},
		},
	}

	factory := &app.FSFactory{FS: fs, Paths: paths}
	s, err := factory.OpenStore(meta)
	require.NoError(t, err)
	defer s.Close()

	require.NoError(t, s.CreateEntity(context.Background(), &entity.Entity{
		ID:   "POL-1",
		Type: "policy",
	}))

	data, err := os.ReadFile(filepath.Join(root, "entities", "policies", "POL-1.md"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "id: POL-1")
}

func TestFSFactoryObserversReceiveWrites(t *testing.T) {
	root := t.TempDir()
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths := &project.Context{
		Root:         root,
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
		CacheDir:     filepath.Join(root, ".rela"),
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"policy": {Plural: "policies"},
		},
	}

	rec := &recordingObserver{}
	factory := &app.FSFactory{FS: fs, Paths: paths}
	factory.AddObserver(rec)
	s, err := factory.OpenStore(meta)
	require.NoError(t, err)
	defer s.Close()

	ctx := context.Background()
	require.NoError(t, s.CreateEntity(ctx, &entity.Entity{
		ID:   "POL-1",
		Type: "policy",
	}))
	_, err = s.RenameEntity(ctx, "POL-1", "POL-2")
	require.NoError(t, err)
	_, err = s.DeleteEntity(ctx, "POL-2", false)
	require.NoError(t, err)

	// Contract per store.EntityObserver:
	//   Create POL-1 → EntityPut(POL-1).
	//   Rename POL-1→POL-2 → EntityRenamed("POL-1", POL-2) ONLY.
	//     (No EntityDelete/EntityPut pair — that was the pre-rename-event
	//     contract; the rename test pins the new single-event contract.)
	//   Delete POL-2 → EntityDelete(POL-2).
	assert.Equal(t, []string{"POL-1"}, rec.putIDs(),
		"create fires put; rename does NOT fire a put")
	assert.Equal(t, []string{"POL-2"}, rec.deletes,
		"final delete fires delete; rename does NOT fire a delete")
	require.Len(t, rec.renames, 1, "rename should fire exactly one EntityRenamed")
	assert.Equal(t, "POL-1", rec.renames[0].OldID)
	require.NotNil(t, rec.renames[0].Renamed)
	assert.Equal(t, "POL-2", rec.renames[0].Renamed.ID,
		"renamed entity carries the new ID so content-driven observers don't need a store lookup")
}

func TestFSFactoryOpenStoreReturnsIndependentStores(t *testing.T) {
	root := t.TempDir()
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths := &project.Context{
		Root:         root,
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
		CacheDir:     filepath.Join(root, ".rela"),
	}

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"policy": {Plural: "policies"},
		},
	}

	factory := &app.FSFactory{FS: fs, Paths: paths}
	s1, err := factory.OpenStore(meta)
	require.NoError(t, err)
	defer s1.Close()

	s2, err := factory.OpenStore(meta)
	require.NoError(t, err)
	defer s2.Close()

	assert.NotSame(t, s1, s2, "each OpenStore call returns a fresh store")
}

// storeWatcher is the watcher capability the production wiring
// (dataentry, mcp) type-asserts on the opened store. Declared here at
// the consumer, like those sites do.
type storeWatcher interface {
	StartWatching() error
	StopWatching()
}

// TestFSFactoryWatcherSuppressesSelfEcho pins the production wiring
// of the fsstore self-echo LRU (BUG-S24X52). It opens the store the
// way appbuild does — FSFactory over SafeFS(OsFS) on a real
// directory — starts the fsnotify watcher, and performs one write
// through the store followed by one genuinely external write that
// bypasses SafeFS. Exactly two events must surface: the store's own
// synchronous Created, and the external Created. If the post-write
// observer is not installed by construction, the watcher re-parses
// the self-write and a spurious Updated for POL-1 appears before the
// external event.
//
// The assertion waits on a POSITIVE signal (the external file's
// Created event) rather than a wall-clock silence: fsnotify delivers
// events for one directory tree in order and POL-1 was written first,
// so by the time POL-2's event has been reconciled, POL-1's has too.
func TestFSFactoryWatcherSuppressesSelfEcho(t *testing.T) {
	root := t.TempDir()
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths := &project.Context{
		Root:         root,
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
		CacheDir:     filepath.Join(root, ".rela"),
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"policy": {Plural: "policies"},
		},
	}
	// The directories must exist before the watcher starts — including
	// the per-type leaf: the watcher adds new subdirectories on their
	// Create event, and a file written into a directory created a
	// moment earlier can slip through unobserved. That would make this
	// test pass because the self-write was never SEEN, not because it
	// was suppressed.
	require.NoError(t, os.MkdirAll(filepath.Join(paths.EntitiesDir, "policies"), 0o755))
	require.NoError(t, os.MkdirAll(paths.RelationsDir, 0o755))

	rec := &recordingObserver{}
	factory := &app.FSFactory{FS: fs, Paths: paths}
	factory.AddObserver(rec)
	s, err := factory.OpenStore(meta)
	require.NoError(t, err)
	defer s.Close()

	w, ok := s.(storeWatcher)
	require.True(t, ok, "fsstore must expose StartWatching/StopWatching")
	require.NoError(t, w.StartWatching())
	defer w.StopWatching()

	events, cancel := s.Subscribe(16)
	defer cancel()

	ctx := context.Background()
	require.NoError(t, s.CreateEntity(ctx, &entity.Entity{ID: "POL-1", Type: "policy"}))

	// An external edit: bypass SafeFS entirely, as a user or another
	// process would.
	external := filepath.Join(paths.EntitiesDir, "policies", "POL-2.md")
	require.NoError(t, os.WriteFile(external, []byte("---\nid: POL-2\ntype: policy\n---\n"), 0o644))

	var got []store.Event
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-events:
			got = append(got, ev)
		case <-deadline:
			t.Fatalf("timed out waiting for the external create of POL-2; events so far: %+v", got)
		}
		if len(got) > 0 && got[len(got)-1].EntityID == "POL-2" {
			break
		}
	}

	want := []store.Event{
		{Op: store.EventEntityCreated, EntityType: "policy", EntityID: "POL-1"},
		{Op: store.EventEntityCreated, EntityType: "policy", EntityID: "POL-2"},
	}
	assert.Equal(t, want, got,
		"the store's own write must not be re-delivered by the watcher as an Updated")
	assert.Equal(t, []string{"POL-1", "POL-2"}, rec.putIDs(),
		"observers must see each entity exactly once")
}
