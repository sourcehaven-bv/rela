package attachment_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

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
      gallery: {type: file, max: 3}
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
	e, err := f.st.GetEntity(context.Background(), entity.Ref{ID: id, Face: face})
	if errors.Is(err, store.ErrNotFound) {
		return false
	}
	if err != nil {
		t.Fatalf("read %s@%s: %v", id, face, err)
	}
	return attachment.References(e, "spec", "spec.txt")
}

// hasBytes reports whether id's spec property has any stored bytes.
func (f facedFixture) hasBytes(t *testing.T, id string) bool {
	t.Helper()
	return len(f.specKeys(t, id)) > 0
}

// specKeys returns the storage keys that have bytes on id's spec property.
func (f facedFixture) specKeys(t *testing.T, id string) []string {
	t.Helper()
	infos, err := f.st.ListAttachments(context.Background(), id)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("list bytes of %s: %v", id, err)
	}
	var keys []string
	for _, info := range infos {
		if info.Property == "spec" {
			keys = append(keys, info.FileName)
		}
	}
	slices.Sort(keys)
	return keys
}

// specValue returns the raw spec value of id at face.
func (f facedFixture) specValue(t *testing.T, id string, face entity.Face) any {
	t.Helper()
	e, err := f.st.GetEntity(context.Background(), entity.Ref{ID: id, Face: face})
	if err != nil {
		t.Fatalf("read %s@%s: %v", id, face, err)
	}
	return e.Properties["spec"]
}

// readSpec reads name through PAGE-1's face's own value, as a download does.
func (f facedFixture) readSpec(t *testing.T, face entity.Face, name string) (string, error) {
	t.Helper()
	e, err := f.st.GetEntity(context.Background(), entity.Ref{ID: "PAGE-1", Face: face})
	if err != nil {
		t.Fatalf("read PAGE-1@%s: %v", face, err)
	}
	rc, err := f.svc.Open(context.Background(), e, "spec", name)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read bytes: %v", err)
	}
	return string(data), nil
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
	live, review := metamodel.FileKeys(f.specValue(t, "PAGE-1", "live")), metamodel.FileKeys(f.specValue(t, "PAGE-1", "review"))
	if !slices.Equal(live, review) || !slices.Equal(f.specKeys(t, "PAGE-1"), live) {
		t.Fatalf("keys: live %v, review %v, stored %v; want one shared key", live, review, f.specKeys(t, "PAGE-1"))
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
	e, err := f.st.GetEntity(context.Background(), entity.Ref{ID: id, Face: face})
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

// seedReview creates PAGE-1's review face with no files.
func (f facedFixture) seedReview(t *testing.T) {
	t.Helper()
	const id = "PAGE-1"
	if err := f.st.CreateEntity(context.Background(), &entity.Entity{
		ID: id, Type: "page", Face: "review", Properties: map[string]any{"title": "review"},
	}); err != nil {
		t.Fatalf("seed %s@review: %v", id, err)
	}
}

// An upload on review with the name of live's file keeps that name, gets
// its own bytes, and leaves live's alone (the file-name oracle fix,
// BUG-CTUW2N PR 4b). Download and detach stay per face.
func TestFaced_SameNameOnTwoFacesIsIsolated(t *testing.T) {
	f := newFacedFixture(t)
	ctx := context.Background()
	f.seedLive(t, "PAGE-1")
	f.seedReview(t)
	f.attachTo(t, entity.Ref{ID: "PAGE-1", Face: "review"}, "spec.txt")

	if got := f.specNames(t, "PAGE-1", "review"); !slices.Equal(got, []string{"spec.txt"}) {
		t.Fatalf("review spec = %v, want [spec.txt] unsuffixed", got)
	}
	if n := len(f.specKeys(t, "PAGE-1")); n != 2 {
		t.Fatalf("stored %d byte blobs, want one per face", n)
	}
	for face, want := range map[entity.Face]string{"live": "spec bytes", "review": "spec.txt bytes"} {
		if got, err := f.readSpec(t, face, "spec.txt"); err != nil || got != want {
			t.Errorf("%s reads %q (%v), want %q", face, got, err, want)
		}
	}

	if _, err := f.svc.Detach(ctx, entity.Ref{ID: "PAGE-1", Face: "review"}, "spec", "spec.txt"); err != nil {
		t.Fatalf("detach on review: %v", err)
	}
	if got, err := f.readSpec(t, "live", "spec.txt"); err != nil || got != "spec bytes" {
		t.Errorf("after review's detach live reads %q (%v)", got, err)
	}
	if n := len(f.specKeys(t, "PAGE-1")); n != 1 {
		t.Errorf("after review's detach %d byte blobs remain, want live's one", n)
	}
	if _, err := f.readSpec(t, "review", "spec.txt"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("review still serves a detached file: %v", err)
	}
}

// On a multi-file property the name is suffixed against the face's own
// names only: another face's file of that name does not count.
func TestFaced_MultiFileSuffixIsPerFace(t *testing.T) {
	f := newFacedFixture(t)
	ctx := context.Background()
	f.seedLive(t, "PAGE-1")
	f.seedReview(t)

	put := func(face entity.Face) string {
		t.Helper()
		e, err := f.st.GetEntity(ctx, entity.Ref{ID: "PAGE-1", Face: face})
		if err != nil {
			t.Fatal(err)
		}
		res, err := f.svc.WriteAttachment(ctx, e, metamodel.PropertyDef{Type: metamodel.PropertyTypeFile, Max: 3},
			"gallery", "a.txt", strings.NewReader(string(face)))
		if err != nil {
			t.Fatalf("upload on %s: %v", face, err)
		}
		return res.FileName
	}
	if got := put("live"); got != "a.txt" {
		t.Fatalf("live upload named %q", got)
	}
	if got := put("review"); got != "a.txt" {
		t.Errorf("review upload named %q, want a.txt: live's file must not count", got)
	}
	if got := put("review"); got != "a (1).txt" {
		t.Errorf("second review upload named %q, want a (1).txt", got)
	}
}

// A copy mapping into the file property may not keep a source name while
// naming other bytes: the storage key must match the source's too.
func TestFaced_CopyForgedStorageKeyRefused(t *testing.T) {
	f := newFacedFixture(t)
	ctx := context.Background()
	f.seedLive(t, "PAGE-1")
	forged := metamodel.KeyedFileEntry("PAGE-1", "spec", strings.Repeat("0", metamodel.FileTokenLen), "spec.txt")
	if _, err := f.mgr.PatchEntity(ctx, entity.Ref{ID: "PAGE-1", Face: "live"}.String(),
		entity.Patch{Properties: map[string]any{"title": forged}}); err != nil {
		t.Fatalf("set title: %v", err)
	}

	_, err := f.mgr.CopyState(ctx, entitymanager.CopyRequest{Definition: "mint-review", SourceID: "PAGE-1"})
	if !errors.Is(err, entitymanager.ErrCopyFileReference) {
		t.Fatalf("copy err = %v, want ErrCopyFileReference", err)
	}
}

// A value stamped before storage keys ("attachments/<id>/<prop>/<name>")
// resolves to the bytes stored under the bare name, so existing data needs
// no migration; replacing it drops those bytes.
func TestFaced_LegacyValueStillServes(t *testing.T) {
	f := newFacedFixture(t)
	ctx := context.Background()
	f.seedReview(t)
	if err := f.st.AttachFile(ctx, "PAGE-1", "spec", "old.txt", strings.NewReader("legacy")); err != nil {
		t.Fatal(err)
	}
	owner, err := entitymanager.AttachmentsOf(f.mgr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.StampAttachments(ctx, entity.Ref{ID: "PAGE-1", Face: "review"}, "spec",
		"attachments/PAGE-1/spec/old.txt"); err != nil {
		t.Fatal(err)
	}
	if got, rerr := f.readSpec(t, "review", "old.txt"); rerr != nil || got != "legacy" {
		t.Fatalf("legacy read = %q (%v)", got, rerr)
	}
	f.attachTo(t, entity.Ref{ID: "PAGE-1", Face: "review"}, "old.txt")
	if got, rerr := f.readSpec(t, "review", "old.txt"); rerr != nil || got != "old.txt bytes" {
		t.Errorf("after replace read = %q (%v)", got, rerr)
	}
	if keys := f.specKeys(t, "PAGE-1"); len(keys) != 1 || keys[0] == "old.txt" {
		t.Errorf("stored keys = %v, want only the new keyed blob", keys)
	}
}

// stampRaw seeds bytes under each key and stamps value on PAGE-1@review's
// prop through the trusted stamp, as sync or a migration could.
func (f facedFixture) stampRaw(t *testing.T, prop string, keys []string, value any) {
	t.Helper()
	ctx := context.Background()
	for _, k := range keys {
		if err := f.st.AttachFile(ctx, "PAGE-1", prop, k, strings.NewReader(k)); err != nil {
			t.Fatal(err)
		}
	}
	owner, err := entitymanager.AttachmentsOf(f.mgr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.StampAttachments(ctx, entity.Ref{ID: "PAGE-1", Face: "review"}, prop, value); err != nil {
		t.Fatal(err)
	}
}

// A single-file property still holding several files (its max was lowered)
// keeps the others when one is deleted.
func TestFaced_DeleteOnLoweredMaxKeepsOtherFiles(t *testing.T) {
	f := newFacedFixture(t)
	f.seedReview(t)
	var keys []string
	var value []any
	for _, n := range []string{"a", "b", "c"} {
		tok := strings.Repeat(n, metamodel.FileTokenLen)
		keys = append(keys, metamodel.FileKey(tok, n+".txt"))
		value = append(value, metamodel.KeyedFileEntry("PAGE-1", "spec", tok, n+".txt"))
	}
	f.stampRaw(t, "spec", keys, value)

	if _, err := f.svc.Detach(context.Background(), entity.Ref{ID: "PAGE-1", Face: "review"}, "spec", "a.txt"); err != nil {
		t.Fatalf("detach: %v", err)
	}
	if got := f.specNames(t, "PAGE-1", "review"); !slices.Equal(got, []string{"b.txt", "c.txt"}) {
		t.Errorf("review spec = %v, want [b.txt c.txt]", got)
	}
	if got := f.specKeys(t, "PAGE-1"); !slices.Equal(got, keys[1:]) {
		t.Errorf("stored keys = %v, want %v", got, keys[1:])
	}
}

// A value mixing legacy and keyed entries lists both, and deleting one keeps
// the other's bytes.
func TestFaced_MixedLegacyAndKeyedEntries(t *testing.T) {
	f := newFacedFixture(t)
	f.seedReview(t)
	tok := strings.Repeat("c", metamodel.FileTokenLen)
	f.stampRaw(t, "gallery", []string{"old.txt", metamodel.FileKey(tok, "new.txt")}, []any{
		"attachments/PAGE-1/gallery/old.txt",
		metamodel.KeyedFileEntry("PAGE-1", "gallery", tok, "new.txt"),
	})
	ref := entity.Ref{ID: "PAGE-1", Face: "review"}
	infos, err := f.svc.List(context.Background(), ref)
	if err != nil || len(infos) != 2 {
		t.Fatalf("list = %v (%v), want two files", infos, err)
	}
	if _, err = f.svc.Detach(context.Background(), ref, "gallery", "old.txt"); err != nil {
		t.Fatalf("detach: %v", err)
	}
	infos, err = f.svc.List(context.Background(), ref)
	if err != nil || len(infos) != 1 || infos[0].FileName != "new.txt" {
		t.Fatalf("after detach list = %v (%v), want [new.txt]", infos, err)
	}
}

// A display name long enough that its storage key would pass the 255-byte
// file-name limit is shortened, keeping its extension, so the upload works
// on the filesystem backend. Linux counts bytes and macOS counts UTF-16
// units, so an ASCII name is the case that fails on both.
func TestService_LongNameIsCapped(t *testing.T) {
	for _, tc := range []struct{ name, upload string }{
		{"multi-byte", strings.Repeat("é", 125) + ".pdf"}, // 254 bytes
		{"ascii", strings.Repeat("a", 251) + ".pdf"},      // 255 bytes
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupAttachmentService(t)
			ctx := context.Background()
			e := entity.New("T-1", "ticket")
			if err := f.st.CreateEntity(ctx, e); err != nil {
				t.Fatal(err)
			}
			res, err := f.svc.WriteAttachment(ctx, e, metamodel.PropertyDef{Type: metamodel.PropertyTypeFile},
				"spec", tc.upload, strings.NewReader("x"))
			if err != nil {
				t.Fatalf("upload: %v", err)
			}
			keyLen := len(metamodel.FileKey(strings.Repeat("0", metamodel.FileTokenLen), res.FileName))
			if !strings.HasSuffix(res.FileName, ".pdf") || !utf8.ValidString(res.FileName) || keyLen > 255 {
				t.Errorf("name = %q (%d bytes), want a valid capped name keeping .pdf", res.FileName, len(res.FileName))
			}
		})
	}
}

// Harmless path variants of a keyed entry still resolve to its key, never
// to a legacy blob of the same name.
func TestStorageKey_CleansEntry(t *testing.T) {
	tok := strings.Repeat("d", metamodel.FileTokenLen)
	for _, v := range []string{
		"attachments/X-1/spec/" + tok + "/a.txt",
		"/attachments/X-1/spec/" + tok + "/a.txt",
		"./attachments//X-1/spec/" + tok + "/a.txt",
	} {
		e := &entity.Entity{Properties: map[string]any{"spec": v}}
		if key, ok := attachment.StorageKey(e, "spec", "a.txt"); !ok || key != metamodel.FileKey(tok, "a.txt") {
			t.Errorf("%q: key = %q (%v)", v, key, ok)
		}
	}
}
