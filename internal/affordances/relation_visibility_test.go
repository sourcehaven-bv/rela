package affordances_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
)

// TKT-B1F5Q1: relation field-level `visible:` redaction, resolved by
// RelationFieldVerdicts. Mirrors the entity FieldVerdicts Visible dimension —
// closed-world per relation type, conditional grants via `when:`, and
// fail-closed under the historical-subject marker (inheriting TKT-73C6B2).

// An unconditional relation `visible:` grant makes only the granted meta fields
// visible; every other actual meta key is closed-world denied.
func TestRelationVisible_ClosedWorld(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  triager:
    relations:
      ticket:
        - relation: has-planning
          visible:
            - field: note
assignments:
  alice: triager
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Edge carries note (granted) + secret (not granted) → secret hidden, note visible.
	keys := []string{"note", "secret"}
	hidden := r.RelationFieldVerdicts(ctxAs("alice"), ticket("T-1", nil), "has-planning", keys)
	if v, ok := hidden["secret"]; !ok || v {
		t.Errorf("secret should be closed-world denied (hidden), got ok=%v v=%v", ok, v)
	}
	if v, ok := hidden["note"]; ok && !v {
		t.Errorf("note is granted → must be visible, got hidden")
	}
}

// A relation type with NO `visible:` block leaves meta fully visible (permissive
// default) — the closed-world only bites for opted-in types.
func TestRelationVisible_NoBlock_Permissive(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  triager:
    relations:
      ticket:
        - relation: has-planning
          fields:
            - field: note
assignments:
  alice: triager
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Only a write-side `fields:` block exists, no `visible:` → nothing denied.
	hidden := r.RelationFieldVerdicts(ctxAs("alice"), ticket("T-1", nil), "has-planning",
		[]string{"note", "secret"})
	for k, v := range hidden {
		if !v {
			t.Errorf("no visible: block → %q must not be redacted, got hidden", k)
		}
	}
}

// A conditional relation `visible:` grant is subject to BOTH its own `when:` and
// the whole-grant `when:`. Here the field predicate keys off the source entity's
// status: visible when open, hidden when done.
func TestRelationVisible_Conditional(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  triager:
    relations:
      ticket:
        - relation: has-planning
          visible:
            - field: note
              when: "entity.status == 'open'"
assignments:
  alice: triager
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	keys := []string{"note"}

	// status=open → note visible.
	hidden := r.RelationFieldVerdicts(ctxAs("alice"),
		ticket("T-1", map[string]any{"status": "open"}), "has-planning", keys)
	if v, ok := hidden["note"]; ok && !v {
		t.Errorf("status=open: note should be visible, got hidden")
	}

	// status=done → note hidden (predicate false; closed-world denies it).
	hidden = r.RelationFieldVerdicts(ctxAs("alice"),
		ticket("T-1", map[string]any{"status": "done"}), "has-planning", keys)
	if v, ok := hidden["note"]; !ok || v {
		t.Errorf("status=done: note must be hidden, got ok=%v v=%v", ok, v)
	}
}

// A relation `visible:` grant conditioned on a subject-world lookup
// (has_relation) fails CLOSED under the historical marker: the live store can't
// answer the source's as-of-version edges, so trusting it would leak a field
// hidden at write time.
func TestRelationVisible_HistoricalHasRelation_FailsClosed(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  triager:
    relations:
      ticket:
        - relation: has-planning
          visible:
            - field: note
              when: "has_relation(entity, 'blocks')"
assignments:
  alice: triager
`)
	// Source T-1 has a live blocks edge.
	r, err := affordances.New(testMeta(t),
		newStubLookup([3]string{"T-1", "blocks", "T-9"}), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	keys := []string{"note"}

	// Live: edge present → grant passes → note visible.
	hidden := r.RelationFieldVerdicts(ctxAs("alice"), ticket("T-1", nil), "has-planning", keys)
	if v, ok := hidden["note"]; ok && !v {
		t.Errorf("live: note should be visible (blocks edge present), got hidden")
	}

	// Historical: marker neuters the subject-world lookup → grant fails → note hidden.
	ctxHist := affordances.WithHistoricalSubject(ctxAs("alice"))
	hidden = r.RelationFieldVerdicts(ctxHist, ticket("T-1", nil), "has-planning", keys)
	if v, ok := hidden["note"]; !ok || v {
		t.Errorf("historical: note must fail closed (hidden), got ok=%v v=%v", ok, v)
	}
}

// RR (role-resolution leak, relation analog): under the historical marker, a
// relation type that ANY role gates with `visible:` gets a TYPE-LEVEL
// closed-world, so a reader who resolves to zero applicable roles fails closed to
// hidden instead of defaulting to all-visible. alice holds no global role here.
func TestRelationVisible_HistoricalTypeLevelClosedWorld_EmptyRoleSet(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  owner:
    relations:
      ticket:
        - relation: has-planning
          visible:
            - field: note
assignments:
  bob: owner
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	keys := []string{"note", "secret"}

	// alice has no role. LIVE: no role opts in for her → all visible.
	hidden := r.RelationFieldVerdicts(ctxAs("alice"), ticket("T-1", nil), "has-planning", keys)
	for k, v := range hidden {
		if !v {
			t.Errorf("live, no role: %q should default visible, got hidden", k)
		}
	}

	// HISTORICAL: has-planning has a visible: block (declared by owner) → type-level
	// closed-world opts in even though alice holds no role → every key hidden.
	fvHist := r.RelationFieldVerdicts(
		affordances.WithHistoricalSubject(ctxAs("alice")), ticket("T-1", nil), "has-planning", keys)
	for _, k := range keys {
		if v, ok := fvHist[k]; !ok || v {
			t.Errorf("historical, no role: %q must fail closed (hidden), got ok=%v v=%v", k, ok, v)
		}
	}
}

// The historical marker is INERT for a relation type no role gates with
// `visible:` — nothing to fail closed, so redaction matches live (all visible).
func TestRelationVisible_HistoricalNoVisiblePolicy_MarkerInert(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  triager:
    relations:
      ticket:
        - relation: has-planning
          fields:
            - field: note
assignments:
  alice: triager
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	hidden := r.RelationFieldVerdicts(
		affordances.WithHistoricalSubject(ctxAs("alice")), ticket("T-1", nil), "has-planning",
		[]string{"note", "secret"})
	for k, v := range hidden {
		if !v {
			t.Errorf("no visible policy: historical marker must be inert, %q hidden unexpectedly", k)
		}
	}
}

// Reader-side grants (current_user) stay live under the marker — an entitled
// reader keeps historical relation-meta visibility; an unentitled one stays
// denied. Confirms the marker only neuters subject-world, not the reader.
func TestRelationVisible_HistoricalReaderSide_Unaffected(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  viewer:
    relations:
      ticket:
        - relation: has-planning
          visible:
            - field: note
              when: "current_user.id == 'auditor'"
assignments:
  auditor: viewer
  bob: viewer
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	keys := []string{"note"}

	// auditor matches reader-side predicate → note visible, live AND historical.
	for _, hist := range []bool{false, true} {
		ctx := ctxAs("auditor")
		if hist {
			ctx = affordances.WithHistoricalSubject(ctx)
		}
		hidden := r.RelationFieldVerdicts(ctx, ticket("T-1", nil), "has-planning", keys)
		if v, ok := hidden["note"]; ok && !v {
			t.Errorf("auditor (historical=%v): note should be visible (reader is live), got hidden", hist)
		}
	}

	// bob does not match → note hidden, live AND historical.
	for _, hist := range []bool{false, true} {
		ctx := ctxAs("bob")
		if hist {
			ctx = affordances.WithHistoricalSubject(ctx)
		}
		hidden := r.RelationFieldVerdicts(ctx, ticket("T-1", nil), "has-planning", keys)
		if v, ok := hidden["note"]; !ok || v {
			t.Errorf("bob (historical=%v): note should be hidden, got ok=%v v=%v", hist, ok, v)
		}
	}
}

// A free-form meta key not declared in the metamodel is still redacted under a
// closed-world block — the deny universe is the edge's actual keys, so a caller
// cannot smuggle a secret past redaction by using an undeclared property name.
func TestRelationVisible_FreeFormKey_ClosedWorld(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  triager:
    relations:
      ticket:
        - relation: has-planning
          visible:
            - field: note
assignments:
  alice: triager
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// undeclared_secret is not in the metamodel's relation properties, but it is
	// present on the edge and not granted → must be hidden.
	hidden := r.RelationFieldVerdicts(ctxAs("alice"), ticket("T-1", nil), "has-planning",
		[]string{"note", "undeclared_secret"})
	if v, ok := hidden["undeclared_secret"]; !ok || v {
		t.Errorf("free-form ungranted key must be hidden, got ok=%v v=%v", ok, v)
	}
}
