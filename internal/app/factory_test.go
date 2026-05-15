package app_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// recordingObserver captures every EntityPut / EntityDelete it
// receives. Used by tests that assert the factory wires observers
// into the store correctly.
type recordingObserver struct {
	puts    []*entity.Entity
	deletes []string
}

func (r *recordingObserver) EntityPut(e *entity.Entity) error {
	r.puts = append(r.puts, e)
	return nil
}

func (r *recordingObserver) EntityDelete(id string) error {
	r.deletes = append(r.deletes, id)
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

	// Create POL-1 → put(POL-1). Rename POL-1→POL-2 → delete(POL-1) + put(POL-2).
	// Delete POL-2 → delete(POL-2).
	assert.Equal(t, []string{"POL-1", "POL-2"}, rec.putIDs(),
		"observer should see one put per create and one per rename target")
	assert.Equal(t, []string{"POL-1", "POL-2"}, rec.deletes,
		"observer should see one delete per rename source and one per delete")
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
