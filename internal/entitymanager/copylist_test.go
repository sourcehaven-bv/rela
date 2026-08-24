package entitymanager_test

import (
	"context"
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

// copyListMeta declares definitions whose sources differ, so a list for one
// face must not offer another face's copies.
const copyListMeta = `
version: "1"
entities:
  page:
    label: Page
    id_prefix: PAGE
    pointers:
      draft: {default: true}
      published: {}
    properties:
      title: {type: string}
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title: {type: string}
copies:
  promote-page:
    from: page@draft
    to: page@published
    fields: all
    label: Publish
    guard:
      permission: promote-page
  unpublish-page:
    from: page@published
    to: page@draft
    fields: all
    guard:
      permission: promote-page
  spawn-followup:
    from: ticket
    to: new ticket
    fields:
      title: "Follow-up"
`

func newCopyListManager(t *testing.T, g entitymanager.CopyGuard) (*entitymanager.Manager, store.Store) {
	t.Helper()
	st := memstore.New()
	meta, err := metamodel.Parse([]byte(copyListMeta))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{},
		Audit: audit.Nop{}, ACL: acl.NopACL{},
		Transitions: statemachine.EmptySet(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
		CopyGuard:   g,
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr, st
}

func offerNames(offers []entitymanager.CopyOffer) []string {
	out := make([]string, 0, len(offers))
	for _, o := range offers {
		out = append(out, o.Name)
	}
	return out
}

// TestCopiesForSource_MatchesTheFaceNotJustTheType pins the face-level match.
//
// `promote-page` reads the DRAFT face and `unpublish-page` reads the
// PUBLISHED one. Both are `page`, so a match on entity type alone would offer
// both on either face — and a UI would render an "unpublish" button on a draft
// that has never been published.
//
// The draft case is the load-bearing one: draft is `default: true`, so its
// STORED coordinate is the zero pointer while its DECLARED name is "draft".
// Comparing declared strings would fail to match `page@draft` against the
// default face. That is why the implementation compares through
// metamodel.StoredPointer.
func TestCopiesForSource_MatchesTheFaceNotJustTheType(t *testing.T) {
	mgr, _ := newCopyListManager(t, allowGuard{allow: true})
	ctx := context.Background()

	t.Run("default face offers the copy declared on its declared name", func(t *testing.T) {
		got := mustOffers(ctx, t, mgr, "page", "", "PAGE-1")
		if len(got) != 1 || got[0].Name != "promote-page" {
			t.Errorf("the DEFAULT face must match `from: page@draft` — draft is "+
				"default:true, so its stored coordinate is the zero pointer; got %v",
				offerNames(got))
		}
	})

	t.Run("published face offers only its own copy", func(t *testing.T) {
		got := mustOffers(ctx, t, mgr, "page", "published", "PAGE-1")
		if len(got) != 1 || got[0].Name != "unpublish-page" {
			t.Errorf("a face must offer only the copies whose `from:` addresses "+
				"IT, or a UI renders unpublish on a never-published draft; got %v",
				offerNames(got))
		}
	})

	t.Run("a different type offers nothing of page's", func(t *testing.T) {
		got := mustOffers(ctx, t, mgr, "ticket", "", "TKT-1")
		if len(got) != 1 || got[0].Name != "spawn-followup" {
			t.Errorf("got %v, want only spawn-followup", offerNames(got))
		}
	})
}

// mustOffers seeds nothing and lists; the source need not exist for the
// MATCHING assertions, only for the allowed ones.
func mustOffers(
	ctx context.Context, t *testing.T, mgr *entitymanager.Manager,
	entityType, pointer, sourceID string,
) []entitymanager.CopyOffer {
	t.Helper()
	got, err := mgr.CopiesForSource(ctx, entityType, pointer, sourceID)
	if err != nil {
		t.Fatalf("CopiesForSource: %v", err)
	}
	return got
}

// TestCopiesForSource_ListsDeniedDefinitionsWithAllowedFalse is the
// config-is-not-secret rule, applied.
//
// A definition the principal may not invoke is still LISTED, with
// Allowed=false. Copy names are operator-authored config in schema.yaml —
// routinely a public repo — so hiding them would conceal something already
// disclosed. What is per-principal is the verdict, and it rides on the entry.
//
// Contrast the entity read gate, which DOES hide existence: whether an entity
// exists is a genuine secret. A definition is config; a row is data.
func TestCopiesForSource_ListsDeniedDefinitionsWithAllowedFalse(t *testing.T) {
	ctx := context.Background()
	seed := func(t *testing.T, st store.Store) {
		t.Helper()
		if err := st.CreateEntity(ctx, &entity.Entity{
			ID: "PAGE-1", Type: "page", Properties: map[string]any{"title": "Draft"},
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	t.Run("guard grants", func(t *testing.T) {
		mgr, st := newCopyListManager(t, allowGuard{allow: true})
		seed(t, st)
		got := mustOffers(ctx, t, mgr, "page", "", "PAGE-1")
		if len(got) != 1 || !got[0].Allowed {
			t.Fatalf("a permitted copy must be Allowed; got %+v", got)
		}
		if got[0].Reason != "" {
			t.Errorf("Reason must be empty when Allowed; got %q", got[0].Reason)
		}
	})

	t.Run("guard denies: still listed, Allowed=false", func(t *testing.T) {
		mgr, st := newCopyListManager(t, allowGuard{allow: false})
		seed(t, st)
		got := mustOffers(ctx, t, mgr, "page", "", "PAGE-1")
		if len(got) != 1 {
			t.Fatalf("a DENIED definition must still be listed — its name is "+
				"config, not a secret; got %v", offerNames(got))
		}
		if got[0].Allowed {
			t.Error("a denied copy must report Allowed=false")
		}
		if got[0].Reason == "" {
			t.Error("a denial should carry a reason for a tooltip")
		}
	})

	t.Run("no guard wired: fails closed, still listed", func(t *testing.T) {
		mgr, st := newCopyListManager(t, nil)
		seed(t, st)
		got := mustOffers(ctx, t, mgr, "page", "", "PAGE-1")
		if len(got) != 1 || got[0].Allowed {
			t.Errorf("a guarded copy with no guard wired must report "+
				"Allowed=false, matching the kernel's fail-closed rule; got %+v", got)
		}
	})
}

// TestCopiesForSource_AllowedAgreesWithInvoke is the invariant that makes the
// hint worth having, and the RULING 11 failure it exists to avoid.
//
// `_actions` said a published face was writable while the write path refused.
// The fix there was to make the map honest. This asserts the same property
// directly: for each guard verdict, Allowed and the actual CopyState outcome
// agree.
//
// It is written as a LOOP over both verdicts rather than a single case,
// because a hint that is always true and a write that always succeeds would
// agree vacuously.
func TestCopiesForSource_AllowedAgreesWithInvoke(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name  string
		guard entitymanager.CopyGuard
	}{
		{"guard grants", allowGuard{allow: true}},
		{"guard denies", allowGuard{allow: false}},
		{"no guard wired", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mgr, st := newCopyListManager(t, tc.guard)
			if err := st.CreateEntity(ctx, &entity.Entity{
				ID: "PAGE-1", Type: "page", Properties: map[string]any{"title": "Draft"},
			}); err != nil {
				t.Fatalf("seed: %v", err)
			}

			offers := mustOffers(ctx, t, mgr, "page", "", "PAGE-1")
			if len(offers) != 1 {
				t.Fatalf("expected one offer, got %v", offerNames(offers))
			}
			hint := offers[0].Allowed

			_, err := mgr.CopyState(ctx, entitymanager.CopyRequest{
				Definition: "promote-page", SourceID: "PAGE-1",
			})
			actuallyAllowed := err == nil

			if hint != actuallyAllowed {
				t.Errorf("the hint and the write must agree: Allowed=%v but "+
					"CopyState err=%v. An affordance that lies is a trap for "+
					"every future consumer (RULING 11)", hint, err)
			}
		})
	}
}

// TestCopiesForSource_LabelFallsBackToName pins RULING 9's operator-
// configurable text and its fallback.
//
// The name is a legible fallback because copy names are operator-authored and
// already read as actions (`promote-page`), which is the same reasoning
// metamodel.Transition.Label uses.
func TestCopiesForSource_LabelFallsBackToName(t *testing.T) {
	mgr, _ := newCopyListManager(t, allowGuard{allow: true})
	ctx := context.Background()

	withLabel := mustOffers(ctx, t, mgr, "page", "", "PAGE-1")
	if len(withLabel) != 1 || withLabel[0].Label != "Publish" {
		t.Errorf("an operator `label:` must be used verbatim; got %+v", withLabel)
	}

	noLabel := mustOffers(ctx, t, mgr, "ticket", "", "TKT-1")
	if len(noLabel) != 1 || noLabel[0].Label != "spawn-followup" {
		t.Errorf("with no `label:`, the definition NAME is the fallback; got %+v", noLabel)
	}
}

// TestCopiesForSource_ReportsTheTargetShape pins the two fields a UI needs to
// know what a button will do: which face it writes, and whether it needs a
// target id.
//
// SameEntity is not cosmetic — a cross-entity copy REQUIRES a target id, so a
// caller that cannot distinguish the two cannot build a valid request.
func TestCopiesForSource_ReportsTheTargetShape(t *testing.T) {
	mgr, _ := newCopyListManager(t, allowGuard{allow: true})
	ctx := context.Background()

	same := mustOffers(ctx, t, mgr, "page", "", "PAGE-1")[0]
	if !same.SameEntity {
		t.Error("page@draft -> page@published writes another face of the SAME entity")
	}
	if same.TargetFace != "page@published" {
		t.Errorf("TargetFace = %q, want the declared target", same.TargetFace)
	}

	cross := mustOffers(ctx, t, mgr, "ticket", "", "TKT-1")[0]
	if cross.SameEntity {
		t.Error("`to: new ticket` creates a DIFFERENT entity")
	}
	if cross.TargetFace != "new ticket" {
		t.Errorf("TargetFace = %q, want the declared target", cross.TargetFace)
	}
}

// TestCopiesForSource_StableOrder pins name ordering, so a UI renders a stable
// button order rather than one that reshuffles with map iteration.
func TestCopiesForSource_StableOrder(t *testing.T) {
	mgr, _ := newCopyListManager(t, allowGuard{allow: true})
	ctx := context.Background()
	first := offerNames(mustOffers(ctx, t, mgr, "page", "", "PAGE-1"))
	for range 8 {
		got := offerNames(mustOffers(ctx, t, mgr, "page", "", "PAGE-1"))
		if len(got) != len(first) {
			t.Fatalf("unstable length: %v vs %v", got, first)
		}
		for i := range got {
			if got[i] != first[i] {
				t.Fatalf("unstable order: %v vs %v", got, first)
			}
		}
	}
}
