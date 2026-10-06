package fsimport_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/comments/memcomments"
	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/datamigration/filemigstate"
	"github.com/Sourcehaven-BV/rela/internal/datamigration/memmigstate"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/fsimport"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const schemaYAML = `
entities:
  item:
    label: Item
    plural: items
    id_type: sequential
    id_prefix: ITEM-
    properties:
      title:
        type: string
      due:
        type: date
      seen:
        type: datetime
  note:
    label: Note
    plural: notes
    id_type: sequential
    id_prefix: NOTE-
    properties:
      title:
        type: string
relations:
  links:
    label: links
    from: [item]
    to: [item, note]
`

// memBackend is a Backend over in-memory stores. Every Open returns the same
// stores, so the verification after the rename reads what the copy wrote.
type memBackend struct {
	target fsimport.Target
	opens  []string
	finish func(root string) error
}

func newMemBackend(t *testing.T) *memBackend {
	t.Helper()
	mem := storage.NewMemFS()
	require.NoError(t, mem.MkdirAll("/kv", 0o755))
	rooted, err := storage.NewRootedFS(mem, "/kv")
	require.NoError(t, err)
	kv, err := state.NewValidatedKV(state.NewFSKV(rooted))
	require.NoError(t, err)
	return &memBackend{target: fsimport.Target{
		Store:      memstore.New(),
		State:      kv,
		Comments:   memcomments.New(),
		Migrations: memmigstate.New(),
	}}
}

func (b *memBackend) backend() fsimport.Backend {
	return fsimport.Backend{
		Open: func(_ context.Context, root string) (*fsimport.Opened, error) {
			b.opens = append(b.opens, root)
			return &fsimport.Opened{Target: b.target, Close: func(context.Context) error { return nil }}, nil
		},
		Finish: func(_ context.Context, root string) error {
			if b.finish != nil {
				return b.finish(root)
			}
			return nil
		},
		Owned: []string{".rela/rela.db"},
	}
}

// writeTree writes files (path -> content) below root.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, content := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
}

// baseProject is a source project with one of every kind of record.
func baseProject(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "source")
	writeTree(t, root, map[string]string{
		"schema.yaml":                        schemaYAML,
		"templates/entities/item.md":         "template\n",
		"scripts/hello.lua":                  "return 1\n",
		"entities/items/ITEM-1.md":           "---\nid: ITEM-1\ntype: item\ntitle: First\ndue: 2026-01-02\nseen: 2026-01-02T10:00:00+02:00\nscore: 2.0\n---\nBody one.\n",
		"entities/items/ITEM-2.md":           "---\nid: ITEM-2\ntype: item\ntitle: Second\n---\n",
		"entities/items/ITEM-2@draft.md":     "---\nid: ITEM-2\ntype: item\ntitle: Second draft\n---\n",
		"entities/notes/NOTE-1.md":           "---\nid: NOTE-1\ntype: note\ntitle: A note\n---\n",
		"relations/ITEM-1--links--NOTE-1.md": "---\nfrom: ITEM-1\nrelation: links\nto: NOTE-1\n---\nwhy\n",
		"attachments/ITEM-1/file/doc.txt":    "attachment bytes",
		".rela/comments/ITEM-1.yaml":         "comments:\n- id: c1\n  author: alice\n  created_at: 2026-01-01T09:00:00+01:00\n  anchor: {kind: property, ref: title}\n  body: hi\n",
		".rela/config.yaml":                  "server: {}\n",
		".rela/secrets.yaml":                 "token: s3cret\n",
		".rela/audit/2026-01.jsonl":          "{}\n",
		".rela/user-defaults.yaml":           "item: {}\n",
		".rela/theme/logo":                   "PNG",
		".rela/scheduler-run-state.json":     `{"tasks":{"t":{"last_run":"2026-01-01T00:00:00Z"}},"runs":{"r":{}}}`,
		".rela/fsstore-index.json":           "{}",
		".rela/documents/x.html":             "cache",
	})
	files, err := filemigstate.New(storage.NewOsFS(), root)
	require.NoError(t, err)
	require.NoError(t, files.Save(context.Background(), &datamigration.State{
		FormatVersion: datamigration.StateFormatVersion,
		Applied: []datamigration.AppliedEntry{
			{Name: "20260101000000-first.yaml", AppliedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
		Projection: json.RawMessage(`{"entities": {}}`),
		UpdatedAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}))
	return root
}

// snapshot reads every file below root, for byte-identity checks.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	require.NoError(t, filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		require.NoError(t, err)
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() {
			out[rel+"/"] = ""
			return nil
		}
		data, err := os.ReadFile(p)
		require.NoError(t, err)
		info, err := d.Info()
		require.NoError(t, err)
		out[rel] = string(data) + "|" + info.Mode().String() + "|" + info.ModTime().String()
		return nil
	}))
	return out
}

func run(t *testing.T, source, target string, b *memBackend) (*fsimport.Report, error) {
	t.Helper()
	return fsimport.Run(context.Background(), fsimport.Options{
		Source:    source,
		Target:    target,
		Backend:   b.backend(),
		Principal: principal.Principal{User: "operator", Tool: principal.ToolCLI},
	})
}

// leftovers lists the entries in dir other than the source.
func leftovers(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var out []string
	for _, e := range entries {
		if e.Name() != "source" {
			out = append(out, e.Name())
		}
	}
	return out
}

func TestRun_CopiesEverything(t *testing.T) {
	ctx := context.Background()
	source := baseProject(t)
	before := snapshot(t, source)
	target := filepath.Join(filepath.Dir(source), "target")
	b := newMemBackend(t)

	rep, err := run(t, source, target, b)
	require.NoError(t, err, "errors: %v", rep.Errors)

	assert.Equal(t, before, snapshot(t, source), "the source must be byte-identical")
	assert.Equal(t, 4, rep.Entities)
	assert.Equal(t, 1, rep.Relations)
	assert.Equal(t, 1, rep.Attachments)
	assert.Equal(t, 1, rep.Comments)
	assert.Equal(t, 3, rep.StateKeys, "user-defaults, theme/logo, scheduler state")
	assert.Equal(t, 2, rep.Normalized, "one date, one datetime")
	assert.Empty(t, rep.Errors)

	// Values are stored in their database form.
	got, err := b.target.Store.GetEntity(ctx, entity.Ref{ID: "ITEM-1"})
	require.NoError(t, err)
	assert.Equal(t, "2026-01-02", got.Properties["due"])
	assert.Equal(t, "2026-01-02T10:00:00+02:00", got.Properties["seen"])
	draft, err := b.target.Store.GetEntity(ctx, entity.Ref{ID: "ITEM-2", Face: "draft"})
	require.NoError(t, err)
	assert.Equal(t, "Second draft", draft.Properties["title"])

	rc, err := b.target.Store.ReadFamilyAttachment(ctx, "ITEM-1", "file", "doc.txt")
	require.NoError(t, err)
	data, _ := io.ReadAll(rc)
	_ = rc.Close()
	assert.Equal(t, "attachment bytes", string(data))

	list, err := b.target.Comments.List(ctx, comments.Target{Type: "item", ID: "ITEM-1"})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "c1", list[0].ID)
	assert.Equal(t, "alice", list[0].Author)

	mig, err := b.target.Migrations.Load(ctx)
	require.NoError(t, err)
	require.NotNil(t, mig)
	require.Len(t, mig.Applied, 1)
	assert.Equal(t, "20260101000000-first.yaml", mig.Applied[0].Name)

	sched, err := b.target.State.Get(ctx, "scheduler-run-state.json")
	require.NoError(t, err)
	assert.NotContains(t, string(sched), "runs", "runs in flight are dropped")
	assert.Contains(t, string(sched), "last_run")

	// Project files, and only those.
	for _, p := range []string{"schema.yaml", "templates/entities/item.md", "scripts/hello.lua",
		".rela/config.yaml", ".rela/secrets.yaml", ".rela/audit/2026-01.jsonl", ".gitignore"} {
		assert.FileExists(t, filepath.Join(target, p))
	}
	for _, p := range []string{"entities", "relations", "attachments", "migrations/applied.json",
		".rela/fsstore-index.json", ".rela/user-defaults.yaml", ".rela/comments"} {
		assert.NoFileExists(t, filepath.Join(target, p))
		assert.NoDirExists(t, filepath.Join(target, p))
	}
	assertMode(t, filepath.Join(target, ".rela"), 0o700)
	assertMode(t, filepath.Join(target, ".rela", "secrets.yaml"), 0o600)
	assertMode(t, filepath.Join(target, ".rela", "audit", "2026-01.jsonl"), 0o600)
	ignore, err := os.ReadFile(filepath.Join(target, ".gitignore"))
	require.NoError(t, err)
	assert.Equal(t, ".rela/\n", string(ignore))

	// One audit record of the run, in the target.
	audit := readAudit(t, target)
	assert.Contains(t, audit, `"op":"fs-import"`)
	assert.Contains(t, audit, `"user":"operator"`)

	assert.Equal(t, []string{rep.Target}, b.opens[1:], "verification reads the renamed target")
	assert.Equal(t, []string{"target"}, leftovers(t, filepath.Dir(source)), "no staging directory is left")

	skipped := map[string]string{}
	for _, s := range rep.Skipped {
		skipped[s.Path] = s.Reason
	}
	assert.Contains(t, skipped, ".rela/fsstore-index.json")
	assert.Contains(t, skipped, ".rela/documents/", "a skipped folder is listed once")
	assert.Contains(t, skipped, "migrations/applied.json")
}

func readAudit(t *testing.T, target string) string {
	t.Helper()
	var all strings.Builder
	dir := filepath.Join(target, ".rela", "audit")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		all.Write(data)
	}
	return all.String()
}

func assertMode(t *testing.T, p string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, want, info.Mode().Perm(), p)
}

func TestRun_ListsFilesTheStoreDoesNotRead(t *testing.T) {
	source := baseProject(t)
	writeTree(t, source, map[string]string{
		"entities/widgets/W-1.md":      "---\nid: W-1\ntype: widget\n---\n",
		"entities/items/not an id!.md": "---\n---\n",
		"entities/items/README.txt":    "notes",
		"relations/garbage.md":         "---\n---\n",
		".rela/comments/GONE-1.yaml":   "comments: []\n",
		".rela/something-new.json":     "{}",
		".gitignore":                   "node_modules\n",
	})
	b := newMemBackend(t)
	target := filepath.Join(filepath.Dir(source), "target")

	rep, err := run(t, source, target, b)
	require.NoError(t, err, "errors: %v", rep.Errors)

	skipped := map[string]string{}
	for _, s := range rep.Skipped {
		skipped[s.Path] = s.Reason
	}
	for p, reason := range map[string]string{
		"entities/widgets/W-1.md":      "not a type the schema declares",
		"entities/items/not an id!.md": "not a valid entity id",
		"entities/items/README.txt":    "not a markdown file",
		"relations/garbage.md":         "not FROM--TYPE--TO",
		".rela/comments/GONE-1.yaml":   "not imported",
		".rela/something-new.json":     "not copied",
	} {
		assert.Contains(t, skipped[p], reason, p)
	}
	ignore, err := os.ReadFile(filepath.Join(target, ".gitignore"))
	require.NoError(t, err)
	assert.Equal(t, "node_modules\n.rela/\n", string(ignore))
}

func TestRun_CollectsEveryRowError(t *testing.T) {
	source := baseProject(t)
	writeTree(t, source, map[string]string{
		// The same id in two type folders.
		"entities/notes/ITEM-1.md": "---\nid: ITEM-1\ntype: note\ntitle: dup\n---\n",
		// A value JSON cannot hold.
		"entities/items/ITEM-9.md": "---\nid: ITEM-9\ntype: item\nscore: .nan\n---\n",
		// The type under the wrong key, and a frontmatter naming other ends.
		"relations/ITEM-2--links--NOTE-1.md": "---\nfrom: ITEM-2\ntype: links\nto: NOTE-1\n---\n",
		"relations/ITEM-1--links--ITEM-2.md": "---\nfrom: ITEM-2\nrelation: links\nto: ITEM-1\n---\n",
	})
	before := snapshot(t, source)
	b := newMemBackend(t)
	target := filepath.Join(filepath.Dir(source), "target")

	rep, err := run(t, source, target, b)
	require.Error(t, err)
	joined := strings.Join(rep.Errors, "\n")
	assert.Contains(t, joined, "ITEM-9")
	assert.Contains(t, joined, "entities/notes/ITEM-1.md: id ITEM-1 is in more than one type folder")
	assert.Contains(t, joined, "entities/items/ITEM-1.md: id ITEM-1 is in more than one type folder")
	assert.Contains(t, joined, "relations/ITEM-2--links--NOTE-1.md: the frontmatter has no relation:")
	assert.Contains(t, joined, "relations/ITEM-1--links--ITEM-2.md: the frontmatter names ITEM-2--links--ITEM-1")
	// One error per file: both ITEM-1 files, ITEM-9, the two relations.
	assert.Len(t, rep.Errors, 5, "%v", rep.Errors)

	assert.NoDirExists(t, target)
	assert.Empty(t, leftovers(t, filepath.Dir(source)), "no staging directory is left")
	assert.Equal(t, before, snapshot(t, source))
}

func TestRun_RefusesEncryptedContent(t *testing.T) {
	source := baseProject(t)
	writeTree(t, source, map[string]string{
		"entities/items/ITEM-7.md":         "\x00GITCRYPT\x00ciphertext",
		"attachments/ITEM-2/file/scan.pdf": "\x00GITCRYPT\x00ciphertext",
	})
	b := newMemBackend(t)
	target := filepath.Join(filepath.Dir(source), "target")

	rep, err := run(t, source, target, b)
	require.Error(t, err)
	joined := strings.Join(rep.Errors, "\n")
	assert.Contains(t, joined, "entity ITEM-7 is encrypted")
	assert.Contains(t, joined, "attachment ITEM-2/file/scan.pdf is encrypted")
	assert.Empty(t, b.opens, "refused before the database is opened")
	assert.Empty(t, leftovers(t, filepath.Dir(source)))
}

func TestRun_FailsWhenTheSourceChanges(t *testing.T) {
	source := baseProject(t)
	b := newMemBackend(t)
	b.finish = func(string) error {
		return os.WriteFile(filepath.Join(source, "entities", "items", "ITEM-8.md"),
			[]byte("---\nid: ITEM-8\ntype: item\n---\n"), 0o644)
	}
	target := filepath.Join(filepath.Dir(source), "target")

	_, err := run(t, source, target, b)
	require.ErrorContains(t, err, "changed during the import")
	assert.NoDirExists(t, target)
	assert.Empty(t, leftovers(t, filepath.Dir(source)))
}

func TestRun_BackendFailureLeavesNothing(t *testing.T) {
	source := baseProject(t)
	b := newMemBackend(t)
	b.finish = func(string) error { return errors.New("checkpoint failed") }
	target := filepath.Join(filepath.Dir(source), "target")

	_, err := run(t, source, target, b)
	require.ErrorContains(t, err, "checkpoint failed")
	assert.Empty(t, leftovers(t, filepath.Dir(source)))
}

func TestRun_RefusesBadPaths(t *testing.T) {
	source := baseProject(t)
	parent := filepath.Dir(source)
	existing := filepath.Join(parent, "existing")
	require.NoError(t, os.Mkdir(existing, 0o755))
	notProject := filepath.Join(parent, "plain")
	require.NoError(t, os.Mkdir(notProject, 0o755))
	withDB := filepath.Join(parent, "withdb")
	writeTree(t, withDB, map[string]string{"schema.yaml": schemaYAML, ".rela/rela.db": "x"})

	cases := []struct {
		name, source, target, want string
	}{
		{"target exists", source, existing, "already exists"},
		{"target is the source", source, source, "already exists"},
		{"target inside source", source, filepath.Join(source, "sub"), "inside the source"},
		{"target deep inside source", source, filepath.Join(source, "templates", "x"), "inside the source"},
		{"target parent missing", source, filepath.Join(parent, "nope", "x"), "target parent"},
		{"source without schema", notProject, filepath.Join(parent, "t1"), "not a rela project"},
		{"source already a database project", withDB, filepath.Join(parent, "t2"), "rela.db"},
		{"no target", source, "", "required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := newMemBackend(t)
			_, err := run(t, tc.source, tc.target, b)
			require.ErrorContains(t, err, tc.want)
			assert.Empty(t, b.opens)
		})
	}
	assert.ElementsMatch(t, []string{"existing", "plain", "withdb"}, leftovers(t, parent))
}

func TestRun_RequiresABackend(t *testing.T) {
	_, err := fsimport.Run(context.Background(), fsimport.Options{Source: "a", Target: "b"})
	require.ErrorContains(t, err, "Backend.Open")
}

func TestRun_ProgressAndEmptyProject(t *testing.T) {
	root := filepath.Join(t.TempDir(), "source")
	writeTree(t, root, map[string]string{"schema.yaml": schemaYAML})
	var progress bytes.Buffer
	b := newMemBackend(t)
	rep, err := fsimport.Run(context.Background(), fsimport.Options{
		Source:   root,
		Target:   filepath.Join(filepath.Dir(root), "target"),
		Backend:  b.backend(),
		Progress: &progress,
	})
	require.NoError(t, err)
	assert.Zero(t, rep.Entities)
	assert.Contains(t, progress.String(), "Verifying")

	n, err := b.target.Store.CountEntities(context.Background(), store.EntityQuery{Faces: store.AllFaces()})
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Contains(t, readAudit(t, filepath.Join(filepath.Dir(root), "target")), `"user":"system:fs-import"`)
}

func TestRun_PendingDeletesAndWarnings(t *testing.T) {
	ctx := context.Background()
	source := baseProject(t)
	writeTree(t, source, map[string]string{
		// ITEM-2 is soft-deleted with undo pending: none of it is copied.
		".rela/pending-deletes.json":         `[{"id":"ITEM-2","deleted_at":"2026-01-01T00:00:00Z"}]`,
		"relations/ITEM-2--links--NOTE-1.md": "---\nfrom: ITEM-2\nrelation: links\nto: NOTE-1\n---\n",
		"attachments/ITEM-2/file/x.txt":      "x",
		// The file name wins over a different frontmatter id.
		"entities/notes/NOTE-2.md": "---\nid: NOTE-9\ntype: note\n---\n",
		".gitattributes":           "entities/** filter=git-crypt diff=git-crypt\n",
	})
	require.NoError(t, os.Symlink("/etc/hosts", filepath.Join(source, "scripts", "link.lua")))
	b := newMemBackend(t)
	target := filepath.Join(filepath.Dir(source), "target")

	rep, err := run(t, source, target, b)
	require.NoError(t, err, "errors: %v", rep.Errors)

	assert.Equal(t, []string{"ITEM-2"}, rep.PendingDeletes)
	_, err = b.target.Store.GetEntity(ctx, entity.Ref{ID: "ITEM-2"})
	require.ErrorIs(t, err, store.ErrNotFound)
	_, err = b.target.Store.GetEntity(ctx, entity.Ref{ID: "NOTE-2"})
	require.NoError(t, err)
	assert.True(t, rep.GitCrypt)
	require.Len(t, rep.Warnings, 1)
	assert.Contains(t, rep.Warnings[0], `the frontmatter says id "NOTE-9"`)

	skipped := map[string]string{}
	for _, s := range rep.Skipped {
		skipped[s.Path] = s.Reason
	}
	assert.Contains(t, skipped["relations/ITEM-2--links--NOTE-1.md"], "undo still pending")
	assert.Contains(t, skipped["attachments/ITEM-2/file/x.txt"], "undo still pending")
	assert.Contains(t, skipped, "scripts/link.lua", "a symlink is listed, not followed")
	assert.NoFileExists(t, filepath.Join(target, "scripts", "link.lua"))
}

func TestRun_CaseCollidingIDs(t *testing.T) {
	source := baseProject(t)
	probe := filepath.Join(source, "entities", "items", "item-1.md")
	if _, err := os.Stat(probe); err == nil {
		t.Skip("case-insensitive filesystem: ITEM-1 and item-1 are one file")
	}
	writeTree(t, source, map[string]string{
		"entities/items/item-1.md": "---\nid: item-1\ntype: item\n---\n",
	})
	b := newMemBackend(t)
	b.target.Store = caseFoldingStore{b.target.Store}

	rep, err := run(t, source, filepath.Join(filepath.Dir(source), "target"), b)
	require.Error(t, err)
	assert.Contains(t, strings.Join(rep.Errors, "\n"), "regardless of case")
}

// caseFoldingStore refuses a second id differing only in case, as the
// database backends do.
type caseFoldingStore struct{ store.Store }

func (s caseFoldingStore) CreateEntity(ctx context.Context, e *entity.Entity) error {
	for got, err := range s.ListEntities(ctx, store.EntityQuery{Faces: store.AllFaces()}) {
		if err == nil && strings.EqualFold(got.ID, e.ID) && got.ID != e.ID {
			return store.ErrConflict
		}
	}
	return s.Store.CreateEntity(ctx, e)
}

func (s caseFoldingStore) Tx(ctx context.Context, fn func(store.Store) error) error {
	return s.Store.Tx(ctx, func(tx store.Store) error { return fn(caseFoldingStore{tx}) })
}

// A project last migrated before the record moved out of .rela/ has only the
// legacy marker. The import reads it the way appbuild does, so the target
// does not replay those migrations.
func TestRun_LegacyMigrationMarker(t *testing.T) {
	root := filepath.Join(t.TempDir(), "source")
	writeTree(t, root, map[string]string{
		"schema.yaml": schemaYAML,
		".rela/migration/state.json": `{"shape_hash":"abc","projection":{"entities":{}},` +
			`"applied":["20260101000000-first.yaml"],"updated_at":"2026-01-02T00:00:00Z"}`,
	})
	b := newMemBackend(t)
	rep, err := run(t, root, filepath.Join(filepath.Dir(root), "target"), b)
	require.NoError(t, err, "errors: %v", rep.Errors)

	got, err := b.target.Migrations.Load(context.Background())
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Applied, 1)
	assert.Equal(t, "20260101000000-first.yaml", got.Applied[0].Name)
}

// A symlink where a store reads would make it copy whatever the link points
// at into the database. The run refuses before opening the source.
func TestRun_RefusesSymlinksInData(t *testing.T) {
	source := baseProject(t)
	secret := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(secret, []byte("private key"), 0o600))
	require.NoError(t, os.Symlink(secret, filepath.Join(source, "attachments", "ITEM-1", "file", "key.pem")))
	require.NoError(t, os.Symlink(secret, filepath.Join(source, ".rela", "comments", "ITEM-2.yaml")))
	b := newMemBackend(t)

	rep, err := run(t, source, filepath.Join(filepath.Dir(source), "target"), b)
	require.ErrorContains(t, err, "symlink")
	joined := strings.Join(rep.Errors, "\n")
	assert.Contains(t, joined, "attachments/ITEM-1/file/key.pem is a symlink")
	assert.Contains(t, joined, ".rela/comments/ITEM-2.yaml is a symlink")
	assert.Empty(t, b.opens)
	assert.Empty(t, leftovers(t, filepath.Dir(source)))
}

func TestRun_GitignoreWithLeadingSpaces(t *testing.T) {
	source := baseProject(t)
	writeTree(t, source, map[string]string{
		".gitignore":         "  .rela/\n",
		"sub/.gitattributes": "secret/** filter=git-crypt\n",
	})
	target := filepath.Join(filepath.Dir(source), "target")
	rep, err := run(t, source, target, newMemBackend(t))
	require.NoError(t, err, "errors: %v", rep.Errors)

	ignore, err := os.ReadFile(filepath.Join(target, ".gitignore"))
	require.NoError(t, err)
	assert.Equal(t, "  .rela/\n.rela/\n", string(ignore), "a leading-space pattern ignores nothing in git")
	assert.True(t, rep.GitCrypt, "a nested .gitattributes counts")
}

// Verification reads the renamed target. When that read finds the data
// missing, the run fails and removes the target it created.
func TestRun_VerifyFailureRemovesTarget(t *testing.T) {
	source := baseProject(t)
	b := newMemBackend(t)
	empty := newMemBackend(t).target
	inner := b.backend()
	be := inner
	be.Open = func(ctx context.Context, root string) (*fsimport.Opened, error) {
		if len(b.opens) == 1 { // the reopen after the rename
			b.opens = append(b.opens, root)
			return &fsimport.Opened{Target: empty, Close: func(context.Context) error { return nil }}, nil
		}
		return inner.Open(ctx, root)
	}
	target := filepath.Join(filepath.Dir(source), "target")

	rep, err := fsimport.Run(context.Background(), fsimport.Options{Source: source, Target: target, Backend: be})
	require.Error(t, err)
	assert.Contains(t, strings.Join(rep.Errors, "\n"), "verify")
	assert.NoDirExists(t, target)
	assert.Empty(t, leftovers(t, filepath.Dir(source)))
}

func TestRun_TargetAppearsDuringImport(t *testing.T) {
	source := baseProject(t)
	target := filepath.Join(filepath.Dir(source), "target")
	b := newMemBackend(t)
	b.finish = func(string) error { return os.Mkdir(target, 0o755) }

	_, err := run(t, source, target, b)
	require.ErrorContains(t, err, "appeared during the import")
	assert.DirExists(t, target, "a directory someone else made is left alone")
	assert.Equal(t, []string{"target"}, leftovers(t, filepath.Dir(source)), "staging is removed")
}

func TestRun_RejectsIncompleteBackend(t *testing.T) {
	source := baseProject(t)
	be := fsimport.Backend{Open: func(context.Context, string) (*fsimport.Opened, error) {
		return &fsimport.Opened{}, nil
	}}
	_, err := fsimport.Run(context.Background(), fsimport.Options{
		Source: source, Target: filepath.Join(filepath.Dir(source), "target"), Backend: be,
	})
	require.ErrorContains(t, err, "incomplete target")
	assert.Empty(t, leftovers(t, filepath.Dir(source)))
}

// A thread whose file is not in time order still verifies: the database
// backends return comments ordered by time.
func TestRun_CommentsOutOfOrder(t *testing.T) {
	source := baseProject(t)
	writeTree(t, source, map[string]string{
		".rela/comments/NOTE-1.yaml": "comments:\n" +
			"- id: c2\n  author: bob\n  created_at: 2026-01-02T09:00:00Z\n  anchor: {kind: property, ref: title}\n  body: later\n" +
			"- id: c1\n  author: alice\n  created_at: 2026-01-01T09:00:00Z\n  anchor: {kind: property, ref: title}\n  body: earlier\n",
	})
	b := newMemBackend(t)
	b.target.Comments = sortedComments{b.target.Comments}
	rep, err := run(t, source, filepath.Join(filepath.Dir(source), "target"), b)
	require.NoError(t, err, "errors: %v", rep.Errors)
	assert.Equal(t, 3, rep.Comments)
}

// sortedComments returns threads ordered by time, as sqlitecomments does.
type sortedComments struct{ comments.Store }

func (s sortedComments) List(ctx context.Context, t comments.Target) ([]comments.Comment, error) {
	list, err := s.Store.List(ctx, t)
	slices.SortFunc(list, func(a, b comments.Comment) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return list, err
}

func TestRun_KeepsOwnerOnlyConfig(t *testing.T) {
	source := baseProject(t)
	require.NoError(t, os.Chmod(filepath.Join(source, ".rela", "config.yaml"), 0o600))
	require.NoError(t, os.Chmod(filepath.Join(source, "scripts"), 0o700))
	target := filepath.Join(filepath.Dir(source), "target")
	rep, err := run(t, source, target, newMemBackend(t))
	require.NoError(t, err, "errors: %v", rep.Errors)
	assertMode(t, filepath.Join(target, ".rela", "config.yaml"), 0o600)
	assertMode(t, filepath.Join(target, "scripts"), 0o700)
}
