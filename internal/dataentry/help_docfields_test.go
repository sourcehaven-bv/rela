package dataentry

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// helpMeta is a metamodel with a state-machine status enum (values + per-value
// descriptions + transitions with labels/help), a plain enum (values only), and
// a non-enum property, exercising every gatherEnumHelp branch (TKT-DUQBD0).
func helpMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"ticket-status": {
				Values:  []string{"todo", "doing", "done"},
				Initial: "todo",
				Labels:  map[string]string{"doing": "In progress"},
				Descriptions: map[string]string{
					"todo":  "Not started yet.",
					"doing": "Actively worked on.",
				},
				Transitions: []metamodel.TransitionDef{
					{From: "todo", To: "doing", Label: "Start", Help: "when someone picks it up"},
					{From: "doing", To: "done", Label: "Complete"},
				},
			},
			"priority": {Values: []string{"high", "low"}},
		},
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label: "Ticket",
				Properties: map[string]metamodel.PropertyDef{
					"title":    {Type: "string"},
					"status":   {Type: "ticket-status"},
					"priority": {Type: "priority"},
				},
			},
		},
	}
}

func findEnum(hs []EnumHelp, prop string) (EnumHelp, bool) {
	for _, h := range hs {
		if h.Property == prop {
			return h, true
		}
	}
	return EnumHelp{}, false
}

// AC1/AC2: gatherEnumHelp collects values + lifecycle for a state machine, values
// only for a plain enum, and nothing for a non-enum property.
func TestGatherEnumHelp(t *testing.T) {
	m := helpMeta()
	def := m.Entities["ticket"]
	hs := gatherEnumHelp(m, &def)

	// title is not an enum → no entry.
	if _, ok := findEnum(hs, "title"); ok {
		t.Error("non-enum property should not appear in enum help")
	}

	status, ok := findEnum(hs, "status")
	if !ok {
		t.Fatal("status (state machine) should appear")
	}
	if status.Initial != "todo" {
		t.Errorf("Initial = %q, want todo", status.Initial)
	}
	if len(status.Values) != 3 {
		t.Fatalf("status values = %d, want 3", len(status.Values))
	}
	// Value carries label + description; a value with no description is empty.
	doing := status.Values[1]
	badDoing := doing.Value != "doing" || doing.Label != "In progress" ||
		!strings.Contains(string(doing.Description), "Actively worked on")
	if badDoing {
		t.Errorf("doing value help mismatch: %+v", doing)
	}
	if status.Values[2].Value != "done" || status.Values[2].Description != "" {
		t.Errorf("done should have no description, got %+v", status.Values[2])
	}
	// Transitions carry move label + help; a transition with no help is empty.
	if len(status.Transitions) != 2 {
		t.Fatalf("status transitions = %d, want 2", len(status.Transitions))
	}
	firstMove := status.Transitions[0]
	badMove := firstMove.Move != "Start" || !strings.Contains(string(firstMove.Help), "picks it up")
	if badMove {
		t.Errorf("first transition help mismatch: %+v", firstMove)
	}
	if status.Transitions[1].Help != "" {
		t.Errorf("Complete transition should have no help, got %q", status.Transitions[1].Help)
	}

	// priority is a plain enum → values, no transitions.
	prio, ok := findEnum(hs, "priority")
	if !ok {
		t.Fatal("priority (plain enum) should appear")
	}
	if len(prio.Values) != 2 || len(prio.Transitions) != 0 {
		t.Errorf("priority should have values but no transitions, got %+v", prio)
	}
}

// AC2: the mermaid diagram carries the entry arrow + one labeled edge per
// transition.
func TestMermaidStateDiagram(t *testing.T) {
	e := EnumHelp{
		Initial: "todo",
		Transitions: []TransitionHelp{
			{Move: "Start", From: "todo", To: "doing"},
			{Move: "done", From: "doing", To: "done"}, // move == to → no label
		},
	}
	got := mermaidStateDiagram(e)

	for _, want := range []string{
		"stateDiagram-v2",
		"[*] --> todo",
		"todo --> doing: Start",
		"doing --> done\n", // no ": done" label because move == to
	} {
		if !strings.Contains(got, want) {
			t.Errorf("diagram missing %q:\n%s", want, got)
		}
	}
}

// AC5: appDescription prefers the data-entry app.description, falling back to the
// metamodel's top-level description, and is empty when neither is set.
func TestAppDescription(t *testing.T) {
	cases := []struct {
		name    string
		appDesc string
		metaDsc string
		want    string
	}{
		{"app wins", "from app", "from meta", "from app"},
		{"fallback to meta", "", "from meta", "from meta"},
		{"neither", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Schema{
				Cfg:  &Config{App: AppConfig{Description: tc.appDesc}},
				Meta: &metamodel.Metamodel{Description: tc.metaDsc},
			}
			if got := appDescription(s); got != tc.want {
				t.Errorf("appDescription = %q, want %q", got, tc.want)
			}
		})
	}
}
