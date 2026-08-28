//go:build postgres

package appbuild_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// TestPostgresBuild_StateIsDatabaseBacked is the wiring assertion for
// TKT-VC27L3: on the postgres build, Services.State() must be the database
// store, NOT an FSKV under the project's .rela/.
//
// Asserting on the concrete type would be brittle and would not prove the data
// actually lands in the database. Instead this writes through the KV and then
// checks the project directory stayed clean — the observable difference an
// operator cares about, and the one that breaks when a future refactor
// accidentally reinstates the filesystem KV.
func TestPostgresBuild_StateIsDatabaseBacked(t *testing.T) {
	dsn := os.Getenv("RELA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("RELA_TEST_DATABASE_URL not set")
	}

	root := writeMinimalProject(t)
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	svc, err := appbuild.New(appbuild.Config{
		FS:           fs,
		Paths:        paths,
		ScriptEngine: script.NewEngine(),
		Audit:        audit.Nop{},
		DatabaseURL:  dsn,
	})
	if err != nil {
		t.Fatalf("appbuild.New: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	ctx := context.Background()
	const key = "documents/DOC-1-abc.html"
	if err = svc.State().Put(ctx, key, []byte("<html>rendered</html>")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := svc.State().Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "<html>rendered</html>" {
		t.Fatalf("Get = %q", got)
	}

	// The write must not have landed on disk: an FSKV would have created
	// .rela/documents/DOC-1-abc.html.
	onDisk := filepath.Join(paths.CacheDir, "documents", "DOC-1-abc.html")
	if _, statErr := os.Stat(onDisk); statErr == nil {
		t.Fatalf("state was written to %s — the postgres build must keep state in "+
			"the database, or it stays node-local across a load-balanced fleet", onDisk)
	}
}
