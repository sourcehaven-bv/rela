// The fixtures are hand-written ENTITY FILES, which only the fs-backed
// build loads. Under `-tags postgres` or `-tags sqlite` entities live in
// the database and the on-disk files are never read, so the probe counts
// zero and the warning legitimately stays silent. The warning logic itself
// is backend-agnostic (two CountEntities calls); the fs build covers it.
//
// The sqlite exclusion is BUG-LL3C07: the tag was `!postgres`, which swept
// sqlite in when that backend arrived, and the resulting failure sat on
// develop unnoticed because CI built the sqlite tag without ever running its
// tests. Both halves are fixed — the exclusion here, and a CI job that would
// have caught it.
//go:build !postgres && !memorybackend && !sqlite

package appbuild_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeEntityFile writes a hand-authored entity file under
// root/entities/<plural>/<name>.
func writeEntityFile(t *testing.T, root, plural, name, content string) {
	t.Helper()
	dir := filepath.Join(root, "entities", plural)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TKT-DOFYR1: a project whose store holds content-state rows boots with
// a warning (never a refusal — a schema edit must not brick the
// project), pointing the operator at `rela analyze states`.
//
// Reuses the warn-capture helper from the membership-warning tests —
// same no-t.Parallel() constraint (global slog).
func TestBuild_StateRows_WarnAtStartup(t *testing.T) {
	root := t.TempDir()
	writeMetamodel(t, root)
	writeEntityFile(t, root, "tickets", "TKT-001.md",
		"---\nid: TKT-001\ntype: ticket\ntitle: default\n---\n")
	writeEntityFile(t, root, "tickets", "TKT-001@draft.md",
		"---\nid: TKT-001\ntype: ticket\ntitle: draft\n---\n")

	logs := captureWarnings(t)

	svc, err := appbuildOnDisk(t, root)
	if err != nil {
		t.Fatalf("appbuild.New: %v", err)
	}
	defer svc.Close()

	got := logs()
	for _, want := range []string{"content-state rows", "analyze states"} {
		if !strings.Contains(got, want) {
			t.Errorf("startup warning missing %q; logs:\n%s", want, got)
		}
	}
}

// A faceless project must stay silent — the warning must not become
// boot noise.
func TestBuild_NoStateRows_NoWarning(t *testing.T) {
	root := t.TempDir()
	writeMetamodel(t, root)
	writeEntityFile(t, root, "tickets", "TKT-001.md",
		"---\nid: TKT-001\ntype: ticket\ntitle: plain\n---\n")

	logs := captureWarnings(t)

	svc, err := appbuildOnDisk(t, root)
	if err != nil {
		t.Fatalf("appbuild.New: %v", err)
	}
	defer svc.Close()

	if got := logs(); strings.Contains(got, "content-state rows") {
		t.Errorf("expected no state warning, got logs:\n%s", got)
	}
}
