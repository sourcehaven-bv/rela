package workspace

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

func TestSyncCountsFromStore(t *testing.T) {
	t.Run("nil store", func(t *testing.T) {
		entities, relations := syncCountsFromStore(nil)
		if entities != 0 || relations != 0 {
			t.Errorf("expected 0,0 got %d,%d", entities, relations)
		}
	})

	t.Run("counts entities and valid relations", func(t *testing.T) {
		s := memstore.New()
		ctx := context.Background()
		_ = s.CreateEntity(ctx, entity.New("A", "t"))
		_ = s.CreateEntity(ctx, entity.New("B", "t"))
		_ = s.CreateEntity(ctx, entity.New("C", "t"))
		_, _ = s.CreateRelation(ctx, "A", "r", "B", nil)
		_, _ = s.CreateRelation(ctx, "B", "r", "C", nil)

		entities, relations := syncCountsFromStore(s)
		if entities != 3 {
			t.Errorf("entities = %d, want 3", entities)
		}
		if relations != 2 {
			t.Errorf("relations = %d, want 2", relations)
		}
	})
}

func TestPathsAccessor(t *testing.T) {
	fs := storage.NewMemFS()
	_ = fs.MkdirAll("/p/.rela", 0o755)
	paths := &project.Context{
		Root:         "/p",
		CacheDir:     "/p/.rela",
		EntitiesDir:  "/p/entities",
		RelationsDir: "/p/relations",
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{"t": {}},
	}
	ws := NewBare(fs, paths, meta)

	if ws.Paths() != paths {
		t.Error("Paths() returned different context")
	}
}

func TestConfigAndStateFallbacks(t *testing.T) {
	// Workspace with no fs/paths falls back to nop.
	ws := NewForTestWithStore(memstore.New(), &metamodel.Metamodel{})
	ctx := context.Background()

	cfg := ws.Config()
	if cfg == nil {
		t.Fatal("Config returned nil")
	}
	if _, err := cfg.Load(ctx, "anything"); err == nil {
		t.Error("expected nop config to return error")
	}

	st := ws.State()
	if st == nil {
		t.Fatal("State returned nil")
	}
	if _, err := st.Get(ctx, "k"); err == nil {
		t.Error("expected nop state Get to return error")
	}
	if err := st.Put(ctx, "k", []byte("v")); err == nil {
		t.Error("expected nop state Put to return error")
	}
}

func TestFindAndCleanupOrphanedTempFiles(t *testing.T) {
	fs := storage.NewMemFS()
	_ = fs.MkdirAll("/p/entities/tickets", 0o755)
	_ = fs.MkdirAll("/p/relations", 0o755)
	_ = fs.MkdirAll("/p/.rela", 0o755)

	// Orphaned temp files.
	_ = fs.WriteFile("/p/entities/tickets/TKT-001.md.new", []byte("partial"), 0o644)
	_ = fs.WriteFile("/p/relations/A--rel--B.md.new", []byte("partial"), 0o644)
	// Regular files that should NOT be flagged.
	_ = fs.WriteFile("/p/entities/tickets/TKT-002.md", []byte("real"), 0o644)

	paths := &project.Context{
		Root:         "/p",
		CacheDir:     "/p/.rela",
		EntitiesDir:  "/p/entities",
		RelationsDir: "/p/relations",
	}
	meta := &metamodel.Metamodel{}
	ws := NewBare(fs, paths, meta)

	orphaned, err := ws.FindOrphanedTempFiles()
	if err != nil {
		t.Fatalf("FindOrphanedTempFiles error: %v", err)
	}
	if len(orphaned) != 2 {
		t.Errorf("expected 2 orphans, got %d: %v", len(orphaned), orphaned)
	}

	removed, err := ws.CleanupOrphanedTempFiles()
	if err != nil {
		t.Fatalf("CleanupOrphanedTempFiles error: %v", err)
	}
	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}

	// Regular file still there.
	if _, err := fs.Stat("/p/entities/tickets/TKT-002.md"); err != nil {
		t.Errorf("regular file should still exist: %v", err)
	}
	// Temp files gone.
	if _, err := fs.Stat("/p/entities/tickets/TKT-001.md.new"); err == nil {
		t.Error("orphaned temp file should have been removed")
	}
}

func TestFindOrphanedTempFiles_NoFS(t *testing.T) {
	ws := NewForTestWithStore(memstore.New(), &metamodel.Metamodel{})
	orphaned, err := ws.FindOrphanedTempFiles()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if orphaned != nil {
		t.Errorf("expected nil, got %v", orphaned)
	}
}
