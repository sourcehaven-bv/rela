package entitymanager_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const fileRuleMetaYAML = `
version: "1"
entities:
  doc:
    label: Doc
    id_prefix: DOC
    properties:
      title: {type: string}
      spec: {type: file}
      gallery: {type: file, max: 3}
`

func fileRuleDeps(t *testing.T) entitymanager.Deps {
	t.Helper()
	meta, err := metamodel.Parse([]byte(fileRuleMetaYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	return entitymanager.Deps{
		Store:            memstore.New(),
		Meta:             meta,
		Templater:        nopTemplater{},
		Audit:            audit.Nop{},
		ACL:              acl.NopACL{},
		Transitions:      statemachine.EmptySet(),
		FieldGate:        entitymanager.AllowAllFieldGate{},
		AttachmentLocker: lock.NewMemoryLocker(),
	}
}

// newFileRuleManager returns a manager over a metamodel with file
// properties, and DOC-1 stored with spec and gallery values.
func newFileRuleManager(t *testing.T) (*entitymanager.Manager, entitymanager.Deps) {
	t.Helper()
	d := fileRuleDeps(t)
	mgr, err := entitymanager.New(d)
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	if err := d.Store.CreateEntity(context.Background(), &entity.Entity{
		ID: "DOC-1", Type: "doc", Properties: map[string]any{
			"title":   "doc",
			"spec":    "attachments/DOC-1/spec/a.txt",
			"gallery": []any{"attachments/DOC-1/gallery/x.png", "attachments/DOC-1/gallery/y.png"},
		},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return mgr, d
}

func TestNew_RequiresAttachmentLockerForFileProperties(t *testing.T) {
	d := fileRuleDeps(t)
	d.AttachmentLocker = nil
	if _, err := entitymanager.New(d); err == nil {
		t.Fatal("New accepted a metamodel with file properties and no AttachmentLocker")
	}
}

func TestAttachmentsOf_RejectsMissingCollaborators(t *testing.T) {
	if _, err := entitymanager.AttachmentsOf(nil); err == nil {
		t.Error("AttachmentsOf(nil) succeeded")
	}
	var zero entitymanager.Attachments
	if _, err := zero.Acquire(context.Background(), "k"); err == nil {
		t.Error("zero Attachments.Acquire succeeded")
	}
	if _, err := zero.StampAttachments(context.Background(), entity.Ref{ID: "DOC-1"}, "spec", nil); err == nil {
		t.Error("zero Attachments.StampAttachments succeeded")
	}
}

// Ordinary writes may not change a file value; echoing it, or changing only
// the stored directory prefix, is no change (RR-EV48RR).
func TestFileProperty_GenericWriteRule(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		write   func(*entitymanager.Manager) error
		wantErr bool
	}{
		{"create with a file value", func(m *entitymanager.Manager) error {
			_, err := m.CreateEntity(ctx, &entity.Entity{Type: "doc", Properties: map[string]any{
				"title": "new", "spec": "attachments/DOC-1/spec/a.txt",
			}}, entity.CreateOptions{})
			return err
		}, true},
		{"create without a file value", func(m *entitymanager.Manager) error {
			_, err := m.CreateEntity(ctx, &entity.Entity{Type: "doc", Properties: map[string]any{
				"title": "new",
			}}, entity.CreateOptions{})
			return err
		}, false},
		{"patch naming another file", func(m *entitymanager.Manager) error {
			_, err := m.PatchEntity(ctx, "DOC-1", entity.Patch{Properties: map[string]any{"spec": "b.txt"}})
			return err
		}, true},
		{"patch unsetting a file value", func(m *entitymanager.Manager) error {
			_, err := m.PatchEntity(ctx, "DOC-1", entity.Patch{MetaUnset: []string{"spec"}})
			return err
		}, true},
		{"patch echoing the value in another order and prefix", func(m *entitymanager.Manager) error {
			_, err := m.PatchEntity(ctx, "DOC-1", entity.Patch{Properties: map[string]any{
				"title": "renamed", "gallery": []string{"y.png", "elsewhere/x.png"},
			}})
			return err
		}, false},
		{"update changing a file value", func(m *entitymanager.Manager) error {
			e := &entity.Entity{ID: "DOC-1", Type: "doc", Properties: map[string]any{
				"title": "doc", "spec": "attachments/DOC-1/spec/a.txt",
				"gallery": []any{"attachments/DOC-1/gallery/x.png"},
			}}
			_, err := m.UpdateEntity(ctx, e)
			return err
		}, true},
		{"update keeping the file values", func(m *entitymanager.Manager) error {
			e := &entity.Entity{ID: "DOC-1", Type: "doc", Properties: map[string]any{
				"title": "doc 2", "spec": "attachments/DOC-1/spec/a.txt",
				"gallery": []any{"attachments/DOC-1/gallery/x.png", "attachments/DOC-1/gallery/y.png"},
			}}
			_, err := m.UpdateEntity(ctx, e)
			return err
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mgr, _ := newFileRuleManager(t)
			err := tc.write(mgr)
			var fwe *entitymanager.FileWriteError
			if tc.wantErr && !errors.As(err, &fwe) {
				t.Fatalf("err = %v, want a FileWriteError", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("err = %v, want success", err)
			}
		})
	}
}

// An echo that passes the base-name check in another form (a foreign
// prefix, another order) is stored as the value the attachment paths wrote,
// never as the caller's spelling.
func TestFileProperty_EchoKeepsStoredValue(t *testing.T) {
	ctx := context.Background()
	mgr, d := newFileRuleManager(t)
	before, err := d.Store.GetEntity(ctx, "DOC-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = mgr.PatchEntity(ctx, "DOC-1", entity.Patch{Properties: map[string]any{
		"title": "renamed", "spec": "javascript:alert(1)//a.txt",
		"gallery": []string{"y.png", "../../x.png"},
	}}); err != nil {
		t.Fatalf("patch: %v", err)
	}
	e := &entity.Entity{ID: "DOC-1", Type: "doc", Properties: map[string]any{
		"title": "saved", "spec": "../a.txt",
		"gallery": []any{"x.png", "y.png"},
	}}
	if _, err = mgr.UpdateEntity(ctx, e); err != nil {
		t.Fatalf("update: %v", err)
	}
	after, err := d.Store.GetEntity(ctx, "DOC-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, prop := range []string{"spec", "gallery"} {
		if !reflect.DeepEqual(after.Properties[prop], before.Properties[prop]) {
			t.Errorf("%s = %#v, want the stored %#v", prop, after.Properties[prop], before.Properties[prop])
		}
	}
	if after.Properties["title"] != "saved" {
		t.Errorf("title = %v, want the ordinary change to land", after.Properties["title"])
	}
}

// StampAttachments may change the named file property and no other.
func TestStampAttachments_ChangesOnlyTheNamedProperty(t *testing.T) {
	ctx := context.Background()
	mgr, d := newFileRuleManager(t)
	owner, err := entitymanager.AttachmentsOf(mgr)
	if err != nil {
		t.Fatalf("AttachmentsOf: %v", err)
	}
	if _, err = owner.StampAttachments(ctx, entity.Ref{ID: "DOC-1"}, "spec", "attachments/DOC-1/spec/b.txt"); err != nil {
		t.Fatalf("stamp: %v", err)
	}
	got, err := d.Store.GetEntity(ctx, "DOC-1")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Properties["spec"] != "attachments/DOC-1/spec/b.txt" {
		t.Errorf("spec = %v, want the stamped value", got.Properties["spec"])
	}
	// The patch StampAttachments runs still refuses other file properties,
	// so a stamp cannot be widened into a generic file write.
	if _, err := mgr.PatchEntity(ctx, "DOC-1", entity.Patch{Properties: map[string]any{"gallery": "z.png"}}); err == nil {
		t.Error("a generic patch changed gallery after a stamp on spec")
	}
}

func TestCarryFileValues(t *testing.T) {
	d := fileRuleDeps(t)
	snap := map[string]any{"title": "old", "spec": "attachments/DOC-1/spec/old.txt", "gallery": "old.png"}
	live := &entity.Entity{ID: "DOC-1", Type: "doc", Properties: map[string]any{
		"title": "new", "spec": "attachments/DOC-1/spec/new.txt",
	}}
	for _, tc := range []struct {
		name string
		live *entity.Entity
		want map[string]any
	}{
		{"keeps live values and drops what live lacks", live, map[string]any{
			"title": "old", "spec": "attachments/DOC-1/spec/new.txt",
		}},
		{"no live entity drops every file value", nil, map[string]any{"title": "old"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := entitymanager.CarryFileValues(d.Meta, "doc", snap, tc.live)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
	if snap["spec"] != "attachments/DOC-1/spec/old.txt" {
		t.Error("CarryFileValues mutated its input")
	}
}

// stampAfterRead stands in for an attachment stamp that lands between a
// save's read and its write: after the first read of DOC-1 it rewrites the
// file value underneath the caller.
type stampAfterRead struct {
	store.Store
	once  sync.Once
	stamp func()
}

func (s *stampAfterRead) GetEntityState(ctx context.Context, id string, f entity.Face) (*entity.Entity, error) {
	e, err := s.Store.GetEntity(ctx, entity.Ref{ID: id, Face: f})
	if err == nil && id == "DOC-1" {
		s.once.Do(s.stamp)
	}
	return e, err
}

// A whole-entity save does not revert a stamp that lands after its read: it
// retries against the new row, so the face keeps naming the bytes the
// attachment service wrote (the old ones may already be gone).
func TestUpdateEntity_DoesNotRevertAConcurrentStamp(t *testing.T) {
	ctx := context.Background()
	d := fileRuleDeps(t)
	inner := d.Store
	const stamped = "attachments/DOC-1/spec/new.txt"
	hooked := &stampAfterRead{Store: inner, stamp: func() {
		e, err := inner.GetEntity(ctx, "DOC-1")
		if err != nil {
			t.Errorf("stamp read: %v", err)
			return
		}
		e.Properties["spec"] = stamped
		if _, err := inner.UpdateEntityIf(ctx, e, store.UpdateCondition{}); err != nil {
			t.Errorf("stamp write: %v", err)
		}
	}}
	d.Store = hooked
	mgr, err := entitymanager.New(d)
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	if err = inner.CreateEntity(ctx, &entity.Entity{ID: "DOC-1", Type: "doc", Properties: map[string]any{
		"title": "doc", "spec": "attachments/DOC-1/spec/a.txt",
	}}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err = mgr.UpdateEntity(ctx, &entity.Entity{ID: "DOC-1", Type: "doc", Properties: map[string]any{
		"title": "saved", "spec": "attachments/DOC-1/spec/a.txt",
	}}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := inner.GetEntity(ctx, "DOC-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Properties["spec"] != stamped {
		t.Errorf("spec = %v, want the concurrent stamp %q kept", got.Properties["spec"], stamped)
	}
	if got.Properties["title"] != "saved" {
		t.Errorf("title = %v, want the save to land", got.Properties["title"])
	}
}

// The one privileged write names a file property; any other is refused.
func TestStampAttachments_RefusesANonFileProperty(t *testing.T) {
	mgr, _ := newFileRuleManager(t)
	owner, err := entitymanager.AttachmentsOf(mgr)
	if err != nil {
		t.Fatalf("AttachmentsOf: %v", err)
	}
	if _, err = owner.StampAttachments(context.Background(), entity.Ref{ID: "DOC-1"}, "title", "x"); err == nil {
		t.Fatal("StampAttachments wrote a non-file property")
	}
}
