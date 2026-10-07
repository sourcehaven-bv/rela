//go:build sqlite

package appbuild_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/fsimport"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

const dataSchemaYAML = `version: "1.0"
entities:
  doc:
    label: Doc
    plural: docs
    id_prefix: "DOC-"
    id_type: sequential
    properties:
      title:
        type: string
      files:
        type: file
relations:
  refs:
    label: refs
    from: [doc]
    to: [doc]
`

// writeDataProject writes a markdown project with two entities carrying
// properties and bodies, one relation with a property and a body, and one
// attachment.
func writeDataProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "schema.yaml", dataSchemaYAML)
	writeFile(t, root, "entities/docs/DOC-1.md",
		"---\nid: DOC-1\ntype: doc\ntitle: First\nfiles: report.txt\n---\n\nBody of the first doc.\n")
	writeFile(t, root, "entities/docs/DOC-2.md",
		"---\nid: DOC-2\ntype: doc\ntitle: Second\n---\n\nBody of the second doc.\n")
	writeFile(t, root, "relations/DOC-1--refs--DOC-2.md",
		"---\nfrom: DOC-1\nrelation: refs\nto: DOC-2\nnote: see also\n---\n\nWhy they relate.\n")
	writeFile(t, root, "attachments/DOC-1/files/report.txt", "attached bytes")
	return root
}

func osFS() storage.FS { return storage.NewSafeFS(storage.NewOsFS()) }

func projectAt(t *testing.T, root string) *project.Context {
	t.Helper()
	paths, err := project.At(root, osFS())
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func importData(t *testing.T, root string, force bool, sink audit.Audit) (*fsimport.Report, error) {
	t.Helper()
	return appbuild.ImportMarkdownData(context.Background(), osFS(), projectAt(t, root), root,
		appbuild.DataImportOptions{Force: force, Audit: sink})
}

// graph is a comparable snapshot of a store's content.
type graph struct {
	Entities    map[string]entity.Entity
	Relations   map[string]entity.Relation
	Attachments map[string]string
}

func snapshot(t *testing.T, st store.Store) graph {
	t.Helper()
	ctx := context.Background()
	g := graph{
		Entities:    map[string]entity.Entity{},
		Relations:   map[string]entity.Relation{},
		Attachments: map[string]string{},
	}
	for e, err := range st.ListEntities(ctx, store.EntityQuery{Faces: store.AllFaces()}) {
		if err != nil {
			t.Fatal(err)
		}
		g.Entities[entity.FormatStateRef(e.ID, e.Face)] = entity.Entity{
			ID: e.ID, Type: e.Type, Face: e.Face, Properties: e.Properties, Content: strings.TrimSpace(e.Content),
		}
		infos, err := st.ListFamilyAttachments(ctx, e.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, info := range infos {
			r, err := st.ReadFamilyAttachment(ctx, info.EntityID, info.Property, info.FileName)
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(r)
			_ = r.Close()
			if err != nil {
				t.Fatal(err)
			}
			g.Attachments[info.EntityID+"/"+info.Property+"/"+info.FileName] = string(data)
		}
	}
	for r, err := range st.ListRelations(ctx, store.RelationQuery{}) {
		if err != nil {
			t.Fatal(err)
		}
		g.Relations[r.From+"--"+r.Type+"--"+r.To] = entity.Relation{
			From: r.From, FromFace: r.FromFace, Type: r.Type, To: r.To,
			Properties: r.Properties, Content: strings.TrimSpace(r.Content),
		}
	}
	return g
}

// withDatabase opens root's SQLite store for reading and passes it to fn.
func withDatabase(t *testing.T, root string, fn func(store.Store)) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: filepath.Join(root, ".rela", "rela.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	st, err := sqlitestore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	fn(st)
}

func TestImportMarkdownData_CopiesEverything(t *testing.T) {
	root := writeDataProject(t)
	sink := audit.NewMemory()

	rep, err := importData(t, root, false, sink)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if got := [3]int{rep.Entities, rep.Relations, rep.Attachments}; got != [3]int{2, 1, 1} {
		t.Fatalf("entities, relations, attachments = %v, want [2 1 1]", got)
	}

	withDatabase(t, root, func(st store.Store) {
		g := snapshot(t, st)
		doc1 := g.Entities["DOC-1"]
		if doc1.Type != "doc" || doc1.Properties["title"] != "First" || doc1.Content != "Body of the first doc." {
			t.Errorf("DOC-1 = %+v", doc1)
		}
		rel := g.Relations["DOC-1--refs--DOC-2"]
		if rel.Properties["note"] != "see also" || rel.Content != "Why they relate." {
			t.Errorf("relation = %+v", rel)
		}
		if got := g.Attachments["DOC-1/files/report.txt"]; got != "attached bytes" {
			t.Errorf("attachment = %q", got)
		}
	})

	records := sink.Records()
	if len(records) != 1 || records[0].Op != audit.OpFSImport {
		t.Fatalf("audit records = %+v, want one %s record", records, audit.OpFSImport)
	}
}

func TestImportMarkdownData_NonEmptyDatabase(t *testing.T) {
	tests := []struct {
		name    string
		force   bool
		wantErr string
	}{
		{name: "refused without force", wantErr: "already holds 2 entities"},
		// The collision rolls the whole transaction back, so the count
		// stays at what the first import wrote.
		{name: "force collides on a stored id", force: true, wantErr: "DOC-1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := writeDataProject(t)
			if _, err := importData(t, root, false, audit.NewMemory()); err != nil {
				t.Fatal(err)
			}
			rep, err := importData(t, root, tc.force, audit.NewMemory())
			// The error says that the import failed; the report lists
			// each row that failed it.
			found := err
			if rep != nil {
				found = errors.Join(err, errors.New(strings.Join(rep.Errors, "\n")))
			}
			if err == nil || !strings.Contains(found.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want one containing %q", found, tc.wantErr)
			}
			withDatabase(t, root, func(st store.Store) {
				n, err := st.CountEntities(context.Background(), store.EntityQuery{Faces: store.AllFaces()})
				if err != nil {
					t.Fatal(err)
				}
				if n != 2 {
					t.Fatalf("entities = %d, want 2", n)
				}
			})
		})
	}
}

// With --force the import goes ahead beside the stored rows, and is one
// transaction: rows with new ids land, and nothing else changes.
func TestImportMarkdownData_ForceAddsBesideStoredRows(t *testing.T) {
	root := writeDataProject(t)
	other := t.TempDir()
	writeFile(t, other, "schema.yaml", dataSchemaYAML)
	writeFile(t, other, "entities/docs/DOC-9.md", "---\nid: DOC-9\ntype: doc\ntitle: Ninth\n---\n")

	if _, err := importData(t, root, false, audit.NewMemory()); err != nil {
		t.Fatal(err)
	}
	rep, err := appbuild.ImportMarkdownData(context.Background(), osFS(), projectAt(t, root), other,
		appbuild.DataImportOptions{Force: true, Audit: audit.NewMemory()})
	if err != nil {
		t.Fatalf("forced import: %v", err)
	}
	if rep.Entities != 1 {
		t.Fatalf("imported %d entities, want 1", rep.Entities)
	}
	withDatabase(t, root, func(st store.Store) {
		if n, _ := st.CountEntities(context.Background(), store.EntityQuery{Faces: store.AllFaces()}); n != 3 {
			t.Fatalf("entities = %d, want 3", n)
		}
	})
}

func TestImportMarkdownData_RequiresAudit(t *testing.T) {
	root := writeDataProject(t)
	if _, err := importData(t, root, false, nil); err == nil {
		t.Fatal("import without an audit sink succeeded")
	}
}

// Export then re-import gives back the same graph: the markdown the export
// writes is a faithful copy of the database.
func TestExportMarkdownData_RoundTrip(t *testing.T) {
	root := writeDataProject(t)
	if _, err := importData(t, root, false, audit.NewMemory()); err != nil {
		t.Fatal(err)
	}
	var want graph
	withDatabase(t, root, func(st store.Store) { want = snapshot(t, st) })

	out := filepath.Join(t.TempDir(), "out")
	sum, err := appbuild.ExportMarkdownData(context.Background(), osFS(), projectAt(t, root), out)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if want := (appbuild.DataSummary{Entities: 2, Relations: 1, Attachments: 1}); sum != want {
		t.Fatalf("summary = %+v, want %+v", sum, want)
	}

	// The export is a markdown project once the schema sits beside it.
	writeFile(t, out, "schema.yaml", dataSchemaYAML)
	if _, err := importData(t, out, false, audit.NewMemory()); err != nil {
		t.Fatalf("re-import: %v", err)
	}
	withDatabase(t, out, func(st store.Store) {
		if got := snapshot(t, st); !reflect.DeepEqual(got, want) {
			t.Fatalf("round trip differs:\n got  %+v\n want %+v", got, want)
		}
	})
}

func TestExportMarkdownData_RefusesExistingDataDirs(t *testing.T) {
	for _, dir := range []string{"entities", "relations", "attachments"} {
		t.Run(dir, func(t *testing.T) {
			root := writeDataProject(t)
			if _, err := importData(t, root, false, audit.NewMemory()); err != nil {
				t.Fatal(err)
			}
			out := t.TempDir()
			if err := os.MkdirAll(filepath.Join(out, dir), 0o755); err != nil {
				t.Fatal(err)
			}
			_, err := appbuild.ExportMarkdownData(context.Background(), osFS(), projectAt(t, root), out)
			if err == nil || !strings.Contains(err.Error(), "already exists") {
				t.Fatalf("err = %v, want an 'already exists' refusal", err)
			}
			entries, err := os.ReadDir(out)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Fatalf("export wrote into %s: %v", out, entries)
			}
		})
	}
}

// A markdown project with no database opens on its files, and nothing that
// only reads the database creates one: a database would make it open empty.
func TestSQLite_MarkdownProjectOpensOnItsFiles(t *testing.T) {
	root := writeDataProject(t)
	paths := projectAt(t, root)
	ctx := context.Background()
	if !appbuild.KeepsMarkdownData(paths) {
		t.Fatal("KeepsMarkdownData = false for a project with markdown entities")
	}

	svc, err := discover(t, root)
	if err != nil {
		t.Fatal(err)
	}
	n, err := svc.Store().CountEntities(ctx, store.EntityQuery{Faces: store.AllFaces()})
	_ = svc.Close()
	if err != nil || n != 2 {
		t.Fatalf("CountEntities = %d, %v; want the 2 markdown entities", n, err)
	}

	_, err = appbuild.DumpProjectConfig(ctx, osFS(), paths, t.TempDir(), false)
	if !errors.Is(err, appbuild.ErrNoDatabase) {
		t.Errorf("DumpProjectConfig err = %v, want ErrNoDatabase", err)
	}
	_, err = appbuild.ExportMarkdownData(ctx, osFS(), paths, t.TempDir())
	if !errors.Is(err, appbuild.ErrNoDatabase) {
		t.Errorf("ExportMarkdownData err = %v, want ErrNoDatabase", err)
	}
	_, err = appbuild.LoadProjectConfig(ctx, osFS(), paths, root)
	if err == nil || !strings.Contains(err.Error(), "import the data first") {
		t.Errorf("LoadProjectConfig err = %v, want a refusal", err)
	}
	if _, err := os.Stat(appbuild.DatabasePath(paths)); !os.IsNotExist(err) {
		t.Fatalf("a read created the database: %v", err)
	}

	// Importing the data creates the database, which opens from then on.
	if _, err := importData(t, root, false, audit.Nop{}); err != nil {
		t.Fatal(err)
	}
	if appbuild.KeepsMarkdownData(paths) {
		t.Error("KeepsMarkdownData = true after the import")
	}
}

// openData opens the project's database with every store it holds.
func openData(t *testing.T, root string) *appbuild.SQLiteData {
	t.Helper()
	data, err := appbuild.OpenSQLiteData(context.Background(), filepath.Join(root, ".rela", "rela.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = data.Close() })
	return data
}

const commentThread = "comments:\n- id: c1\n  author: alice\n" +
	"  created_at: 2026-01-01T09:00:00Z\n  anchor: {kind: property, ref: title}\n  body: hi\n"

// The import carries what the markdown project keeps beside its data:
// comment threads, runtime state and the applied-migration record.
func TestImportMarkdownData_CopiesCommentsStateAndMigrations(t *testing.T) {
	root := writeDataProject(t)
	writeFile(t, root, ".rela/comments/DOC-1.yaml", commentThread)
	writeFile(t, root, ".rela/user-defaults.yaml", "doc: {}\n")
	writeFile(t, root, ".rela/migration/state.json", `{"shape_hash":"abc","projection":{"entities":{}},`+
		`"applied":["20260101000000-first.yaml"],"updated_at":"2026-01-02T00:00:00Z"}`)

	rep, err := importData(t, root, false, audit.NewMemory())
	if err != nil {
		t.Fatalf("import: %v (problems: %v)", err, rep)
	}
	// Two state keys: the user defaults and the legacy migration marker,
	// which the migration record is read from but which is state too.
	if rep.Comments != 1 || rep.StateKeys != 2 {
		t.Fatalf("comments, state keys = %d, %d; want 1, 2", rep.Comments, rep.StateKeys)
	}

	data := openData(t, root)
	ctx := context.Background()
	list, err := data.Comments.List(ctx, comments.Target{Type: "doc", ID: "DOC-1"})
	if err != nil || len(list) != 1 || list[0].Author != "alice" {
		t.Fatalf("comments = %+v, %v", list, err)
	}
	got, err := data.State.Get(ctx, "user-defaults.yaml")
	if err != nil || string(got) != "doc: {}\n" {
		t.Fatalf("state = %q, %v", got, err)
	}
	st, err := data.Migrations.Load(ctx)
	if err != nil || st == nil || len(st.Applied) != 1 {
		t.Fatalf("migration record = %+v, %v", st, err)
	}
}

// One failing row rolls back everything, the state and comments written on
// the same transaction included.
func TestImportMarkdownData_FailureWritesNothing(t *testing.T) {
	root := writeDataProject(t)
	writeFile(t, root, ".rela/comments/DOC-1.yaml", commentThread)
	writeFile(t, root, ".rela/user-defaults.yaml", "doc: {}\n")
	// A value JSON cannot hold fails its row.
	writeFile(t, root, "entities/docs/DOC-3.md", "---\nid: DOC-3\ntype: doc\ntitle: .nan\n---\n")

	rep, err := importData(t, root, false, audit.NewMemory())
	if err == nil {
		t.Fatal("import succeeded; want the unstorable value to fail it")
	}
	if rep == nil || len(rep.Errors) == 0 {
		t.Fatalf("report lists no problems: %+v", rep)
	}

	data := openData(t, root)
	ctx := context.Background()
	if n, _ := data.Store.CountEntities(ctx, store.EntityQuery{Faces: store.AllFaces()}); n != 0 {
		t.Fatalf("entities = %d, want 0", n)
	}
	if _, err := data.State.Get(ctx, "user-defaults.yaml"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("state survived the rollback: %v", err)
	}
	if list, _ := data.Comments.List(ctx, comments.Target{Type: "doc", ID: "DOC-1"}); len(list) != 0 {
		t.Fatalf("comments survived the rollback: %+v", list)
	}
}

// A setting the database already holds is kept on a forced import, and the
// report says so.
func TestImportMarkdownData_KeepsExistingState(t *testing.T) {
	root := writeDataProject(t)
	if _, err := importData(t, root, false, audit.NewMemory()); err != nil {
		t.Fatal(err)
	}
	func() {
		data := openData(t, root)
		if err := data.State.Put(context.Background(), "user-defaults.yaml", []byte("mine\n")); err != nil {
			t.Fatal(err)
		}
		_ = data.Close()
	}()

	other := t.TempDir()
	writeFile(t, other, "schema.yaml", dataSchemaYAML)
	writeFile(t, other, "entities/docs/DOC-9.md", "---\nid: DOC-9\ntype: doc\ntitle: Ninth\n---\n")
	writeFile(t, other, ".rela/user-defaults.yaml", "theirs\n")
	rep, err := appbuild.ImportMarkdownData(context.Background(), osFS(), projectAt(t, root), other,
		appbuild.DataImportOptions{Force: true, Audit: audit.NewMemory()})
	if err != nil {
		t.Fatalf("forced import: %v (%+v)", err, rep)
	}
	got, err := openData(t, root).State.Get(context.Background(), "user-defaults.yaml")
	if err != nil || string(got) != "mine\n" {
		t.Fatalf("state = %q, %v; want the database's own value kept", got, err)
	}
}
