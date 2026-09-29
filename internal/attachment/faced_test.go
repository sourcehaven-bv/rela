package attachment_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/templating"
)

// facedAttachmentMeta declares a faced type with a file property, copies
// between its faces (every field, one mapped non-file field, and a mapping
// into the file property), and a cross-entity copy to an unfaced type.
const facedAttachmentMeta = `version: "1"
entities:
  page:
    label: Page
    id_prefix: PAGE
    faces:
      live: {}
      review: {}
    properties:
      title: {type: string}
      spec: {type: file}
  sheet:
    label: Sheet
    id_prefix: SHEET
    properties:
      title: {type: string}
      spec: {type: file}
copies:
  stage-review:
    from: page@live
    to: page@review
    fields: all
    guard:
      permission: stage
  retitle-review:
    from: page@live
    to: page@review
    fields:
      title: "{{new.title}}"
    guard:
      permission: stage
  mint-review:
    from: page@live
    to: page@review
    fields:
      spec: "{{new.title}}"
    guard:
      permission: stage
  page-to-sheet:
    from: page@live
    to: sheet
    fields:
      title: "{{new.title}}"
      spec: "{{new.spec}}"
`

type nopTemplater struct{}

func (nopTemplater) EntityTemplate(context.Context, string, string) (*templating.Template, error) {
	return nil, nil //nolint:nilnil // miss is not an error at this layer
}

func (nopTemplater) RelationTemplate(context.Context, string) (*templating.Template, error) {
	return nil, nil //nolint:nilnil // miss is not an error at this layer
}

type allowCopies struct{}

func (allowCopies) HoldsPermission(context.Context, string, string) bool { return true }

type facedFixture struct {
	svc *attachment.Service
	st  store.Store
	mgr *entitymanager.Manager
}

func newFacedFixture(t *testing.T) facedFixture {
	t.Helper()
	meta, err := metamodel.Parse([]byte(facedAttachmentMeta))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:            st,
		Meta:             meta,
		Templater:        nopTemplater{},
		Audit:            audit.Nop{},
		ACL:              acl.NopACL{},
		Transitions:      statemachine.EmptySet(),
		FieldGate:        entitymanager.AllowAllFieldGate{},
		CopyGuard:        allowCopies{},
		AttachmentLocker: lock.NewMemoryLocker(),
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	owner, err := entitymanager.AttachmentsOf(mgr)
	if err != nil {
		t.Fatalf("AttachmentsOf: %v", err)
	}
	svc, err := attachment.New(attachment.Deps{
		Store: st, Meta: meta, EntityManager: owner, Locker: owner,
		Authorizer: attachment.AllowAllWrites{},
	})
	if err != nil {
		t.Fatalf("attachment.New: %v", err)
	}
	return facedFixture{svc: svc, st: st, mgr: mgr}
}

// seedLive creates id at the live face with spec.txt attached there.
func (f facedFixture) seedLive(t *testing.T, id string) {
	t.Helper()
	ctx := context.Background()
	if err := f.st.CreateEntity(ctx, &entity.Entity{
		ID: id, Type: "page", Face: "live", Properties: map[string]any{"title": "live"},
	}); err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
	src := filepath.Join(t.TempDir(), "spec.txt")
	if err := os.WriteFile(src, []byte("spec bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Attach(ctx, entity.Ref{ID: id, Face: "live"}, src, "spec"); err != nil {
		t.Fatalf("attach on %s@live: %v", id, err)
	}
}

func (f facedFixture) references(t *testing.T, id string, face entity.Face) bool {
	t.Helper()
	e, err := f.st.GetEntityState(context.Background(), id, face)
	if errors.Is(err, store.ErrNotFound) {
		return false
	}
	if err != nil {
		t.Fatalf("read %s@%s: %v", id, face, err)
	}
	return attachment.References(e, "spec", "spec.txt")
}

func (f facedFixture) hasBytes(t *testing.T, id string) bool {
	t.Helper()
	rc, err := f.st.ReadAttachment(context.Background(), id, "spec", "spec.txt")
	if errors.Is(err, store.ErrNotFound) {
		return false
	}
	if err != nil {
		t.Fatalf("read bytes of %s: %v", id, err)
	}
	_ = rc.Close()
	return true
}

// A copy with `fields: all` gives the target face a reference to the same
// bytes; a detach on one face keeps them, and the last one removes them.
func TestFaced_CopyFieldsAllSharesBytes(t *testing.T) {
	f := newFacedFixture(t)
	ctx := context.Background()
	f.seedLive(t, "PAGE-1")

	if _, err := f.mgr.CopyState(ctx, entitymanager.CopyRequest{
		Definition: "stage-review", SourceID: "PAGE-1",
	}); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if !f.references(t, "PAGE-1", "review") {
		t.Fatal("the copy did not carry the file reference to review")
	}

	if _, err := f.svc.Detach(ctx, entity.Ref{ID: "PAGE-1", Face: "live"}, "spec", "spec.txt"); err != nil {
		t.Fatalf("detach on live: %v", err)
	}
	if !f.hasBytes(t, "PAGE-1") {
		t.Fatal("detach on live removed bytes review still references")
	}
	if _, err := f.svc.Detach(ctx, entity.Ref{ID: "PAGE-1", Face: "review"}, "spec", "spec.txt"); err != nil {
		t.Fatalf("detach on review: %v", err)
	}
	if f.hasBytes(t, "PAGE-1") {
		t.Error("bytes survived the last reference's detach")
	}
}

// A copy racing a detach of its source file ends in one of two states:
// the target references the file and the bytes exist, or neither. Never a
// reference to deleted bytes. Run under -race.
func TestFaced_CopyRacingDetachKeepsReferencesAndBytesTogether(t *testing.T) {
	f := newFacedFixture(t)
	ctx := context.Background()
	const rounds = 20
	for i := range rounds {
		id := fmt.Sprintf("PAGE-%d", i+1)
		f.seedLive(t, id)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := f.mgr.CopyState(ctx, entitymanager.CopyRequest{
				Definition: "stage-review", SourceID: id,
			}); err != nil {
				t.Errorf("copy %s: %v", id, err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := f.svc.Detach(ctx, entity.Ref{ID: id, Face: "live"}, "spec", "spec.txt"); err != nil {
				t.Errorf("detach %s: %v", id, err)
			}
		}()
		wg.Wait()

		ref, data := f.references(t, id, "review"), f.hasBytes(t, id)
		if ref != data {
			t.Errorf("%s: review references the file = %v, bytes exist = %v; want both or neither", id, ref, data)
		}
	}
}

// A bare id on a faced entity is refused, naming the faces; an addressed
// face lists only its own files.
func TestFaced_BareIDRefusedAndListIsPerFace(t *testing.T) {
	f := newFacedFixture(t)
	ctx := context.Background()
	f.seedLive(t, "PAGE-1")
	if err := f.st.CreateEntity(ctx, &entity.Entity{
		ID: "PAGE-1", Type: "page", Face: "review", Properties: map[string]any{"title": "review"},
	}); err != nil {
		t.Fatalf("seed review: %v", err)
	}

	src := filepath.Join(t.TempDir(), "other.txt")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.Attach(ctx, entity.Ref{ID: "PAGE-1"}, src, "spec")
	if !errors.Is(err, attachment.ErrFaceRequired) {
		t.Fatalf("bare-id attach err = %v, want ErrFaceRequired", err)
	}
	for _, addr := range []string{"PAGE-1@live", "PAGE-1@review"} {
		if !strings.Contains(err.Error(), addr) {
			t.Errorf("refusal %q does not name %s", err, addr)
		}
	}

	for face, want := range map[entity.Face]int{"live": 1, "review": 0} {
		infos, lerr := f.svc.List(ctx, entity.Ref{ID: "PAGE-1", Face: face})
		if lerr != nil {
			t.Fatalf("list %s: %v", face, lerr)
		}
		if len(infos) != want {
			t.Errorf("%s lists %d files, want %d", face, len(infos), want)
		}
	}
}

// attachTo uploads name to the spec property at ref.
func (f facedFixture) attachTo(t *testing.T, ref entity.Ref, name string) {
	t.Helper()
	src := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(src, []byte(name+" bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Attach(context.Background(), ref, src, "spec"); err != nil {
		t.Fatalf("attach %s to %s@%s: %v", name, ref.ID, ref.Face, err)
	}
}

func (f facedFixture) specNames(t *testing.T, id string, face entity.Face) []string {
	t.Helper()
	e, err := f.st.GetEntityState(context.Background(), id, face)
	if err != nil {
		t.Fatalf("read %s@%s: %v", id, face, err)
	}
	return metamodel.FileNames(e.Properties["spec"])
}

// A copy that does not write the file property leaves the target face's own
// upload alone, even though the source face does not hold it.
func TestFaced_PartialCopyKeepsTargetUpload(t *testing.T) {
	f := newFacedFixture(t)
	ctx := context.Background()
	f.seedLive(t, "PAGE-1")
	if err := f.st.CreateEntity(ctx, &entity.Entity{
		ID: "PAGE-1", Type: "page", Face: "review", Properties: map[string]any{"title": "review"},
	}); err != nil {
		t.Fatalf("seed review: %v", err)
	}
	f.attachTo(t, entity.Ref{ID: "PAGE-1", Face: "review"}, "own.txt")

	if _, err := f.mgr.CopyState(ctx, entitymanager.CopyRequest{
		Definition: "retitle-review", SourceID: "PAGE-1",
	}); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if got := f.specNames(t, "PAGE-1", "review"); !slices.Equal(got, []string{"own.txt"}) {
		t.Errorf("review spec = %v, want its own upload [own.txt]", got)
	}
}

// A copy mapping another value into a file property cannot mint a
// reference to bytes the source face does not hold.
func TestFaced_CopyMappingIntoFilePropertyRefused(t *testing.T) {
	f := newFacedFixture(t)
	f.seedLive(t, "PAGE-1")

	_, err := f.mgr.CopyState(context.Background(), entitymanager.CopyRequest{
		Definition: "mint-review", SourceID: "PAGE-1",
	})
	if !errors.Is(err, entitymanager.ErrCopyFileReference) {
		t.Fatalf("copy err = %v, want ErrCopyFileReference", err)
	}
	if f.references(t, "PAGE-1", "review") {
		t.Error("the refused copy left a reference on review")
	}
}

// Bytes are keyed per entity, so a cross-entity copy does not carry file
// values: the target keeps its own.
func TestFaced_CrossEntityCopyKeepsTargetFileValue(t *testing.T) {
	f := newFacedFixture(t)
	ctx := context.Background()
	f.seedLive(t, "PAGE-1")
	if err := f.st.CreateEntity(ctx, &entity.Entity{
		ID: "SHEET-1", Type: "sheet", Properties: map[string]any{"title": "sheet"},
	}); err != nil {
		t.Fatalf("seed sheet: %v", err)
	}
	f.attachTo(t, entity.Ref{ID: "SHEET-1"}, "own.txt")

	if _, err := f.mgr.CopyState(ctx, entitymanager.CopyRequest{
		Definition: "page-to-sheet", SourceID: "PAGE-1", TargetID: "SHEET-1",
	}); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if got := f.specNames(t, "SHEET-1", ""); !slices.Equal(got, []string{"own.txt"}) {
		t.Errorf("sheet spec = %v, want its own upload [own.txt]", got)
	}
}
