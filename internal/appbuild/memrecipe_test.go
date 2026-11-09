package appbuild

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/datamigration/filemigstate"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// memWorldSchema has one required property (validation), two automations
// (one writes a file through rela.write_file), and commenting switched on, so
// a write through the world touches each of them.
const memWorldSchema = `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "TKT-"
    id_type: sequential
    properties:
      title: {type: string, required: true}
      status: {type: string}
      note: {type: string}
relations: {}
comments:
  enabled: true
  on: [ticket]
automations:
  - name: close-note
    on:
      entity: [ticket]
      property: status
      becomes: closed
    do:
      - set: note
        value: closed by automation
  - name: close-report
    on:
      entity: [ticket]
      property: status
      becomes: closed
    do:
      - lua: |
          rela.write_file("report.txt", "closed")
        capabilities: {write_file: true}
`

// memWorldPolicy lets alice write tickets and bob only read them.
const memWorldPolicy = `roles:
  editor:
    read: [ticket]
    create: [ticket]
    update: [ticket]
  viewer:
    read: [ticket]
assignments:
  alice: editor
  bob: viewer
`

// writeMemWorldProject lays down a project that uses every subsystem that
// writes runtime state: comments and piles on disk under .rela/, an entity
// file, and a pending data migration recorded against an older shape.
func writeMemWorldProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "schema.yaml", memWorldSchema)
	writeFile(t, root, "acl.yaml", memWorldPolicy)
	writeFile(t, root, "entities/tickets/TKT-1.md", "---\nid: TKT-1\ntype: ticket\ntitle: On disk\n---\n")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "relations"), 0o755))

	paths := &project.Context{Root: root, CacheDir: filepath.Join(root, project.CacheDir)}
	meta, err := metamodel.Parse([]byte(memWorldSchema))
	require.NoError(t, err)
	disk := storage.NewOsFS()
	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolCLI})

	// Comments and piles through the same file backends the default build
	// uses, so the layout on disk is the real one.
	commentSvc, err := buildComments(disk, paths, meta, nil)
	require.NoError(t, err)
	_, err = commentSvc.Add(ctx, comments.Target{Type: "ticket", ID: "TKT-1"}, comments.AddRequest{Anchor: titleAnchor, Body: "on disk"})
	require.NoError(t, err)

	pileStore, err := pileBackend(disk, paths, memstore.New())
	require.NoError(t, err)
	now := time.Now().UTC()
	require.NoError(t, pileStore.CreatePile(ctx, piles.Pile{
		ID: "pile-1", Owner: "alice", Name: "Inbox", Icon: piles.DefaultIcon, Created: now, Updated: now,
	}, []entity.Ref{{ID: "TKT-1"}}, 10, 10))

	// A pending migration: the record says the data has a `legacy` property
	// the live schema dropped, and the drafted file is not in the applied list.
	// The world does not see this record (its MemFS starts empty, so the gate
	// reports the store as unbaselined); it is here so the tree check covers
	// migrations/applied.json, which the world's record writes would hit.
	older, err := metamodel.Parse([]byte(strings.Replace(memWorldSchema,
		"      note: {type: string}\n", "      note: {type: string}\n      legacy: {type: string}\n", 1)))
	require.NoError(t, err)
	olderShape := metamodel.NewShapeProjection(older)
	draft, err := datamigration.Generate(olderShape, metamodel.NewShapeProjection(meta), "drop legacy", now)
	require.NoError(t, err)
	require.NotNil(t, draft, "dropping a property must draft a migration")
	writeFile(t, root, filepath.Join("migrations", draft.FileName), string(draft.Content))
	projection, err := olderShape.JSON()
	require.NoError(t, err)
	record, err := filemigstate.New(disk, root)
	require.NoError(t, err)
	require.NoError(t, record.Save(context.Background(), &datamigration.State{
		FormatVersion: datamigration.StateFormatVersion, Projection: projection, UpdatedAt: now,
	}))
	return root
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}

// treeHash describes every path under root by its mode, modification time
// and content hash, one sorted line per path. Directory modification times
// are included, so a file created and removed again (a lock file) still
// changes the result. Lines rather than one digest, so a failure names the
// path that changed.
func treeHash(t *testing.T, root string) []string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		line := fmt.Sprintf("%s %s %d", rel, info.Mode(), info.ModTime().UnixNano())
		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			h := sha256.New()
			_, err = io.Copy(h, f)
			_ = f.Close()
			if err != nil {
				return err
			}
			line += " " + hex.EncodeToString(h.Sum(nil))
		}
		lines = append(lines, line)
		return nil
	})
	require.NoError(t, err)
	sort.Strings(lines)
	return lines
}

func memWorldAs(t *testing.T, svc *Services, user string) context.Context {
	t.Helper()
	ctx, err := MustIdentityBinder(svc)(principal.With(context.Background(),
		principal.Principal{User: user, Tool: principal.ToolMCP}))
	require.NoError(t, err)
	return ctx
}

// TestNewInMemory_LeavesProjectTreeUntouched builds a world, writes through
// every subsystem that keeps runtime state, closes it, and checks the project
// tree is byte-identical.
func TestNewInMemory_LeavesProjectTreeUntouched(t *testing.T) {
	root := writeMemWorldProject(t)
	before := treeHash(t, root)

	svc, err := NewInMemory(root)
	require.NoError(t, err)
	_, isMem := svc.FS().(*storage.MemFS)
	require.True(t, isMem, "Config.FS must be a MemFS")

	alice := memWorldAs(t, svc, "alice")
	created, err := svc.EntityManager().CreateEntity(alice,
		&entity.Entity{Type: "ticket", Properties: map[string]any{"title": "In memory", "status": "open"}},
		entity.CreateOptions{})
	require.NoError(t, err)
	id := created.Entity.ID
	res, err := svc.EntityManager().PatchEntity(alice, id, entity.Patch{Properties: map[string]any{"status": "closed"}})
	require.NoError(t, err)
	require.Empty(t, res.AutomationErrors)
	// rela.write_file lands in the world's temporary output directory.
	report := filepath.Join(svc.base.opts.seams.outputDir, "report.txt")
	_, err = os.Stat(report)
	require.NoError(t, err, "write_file must write to the default output directory")

	_, err = svc.Comments().Add(alice, comments.Target{Type: "ticket", ID: id},
		comments.AddRequest{Anchor: titleAnchor, Body: "hi"})
	require.NoError(t, err)
	_, err = svc.Piles().Create(alice, piles.CreateRequest{Name: "Today", Refs: []entity.Ref{{ID: id}}})
	require.NoError(t, err)
	_, err = svc.leases.Claim(alice, lease.ClaimRequest{Addr: id})
	require.NoError(t, err)
	require.NoError(t, svc.State().Put(context.Background(), "memworld/key", []byte("v")))
	require.NoError(t, svc.MigState().Save(context.Background(), &datamigration.State{
		FormatVersion: datamigration.StateFormatVersion, Projection: []byte(`{}`), UpdatedAt: time.Now().UTC(),
	}))

	require.NoError(t, svc.Close())
	require.Equal(t, before, treeHash(t, root), "the project tree changed")
}

// TestNewInMemory_WritesRunTheWritePath checks that a write through the world
// runs automations, validation and ACL, as on the default build. Each case
// builds its own world, so it can run alone.
func TestNewInMemory_WritesRunTheWritePath(t *testing.T) {
	// Pin the mode: a developer's shell may set it, and the validation case
	// expects advisory warnings.
	t.Setenv(ValidationModeEnv, entitymanager.ValidationAdvisory.String())
	root := writeMemWorldProject(t)
	type world struct {
		svc *Services
		id  string
	}
	newWorld := func(t *testing.T) world {
		t.Helper()
		svc, err := NewInMemory(root)
		require.NoError(t, err)
		t.Cleanup(func() { _ = svc.Close() })
		alice := memWorldAs(t, svc, "alice")
		created, err := svc.EntityManager().CreateEntity(alice,
			&entity.Entity{Type: "ticket", Properties: map[string]any{"title": "T", "status": "open"}},
			entity.CreateOptions{})
		require.NoError(t, err)
		return world{svc: svc, id: created.Entity.ID}
	}
	stored := func(t *testing.T, w world) *entity.Entity {
		t.Helper()
		got, err := w.svc.Store().GetEntity(context.Background(), entity.Ref{ID: w.id})
		require.NoError(t, err)
		return got
	}

	t.Run("store starts empty", func(t *testing.T) {
		w := newWorld(t)
		_, err := w.svc.Store().GetEntity(context.Background(), entity.Ref{ID: "TKT-1"})
		require.ErrorIs(t, err, store.ErrNotFound, "entities on disk must not be loaded")
	})
	t.Run("automation fires", func(t *testing.T) {
		w := newWorld(t)
		res, err := w.svc.EntityManager().PatchEntity(memWorldAs(t, w.svc, "alice"), w.id,
			entity.Patch{Properties: map[string]any{"status": "closed"}})
		require.NoError(t, err)
		require.Empty(t, res.AutomationErrors)
		require.Equal(t, "closed by automation", stored(t, w).Properties["note"])
	})
	t.Run("validation reports a missing required property", func(t *testing.T) {
		w := newWorld(t)
		res, err := w.svc.EntityManager().CreateEntity(memWorldAs(t, w.svc, "alice"),
			&entity.Entity{Type: "ticket", Properties: map[string]any{"status": "open"}}, entity.CreateOptions{})
		require.NoError(t, err, "advisory mode keeps the write")
		paths := make([]string, 0, len(res.Warnings))
		for _, warning := range res.Warnings {
			paths = append(paths, warning.Path)
		}
		require.Contains(t, paths, "/properties/title")
	})
	t.Run("acl denies a viewer's write", func(t *testing.T) {
		w := newWorld(t)
		bob := memWorldAs(t, w.svc, "bob")
		_, err := w.svc.EntityManager().PatchEntity(bob, w.id,
			entity.Patch{Properties: map[string]any{"title": "hijacked"}})
		require.ErrorIs(t, err, acl.ErrForbidden)
		require.Equal(t, "T", stored(t, w).Properties["title"])
	})
	t.Run("audit records the writes in memory", func(t *testing.T) {
		w := newWorld(t)
		mem, ok := w.svc.Audit().(*audit.Memory)
		require.True(t, ok, "audit must be in memory")
		require.NotEmpty(t, mem.Records())
	})
}

// cronRunner is the top frame of the goroutine neoq's memory backend starts
// and never stops (the KNOWN LEAK on jobs.neoqQueue.Close).
const cronRunner = "github.com/robfig/cron.(*Cron).run"

// TestNewInMemory_CloseStopsEveryGoroutine builds and closes 100 worlds and
// checks that none leaves a goroutine behind.
//
// The cron runner is exempt, at most one per world: neoq's Shutdown never
// stops it. It idles in a select. TKT-L98KKVX tracks the fix in the neoq
// fork. The count check keeps the exemption from hiding a second leak of the
// same function.
func TestNewInMemory_CloseStopsEveryGoroutine(t *testing.T) {
	const worlds = 100
	root := writeMemWorldProject(t)
	ignore := []goleak.Option{goleak.IgnoreCurrent(), goleak.IgnoreTopFunction(cronRunner)}
	cronBefore := countGoroutines(cronRunner)
	for range worlds {
		svc, err := NewInMemory(root)
		require.NoError(t, err)
		_, err = svc.EntityManager().CreateEntity(memWorldAs(t, svc, "alice"),
			&entity.Entity{Type: "ticket", Properties: map[string]any{"title": "T"}}, entity.CreateOptions{})
		require.NoError(t, err)
		require.NoError(t, svc.Close())
	}
	goleak.VerifyNone(t, ignore...)
	require.LessOrEqual(t, countGoroutines(cronRunner)-cronBefore, worlds,
		"more cron runners than worlds: something else leaks one")
}

// countGoroutines counts the goroutines whose stack names fn.
func countGoroutines(fn string) int {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return strings.Count(string(buf[:n]), fn+"(")
		}
		buf = make([]byte, 2*len(buf))
	}
}

func TestNewInMemory_RejectsBadRoot(t *testing.T) {
	file := filepath.Join(t.TempDir(), "schema.yaml.txt")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	tests := []struct {
		name string
		root string
	}{
		{"empty", ""},
		{"not a project", t.TempDir()},
		{"a regular file", file},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewInMemory(tc.root)
			require.Error(t, err)
		})
	}
}

// The lock-file sites write with os calls, so they must choose a
// process-private lock for an in-memory FS. The tree-hash test does not
// reach the migration lock: only Gate.Persist and a GC tick take it.
func TestInMemoryFS_SelectsProcessLocks(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), ".rela")
	paths := &project.Context{Root: filepath.Dir(cacheDir), CacheDir: cacheDir}
	tests := []struct {
		name     string
		fs       storage.FS
		inMemory bool
		lockDir  string
	}{
		{"memfs", storage.NewMemFS(), true, ""},
		{"disk", storage.NewOsFS(), false, cacheDir},
		{"safe disk", storage.NewSafeFS(storage.NewOsFS()), false, cacheDir},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.inMemory, inMemoryFS(tc.fs))
			require.Equal(t, tc.lockDir, migrationLockDir(Config{FS: tc.fs, Paths: paths}))
		})
	}
}

// titleAnchor attaches a comment to the ticket's title.
var titleAnchor = comments.Anchor{Kind: comments.AnchorProperty, Ref: "title"}
