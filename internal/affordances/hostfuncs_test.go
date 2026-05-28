package affordances_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// metaWithRelations adds list and boolean properties plus the relation
// types the host-func tests reference.
func metaWithExtras(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m := testMeta(t)
	def := m.Entities["ticket"]
	def.Properties["labels"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeString, List: true}
	def.Properties["urgent"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeBoolean}
	m.Entities["ticket"] = def
	return m
}

// has_relation gates a field on whether the entity has an outgoing
// edge of a given type.
func TestHostFunc_HasRelation(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  triager:
    fields:
      ticket:
        - field: status
          when: "has_relation(entity, 'blocks')"
assignments:
  alice: triager
`)
	// T-1 has a blocks edge; T-2 does not.
	lookup := newStubLookup([3]string{"T-1", "blocks", "T-9"})
	r, err := affordances.New(p, metaWithExtras(t), lookup)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, denied := r.FieldVerdicts(ctxAs("alice"), ticket("T-1", nil)).Writable["status"]; denied {
		t.Errorf("T-1 has blocks edge → status should be writable")
	}
	if v, ok := r.FieldVerdicts(ctxAs("alice"), ticket("T-2", nil)).Writable["status"]; !ok || v {
		t.Errorf("T-2 has no blocks edge → status should be denied")
	}
}

// count_relations gates a field on an edge count.
func TestHostFunc_CountRelations(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  triager:
    fields:
      ticket:
        - field: status
          when: "count_relations(entity, 'blocks') >= 2"
assignments:
  alice: triager
`)
	lookup := newStubLookup(
		[3]string{"T-1", "blocks", "A"},
		[3]string{"T-1", "blocks", "B"},
		[3]string{"T-2", "blocks", "A"},
	)
	r, err := affordances.New(p, metaWithExtras(t), lookup)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, denied := r.FieldVerdicts(ctxAs("alice"), ticket("T-1", nil)).Writable["status"]; denied {
		t.Errorf("T-1 has 2 blocks → status writable")
	}
	if v, ok := r.FieldVerdicts(ctxAs("alice"), ticket("T-2", nil)).Writable["status"]; !ok || v {
		t.Errorf("T-2 has 1 block → status denied")
	}
}

// string_in_list gates membership of a scalar in a list-typed property.
// The predicate parser accepts table literals only as named-args to a
// function call, so the list argument is an entity list property, not
// an inline literal — the canonical usage.
func TestHostFunc_StringInList(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  triager:
    fields:
      ticket:
        - field: status
          when: "string_in_list(entity.status, entity.labels)"
assignments:
  alice: triager
`)
	r, err := affordances.New(p, metaWithExtras(t), newStubLookup())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// status "open" is in labels → writable.
	if _, denied := r.FieldVerdicts(ctxAs("alice"),
		ticket("T-1", map[string]interface{}{
			"status": "open", "labels": []interface{}{"open", "review"},
		})).Writable["status"]; denied {
		t.Errorf("status in labels → writable")
	}
	// status "done" not in labels → denied.
	if v, ok := r.FieldVerdicts(ctxAs("alice"),
		ticket("T-2", map[string]interface{}{
			"status": "done", "labels": []interface{}{"open", "review"},
		})).Writable["status"]; !ok || v {
		t.Errorf("status not in labels → denied")
	}
}

// Boolean property coercion: a bool-typed property binds as a predicate
// bool and compares correctly.
func TestCoerce_Boolean(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  triager:
    fields:
      ticket:
        - field: status
          when: "entity.urgent == true"
assignments:
  alice: triager
`)
	r, err := affordances.New(p, metaWithExtras(t), newStubLookup())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, denied := r.FieldVerdicts(ctxAs("alice"),
		ticket("T-1", map[string]interface{}{"urgent": true})).Writable["status"]; denied {
		t.Errorf("urgent=true → status writable")
	}
	// stored as the string "false" → coerces to bool false → denied.
	if v, ok := r.FieldVerdicts(ctxAs("alice"),
		ticket("T-2", map[string]interface{}{"urgent": "false"})).Writable["status"]; !ok || v {
		t.Errorf("urgent='false' coerces to false → status denied")
	}
}

// M3: a list property whose stored elements aren't all strings
// coerces non-string elements to Nil holes rather than failing Eval.
// The string elements still match.
func TestCoerce_List_NonStringElements(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  triager:
    fields:
      ticket:
        - field: status
          when: "string_in_list('vip', entity.labels)"
assignments:
  alice: triager
`)
	r, err := affordances.New(p, metaWithExtras(t), newStubLookup())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// labels mixes a string and a number; "vip" is still found.
	if _, denied := r.FieldVerdicts(ctxAs("alice"),
		ticket("T-1", map[string]interface{}{
			"labels": []interface{}{"vip", 42},
		})).Writable["status"]; denied {
		t.Errorf("vip present among mixed elements → status writable, no Eval failure")
	}
}

// List property coercion + string_in_list over the entity's own list.
func TestCoerce_List(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  triager:
    fields:
      ticket:
        - field: status
          when: "string_in_list('vip', entity.labels)"
assignments:
  alice: triager
`)
	r, err := affordances.New(p, metaWithExtras(t), newStubLookup())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, denied := r.FieldVerdicts(ctxAs("alice"),
		ticket("T-1", map[string]interface{}{"labels": []interface{}{"vip", "urgent"}})).Writable["status"]; denied {
		t.Errorf("labels contains vip → status writable")
	}
	// single scalar promoted to one-element list; "vip" not present.
	if v, ok := r.FieldVerdicts(ctxAs("alice"),
		ticket("T-2", map[string]interface{}{"labels": "other"})).Writable["status"]; !ok || v {
		t.Errorf("labels=[other] lacks vip → status denied")
	}
}
