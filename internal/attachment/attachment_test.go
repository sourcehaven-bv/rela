package attachment_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"errors"

	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/templating"
)

// attachmentTestMetamodel adds a file-typed property to the ticket
// type so Attach can exercise its property-resolution paths.
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

// attachmentFixture is the bundle setupAttachmentService returns so
// tests can seed entities directly via the store without re-deriving
// it from the service.
type attachmentFixture struct {
	svc  *attachment.Service
	st   store.Store
	root string
}

// setupAttachmentService builds a real-FS-backed service on a
// tempdir. Uses app.FSFactory so the store knows where attachments
// live; mirrors the production wiring path.
func setupAttachmentService(t *testing.T) attachmentFixture {
	t.Helper()
	root := t.TempDir()
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
	meta, err := metamodel.Parse([]byte(attachmentTestMetamodel))
	if err != nil {
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
	factory := &app.FSFactory{FS: fs, Paths: ctx}
	st, err := factory.OpenStore(meta)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:     st,
		Meta:      meta,
		Templater: templating.NewFSTemplater(fs, ctx),
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	svc, err := attachment.New(attachment.Deps{
		Store:         st,
		Meta:          meta,
		EntityManager: mgr,
	})
	if err != nil {
		t.Fatalf("attachment.New: %v", err)
	}
	return attachmentFixture{svc: svc, st: st, root: root}
}

func TestService_AttachAndList(t *testing.T) {
	f := setupAttachmentService(t)

	e := entity.New("T-1", "ticket")
	e.Properties["title"] = "host"
	if err := f.st.CreateEntity(context.Background(), e); err != nil {
		t.Fatalf("create entity: %v", err)
	}

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "design.pdf")
	payload := []byte("pretend pdf")
	if err := os.WriteFile(srcPath, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := f.svc.Attach(context.Background(), "T-1", srcPath, "spec")
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	if result.FileName != "design.pdf" {
		t.Errorf("FileName = %q, want design.pdf", result.FileName)
	}

	onDisk := filepath.Join(f.root, "attachments", "T-1", "spec", "design.pdf")
	got, err := os.ReadFile(onDisk)
	if err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload on disk = %q, want %q", got, payload)
	}

	infos, err := f.svc.List(context.Background(), "T-1")
	if err != nil {
		t.Fatalf("List: %v", err)
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

func TestService_Attach_UnknownEntity(t *testing.T) {
	f := setupAttachmentService(t)
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "x.bin")
	_ = os.WriteFile(src, []byte("x"), 0o644)

	_, err := f.svc.Attach(context.Background(), "T-MISSING", src, "spec")
	if err == nil {
		t.Fatal("expected error for missing entity")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v, want store.ErrNotFound (wrapped)", err)
	}
	if !strings.Contains(err.Error(), "T-MISSING") {
		t.Errorf("err = %v, want entity ID in message", err)
	}
}

func TestService_Attach_NonFileProperty(t *testing.T) {
	f := setupAttachmentService(t)

	e := entity.New("T-2", "ticket")
	e.Properties["title"] = "host"
	if err := f.st.CreateEntity(context.Background(), e); err != nil {
		t.Fatalf("create entity: %v", err)
	}

	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "x.bin")
	_ = os.WriteFile(src, []byte("x"), 0o644)

	_, err := f.svc.Attach(context.Background(), "T-2", src, "title")
	if err == nil {
		t.Fatal("expected error for non-file property")
	}
	if !strings.Contains(err.Error(), "not a file type") {
		t.Errorf("err = %v, want non-file-type", err)
	}
}

func TestService_New_RejectsNilDeps(t *testing.T) {
	cases := []struct {
		name string
		d    attachment.Deps
		want string
	}{
		{"nil store", attachment.Deps{Store: nil}, "Store is required"},
		{"nil meta", attachment.Deps{Store: storeStub{}}, "Meta is required"},
		{"nil em", attachment.Deps{Store: storeStub{}, Meta: &metamodel.Metamodel{}}, "EntityManager is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := attachment.New(tc.d)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want substring %q", err, tc.want)
			}
		})
	}
}

// storeStub satisfies store.Store via embedded nil — type-correct
// but every method would nil-deref. Used ONLY in
// TestService_New_RejectsNilDeps to advance past the Store nil-check
// in New so the subsequent Meta / EntityManager nil-checks can be
// exercised. Never invoked.
type storeStub struct{ store.Store }
