//go:build sqlite

package appbuild_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const helloCard = "dashboard:\n  cards:\n    - title: Hello\n      display: count\n      query: \"type:doc prop:title=hello\"\n"

func writeDataEntry(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "data-entry.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func boot(t *testing.T, root string) {
	t.Helper()
	svc, err := discover(t, root)
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.Close(); err != nil {
		t.Fatal(err)
	}
}

func dbPath(root string) string { return filepath.Join(root, ".rela", "rela.db") }

// derivedQueryIndexes counts the derived query indexes in the project's
// database.
func derivedQueryIndexes(t *testing.T, root string) int {
	t.Helper()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: dbPath(root)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var n int
	if err := db.DB().QueryRowContext(context.Background(),
		`SELECT count(*) FROM sqlite_schema WHERE type = 'index' AND name GLOB 'rela_derived_query__*'`,
	).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func reconcileProject(t *testing.T, root string, opts store.ReconcileOptions) ([]store.DerivedObjectOutcome, error) {
	t.Helper()
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	if err != nil {
		t.Fatal(err)
	}
	return appbuild.ReconcileDerivedIndexes(context.Background(), fs, paths, opts)
}

// Boot converges the derived indexes the data-entry config asks for, reading
// that config through the same seam the server does (TKT-B51CYD).
func TestSQLiteBootCreatesDerivedIndexes(t *testing.T) {
	root := writeMinimalProject(t)
	writeDataEntry(t, root, helloCard)
	boot(t, root)
	if n := derivedQueryIndexes(t, root); n != 1 {
		t.Fatalf("derived query indexes after boot = %d, want 1", n)
	}
}

// An invalid config is not an empty desired set: boot keeps the indexes it
// has, and an explicit reconcile refuses rather than dropping them.
func TestSQLiteInvalidDataEntryKeepsDerivedIndexes(t *testing.T) {
	root := writeMinimalProject(t)
	writeDataEntry(t, root, helloCard)
	boot(t, root)

	writeDataEntry(t, root, "dashboard: [")
	boot(t, root)
	if n := derivedQueryIndexes(t, root); n != 1 {
		t.Fatalf("derived query indexes after an invalid config = %d, want 1", n)
	}
	if _, err := reconcileProject(t, root, store.ReconcileOptions{}); err == nil {
		t.Fatal("reconcile accepted an invalid data-entry config")
	}
	if n := derivedQueryIndexes(t, root); n != 1 {
		t.Fatalf("derived query indexes after a refused reconcile = %d, want 1", n)
	}
}

// A dry run neither creates the database nor changes its indexes.
func TestSQLiteReconcileDryRun(t *testing.T) {
	root := writeMinimalProject(t)
	writeDataEntry(t, root, helloCard)

	if _, err := reconcileProject(t, root, store.ReconcileOptions{DryRun: true}); !errors.Is(err, appbuild.ErrNoDatabase) {
		t.Fatalf("dry run without a database: err = %v, want ErrNoDatabase", err)
	}
	if _, err := os.Stat(dbPath(root)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry run created the database (stat err %v)", err)
	}

	boot(t, root)
	writeDataEntry(t, root, "")
	out, err := reconcileProject(t, root, store.ReconcileOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].State != store.DerivedDropped || !out[0].WouldChange {
		t.Fatalf("dry run outcomes = %+v, want one would-drop", out)
	}
	if n := derivedQueryIndexes(t, root); n != 1 {
		t.Fatalf("dry run changed the indexes: %d, want 1", n)
	}

	if _, err = reconcileProject(t, root, store.ReconcileOptions{}); err != nil {
		t.Fatal(err)
	}
	if n := derivedQueryIndexes(t, root); n != 0 {
		t.Fatalf("reconcile kept an undeclared index: %d", n)
	}
}
