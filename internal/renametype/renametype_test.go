package renametype_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/renametype"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// renametypeTestMetamodel defines two types so Rename has a real
// entity to rewrite. The `tickets` type uses plural form `tickets`
// to match the on-disk directory layout below.
const renametypeTestMetamodel = `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "T-"
    id_type: sequential
    properties:
      title:
        type: string
  decision:
    label: Decision
    plural: decisions
    id_prefix: "D-"
    id_type: sequential
    properties:
      title:
        type: string
`

type renametypeFixture struct {
	svc  *renametype.Service
	fs   storage.FS
	root string
	ctx  *project.Context
}

func setupService(t *testing.T) renametypeFixture {
	t.Helper()
	fs := storage.NewMemFS()
	root := "/proj"
	metaPath := filepath.Join(root, "metamodel.yaml")
	if err := fs.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.WriteFile(metaPath, []byte(renametypeTestMetamodel), 0o644); err != nil {
		t.Fatal(err)
	}
	meta, err := metamodel.Parse([]byte(renametypeTestMetamodel))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	ctx := &project.Context{
		Root:               root,
		MetamodelPath:      metaPath,
		EntitiesDir:        filepath.Join(root, "entities"),
		RelationsDir:       filepath.Join(root, "relations"),
		EntityTemplatesDir: filepath.Join(root, "templates", "entities"),
	}
	svc, err := renametype.New(renametype.Deps{
		FS:    fs,
		Meta:  meta,
		Paths: ctx,
	})
	if err != nil {
		t.Fatalf("renametype.New: %v", err)
	}
	return renametypeFixture{svc: svc, fs: fs, root: root, ctx: ctx}
}

func TestService_Rename(t *testing.T) {
	f := setupService(t)

	ticketsDir := filepath.Join(f.root, "entities", "tickets")
	_ = f.fs.MkdirAll(ticketsDir, 0o755)
	_ = f.fs.WriteFile(filepath.Join(ticketsDir, "T-1.md"), []byte(`---
id: T-1
type: ticket
title: First
---
body
`), 0o644)
	_ = f.fs.WriteFile(filepath.Join(ticketsDir, "T-2.md"), []byte(`---
id: T-2
type: ticket
title: Second
---
`), 0o644)

	count, err := f.svc.Rename("ticket", "issue", "issues")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	newDir := filepath.Join(f.root, "entities", "issues")
	got, err := f.fs.ReadFile(filepath.Join(newDir, "T-1.md"))
	if err != nil {
		t.Fatalf("read renamed file: %v", err)
	}
	if !strings.Contains(string(got), "type: issue") {
		t.Errorf("T-1 type not rewritten:\n%s", got)
	}

	meta, _ := f.fs.ReadFile(f.ctx.MetamodelPath)
	if !strings.Contains(string(meta), "issue:") {
		t.Errorf("metamodel not updated:\n%s", meta)
	}
}

func TestService_Rename_UnknownType(t *testing.T) {
	f := setupService(t)
	_, err := f.svc.Rename("does-not-exist", "x", "xs")
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
	if !strings.Contains(err.Error(), "unknown entity type") {
		t.Errorf("err = %v, want unknown-entity-type message", err)
	}
}

func TestService_Rename_MissingEntityDirSucceeds(t *testing.T) {
	f := setupService(t)
	// No entities/tickets dir created — rename should still succeed
	// (metamodel update happens; per-file step finds nothing to do).
	count, err := f.svc.Rename("ticket", "issue", "issues")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestService_New_RejectsNilDeps(t *testing.T) {
	// Zero-value Metamodel/Context are passed only to advance past
	// earlier nil-checks so the *next* nil-check is exercised. They
	// are never dereferenced. fs (MemFS) is similarly a real value
	// that the constructor only checks for non-nil.
	fs := storage.NewMemFS()
	meta := &metamodel.Metamodel{}
	ctx := &project.Context{}

	cases := []struct {
		name string
		d    renametype.Deps
		want string
	}{
		{"nil fs", renametype.Deps{Meta: meta, Paths: ctx}, "FS is required"},
		{"nil meta", renametype.Deps{FS: fs, Paths: ctx}, "Meta is required"},
		{"nil paths", renametype.Deps{FS: fs, Meta: meta}, "Paths is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := renametype.New(tc.d)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want substring %q", err, tc.want)
			}
		})
	}
}
