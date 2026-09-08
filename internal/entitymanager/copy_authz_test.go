package entitymanager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// copyAuthzMeta declares the shapes the write-authorization tests need: a
// same-entity copy INTO the bare face with no guard (legal to declare — only a
// non-bare target makes the guard mandatory), a cross-entity copy whose target
// the caller names, and a `unique:` natural key.
const copyAuthzMeta = `
version: "1"
entities:
  page:
    label: Page
    id_prefix: PAGE
    bare_face: draft
    faces:
      draft: {}
      published: {}
    properties:
      title: {type: string}
      slug: {type: string, unique: true}
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title: {type: string}
      points: {type: integer}
      tags: {type: string, list: true}
  note:
    label: Note
    id_type: manual
    properties:
      title: {type: string}
copies:
  revert:
    from: page@published
    to: page@draft
    fields: all
  spawn:
    from: ticket
    to: new ticket
    fields:
      title: "{{new.title}}"
      points: "{{new.points}}"
      tags: "{{new.tags}}"
  dup-page:
    from: page
    to: new page
    fields:
      title: "{{new.title}}"
      slug: "{{new.slug}}"
`

// createOnlyACL grants every create and refuses every other write — the
// principal who may make new entities but not touch existing ones.
type createOnlyACL struct{ acl.NopACL }

func (createOnlyACL) AuthorizeWrite(_ context.Context, req acl.WriteRequest) acl.Decision {
	if req.Op == acl.OpCreate {
		return acl.Decision{Allow: true, RuleKind: "test"}
	}
	return acl.Decision{Allow: false, RuleKind: "test", Reason: "create only"}
}

func newCopyAuthzManager(t *testing.T, a acl.ACL) (*entitymanager.Manager, store.Store) {
	t.Helper()
	st := memstore.New()
	meta, err := metamodel.Parse([]byte(copyAuthzMeta))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{},
		Audit: audit.Nop{}, ACL: a,
		Transitions: statemachine.EmptySet(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
		CopyGuard:   allowGuard{allow: true},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr, st
}

func seedRaw(ctx context.Context, t *testing.T, st store.Store, e *entity.Entity) {
	t.Helper()
	if err := st.CreateEntity(ctx, e); err != nil {
		t.Fatalf("seed %s: %v", e.ID, err)
	}
}

// TestCopy_IntoTheBareFaceNeedsUpdate is the hole an unguarded `revert`
// definition opened: a same-entity copy INTO the bare face needs no guard to
// declare, and the write check was skipped for every same-entity copy — so
// under a read-only ACL, anyone who could read the published face could
// overwrite the draft. Reverting the draft is editing the draft; it needs what
// editing the draft needs.
func TestCopy_IntoTheBareFaceNeedsUpdate(t *testing.T) {
	ctx := context.Background()
	mgr, st := newCopyAuthzManager(t, acl.ReadOnlyACL{})
	seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-1", Type: "page", Properties: map[string]any{"title": "draft"}})
	seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-1", Type: "page", Face: "published",
		Properties: map[string]any{"title": "PUBLISHED"}})

	// Precondition: the ACL refuses a direct edit of the draft, so a
	// successful revert below would be a write the principal could not make
	// by hand.
	if _, err := mgr.UpdateEntity(ctx, &entity.Entity{ID: "PAGE-1", Type: "page",
		Properties: map[string]any{"title": "x"}}); err == nil {
		t.Fatal("precondition: ReadOnlyACL must refuse a direct update")
	}

	_, err := mgr.CopyState(ctx, entitymanager.CopyRequest{Definition: "revert", SourceID: "PAGE-1"})
	var forbidden *acl.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("a copy into the bare face must be authorized as an UPDATE of it; got err=%v", err)
	}
	got, gerr := st.GetEntity(ctx, "PAGE-1")
	if gerr != nil || got.Properties["title"] != "draft" {
		t.Errorf("the draft must be untouched after a refused revert; got %v %v", got.Properties, gerr)
	}
}

// TestCopy_GuardedFaceStaysExemptFromUpdate is the other half: the promote
// into a GUARDED face is still authorized by its guard alone — nobody holds
// `update` on published by design, and requiring it would make every promote
// impossible. Without this the test above could pass by refusing all copies.
func TestCopy_GuardedFaceStaysExemptFromUpdate(t *testing.T) {
	ctx := context.Background()
	mgr, st := newCopyManager(t, nil, allowGuard{allow: true})
	seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-1", Type: "page", Properties: map[string]any{"title": "draft"}})
	if _, err := mgr.CopyState(ctx, entitymanager.CopyRequest{Definition: "promote-page", SourceID: "PAGE-1"}); err != nil {
		t.Fatalf("a guarded promote needs only its guard; got %v", err)
	}
}

// TestCopy_ExistingCrossEntityTargetIsAnUpdate pins the target probe running
// BEFORE authorization. Authorizing `create` and then discovering the target
// exists let a create-only principal overwrite any entity by naming it.
func TestCopy_ExistingCrossEntityTargetIsAnUpdate(t *testing.T) {
	ctx := context.Background()
	mgr, st := newCopyAuthzManager(t, createOnlyACL{})
	seedRaw(ctx, t, st, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "src"}})
	seedRaw(ctx, t, st, &entity.Entity{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "victim"}})

	t.Run("a fresh target is a create, which this principal may do", func(t *testing.T) {
		if _, err := mgr.CopyState(ctx, entitymanager.CopyRequest{
			Definition: "spawn", SourceID: "TKT-1", TargetID: "TKT-9",
		}); err != nil {
			t.Fatalf("create-only principal must be able to spawn a NEW entity; got %v", err)
		}
	})

	t.Run("an existing target is an update, which this principal may not do", func(t *testing.T) {
		_, err := mgr.CopyState(ctx, entitymanager.CopyRequest{
			Definition: "spawn", SourceID: "TKT-1", TargetID: "TKT-2",
		})
		var forbidden *acl.ForbiddenError
		if !errors.As(err, &forbidden) {
			t.Fatalf("overwriting TKT-2 must be authorized as an update; got err=%v", err)
		}
		v, _ := st.GetEntity(ctx, "TKT-2")
		if v.Properties["title"] != "victim" {
			t.Errorf("TKT-2 must be untouched; got %v", v.Properties)
		}
	})
}

// TestCopy_TargetOfAnotherTypeIsRefused: a cross-entity target stored under a
// different type is refused outright. Writing it would re-type the entity —
// on fsstore a second file under another type's directory — the corruption
// ErrTypeImmutable exists to prevent.
func TestCopy_TargetOfAnotherTypeIsRefused(t *testing.T) {
	ctx := context.Background()
	mgr, st := newCopyAuthzManager(t, acl.NopACL{})
	seedRaw(ctx, t, st, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "src"}})
	seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-9", Type: "page", Properties: map[string]any{"title": "a page"}})

	_, err := mgr.CopyState(ctx, entitymanager.CopyRequest{
		Definition: "spawn", SourceID: "TKT-1", TargetID: "PAGE-9",
	})
	if !errors.Is(err, entitymanager.ErrCopyTargetTypeMismatch) {
		t.Fatalf("want ErrCopyTargetTypeMismatch, got %v", err)
	}
	p, _ := st.GetEntity(ctx, "PAGE-9")
	if p.Type != "page" {
		t.Errorf("PAGE-9 must keep its type; got %q", p.Type)
	}
}

// TestCopy_TargetIDShapeIsValidated: a same-entity request naming a target is
// asking for a copy the definition cannot do, and a cross-entity request
// naming none has nothing to write. Both used to be silently reinterpreted.
func TestCopy_TargetIDShapeIsValidated(t *testing.T) {
	ctx := context.Background()
	mgr, st := newCopyAuthzManager(t, acl.NopACL{})
	seedRaw(ctx, t, st, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "src"}})
	seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-1", Type: "page", Properties: map[string]any{"title": "d"}})
	seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-1", Type: "page", Face: "published",
		Properties: map[string]any{"title": "p"}})

	_, err := mgr.CopyState(ctx, entitymanager.CopyRequest{Definition: "spawn", SourceID: "TKT-1"})
	if !errors.Is(err, entitymanager.ErrCopyTargetRequired) {
		t.Errorf("cross-entity without a target: want ErrCopyTargetRequired, got %v", err)
	}
	_, err = mgr.CopyState(ctx, entitymanager.CopyRequest{
		Definition: "revert", SourceID: "PAGE-1", TargetID: "PAGE-2",
	})
	if !errors.Is(err, entitymanager.ErrCopyTargetNotAllowed) {
		t.Errorf("same-entity with a target: want ErrCopyTargetNotAllowed, got %v", err)
	}
}

// TestCopy_SingleReferenceKeepsTheStoredValue: `points: "{{new.points}}"`
// used to render through string interpolation, which stringifies every
// non-string as "" — the integer and the list were erased on every copy.
func TestCopy_SingleReferenceKeepsTheStoredValue(t *testing.T) {
	ctx := context.Background()
	mgr, st := newCopyAuthzManager(t, acl.NopACL{})
	seedRaw(ctx, t, st, &entity.Entity{ID: "TKT-1", Type: "ticket",
		Properties: map[string]any{"title": "src", "points": 3, "tags": []any{"a", "b"}}})

	if _, err := mgr.CopyState(ctx, entitymanager.CopyRequest{
		Definition: "spawn", SourceID: "TKT-1", TargetID: "TKT-5",
	}); err != nil {
		t.Fatalf("CopyState: %v", err)
	}
	v, _ := st.GetEntity(ctx, "TKT-5")
	if v.Properties["points"] != 3 {
		t.Errorf("points must survive as the stored integer; got %#v", v.Properties["points"])
	}
	tags, _ := v.Properties["tags"].([]any)
	if len(tags) != 2 {
		t.Errorf("tags must survive as the stored list; got %#v", v.Properties["tags"])
	}
	if v.Properties["title"] != "src" {
		t.Errorf("a plain string reference still copies; got %#v", v.Properties["title"])
	}
}

// TestCopy_TargetPassesUniqueAndValidation: the kernel writes to the store
// view directly, so it was the one entry point that could persist a duplicate
// natural key. It now runs the same structural checks a hand-written create
// would.
func TestCopy_TargetPassesUniqueAndValidation(t *testing.T) {
	ctx := context.Background()
	mgr, st := newCopyAuthzManager(t, acl.NopACL{})
	seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-1", Type: "page",
		Properties: map[string]any{"title": "a", "slug": "same"}})

	// Precondition: the ordinary create path refuses the duplicate.
	if _, err := mgr.CreateEntity(ctx, &entity.Entity{ID: "PAGE-3", Type: "page",
		Properties: map[string]any{"title": "b", "slug": "same"}}, entity.CreateOptions{}); err == nil {
		t.Fatal("precondition: CreateEntity must refuse a duplicate unique slug")
	}

	_, err := mgr.CopyState(ctx, entitymanager.CopyRequest{
		Definition: "dup-page", SourceID: "PAGE-1", TargetID: "PAGE-2",
	})
	if err == nil {
		t.Fatal("a copy that would persist a duplicate unique slug must be refused")
	}
	if _, gerr := st.GetEntity(ctx, "PAGE-2"); !errors.Is(gerr, store.ErrNotFound) {
		t.Errorf("no target may be written when validation refuses; got err=%v", gerr)
	}
}
