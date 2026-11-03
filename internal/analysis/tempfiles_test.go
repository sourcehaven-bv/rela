package analysis_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/analysis"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer/tracertest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// AC6: `rela gc --temp-files` reports stale temp files of every form and
// never a fresh one, which may be another process's write in flight. Under
// attachments/ only unfinished uploads count: a real upload named like a
// temp file is a user's file.
func TestFindOrphanedTempFiles_StaleOnly(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := &project.Context{
		Root:         root,
		EntitiesDir:  filepath.Join(root, "entities"),
		RelationsDir: filepath.Join(root, "relations"),
	}
	stale := storage.TempStaleAfter + time.Minute
	files := map[string]time.Duration{
		"entities/requirements/REQ-1.md.0123456789abcdef.tmp":      stale,
		"entities/requirements/REQ-2.md.tmp":                       stale,
		"entities/requirements/REQ-3.md.new":                       stale,
		"relations/A--rel--B.md.0123456789abcdef.tmp":              stale,
		"entities/requirements/REQ-4.md.fedcba9876543210.tmp":      time.Second,
		"relations/A--rel--C.md.new":                               time.Second,
		"entities/requirements/REQ-5.md":                           stale,
		"attachments/DOC-1/spec/.rela-attach-tmp-0123456789abcdef": stale,
		"attachments/DOC-1/spec/.rela-attach-tmp-fedcba9876543210": time.Second,
		"attachments/DOC-1/spec/report.new":                        stale,
	}
	for rel, age := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		mtime := time.Now().Add(-age)
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}

	st := memstore.New()
	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{}}
	tr := tracertest.Must(st, store.TrivialScope())
	svc, err := analysis.New(analysis.Deps{
		Store: st, Meta: meta, Tracer: tr, FS: storage.NewOsFS(), Paths: paths,
		LuaReadDeps: lua.ReadDeps{
			VisibleReader: visibility.Unrestricted(st).WithWorld(visibility.WorldOf(store.TrivialScope())),
			Tracer:        tr, Meta: meta, World: store.TrivialScope(),
		},
	})
	if err != nil {
		t.Fatalf("analysis.New: %v", err)
	}

	got, err := svc.FindOrphanedTempFiles()
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range got {
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			t.Fatal(relErr)
		}
		got[i] = filepath.ToSlash(rel)
	}
	slices.Sort(got)
	want := []string{
		"attachments/DOC-1/spec/.rela-attach-tmp-0123456789abcdef",
		"entities/requirements/REQ-1.md.0123456789abcdef.tmp",
		"entities/requirements/REQ-2.md.tmp",
		"entities/requirements/REQ-3.md.new",
		"relations/A--rel--B.md.0123456789abcdef.tmp",
	}
	if !slices.Equal(got, want) {
		t.Errorf("FindOrphanedTempFiles = %v, want %v", got, want)
	}
}
