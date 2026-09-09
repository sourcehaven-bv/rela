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
// non-bare target makes the guard mandatory), the GUARDED counterpart of that
// same shape, a guarded cross-entity copy, a cross-entity copy whose target
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
  # The atlas shape: the bare face is the ADOPTED text, and the only way to
  # change it is this guarded promote from the draft face.
  adopt:
    from: page@published
    to: page@draft
    fields: all
    guard:
      permission: adopt-page
  # A guarded copy whose endpoints COINCIDE. IsSameEntity compares types, not
  # faces, so this is same-entity too - but it moves nothing between faces, so
  # the guard must NOT stand in for the write check.
  self-mangle:
    from: page
    to: page
    fields:
      title: "MANGLED"
    guard:
      permission: adopt-page
  # A guarded CROSS-entity copy. The guard must NOT overrule the write check
  # here, because the caller picks the target.
  # Cross-entity AND face-crossing, so it satisfies every clause of the
  # exemption except IsSameEntity. Without that clause the caller could name
  # any page and overwrite its published face on the strength of one guard.
  guarded-spawn:
    from: ticket
    to: page@published
    fields:
      title: "{{new.title}}"
    guard:
      permission: adopt-page
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
	return newCopyAuthzManagerWithGuard(t, a, allowGuard{allow: true})
}

func newCopyAuthzManagerWithGuard(
	t *testing.T, a acl.ACL, guard entitymanager.CopyGuard,
) (*entitymanager.Manager, store.Store) {
	t.Helper()
	return newCopyAuthzManagerFull(t, a, guard, entitymanager.AllowAllCopyReadGate{})
}

func newCopyAuthzManagerFull(
	t *testing.T, a acl.ACL, guard entitymanager.CopyGuard, gate entitymanager.CopyReadGate,
) (*entitymanager.Manager, store.Store) {
	t.Helper()
	st := memstore.New()
	meta, err := metamodel.Parse([]byte(copyAuthzMeta))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{},
		Audit: audit.Nop{}, ACL: a,
		Transitions:  statemachine.EmptySet(),
		FieldGate:    entitymanager.AllowAllFieldGate{},
		CopyGuard:    guard,
		CopyReadGate: gate,
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

// denyReadGate refuses to read every copy source — the read half of the ACL,
// independent of the write half a test's acl.ACL expresses.
//
// It records the call because ErrCopySourceMissing is ALSO what readCopySource
// returns for a genuinely absent entity, and readCopySource runs first. A test
// asserting only the error value would still pass if check (1) stopped being
// consulted; asserting `called` makes it about the gate.
type denyReadGate struct{ called bool }

func (g *denyReadGate) PermitsReadFace(
	context.Context, string, string, entity.Face,
) (bool, error) {
	g.called = true
	return false, nil
}

// TestCopy_GuardIsTheAuthorizationForASameEntityCopy pins the exemption on the
// GUARD rather than on which face the copy targets.
//
// The shape this exists for is the adopted-document control: the bare face IS
// the published text (`bare_face: vastgesteld`), so every reader without a
// face address gets it, and the ISMS control is that it changes only through
// the guarded promote. Keying the exemption on the target face made that
// unexpressible — the promote demanded `update` on the type, and that same
// grant is what makes the face editable by hand, which is precisely what the
// guard exists to prevent. There was no grant that permitted the promote and
// forbade the hand edit.
func TestCopy_GuardIsTheAuthorizationForASameEntityCopy(t *testing.T) {
	ctx := context.Background()

	t.Run("holding the guard suffices without update on the type", func(t *testing.T) {
		mgr, st := newCopyAuthzManagerWithGuard(t, acl.ReadOnlyACL{}, allowGuard{allow: true})
		seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-1", Type: "page",
			Properties: map[string]any{"title": "adopted"}})
		seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-1", Type: "page", Face: "published",
			Properties: map[string]any{"title": "NEXT"}})

		// Precondition: this principal may NOT edit the bare face by hand.
		// That is the whole point — the guard must not require the grant it
		// exists to make unnecessary.
		if _, err := mgr.UpdateEntity(ctx, &entity.Entity{ID: "PAGE-1", Type: "page",
			Properties: map[string]any{"title": "x"}}); err == nil {
			t.Fatal("precondition: ReadOnlyACL must refuse a direct update of the bare face")
		}

		if _, err := mgr.CopyState(ctx, entitymanager.CopyRequest{
			Definition: "adopt", SourceID: "PAGE-1",
		}); err != nil {
			t.Fatalf("a guarded same-entity copy is authorized by its guard alone; got %v", err)
		}
		got, gerr := st.GetEntity(ctx, "PAGE-1")
		if gerr != nil || got.Properties["title"] != "NEXT" {
			t.Errorf("the promote must have landed in the bare face; got %v %v", got.Properties, gerr)
		}
	})

	t.Run("lacking the guard is refused even under an allow-all ACL", func(t *testing.T) {
		mgr, st := newCopyAuthzManagerWithGuard(t, acl.NopACL{}, allowGuard{allow: false})
		seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-1", Type: "page",
			Properties: map[string]any{"title": "adopted"}})
		seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-1", Type: "page", Face: "published",
			Properties: map[string]any{"title": "NEXT"}})

		_, err := mgr.CopyState(ctx, entitymanager.CopyRequest{
			Definition: "adopt", SourceID: "PAGE-1",
		})
		var forbidden *acl.ForbiddenError
		if !errors.As(err, &forbidden) {
			t.Fatalf("want *acl.ForbiddenError, got %v", err)
		}
		if forbidden.Decision.RuleKind != "copy-guard" {
			t.Errorf("RuleKind must name the guard, got %q", forbidden.Decision.RuleKind)
		}
		got, _ := st.GetEntity(ctx, "PAGE-1")
		if got.Properties["title"] != "adopted" {
			t.Errorf("the bare face must be untouched; got %v", got.Properties)
		}
	})

	t.Run("the guard never substitutes for reading the source", func(t *testing.T) {
		// The read gate refuses the source. The guard says "you may perform
		// this promotion", never "you may read this document" — without
		// check (1) the promote would be a way to publish a draft the
		// principal cannot see.
		gate := &denyReadGate{}
		mgr, st := newCopyAuthzManagerFull(t, acl.NopACL{}, allowGuard{allow: true}, gate)
		seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-1", Type: "page",
			Properties: map[string]any{"title": "adopted"}})
		seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-1", Type: "page", Face: "published",
			Properties: map[string]any{"title": "NEXT"}})

		_, err := mgr.CopyState(ctx, entitymanager.CopyRequest{
			Definition: "adopt", SourceID: "PAGE-1",
		})
		if !errors.Is(err, entitymanager.ErrCopySourceMissing) {
			t.Fatalf("want ErrCopySourceMissing, got %v", err)
		}
		if !gate.called {
			t.Error("the read gate must be consulted; the error alone does not " +
				"prove it, since an absent source produces the same one")
		}
		got, _ := st.GetEntity(ctx, "PAGE-1")
		if got.Properties["title"] != "adopted" {
			t.Errorf("the bare face must be untouched; got %v", got.Properties)
		}
	})
}

// TestCopy_GuardDoesNotOverruleACrossEntityWrite: the exemption is scoped to
// IsSameEntity() because that is what makes the target non-negotiable — the
// operator wrote both endpoints. On a cross-entity copy the CALLER names the
// target, so letting one guard permission stand in for the write check would
// turn it into a way to write entities the principal could not write by hand.
//
// It uses createOnlyACL against an EXISTING target rather than a blanket
// read-only ACL, and asserts WHICH rule refused. A read-only ACL denies every
// write with one fixed decision, so the test would pass identically if the
// exemption were widened to every guarded copy — it could not tell correct
// scoping from an ACL that says no to everything.
func TestCopy_GuardDoesNotOverruleACrossEntityWrite(t *testing.T) {
	ctx := context.Background()
	mgr, st := newCopyAuthzManagerWithGuard(t, createOnlyACL{}, allowGuard{allow: true})
	seedRaw(ctx, t, st, &entity.Entity{ID: "TKT-1", Type: "ticket",
		Properties: map[string]any{"title": "src"}})
	seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-1", Type: "page",
		Properties: map[string]any{"title": "draft"}})
	seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-1", Type: "page", Face: "published",
		Properties: map[string]any{"title": "victim"}})

	_, err := mgr.CopyState(ctx, entitymanager.CopyRequest{
		Definition: "guarded-spawn", SourceID: "TKT-1", TargetID: "PAGE-1",
	})
	var forbidden *acl.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("a guarded cross-entity copy still needs create/update on the target; got %v", err)
	}
	// The refusal must come from check (3) reaching the ACL, not from the
	// guard: the guard ALLOWS here, so a "copy-guard" verdict would mean the
	// test proved nothing about scoping.
	if forbidden.Decision.RuleKind != "test" {
		t.Errorf("the ordinary write check must be what refuses; got RuleKind=%q reason=%q",
			forbidden.Decision.RuleKind, forbidden.Decision.Reason)
	}
	v, _ := st.GetEntityState(ctx, "PAGE-1", "published")
	if v.Properties["title"] != "victim" {
		t.Errorf("the published face must be untouched; got %v", v.Properties)
	}
}

// TestCopy_GuardDoesNotOverruleASameFaceCopy is the third condition on the
// exemption, and the one a `IsSameEntity()`-only rule silently dropped.
//
// `IsSameEntity()` compares TYPES, not faces, so `from: page` / `to: page`
// satisfies it while moving nothing between faces. There is no guarded face
// involved — on a type declaring none there are no faces at all — so the
// promote argument does not apply and the target is the ordinary row `update`
// governs. Exempting it would turn any guarded definition into "mutate this
// entity in place, authorized by a permission noun".
func TestCopy_GuardDoesNotOverruleASameFaceCopy(t *testing.T) {
	ctx := context.Background()
	mgr, st := newCopyAuthzManagerWithGuard(t, acl.ReadOnlyACL{}, allowGuard{allow: true})
	seedRaw(ctx, t, st, &entity.Entity{ID: "PAGE-1", Type: "page",
		Properties: map[string]any{"title": "original"}})

	_, err := mgr.CopyState(ctx, entitymanager.CopyRequest{
		Definition: "self-mangle", SourceID: "PAGE-1",
	})
	var forbidden *acl.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("a guarded copy that crosses no face boundary still needs update; got %v", err)
	}
	got, _ := st.GetEntity(ctx, "PAGE-1")
	if got.Properties["title"] != "original" {
		t.Errorf("the entity must be untouched; got %v", got.Properties)
	}
}
