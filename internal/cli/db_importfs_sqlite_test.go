//go:build sqlite

package cli

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const importSchema = `
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
      notes:
        type: file
  policy:
    label: Policy
    plural: policies
    id_type: sequential
    id_prefix: POL-
    faces:
      draft: { label: Draft }
      published: { label: Published }
    properties:
      title:
        type: string
relations:
  links:
    label: links
    from: [item]
    to: [item]
    properties:
      since:
        type: date
`

func writeImportFixture(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"schema.yaml": importSchema,
		"entities/items/ITEM-1.md": "---\nid: ITEM-1\ntype: item\ntitle: Gearbox\ndue: 2026-03-04\n" +
			"seen: 2026-03-04T10:15:00+01:00\n---\nThe gearbox body.\n",
		"entities/policies/POL-1@draft.md":     "---\nid: POL-1\ntype: policy\ntitle: Draft text\n---\n",
		"entities/policies/POL-1@published.md": "---\nid: POL-1\ntype: policy\ntitle: Published text\n---\n",
		"entities/items/ITEM-2.md":             "---\nid: ITEM-2\ntype: item\ntitle: Second\n---\n",
		"relations/ITEM-1--links--ITEM-2.md":   "---\nfrom: ITEM-1\nrelation: links\nto: ITEM-2\nsince: 2026-01-01\n---\n",
		"attachments/ITEM-1/notes/a.txt":       "attached",
		".rela/comments/ITEM-1.yaml": "comments:\n- id: c1\n  author: alice\n" +
			"  created_at: 2026-01-01T09:00:00Z\n  anchor: {kind: property, ref: title}\n  body: hi\n",
		".rela/user-defaults.yaml": "item: {}\n",
	}
	for p, content := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
}

// TestDBImportFS_SQLite runs the command's backend end to end and opens the
// result the way the SQLite build does.
func TestDBImportFS_SQLite(t *testing.T) {
	ctx := principal.With(context.Background(), principal.Principal{User: "op", Tool: principal.ToolCLI})
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	writeImportFixture(t, source)

	before := treeBytes(t, source)
	require.NoError(t, runDBImportFS(ctx, source, target))
	assert.Equal(t, before, treeBytes(t, source), "the source is unchanged")

	for _, side := range []string{"-wal", "-shm"} {
		assert.NoFileExists(t, filepath.Join(target, ".rela", dbFileName+side))
	}
	_, err := os.Stat(filepath.Join(source, ".rela", dbFileName))
	assert.True(t, os.IsNotExist(err), "the source gets no database")

	svc, err := appbuild.At(target, script.NewEngine())
	require.NoError(t, err)

	got, err := svc.Store().GetEntity(ctx, entity.Ref{ID: "ITEM-1"})
	require.NoError(t, err)
	assert.Equal(t, "Gearbox", got.Properties["title"])
	assert.Equal(t, "2026-03-04", got.Properties["due"])
	assert.Equal(t, "The gearbox body.", got.Content)
	assert.Equal(t, "2026-03-04T10:15:00+01:00", got.Properties["seen"])

	for face, title := range map[entity.Face]string{"draft": "Draft text", "published": "Published text"} {
		pol, getErr := svc.Store().GetEntity(ctx, entity.Ref{ID: "POL-1", Face: face})
		require.NoError(t, getErr, string(face))
		assert.Equal(t, title, pol.Properties["title"])
	}

	var rels []*entity.Relation
	for r, err := range svc.Store().ListRelations(ctx, store.RelationQuery{From: "ITEM-1"}) {
		require.NoError(t, err)
		rels = append(rels, r)
	}
	require.Len(t, rels, 1)
	assert.Equal(t, "ITEM-2", rels[0].To)
	assert.Equal(t, "2026-01-01", rels[0].Properties["since"])

	rc, err := svc.Store().ReadFamilyAttachment(ctx, "ITEM-1", "notes", "a.txt")
	require.NoError(t, err)
	data, _ := io.ReadAll(rc)
	_ = rc.Close()
	assert.Equal(t, "attached", string(data))

	defaults, err := svc.State().Get(ctx, "user-defaults.yaml")
	require.NoError(t, err)
	assert.Equal(t, "item: {}\n", string(defaults))

	var hits []string
	for h, err := range svc.Searcher().Search(ctx, search.Query{Text: "gearbox", World: store.TrivialScope()}) {
		require.NoError(t, err)
		hits = append(hits, h.ID)
	}
	assert.Equal(t, []string{"ITEM-1"}, hits, "search indexes the imported rows")
	require.NoError(t, svc.Close())

	// The comment service is wired only for the data-entry server; read
	// the comment table directly.
	db, err := appbuild.OpenSQLiteData(ctx, filepath.Join(target, ".rela", dbFileName))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	list, err := db.Comments.List(ctx, comments.Target{Type: "item", ID: "ITEM-1"})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "alice", list[0].Author)
}

// treeBytes reads every file below root.
func treeBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	require.NoError(t, filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		out[p] = string(data)
		return err
	}))
	return out
}

// TestDBImportFS_SQLiteTablesAreAccounted fails when the database gains a
// table the import has not decided about. A new table holds data an fs
// project may have; whoever adds it must say whether the import copies it.
func TestDBImportFS_SQLiteTablesAreAccounted(t *testing.T) {
	accounted := map[string]string{
		"entities":          "copied",
		"relations":         "copied",
		"attachments":       "copied",
		"comments":          "copied",
		"state_kv":          "copied",
		"migration_state":   "copied",
		"entity_versions":   "history starts at the import (git history is not imported)",
		"relation_versions": "history starts at the import",
		"schema_versions":   "history starts at the import",
		"rel_record_seq":    "sequence for relation versions",
		"marked_entities":   "soft deletes; pending deletes are not copied",
		"marked_relations":  "soft deletes; pending deletes are not copied",
		"project_files":     "config stays on disk, which wins over a copy in the database",
	}
	data, err := appbuild.OpenSQLiteData(context.Background(), filepath.Join(t.TempDir(), "x.db"))
	require.NoError(t, err)
	defer func() { _ = data.Close() }()

	tables, err := data.Tables(context.Background())
	require.NoError(t, err)
	for _, name := range tables {
		assert.Contains(t, accounted, name,
			"table %s is new: decide whether `db import-fs` copies it, then list it here", name)
	}
}
