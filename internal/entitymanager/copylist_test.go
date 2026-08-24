package entitymanager_test

import (
	"context"
	"testing"

	"sync"

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
	got, err := entitymanager.CopiesForSource(ctx, mgr, entityType, pointer, sourceID)
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
// # Mutation-checked, and the first attempt found something about the CODE
//
// Deleting copyInvocable's `authorizeCopy` call killed nothing — not because
// this test is weak, but because `planCopy` calls `authorizeCopy` itself, so
// the second call was redundant. The redundancy is now removed and the reason
// documented at the call site.
//
// The real mechanism is `planCopy`, and it IS pinned: making planCopy stop
// authorizing fails 8 tests across this package. The `guard denies` and
// `no guard wired` rows are what make this one of them — with a permissive
// guard alone, "the check ran" and "the check was skipped" both yield
// allowed, and the assertion could not tell them apart.
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

// TestCopiesForSource_AffordanceProbeEmitsNoAuditRecords is the regression
// test for a defect code review found: a READ-ONLY list emitted `denied-write`
// audit records.
//
// # Why it happened
//
// `Allowed` is computed by running the real authorization path — that is what
// stops the hint drifting from the write. But that path AUDITS its denials, so
// asking the question recorded an answer as though someone had attempted a
// write. A SPA rendering an entity view would append N rows per page load.
//
// # Why it matters more than volume
//
// `op=denied-write` would stop meaning "someone tried to write and was
// refused" and start meaning "someone looked at a page". Anyone alerting on
// denied-write volume gets paged by ordinary browsing, and the real signal
// drowns. The audit log's value is that it does not lie.
//
// The verdict is still computed identically — only the RECORD is suppressed —
// so the hint cannot drift as a result of this fix.
//
// # Mutation-checked, and the result is worth recording
//
// Removing the suppression in authorizeAndAudit does NOT fail this test, and
// that is not a flaw in the assertion — it is because the OTHER fix from the
// same review (CopyOffer.Indeterminate) removed the path that audited. The
// records came from planCopyEdges on CROSS-ENTITY definitions, and cross-entity
// offers are no longer probed at all.
//
// So today the suppression is BELT-AND-BRACES, not the mechanism. It stays
// because the reachable-path set is a property of the current call graph
// rather than of the design: anything that probes a definition performing a
// per-edge authorization re-opens the path immediately, and the failure would
// be silent pollution of an append-only log.
//
// This test therefore asserts the PROPERTY (a read-only query writes nothing),
// not the mechanism, and it would catch a regression from either direction.
func TestCopiesForSource_AffordanceProbeEmitsNoAuditRecords(t *testing.T) {
	st := memstore.New()
	meta, err := metamodel.Parse([]byte(copyListMeta))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	rec := &recordingAudit{}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{},
		Audit: rec, ACL: denyAllACL{},
		Transitions: statemachine.EmptySet(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
		CopyGuard:   allowGuard{allow: true},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	ctx := context.Background()
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "PAGE-1", Type: "page", Properties: map[string]any{"title": "Draft"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := entitymanager.CopiesForSource(ctx, mgr, "page", "", "PAGE-1"); err != nil {
		t.Fatalf("CopiesForSource: %v", err)
	}

	if n := rec.count(); n != 0 {
		t.Errorf("a read-only affordance query wrote %d audit record(s). "+
			"`denied-write` must mean an attempted write was refused, not "+
			"that someone rendered a page; records: %v", n, rec.ops())
	}
}

// TestCopiesForSource_CrossEntityIsIndeterminate pins the honest answer for an
// offer whose invocability cannot be known yet.
//
// A cross-entity copy's target id is chosen at INVOKE time, so the create
// check would authorize the empty id — and a policy keyed on the target's
// identity would be evaluated against the wrong subject. Code review
// demonstrated list=allowed / invoke=forbidden on exactly that.
//
// Reporting `Allowed: true` there is the RULING 11 defect. So the offer says
// "indeterminate" instead of guessing, and — asserted here — it does not
// claim Allowed.
func TestCopiesForSource_CrossEntityIsIndeterminate(t *testing.T) {
	mgr, _ := newCopyListManager(t, allowGuard{allow: true})
	ctx := context.Background()

	same := mustOffers(ctx, t, mgr, "page", "", "PAGE-1")[0]
	if same.Indeterminate {
		t.Error("a SAME-entity offer's target is the source, so it is answerable")
	}

	cross := mustOffers(ctx, t, mgr, "ticket", "", "TKT-1")[0]
	if !cross.Indeterminate {
		t.Error("a CROSS-entity offer cannot be authorized before the client " +
			"chooses a target id — saying `allowed` would be an affordance " +
			"that lies (RULING 11)")
	}
	if cross.Allowed {
		t.Error("an indeterminate offer must not also claim Allowed: a client " +
			"reading only `allowed` would treat it as a confirmed verdict")
	}
}

// recordingAudit counts audit records so a test can assert that a read-only
// query wrote none.
type recordingAudit struct {
	mu      sync.Mutex
	records []string
}

func (r *recordingAudit) Record(rec audit.Record) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec.Op)
}

func (r *recordingAudit) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.records)
}

func (r *recordingAudit) ops() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.records...)
}

// denyAllACL refuses every write, so any authorization the affordance query
// performs produces a denial worth auditing — which is what makes the
// zero-records assertion meaningful rather than vacuous.
type denyAllACL struct{ acl.NopACL }

func (denyAllACL) AuthorizeWrite(_ context.Context, _ acl.WriteRequest) acl.Decision {
	return acl.Decision{Allow: false, RuleKind: "test", Reason: "denied for test"}
}
