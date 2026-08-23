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

// replaceTailMetaYAML stages a copy INTO a non-default face, which is the
// only shape where the tail-dropping bug is visible: when the target is the
// default face, dropping the tail happens to address the same edge.
const replaceTailMetaYAML = `
version: "1"
entities:
  page:
    label: Page
    id_prefix: PAGE
    pointers:
      live: {default: true}
      review: {}
    properties:
      title: {type: string}
relations:
  references:
    label: References
    scope: content
    from: [page]
    to: [page]
copies:
  stage-review:
    from: page@live
    to: page@review
    fields: all
    relations:
      references: replace
    guard:
      permission: stage
`

// TestCopyReplace_DeletesTheTargetTailNotTheDefaultFacesEdge is named for the
// corruption it prevents, and it reproduces a bug that was live in PR-C.
//
// THE BUG: applyCopyEdges listed the target face's edges filtered BY TAIL,
// then called DeleteRelation with the tail DROPPED. Every backend's
// DeleteRelation is default-tail-only (pgstore `AND from_pointer = ”`,
// memstore defaultTailKey, fsstore's bare key), so the delete did not fail —
// it removed the DEFAULT face's edge on the same triple and returned nil,
// while the edge being replaced survived.
//
// Verified against memstore before the fix:
//
//	delete returned: nil
//	edges surviving the intended delete of the PUBLISHED-tail edge:
//	  [PAGE-1@published--references--SPEC-1]
//
// So `relations: replace` into a non-default face corrupted a face the copy
// had no business touching AND failed its own job, silently, in all three
// backends. `promote-page` with `relations: replace` is the headline use case
// in design doc §9.1.
//
// Both assertions matter and neither is redundant: the target-side one
// catches "the edge I meant to replace survived", the source-side one catches
// "I destroyed a bystander". The bug produced BOTH at once.
func TestCopyReplace_DeletesTheTargetTailNotTheDefaultFacesEdge(t *testing.T) {
	var st store.Store = memstore.New()
	meta, err := metamodel.Parse([]byte(replaceTailMetaYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:       st,
		Meta:        meta,
		Templater:   nopTemplater{},
		Audit:       audit.Nop{},
		ACL:         acl.NopACL{},
		Transitions: statemachine.EmptySet(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
		CopyGuard:   allowGuard{allow: true},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}

	ctx := context.Background()
	seed := func(id string, p entity.Pointer, title string) {
		t.Helper()
		e := &entity.Entity{
			ID: id, Type: "page", Pointer: p,
			Properties: map[string]any{"title": title},
		}
		if cerr := st.CreateEntity(ctx, e); cerr != nil {
			t.Fatalf("seed %s@%s: %v", id, p, cerr)
		}
	}
	// A state row cannot exist headless, so the default face comes first.
	seed("PAGE-1", "", "live")
	seed("PAGE-1", "review", "staged")
	seed("OLD-1", "", "old target")
	seed("NEW-1", "", "new target")

	reviewTail := entity.Pointer("review")
	// Two edges differing ONLY by tail — these are two distinct relations.
	// The SOURCE (default) face points at NEW-1; this is what gets copied.
	if _, cerr := st.CreateRelation(ctx, "PAGE-1", "references", "NEW-1", nil); cerr != nil {
		t.Fatalf("seed default-tail edge: %v", cerr)
	}
	// The TARGET (review) face points at OLD-1; `replace` must remove this.
	if _, cerr := st.CreateRelation(ctx, "PAGE-1", "references", "OLD-1",
		&store.RelationData{FromPointer: reviewTail}); cerr != nil {
		t.Fatalf("seed review-tail edge: %v", cerr)
	}

	if _, cerr := mgr.CopyState(ctx, entitymanager.CopyRequest{
		Definition: "stage-review", SourceID: "PAGE-1",
	}); cerr != nil {
		t.Fatalf("copy: %v", cerr)
	}

	targets := func(p *entity.Pointer) []string {
		t.Helper()
		q := store.RelationQuery{From: "PAGE-1", Type: "references"}
		var out []string
		for r, lerr := range st.ListRelations(ctx, q) {
			if lerr != nil {
				t.Fatalf("list: %v", lerr)
			}
			switch {
			case p == nil && r.FromPointer.IsDefault():
				out = append(out, r.To)
			case p != nil && r.FromPointer == *p:
				out = append(out, r.To)
			}
		}
		return out
	}

	// The TARGET face must now hold NEW-1 only: its OLD-1 edge was replaced.
	// Under the bug OLD-1 survived here, because the delete went elsewhere.
	if got := targets(&reviewTail); len(got) != 1 || got[0] != "NEW-1" {
		t.Errorf("replace must swap the TARGET face's edges; got %v, want [NEW-1] "+
			"— a surviving OLD-1 means the delete addressed the wrong tail", got)
	}

	// The SOURCE (default) face's edge must be untouched. Under the bug this
	// is the edge that got deleted — a face the copy never addressed.
	if got := targets(nil); len(got) != 1 || got[0] != "NEW-1" {
		t.Errorf("the DEFAULT face's edge must survive a replace on another face; "+
			"got %v, want [NEW-1] — losing it means the copy corrupted a bystander",
			got)
	}
}
