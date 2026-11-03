package fsstore_test

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// openOSStore opens an fsstore on the real filesystem at root, as one CLI
// process would.
func openOSStore(tb testing.TB, root string) *fsstore.FSStore {
	tb.Helper()
	fs := storage.NewOsFS()
	rooted, err := storage.NewRootedFS(fs, root)
	require.NoError(tb, err)
	s, err := fsstore.New(fsstore.Config{
		FS:             fs,
		Rooted:         rooted,
		EntitiesKey:    "entities",
		RelationsKey:   "relations",
		AttachmentsKey: "attachments",
		CacheKey:       ".rela",
		Schemas:        defaultTestSchemas,
	})
	require.NoError(tb, err)
	return s
}

// A store that closes after another process wrote files must not hide
// those files from the next open. An index saved on close used to record
// the first store's view with the folder times after the second store's
// write, so the next open trusted it and lost the new files.
func TestIndex_ConcurrentWriterIsSeenOnReopen(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	a := openOSStore(t, root)

	b := openOSStore(t, root)
	require.NoError(t, b.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
	require.NoError(t, b.CreateEntity(ctx, entity.New("SOL-1", "solution")))
	_, err := b.CreateRelation(ctx, entity.RelationKey{From: "SOL-1", Type: "implements", To: "REQ-1"}, nil)
	require.NoError(t, err)
	require.NoError(t, b.Close())

	require.NoError(t, a.Close())

	c := openOSStore(t, root)
	defer func() { _ = c.Close() }()
	_, err = c.GetEntity(ctx, entity.Ref{ID: "REQ-1"})
	require.NoError(t, err, "the entity written by the other store must be found")
	_, err = c.GetRelation(ctx, entity.RelationKey{From: "SOL-1", Type: "implements", To: "REQ-1"})
	require.NoError(t, err, "the relation written by the other store must be found")
}

// A schema change is seen on the next open: a type added for a directory
// that already holds files picks those files up, and a type the schema no
// longer declares drops its files from the index.
func TestIndex_SchemaChangeIsSeenOnReopen(t *testing.T) {
	withWidget := maps.Clone(defaultTestSchemas)
	withWidget["widget"] = store.EntityTypeSchema{Plural: "widgets", PropertyOrder: []string{"title"}}

	for _, tc := range []struct {
		name          string
		before, after map[string]store.EntityTypeSchema
		found         bool
	}{
		{name: "type added", before: defaultTestSchemas, after: withWidget, found: true},
		{name: "type removed", before: withWidget, after: defaultTestSchemas, found: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			fs := storage.NewMemFS()
			require.NoError(t, fs.MkdirAll("/entities/widgets", 0o755))
			require.NoError(t, fs.WriteFile("/entities/widgets/WID-1.md",
				[]byte("---\nid: WID-1\ntype: widget\ntitle: A widget\n---\n"), 0o644))

			openWith := func(schemas map[string]store.EntityTypeSchema) *fsstore.FSStore {
				t.Helper()
				cfg := newConfig(fs)
				cfg.Schemas = schemas
				s, err := fsstore.New(cfg)
				require.NoError(t, err)
				return s
			}

			before := openWith(tc.before)
			_, err := before.GetEntity(ctx, entity.Ref{ID: "WID-1"})
			if tc.found {
				require.ErrorIs(t, err, store.ErrNotFound, "precondition: the type is not in the layout yet")
			} else {
				require.NoError(t, err, "precondition: the type is in the layout")
			}
			require.NoError(t, before.Close())

			after := openWith(tc.after)
			defer func() { _ = after.Close() }()
			got, err := after.GetEntity(ctx, entity.Ref{ID: "WID-1"})
			if !tc.found {
				require.ErrorIs(t, err, store.ErrNotFound)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "widget", got.Type)
		})
	}
}

// Close writes no index file, and an index file left by an older version
// does not change what the next open finds.
func TestIndex_NoPersistedIndex(t *testing.T) {
	ctx := context.Background()
	fs := storage.NewMemFS()

	s1 := openStore(t, fs)
	require.NoError(t, s1.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
	require.NoError(t, s1.CreateEntity(ctx, entity.New("REQ-2", "requirement")))
	require.NoError(t, s1.Close())

	const indexPath = "/.rela/fsstore-index.json"
	_, err := fs.Stat(indexPath)
	require.ErrorIs(t, err, os.ErrNotExist, "Close must not write an index file")

	// Plant the format older versions wrote, with folder times that match
	// the tree and REQ-2 missing. Those versions trusted such a file.
	entitiesDir, err := fs.Stat("/entities")
	require.NoError(t, err)
	typeDir, err := fs.Stat("/entities/requirements")
	require.NoError(t, err)
	newest := entitiesDir.ModTime()
	if typeDir.ModTime().After(newest) {
		newest = typeDir.ModTime()
	}
	stale, err := json.Marshal(map[string]any{
		"entities_dir_mtime": newest,
		"entities": map[string]any{
			"REQ-1": map[string]any{"id": "REQ-1", "type": "requirement"},
		},
	})
	require.NoError(t, err)
	require.NoError(t, fs.MkdirAll("/.rela", 0o755))
	require.NoError(t, fs.WriteFile(indexPath, stale, 0o644))

	s2 := openStore(t, fs)
	defer func() { _ = s2.Close() }()
	_, err = s2.GetEntity(ctx, entity.Ref{ID: "REQ-2"})
	require.NoError(t, err, "a stale index file must have no effect")
}

// BenchmarkOpen measures the cost of opening a store over a tree of a few
// thousand entity and relation files on the real filesystem. Opening scans
// the directories and reads no file bodies.
func BenchmarkOpen(b *testing.B) {
	const (
		perType   = 1000
		relations = 2000
	)
	root := b.TempDir()
	types := []string{"requirements", "solutions", "tickets"}
	for _, plural := range types {
		dir := filepath.Join(root, "entities", plural)
		require.NoError(b, os.MkdirAll(dir, 0o755))
		for i := range perType {
			name := filepath.Join(dir, fmt.Sprintf("%s-%04d.md", strings.ToUpper(plural[:3]), i))
			require.NoError(b, os.WriteFile(name, []byte("---\ntitle: x\n---\n"), 0o644))
		}
	}
	relDir := filepath.Join(root, "relations")
	require.NoError(b, os.MkdirAll(relDir, 0o755))
	for i := range relations {
		name := filepath.Join(relDir, fmt.Sprintf("SOL-%04d--implements--REQ-%04d.md", i%perType, i/2))
		require.NoError(b, os.WriteFile(name, []byte("---\n---\n"), 0o644))
	}

	b.ReportAllocs()
	for b.Loop() {
		s := openOSStore(b, root)
		require.NoError(b, s.Close())
	}
}
