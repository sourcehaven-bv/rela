//go:build sqlite

package appbuild_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// bake stores root's config in root's database, as `rela db load` does.
func bake(t *testing.T, root string) []string {
	t.Helper()
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	if err != nil {
		t.Fatal(err)
	}
	names, err := appbuild.LoadProjectConfig(context.Background(), fs, paths, root,
		appbuild.ConfigImportOptions{Audit: audit.Nop{}})
	if err != nil {
		t.Fatalf("LoadProjectConfig: %v", err)
	}
	return names
}

func dump(t *testing.T, root, dir string, overwrite bool) ([]string, error) {
	t.Helper()
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	if err != nil {
		t.Fatal(err)
	}
	return appbuild.DumpProjectConfig(context.Background(), fs, paths, dir, overwrite)
}

func removeAll(t *testing.T, root string, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		if err := os.RemoveAll(filepath.Join(root, rel)); err != nil {
			t.Fatal(err)
		}
	}
}

// A project whose config lives only in its database boots: the schema is
// read through the layered loader, which needs the database open BEFORE
// prepare runs (FEAT-UP14BT).
func TestSQLite_BootsFromBakedConfig(t *testing.T) {
	root := writeMinimalProject(t)
	bake(t, root)
	removeAll(t, root, "metamodel.yaml", "entities", "relations")

	svc, err := discover(t, root)
	if err != nil {
		t.Fatalf("boot from baked config: %v", err)
	}
	defer func() { _ = svc.Close() }()
	if _, ok := svc.Meta().Entities["doc"]; !ok {
		t.Fatalf("baked schema not loaded; entity types = %v", svc.Meta().Entities)
	}
}

// A file on disk wins over the baked copy: a project holding both is being
// edited, and the file the operator just wrote is the one that counts.
func TestSQLite_DiskConfigShadowsBaked(t *testing.T) {
	root := writeMinimalProject(t)
	bake(t, root)
	edited := strings.Replace(metamodelYAML, "  doc:\n", "  note:\n    label: Note\n    plural: notes\n    id_prefix: \"NOTE-\"\n    id_type: sequential\n    properties:\n      title:\n        type: string\n  doc:\n", 1)
	writeFile(t, root, "metamodel.yaml", edited)

	svc, err := discover(t, root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = svc.Close() }()
	if _, ok := svc.Meta().Entities["note"]; !ok {
		t.Fatal("disk schema did not shadow the baked one")
	}
}

// acl.yaml is read through the same loader: a baked policy that does not
// parse fails the boot, which proves the baked copy is what was read.
func TestSQLite_BakedACLIsRead(t *testing.T) {
	root := writeMinimalProject(t)
	writeFile(t, root, "acl.yaml", "roles: [this is not a map\n")
	bake(t, root)
	removeAll(t, root, "acl.yaml")

	svc, err := discover(t, root)
	if err == nil {
		_ = svc.Close()
		t.Fatal("boot succeeded with a broken baked acl.yaml; it was not read")
	}
	if !strings.Contains(err.Error(), "acl.yaml") {
		t.Fatalf("error does not name acl.yaml: %v", err)
	}
}

func TestSQLite_CollectProjectConfig(t *testing.T) {
	root := writeMinimalProject(t)
	writeFile(t, root, "data-entry.yaml", helloCard)
	writeFile(t, root, "scripts/docs/report.lua", "return 1\n")
	writeFile(t, root, "scripts/.hidden.lua", "return 2\n")
	writeFile(t, root, "templates/entities/doc.md", "---\n---\n")
	writeFile(t, root, "migrations/applied.json", "[]\n")
	writeFile(t, root, "README.md", "not config\n")

	files, err := appbuild.CollectProjectConfig(storage.NewSafeFS(storage.NewOsFS()), root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for name := range files {
		got = append(got, name)
	}
	slices.Sort(got)
	want := []string{"data-entry.yaml", "schema.yaml", "scripts/docs/report.lua", "templates/entities/doc.md"}
	if !slices.Equal(got, want) {
		t.Fatalf("collected %v, want %v", got, want)
	}
	if string(files["schema.yaml"]) != metamodelYAML {
		t.Error("legacy metamodel.yaml was not stored under schema.yaml")
	}
}

// Load replaces the stored set: a file removed from disk does not live on.
func TestSQLite_LoadReplacesTheSet(t *testing.T) {
	root := writeMinimalProject(t)
	writeFile(t, root, "scripts/old.lua", "return 1\n")
	bake(t, root)
	removeAll(t, root, "scripts/old.lua")
	names := bake(t, root)
	if slices.Contains(names, "scripts/old.lua") {
		t.Fatalf("second load kept a removed file: %v", names)
	}

	out := t.TempDir()
	dumped, err := dump(t, root, out, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(dumped, []string{"schema.yaml"}) {
		t.Fatalf("dumped %v, want only schema.yaml", dumped)
	}
}

func TestSQLite_DumpRoundTripsAndRefusesOverwrite(t *testing.T) {
	root := writeMinimalProject(t)
	writeFile(t, root, "data-entry.yaml", helloCard)
	writeFile(t, root, "scripts/docs/report.lua", "return 1\n")
	bake(t, root)

	out := t.TempDir()
	if _, err := dump(t, root, out, false); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]string{
		"schema.yaml":             metamodelYAML,
		"data-entry.yaml":         helloCard,
		"scripts/docs/report.lua": "return 1\n",
	} {
		got, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q (%v), want %q", rel, got, err, want)
		}
	}

	if _, err := dump(t, root, out, false); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second dump without overwrite: err = %v, want an already-exists refusal", err)
	}
	if _, err := dump(t, root, out, true); err != nil {
		t.Fatalf("dump with overwrite: %v", err)
	}
}

// insertRawProjectFile writes a row straight into project_files, bypassing
// configsql's validation, as a database built by something other than rela
// could.
func insertRawProjectFile(t *testing.T, root, name string) {
	t.Helper()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: dbPath(root)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.DB().Exec(
		`INSERT INTO project_files (path, content, updated_at) VALUES (?, x'78', '')`, name,
	); err != nil {
		t.Fatal(err)
	}
}

// Dump re-checks every stored name: a handed-over database may hold a path
// that would escape the target directory.
func TestSQLite_DumpRefusesEscapingNames(t *testing.T) {
	for _, name := range []string{
		"../escape.yaml", "/abs.yaml", "a/../../b.yaml",
		// Inside the target, but not a config file a project can have.
		".git/config", ".rela/mail.yaml", "scripts/.hidden.lua", "entities/docs/DOC-1.md", "install.sh",
	} {
		t.Run(name, func(t *testing.T) {
			root := writeMinimalProject(t)
			bake(t, root)
			insertRawProjectFile(t, root, name)
			out := filepath.Join(t.TempDir(), "out")
			if _, err := dump(t, root, out, true); err == nil || !strings.Contains(err.Error(), "refusing") {
				t.Fatalf("dump err = %v, want a refusal", err)
			}
			if _, err := os.Stat(filepath.Join(out, "schema.yaml")); !os.IsNotExist(err) {
				t.Error("dump wrote files before refusing")
			}
		})
	}
}

// Dump never writes through a symlink, at the leaf or at a parent directory.
func TestSQLite_DumpRefusesSymlinks(t *testing.T) {
	root := writeMinimalProject(t)
	writeFile(t, root, "scripts/a.lua", "return 1\n")
	bake(t, root)

	elsewhere := t.TempDir()
	for name, link := range map[string]string{
		"leaf":   "schema.yaml",
		"parent": "scripts",
	} {
		t.Run(name, func(t *testing.T) {
			out := t.TempDir()
			if err := os.Symlink(elsewhere, filepath.Join(out, link)); err != nil {
				t.Fatal(err)
			}
			if _, err := dump(t, root, out, true); err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("dump err = %v, want a symlink refusal", err)
			}
		})
	}
	if entries, _ := os.ReadDir(elsewhere); len(entries) != 0 {
		t.Fatalf("dump wrote through a symlink: %v", entries)
	}
}

// A symlink anywhere in the config set fails the collection: following one
// could bake .rela/secrets.yaml into a file meant to be shipped, and skipping
// it would ship a database that boots differently from its directory.
func TestSQLite_CollectRefusesSymlinks(t *testing.T) {
	cases := map[string]func(t *testing.T, root, secret string){
		"root file": func(t *testing.T, root, secret string) {
			t.Helper()
			mustSymlink(t, secret, filepath.Join(root, "acl.yaml"))
		},
		"file in a config dir": func(t *testing.T, root, secret string) {
			t.Helper()
			writeFile(t, root, "scripts/ok.lua", "return 1\n")
			mustSymlink(t, secret, filepath.Join(root, "scripts", "link.lua"))
		},
		"config dir": func(t *testing.T, root, _ string) {
			t.Helper()
			mustSymlink(t, t.TempDir(), filepath.Join(root, "scripts"))
		},
		"schema include of a hidden file": func(t *testing.T, root, _ string) {
			t.Helper()
			appendInclude(t, root, ".rela/secrets.yaml")
		},
		"schema include in a symlinked directory": func(t *testing.T, root, _ string) {
			t.Helper()
			elsewhere := t.TempDir()
			writeFile(t, elsewhere, "types.yaml", "x: 1\n")
			mustSymlink(t, elsewhere, filepath.Join(root, "shared"))
			appendInclude(t, root, "shared/types.yaml")
		},
	}
	for name, plant := range cases {
		t.Run(name, func(t *testing.T) {
			root := writeMinimalProject(t)
			writeFile(t, root, ".rela/secrets.yaml", "smtp_password: hunter2\n")
			plant(t, root, filepath.Join(root, ".rela", "secrets.yaml"))
			_, err := appbuild.CollectProjectConfig(storage.NewSafeFS(storage.NewOsFS()), root)
			if err == nil || !strings.Contains(err.Error(), "refusing") {
				t.Fatalf("err = %v, want a refusal", err)
			}
		})
	}
}

// appendInclude adds name to the fixture schema's includes.
func appendInclude(t *testing.T, root, name string) {
	t.Helper()
	path := filepath.Join(root, "metamodel.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte("includes:\n  - "+name+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

// Scripts baked into the database are read through the same seam as the
// schema: with the files gone from disk, the Lua deps and ProjectFiles still
// serve them.
func TestSQLite_BakedScriptsAreRead(t *testing.T) {
	root := writeMinimalProject(t)
	writeFile(t, root, "scripts/docs/report.lua", "return 'baked'\n")
	bake(t, root)
	removeAll(t, root, "scripts")

	svc, err := discover(t, root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = svc.Close() }()
	ctx := context.Background()
	got, err := svc.LuaReadDeps().ReadScript(ctx, "scripts", "docs/report.lua")
	if err != nil || got != "return 'baked'\n" {
		t.Fatalf("ReadScript = %q, %v", got, err)
	}
	if _, err := svc.ProjectFiles().Load(ctx, "scripts/docs/report.lua"); err != nil {
		t.Fatalf("ProjectFiles().Load: %v", err)
	}
}

// custom/ and apps/ are served through ProjectFiles too, which must support
// the Stat and Dirs the data-entry handlers need.
func TestSQLite_BakedAssetsAreServed(t *testing.T) {
	root := writeMinimalProject(t)
	writeFile(t, root, "custom/theme.css", "body{}\n")
	writeFile(t, root, "apps/demo/index.html", "<html></html>\n")
	bake(t, root)
	removeAll(t, root, "custom", "apps")

	svc, err := discover(t, root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = svc.Close() }()
	ctx := context.Background()
	files := svc.ProjectFiles()
	stater, ok := files.(config.Stater)
	if !ok {
		t.Fatalf("ProjectFiles (%T) is not a config.Stater", files)
	}
	if info, err := stater.Stat(ctx, "custom/theme.css"); err != nil || info.Size() != int64(len("body{}\n")) {
		t.Fatalf("Stat(custom/theme.css) = %v, %v", info, err)
	}
	lister, ok := files.(config.DirLister)
	if !ok {
		t.Fatalf("ProjectFiles (%T) is not a config.DirLister", files)
	}
	if dirs, err := lister.Dirs(ctx, "apps"); err != nil || !slices.Equal(dirs, []string{"demo"}) {
		t.Fatalf("Dirs(apps) = %v, %v", dirs, err)
	}
}

// Entity templates the database carries are used when the files are gone.
func TestSQLite_BakedTemplatesAreUsed(t *testing.T) {
	root := writeMinimalProject(t)
	writeFile(t, root, "templates/entities/doc.md", "---\ntitle: From the database\n---\nBody\n")
	writeFile(t, root, "templates/entities/doc--short.md", "---\ntitle: Short\n---\n")
	bake(t, root)
	removeAll(t, root, "templates")

	svc, err := discover(t, root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = svc.Close() }()
	ctx := context.Background()
	tmpl, err := svc.Templater().EntityTemplate(ctx, "doc", "")
	if err != nil || tmpl == nil {
		t.Fatalf("EntityTemplate = %v, %v; want the baked template", tmpl, err)
	}
	all, err := svc.Templater().EntityTemplates(ctx, "doc")
	if err != nil || len(all) != 2 {
		t.Fatalf("EntityTemplates = %d templates, %v; want 2", len(all), err)
	}
}

// A config load leaves one audit record, like the data import: it bypasses
// the entitymanager, so the record is its only trace. The record names the
// source and the count, never a stored file's content.
func TestSQLite_LoadProjectConfigWritesOneAuditRecord(t *testing.T) {
	root := writeMinimalProject(t)
	const secretBody = "return 'do-not-echo'\n"
	writeFile(t, root, "scripts/report.lua", secretBody)
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	if err != nil {
		t.Fatal(err)
	}
	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: "cli"})
	sink := audit.NewMemory()

	names, err := appbuild.LoadProjectConfig(ctx, fs, paths, root, appbuild.ConfigImportOptions{Audit: sink})
	if err != nil {
		t.Fatal(err)
	}

	records := sink.Records()
	if len(records) != 1 || records[0].Op != audit.OpConfigImport {
		t.Fatalf("audit records = %+v, want one %s record", records, audit.OpConfigImport)
	}
	rec := records[0]
	if rec.Principal.User != "alice" || rec.Principal.Tool != "cli" {
		t.Errorf("principal = %+v, want alice/cli", rec.Principal)
	}
	if !strings.Contains(rec.Summary, root) || !strings.Contains(rec.Summary, strconv.Itoa(len(names))+" files") {
		t.Errorf("summary %q does not name the source and the file count", rec.Summary)
	}
	if strings.Contains(rec.Summary, "do-not-echo") || strings.Contains(rec.Summary, "report.lua") {
		t.Errorf("summary %q echoes a stored file", rec.Summary)
	}
}

func TestSQLite_ConfigImportRequiresAudit(t *testing.T) {
	root := writeMinimalProject(t)
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		run  func() error
	}{
		{"load", func() error {
			_, err := appbuild.LoadProjectConfig(context.Background(), fs, paths, root, appbuild.ConfigImportOptions{})
			return err
		}},
		{"store", func() error {
			_, err := appbuild.StoreProjectConfig(context.Background(), paths, map[string][]byte{},
				appbuild.ConfigImportOptions{})
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil || !strings.Contains(err.Error(), "audit sink") {
				t.Fatalf("err = %v, want an audit sink refusal", err)
			}
			if _, err := os.Stat(appbuild.DatabasePath(paths)); !os.IsNotExist(err) {
				t.Errorf("refused import created the database (stat err = %v)", err)
			}
		})
	}
}
