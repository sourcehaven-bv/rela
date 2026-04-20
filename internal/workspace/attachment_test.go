package workspace

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// attachmentTestMetamodel adds a file-typed property to the ticket
// type so AttachFile can exercise its property-resolution paths.
const attachmentTestMetamodel = `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "T-"
    id_type: sequential
    properties:
      title:
        type: string
      spec:
        type: file
`

// setupAttachmentWorkspace builds a real-FS workspace on a tempdir
// wired through app.FSFactory so the store knows where attachments
// live.
func setupAttachmentWorkspace(t *testing.T) (ws *Workspace, root string) {
	t.Helper()
	root = t.TempDir()
	for _, d := range []string{
		filepath.Join(root, ".rela"),
		filepath.Join(root, "entities", "tickets"),
		filepath.Join(root, "relations"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	metaPath := filepath.Join(root, "metamodel.yaml")
	if err := os.WriteFile(metaPath, []byte(attachmentTestMetamodel), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := metamodel.Parse([]byte(attachmentTestMetamodel)); err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	ctx := &project.Context{
		Root:          root,
		MetamodelPath: metaPath,
		CacheDir:      filepath.Join(root, ".rela"),
		EntitiesDir:   filepath.Join(root, "entities"),
		RelationsDir:  filepath.Join(root, "relations"),
	}
	fs := storage.NewSafeFS(storage.NewOsFS())
	w, err := New(fs, ctx, NopScriptExecutor, WithStoreFactory(&app.FSFactory{FS: fs, Paths: ctx}))
	if err != nil {
		t.Fatalf("New workspace: %v", err)
	}
	return w, root
}

func TestWorkspace_AttachAndList(t *testing.T) {
	ws, root := setupAttachmentWorkspace(t)

	e := entity.New("T-1", "ticket")
	e.Properties["title"] = "host"
	ws.SeedEntityForTest(e)

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "design.pdf")
	payload := []byte("pretend pdf")
	if err := os.WriteFile(srcPath, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := ws.AttachFile("T-1", srcPath, "spec")
	if err != nil {
		t.Fatalf("AttachFile: %v", err)
	}
	if result.FileName != "design.pdf" {
		t.Errorf("FileName = %q, want design.pdf", result.FileName)
	}

	onDisk := filepath.Join(root, "attachments", "T-1", "spec", "design.pdf")
	got, err := os.ReadFile(onDisk)
	if err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload on disk = %q, want %q", got, payload)
	}

	infos, err := ws.ListAttachments("T-1")
	if err != nil {
		t.Fatalf("ListAttachments: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("got %d attachments, want 1", len(infos))
	}
	info := infos[0]
	if info.FileName != "design.pdf" {
		t.Errorf("FileName = %q", info.FileName)
	}
	if info.Property != "spec" {
		t.Errorf("Property = %q", info.Property)
	}
	if !strings.Contains(info.Path, "attachments/T-1/spec/design.pdf") {
		t.Errorf("Path = %q", info.Path)
	}
	if info.ContentType != "application/pdf" {
		t.Errorf("ContentType = %q, want application/pdf", info.ContentType)
	}
}

func TestWorkspace_AttachFile_UnknownEntity(t *testing.T) {
	ws, _ := setupAttachmentWorkspace(t)
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "x.bin")
	_ = os.WriteFile(src, []byte("x"), 0o644)

	_, err := ws.AttachFile("T-MISSING", src, "spec")
	if err == nil {
		t.Fatal("expected error for missing entity")
	}
	if !strings.Contains(err.Error(), "entity not found") {
		t.Errorf("err = %v, want entity-not-found", err)
	}
}

func TestWorkspace_AttachFile_NonFileProperty(t *testing.T) {
	ws, _ := setupAttachmentWorkspace(t)
	e := entity.New("T-2", "ticket")
	e.Properties["title"] = "host"
	ws.SeedEntityForTest(e)

	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "x.bin")
	_ = os.WriteFile(src, []byte("x"), 0o644)

	_, err := ws.AttachFile("T-2", src, "title")
	if err == nil {
		t.Fatal("expected error for non-file property")
	}
	if !strings.Contains(err.Error(), "not a file type") {
		t.Errorf("err = %v, want non-file-type", err)
	}
}

func TestContentTypeForName(t *testing.T) {
	cases := map[string]string{
		"foo.pdf":     "application/pdf",
		"diagram.png": "image/png",
		"no-ext":      "application/octet-stream",
		"":            "application/octet-stream",
	}
	for in, want := range cases {
		got := contentTypeForName(in)
		if got != want && !strings.HasPrefix(got, want) {
			t.Errorf("contentTypeForName(%q) = %q, want %q (or prefix)", in, got, want)
		}
	}
}
