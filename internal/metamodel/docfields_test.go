package metamodel

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// docFieldsYAML is a metamodel exercising all three doc-fields (TKT-0YBFT8):
// top-level description, per-value CustomType descriptions, and TransitionDef
// help. It also carries labels + a transition label so the tests can confirm the
// new fields are DISTINCT from (not colliding with) the existing display fields.
const docFieldsYAML = `version: "1.0"
description: |
  A demo ticket tracker. This is the deployment description that end-user docs
  should surface.
types:
  ticket-status:
    values: [todo, doing, done]
    initial: todo
    labels:
      todo: To do
      doing: In progress
    descriptions:
      todo: Work that hasn't been started yet.
      doing: Work actively in progress by an assignee.
    transitions:
      - from: todo
        to: doing
        label: Start progress
        help: Begin work once the ticket is assigned and ready.
      - from: doing
        to: done
        label: Complete
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    properties:
      title:
        type: string
      status:
        type: ticket-status
`

// AC1/AC2/AC3: each doc-field parses into its struct field.
func TestParse_DocFields_Present(t *testing.T) {
	m, err := Parse([]byte(docFieldsYAML))
	assertNoError(t, err)

	// AC1: top-level description.
	if m.Description == "" {
		t.Error("Metamodel.Description should be populated from `description:`")
	}

	ct := m.Types["ticket-status"]

	// AC2: per-value descriptions, keyed by value, distinct from Labels.
	assertEqual(t, ct.Descriptions["todo"], "Work that hasn't been started yet.")
	assertEqual(t, ct.Descriptions["doing"], "Work actively in progress by an assignee.")
	// Labels still carry the short display text — the two maps are independent.
	assertEqual(t, ct.Labels["todo"], "To do")
	// A value with no description entry is simply absent (not an error).
	if _, ok := ct.Descriptions["done"]; ok {
		t.Error("value with no `descriptions:` entry should be absent from the map")
	}

	// AC3: transition help, distinct from the transition label.
	tr := ct.Transitions[0]
	assertEqual(t, tr.From, "todo")
	assertEqual(t, tr.Label, "Start progress")
	assertEqual(t, tr.Help, "Begin work once the ticket is assigned and ready.")
	// A transition with no help is simply empty.
	assertEqual(t, ct.Transitions[1].Help, "")
}

// AC4 (absence): a metamodel with none of the doc-fields loads unchanged, with
// zero-valued doc-fields.
func TestParse_DocFields_Absent(t *testing.T) {
	const yaml = `version: "1.0"
types:
  ticket-status:
    values: [todo, done]
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    properties:
      title:
        type: string
`
	m, err := Parse([]byte(yaml))
	assertNoError(t, err)

	assertEqual(t, m.Description, "")
	ct := m.Types["ticket-status"]
	if ct.Descriptions != nil {
		t.Errorf("absent `descriptions:` should be nil, got %v", ct.Descriptions)
	}
}

// AC5: `description:` is a valid top-level key — checkUnknownKeys must not flag
// it. (Regression guard for the validTopLevelKeys allowlist change.)
func TestParse_DocFields_TopLevelDescriptionAccepted(t *testing.T) {
	const yaml = `version: "1.0"
description: A one-line deployment description.
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    properties:
      title:
        type: string
`
	m, err := Parse([]byte(yaml))
	assertNoError(t, err) // would be a SchemaValidationError without the allowlist entry
	assertEqual(t, m.Description, "A one-line deployment description.")
}

// AC4 (round-trip): parse -> marshal -> parse preserves all three doc-fields.
func TestParse_DocFields_RoundTrip(t *testing.T) {
	m1, err := Parse([]byte(docFieldsYAML))
	assertNoError(t, err)

	out, err := yaml.Marshal(m1)
	assertNoError(t, err)

	m2, err := Parse(out)
	assertNoError(t, err)

	assertEqual(t, m2.Description, m1.Description)
	ct1, ct2 := m1.Types["ticket-status"], m2.Types["ticket-status"]
	assertEqual(t, ct2.Descriptions["todo"], ct1.Descriptions["todo"])
	assertEqual(t, ct2.Descriptions["doing"], ct1.Descriptions["doing"])
	assertEqual(t, ct2.Transitions[0].Help, ct1.Transitions[0].Help)
}
