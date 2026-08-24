package appbuild_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// copyWiringMeta declares a same-entity promote, guarded as the load rules
// require for a non-default target face.
const copyWiringMeta = `
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
copies:
  promote-page:
    from: page@draft
    to: page@published
    fields: all
    label: Publish
    guard:
      permission: promote-page
`

// TestCopyDepsAreWired is the regression test for a defect code review found:
// the copy feature was DEAD ON ARRIVAL in every production deployment.
//
// # What was broken
//
// `buildEntityManager` set `TransitionGuard` but none of `CopyGuard`,
// `CopyReadGate` or `CopyVisibility` — those three were assigned only in
// entitymanager's own tests. Compose that with two rules that are individually
// correct:
//
//   - a copy targeting a non-default face MUST declare a `guard:` (load-time
//     refusal, metamodel/copies.go), and
//   - `authorizeCopy` fails CLOSED when a guarded definition has no guard
//     wired,
//
// and every promote/publish — the entire motivating use case — returned 403
// everywhere. It failed closed, so it was never a security hole; it was a
// feature that could not work.
//
// # Why no existing test caught it
//
// Every entitymanager copy test constructs `Deps` by hand and sets the guard;
// every dataentry handler test uses a stub service. Nothing crossed the seam
// between the HTTP surface and a REALLY-WIRED manager, which is exactly where
// the defect lived. This test crosses it.
//
// # What it asserts
//
// That a manager built by the composition root can actually run a guarded
// copy. It deliberately asserts on the OUTCOME rather than on the presence of
// the deps: a test checking "CopyGuard != nil" would pass against a wiring
// that set it to something inert.
func TestCopyDepsAreWired(t *testing.T) {
	meta, err := metamodel.Parse([]byte(copyWiringMeta))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	st := memstore.New()
	svc := appbuildtest.New(meta, appbuildtest.WithStore(st))
	mgr, ok := svc.EntityManager().(*entitymanager.Manager)
	if !ok {
		t.Fatalf("the composition root must build a *entitymanager.Manager; got %T",
			svc.EntityManager())
	}

	ctx := context.Background()
	if serr := st.CreateEntity(ctx, &entity.Entity{
		ID: "PAGE-1", Type: "page", Properties: map[string]any{"title": "Draft"},
	}); serr != nil {
		t.Fatalf("seed: %v", serr)
	}

	res, err := mgr.CopyState(ctx, entitymanager.CopyRequest{
		Definition: "promote-page", SourceID: "PAGE-1",
	})
	if err != nil {
		t.Fatalf("a guarded copy must be RUNNABLE on a composition-root-built "+
			"manager. Before item 5 this returned \"requires permission ... but "+
			"no guard is wired\" in every deployment, because appbuild set no "+
			"Copy* deps at all — a feature that could not work anywhere. "+
			"Check buildEntityManager still sets CopyGuard / CopyReadGate / "+
			"CopyVisibility.\ngot: %v", err)
	}
	if res == nil || res.Entity == nil {
		t.Fatal("a successful copy must report the face it wrote")
	}
	if res.Entity.Pointer != entity.Pointer("published") {
		t.Errorf("the copy must write the PUBLISHED face; got pointer %q",
			res.Entity.Pointer)
	}
	if !res.Created {
		t.Error("the first promote CREATES the published face")
	}
	if got, _ := res.Entity.Properties["title"].(string); got != "Draft" {
		t.Errorf("`fields: all` copies the source's properties; got title %q", got)
	}
}

// TestCopyAffordancesAreWired is the same crossing for the READ half: the
// affordance query a UI calls must work on a composition-root-built manager,
// and must AGREE with what the write actually does.
//
// Separate from the test above because they fail differently: a wiring that
// made copies runnable but left the affordance query broken would ship a
// working endpoint whose buttons never appear.
func TestCopyAffordancesAreWired(t *testing.T) {
	meta, err := metamodel.Parse([]byte(copyWiringMeta))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	st := memstore.New()
	svc := appbuildtest.New(meta, appbuildtest.WithStore(st))
	mgr, ok := svc.EntityManager().(*entitymanager.Manager)
	if !ok {
		t.Fatalf("expected *entitymanager.Manager, got %T", svc.EntityManager())
	}

	ctx := context.Background()
	if serr := st.CreateEntity(ctx, &entity.Entity{
		ID: "PAGE-1", Type: "page", Properties: map[string]any{"title": "Draft"},
	}); serr != nil {
		t.Fatalf("seed: %v", serr)
	}

	offers, err := entitymanager.CopiesForSource(ctx, mgr, "page", "", "PAGE-1")
	if err != nil {
		t.Fatalf("CopiesForSource: %v", err)
	}
	if len(offers) != 1 {
		t.Fatalf("the draft face must offer promote-page; got %d offers", len(offers))
	}
	offer := offers[0]
	if offer.Label != "Publish" {
		t.Errorf("Label = %q, want the operator's `label:`", offer.Label)
	}
	if !offer.Allowed {
		t.Errorf("the affordance must report ALLOWED on a wired deployment. "+
			"Reporting allowed=false here is what a missing CopyGuard looked "+
			"like, and a UI would simply never render the button; reason=%q",
			offer.Reason)
	}

	// And the hint must agree with the write — on a real manager, not a stub.
	if _, err := mgr.CopyState(ctx, entitymanager.CopyRequest{
		Definition: "promote-page", SourceID: "PAGE-1",
	}); err != nil {
		t.Errorf("the affordance said allowed, so the write must succeed — "+
			"an affordance that lies is a trap for every consumer (RULING 11); "+
			"got %v", err)
	}
}
