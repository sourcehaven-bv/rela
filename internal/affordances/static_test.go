package affordances_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
)

// Under WithStaticGrants every `when:` passes, so the answer is the most a row
// could show. Closed-world, the union across roles and unconditional denials
// are unchanged, because the same resolver computes them.
func TestStaticGrants_FieldVerdicts(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  triager:
    read: [ticket]
    visible:
      ticket:
        - field: title
        - field: assignee
          when: "entity.status == 'open'"
assignments:
  alice: triager
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	done := ticket("T-1", map[string]any{"status": "done"})

	live := r.FieldVerdicts(ctxAs("alice"), done).Visible
	if v, ok := live["assignee"]; !ok || v {
		t.Fatalf("live: assignee must be hidden on a done ticket, got %v", live)
	}

	static := r.FieldVerdicts(affordances.WithStaticGrants(ctxAs("alice")), done).Visible
	if _, hidden := static["assignee"]; hidden {
		t.Errorf("static: a conditional grant counts as granted, got %v", static)
	}
	if _, hidden := static["title"]; hidden {
		t.Errorf("static: title is granted unconditionally, got %v", static)
	}
	for _, f := range []string{"status", "priority", "due"} {
		if v, ok := static[f]; !ok || v {
			t.Errorf("static: %s is never granted and must stay hidden, got %v", f, static)
		}
	}
}

func TestStaticGrants_RelationFieldVerdicts(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  triager:
    relations:
      ticket:
        - relation: has-planning
          when: "entity.status == 'open'"
          visible:
            - field: note
assignments:
  alice: triager
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	done := ticket("T-1", map[string]any{"status": "done"})
	keys := []string{"note", "secret"}
	if hidden := r.RelationFieldVerdicts(ctxAs("alice"), done, "has-planning", keys); !hasKey(hidden, "note") {
		t.Fatalf("live: note must be hidden on a done ticket, got %v", hidden)
	}
	hidden := r.RelationFieldVerdicts(affordances.WithStaticGrants(ctxAs("alice")), done, "has-planning", keys)
	if _, ok := hidden["note"]; ok {
		t.Errorf("static: note is visible when the grant's when passes, got %v", hidden)
	}
	if _, ok := hidden["secret"]; !ok {
		t.Errorf("static: secret is never granted, got %v", hidden)
	}
}

func hasKey(m map[string]bool, k string) bool {
	_, ok := m[k]
	return ok
}
