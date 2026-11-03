package affordances_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
)

// TKT-73C6B2: under WithHistoricalSubject, a `visible:` grant conditioned on
// the subject's edges fails CLOSED. The live store cannot answer the entity's
// as-of-version edges, so trusting it would let the grant flip open and leak a
// field hidden at write time. traversalFor refuses a historical subject, so
// related(...) evaluates as denied and the field hides. Live reads (no marker)
// are unaffected: the edge is seen.
//
// has_relation and count_relations used to be pinned here too. A `visible:`
// condition must now lower to a store query, so both are refused at load
// (TestNew_VisibleWhenMustLower); related(...) is the form that remains.

// Scenario 1 (edge present at capture, gone/untrusted at read): a
// related()-gated visible field is VISIBLE live but HIDDEN historical.
func TestHistorical_RelatedVisible_FailsClosed(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  triager:
    visible:
      ticket:
        - field: assignee
          when: "related(entity, 'blocks')"
assignments:
  alice: triager
`)
	// Edge present in the LIVE graph.
	b := &stubBinder{answer: map[string]bool{"T-1": true}}
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p), affordances.WithTraversals(b))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	fvLive := r.FieldVerdicts(ctxFor(t, r, "alice"), ticket("T-1", nil))
	if v, ok := fvLive.Visible["assignee"]; ok && !v {
		t.Errorf("live: assignee should be visible (blocks edge present), got hidden")
	}

	// Historical read: the traversal is refused → grant fails → assignee
	// hidden, regardless of what the live store still holds.
	fvHist := r.FieldVerdicts(affordances.WithHistoricalSubject(ctxFor(t, r, "alice")), ticket("T-1", nil))
	if v, ok := fvHist.Visible["assignee"]; !ok || v {
		t.Errorf("historical: assignee must fail closed (hidden), got ok=%v v=%v", ok, v)
	}
}

// Scenario 3/5: the marker does NOT touch reader-side inputs. A grant gated on
// current_user (the reader) resolves identically live and historical — the
// reader stays live, so an entitled reader keeps historical visibility and an
// unentitled one stays denied, regardless of the marker. (This is why the
// marker only neuters subject-world edges, not the reader.) The condition
// compares the reader with a property the frozen record carries.
func TestHistorical_ReaderSideGrant_Unaffected(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  viewer:
    visible:
      ticket:
        - field: assignee
          when: "is_current_user(entity.assignee)"
assignments:
  auditor: viewer
  bob: viewer
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// auditor matches the reader-side predicate → assignee visible, live AND
	// historical (reader side is evaluated live under the marker).
	for _, hist := range []bool{false, true} {
		ctx := ctxFor(t, r, "auditor")
		if hist {
			ctx = affordances.WithHistoricalSubject(ctx)
		}
		fv := r.FieldVerdicts(ctx, ticket("T-1", map[string]any{"assignee": "auditor"}))
		if v, ok := fv.Visible["assignee"]; ok && !v {
			t.Errorf("auditor (historical=%v): assignee should be visible (reader is live), got hidden", hist)
		}
	}

	// bob does not match → assignee hidden, live AND historical (unchanged).
	for _, hist := range []bool{false, true} {
		ctx := ctxFor(t, r, "bob")
		if hist {
			ctx = affordances.WithHistoricalSubject(ctx)
		}
		fv := r.FieldVerdicts(ctx, ticket("T-1", map[string]any{"assignee": "auditor"}))
		if v, ok := fv.Visible["assignee"]; !ok || v {
			t.Errorf("bob (historical=%v): assignee should be hidden, got ok=%v v=%v", hist, ok, v)
		}
	}
}

// RR (role-resolution leak): under the historical marker, a type that ANY role
// gates with `visible:` gets a TYPE-LEVEL closed-world, so a reader who resolves
// to FEWER roles historically (e.g. a local role dropped because it was
// conferred by a now-untrusted live edge) fails closed to hidden instead of
// defaulting to all-visible. Here alice holds NO global role, so the historical
// role set is empty — every field must be hidden, not shown.
func TestHistorical_TypeLevelClosedWorld_EmptyRoleSet(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  owner:
    visible:
      ticket:
        - field: title
assignments:
  bob: owner
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// alice has no role at all. LIVE read: no role declares a visible: block for
	// her → closed-world never opts in → all fields visible (protected by the
	// row gate live, not field redaction).
	fvLive := r.FieldVerdicts(ctxFor(t, r, "alice"), ticket("T-1", nil))
	if v, ok := fvLive.Visible["priority"]; ok && !v {
		t.Errorf("live, no role: priority should default visible, got hidden")
	}

	// HISTORICAL read: ticket has a visible: block (declared by owner), so the
	// type-level closed-world opts in even though alice holds no role → every
	// field in the universe is hidden.
	fvHist := r.FieldVerdicts(affordances.WithHistoricalSubject(ctxFor(t, r, "alice")), ticket("T-1", nil))
	if v, ok := fvHist.Visible["priority"]; !ok || v {
		t.Errorf("historical, no role: priority must fail closed (hidden), got ok=%v v=%v", ok, v)
	}
	if v, ok := fvHist.Visible["title"]; !ok || v {
		t.Errorf("historical, no role: title must fail closed (hidden), got ok=%v v=%v", ok, v)
	}
}

// The historical marker is INERT for a type no role gates with `visible:` —
// there is nothing to fail closed, so redaction matches live (all visible). This
// guards against the type-level closed-world over-firing on unredacted types.
func TestHistorical_NoVisiblePolicyForType_MarkerInert(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  triager:
    fields:
      ticket:
        - field: status
assignments:
  alice: triager
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// No visible: block anywhere → historical marker changes nothing.
	fv := r.FieldVerdicts(affordances.WithHistoricalSubject(ctxFor(t, r, "alice")), ticket("T-1", nil))
	if v, ok := fv.Visible["priority"]; ok && !v {
		t.Errorf("no visible policy: historical marker must be inert, priority hidden unexpectedly")
	}
}

// Scenario 7: a visible: grant that references a property the frozen record
// does not carry (e.g. today's policy references a since-renamed/removed
// property) binds that property as Nil (DR-C2 coerce-not-fail), the predicate
// evaluates false, and the field FAILS CLOSED — a live policy over a drifted
// schema over-redacts, never leaks. (This holds live too; the marker is
// orthogonal here — pinned under both.)
func TestHistorical_MissingReferencedProperty_FailsClosed(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  triager:
    visible:
      ticket:
        - field: assignee
          when: "entity.status == 'done'"
assignments:
  alice: triager
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Ticket has NO status property set → entity.status binds Nil → predicate
	// false → assignee hidden, live AND historical.
	for _, hist := range []bool{false, true} {
		ctx := ctxFor(t, r, "alice")
		if hist {
			ctx = affordances.WithHistoricalSubject(ctx)
		}
		fv := r.FieldVerdicts(ctx, ticket("T-1", nil))
		if v, ok := fv.Visible["assignee"]; !ok || v {
			t.Errorf("missing referenced property (historical=%v): assignee must fail closed, got ok=%v v=%v", hist, ok, v)
		}
	}
}

// An unconditional visible: grant is unaffected by the marker — nothing to fail
// closed, so historical redaction matches live (the field stays visible).
func TestHistorical_UnconditionalVisible_Unaffected(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  triager:
    visible:
      ticket:
        - field: assignee
assignments:
  alice: triager
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	fv := r.FieldVerdicts(affordances.WithHistoricalSubject(ctxFor(t, r, "alice")), ticket("T-1", nil))
	if v, ok := fv.Visible["assignee"]; ok && !v {
		t.Errorf("historical: unconditional visible grant should keep assignee visible, got hidden")
	}
}
