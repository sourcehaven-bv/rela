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

// Reader-side grants (current_user) resolve normally — an entitled reader sees
// the field, an unentitled one doesn't. (Relation redaction is always resolved
// live against the source now; there is no historical-marker special case at the
// resolver — deleted-source history serves no meta at the handler instead.)
func TestRelationVisible_ReaderSideGrant(t *testing.T) {
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

	// auditor matches → note visible.
	hidden := r.RelationFieldVerdicts(ctxAs("auditor"), ticket("T-1", nil), "has-planning", keys)
	if v, ok := hidden["note"]; ok && !v {
		t.Errorf("auditor: note should be visible, got hidden")
	}

	// bob does not match → note hidden.
	hidden = r.RelationFieldVerdicts(ctxAs("bob"), ticket("T-1", nil), "has-planning", keys)
	if v, ok := hidden["note"]; !ok || v {
		t.Errorf("bob: note should be hidden, got ok=%v v=%v", ok, v)
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
