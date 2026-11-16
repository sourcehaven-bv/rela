package dataentryconfig

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func llmTestMeta() *metamodel.Metamodel {
	one := 1
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"title":    {Type: metamodel.PropertyTypeString},
				"priority": {Type: metamodel.PropertyTypeEnum, Values: []string{"low", "high"}},
				"score":    {Type: metamodel.PropertyTypeInteger, Computed: "1"},
				"doc":      {Type: metamodel.PropertyTypeFile},
			}},
			"concept": {Properties: map[string]metamodel.PropertyDef{
				"title": {Type: metamodel.PropertyTypeString},
			}},
			"document": {Faces: map[string]metamodel.FaceDef{"draft": {}}},
		},
		Relations: map[string]metamodel.RelationDef{
			"affects":   {From: []string{"ticket"}, To: []string{"concept"}},
			"owner":     {From: []string{"ticket"}, To: []string{"concept"}, MaxOutgoing: &one},
			"cites":     {From: []string{"ticket"}, To: []string{"document"}},
			"weighted":  {From: []string{"ticket"}, To: []string{"concept"}, Properties: map[string]metamodel.PropertyDef{"w": {Type: "integer", Required: true}}},
			"describes": {From: []string{"concept"}, To: []string{"ticket"}},
		},
	}
}

func llmAction(mut func(a *Action)) Action {
	a := Action{
		Permission:  "ticket.triage",
		AvailableOn: &ActionScope{EntityTypes: []string{"ticket"}},
		LLM: &LLMAction{
			Prompt: "Triage the ticket.",
			Inputs: LLMInputs{
				Properties: []string{"title"},
				Content:    &LLMContentInput{},
				Related:    []LLMRelatedInput{{Relation: "describes", Direction: "incoming", Fields: []string{"title"}}},
			},
			Outputs: LLMOutputs{
				Set:    []string{"priority"},
				Relate: []LLMRelateOutput{{Relation: "affects", Fields: []string{"title"}, Max: 3}},
			},
		},
	}
	if mut != nil {
		mut(&a)
	}
	return a
}

func TestValidateActions_LLM(t *testing.T) {
	tests := []struct {
		name    string
		meta    func(*metamodel.Metamodel)
		action  Action
		wantErr string
	}{
		{name: "valid", action: llmAction(nil)},
		{
			name:    "with script",
			action:  llmAction(func(a *Action) { a.Script = "x.lua" }),
			wantErr: "an llm action has no script or set",
		},
		{
			name:    "no permission",
			action:  llmAction(func(a *Action) { a.Permission = "" }),
			wantErr: "permission is required",
		},
		{
			name:    "no prompt",
			action:  llmAction(func(a *Action) { a.LLM.Prompt = "  " }),
			wantErr: "prompt is required",
		},
		{
			name:    "capabilities",
			action:  llmAction(func(a *Action) { a.Capabilities.AI = true }),
			wantErr: "capabilities: applies only to a script action",
		},
		{
			name:    "elevation",
			action:  llmAction(func(a *Action) { a.AllowACLBypass = metamodel.ACLBypassWrite }),
			wantErr: "allow_acl_bypass and elevated_updates apply only to a script action",
		},
		{
			name:    "no available_on",
			action:  llmAction(func(a *Action) { a.AvailableOn = nil }),
			wantErr: "needs available_on with exactly one entity type",
		},
		{
			name: "two subject types",
			action: llmAction(func(a *Action) {
				a.AvailableOn.EntityTypes = []string{"ticket", "concept"}
			}),
			wantErr: "needs available_on with exactly one entity type",
		},
		{
			name: "faced subject",
			action: llmAction(func(a *Action) {
				a.AvailableOn.EntityTypes = []string{"document"}
				a.LLM.Inputs = LLMInputs{}
				a.LLM.Outputs = LLMOutputs{Set: []string{"x"}}
			}),
			wantErr: `"document" has content states`,
		},
		{
			name:    "unknown input property",
			action:  llmAction(func(a *Action) { a.LLM.Inputs.Properties = []string{"nope"} }),
			wantErr: `inputs.properties: unknown property "nope"`,
		},
		{
			name:    "content cap too large",
			action:  llmAction(func(a *Action) { a.LLM.Inputs.Content.MaxChars = MaxLLMContentChars + 1 }),
			wantErr: "inputs.content.max_chars",
		},
		{
			name:    "related wrong direction",
			action:  llmAction(func(a *Action) { a.LLM.Inputs.Related[0].Direction = "" }),
			wantErr: `relation "describes" does not connect "ticket" in direction outgoing`,
		},
		{
			name:    "related bad direction",
			action:  llmAction(func(a *Action) { a.LLM.Inputs.Related[0].Direction = "both" }),
			wantErr: "direction must be",
		},
		{
			name:    "related unknown field",
			action:  llmAction(func(a *Action) { a.LLM.Inputs.Related[0].Fields = []string{"nope"} }),
			wantErr: `fields: property "nope" is not declared on concept`,
		},
		{
			name:    "no outputs",
			action:  llmAction(func(a *Action) { a.LLM.Outputs = LLMOutputs{} }),
			wantErr: "outputs needs at least one",
		},
		{
			name:    "computed output",
			action:  llmAction(func(a *Action) { a.LLM.Outputs.Set = []string{"score"} }),
			wantErr: `property "score" cannot be proposed: it is computed`,
		},
		{
			name:    "file output",
			action:  llmAction(func(a *Action) { a.LLM.Outputs.Set = []string{"doc"} }),
			wantErr: `property "doc" cannot be proposed: it is a file`,
		},
		{
			name:    "duplicate output",
			action:  llmAction(func(a *Action) { a.LLM.Outputs.Set = []string{"priority", "priority"} }),
			wantErr: "listed twice",
		},
		{
			name:    "relate not from subject",
			action:  llmAction(func(a *Action) { a.LLM.Outputs.Relate[0].Relation = "describes" }),
			wantErr: `"ticket" is not a from type of relation "describes"`,
		},
		{
			name:    "relate faced target",
			action:  llmAction(func(a *Action) { a.LLM.Outputs.Relate[0] = LLMRelateOutput{Relation: "cites"} }),
			wantErr: `target type "document" has content states`,
		},
		{
			name:    "relate required relation property",
			action:  llmAction(func(a *Action) { a.LLM.Outputs.Relate[0] = LLMRelateOutput{Relation: "weighted"} }),
			wantErr: `has required property "w"`,
		},
		{
			name:    "relate max over max_outgoing",
			action:  llmAction(func(a *Action) { a.LLM.Outputs.Relate[0] = LLMRelateOutput{Relation: "owner", Max: 2} }),
			wantErr: "exceeds max_outgoing 1",
		},
		{
			name:    "relate limit too large",
			action:  llmAction(func(a *Action) { a.LLM.Outputs.Relate[0].Limit = MaxLLMCandidateLimit + 1 }),
			wantErr: "limit must be between 0 (the default) and 200",
		},
		{
			name: "limits add up too high",
			action: llmAction(func(a *Action) {
				a.LLM.Outputs.Relate[0].Limit = MaxLLMCandidateLimit
				for range 4 {
					a.LLM.Inputs.Related = append(a.LLM.Inputs.Related, a.LLM.Inputs.Related[0])
				}
				for i := range a.LLM.Inputs.Related {
					a.LLM.Inputs.Related[i].Limit = MaxLLMRelatedLimit
				}
			}),
			wantErr: "add up to 450; at most 400",
		},
		{
			name:    "unknown relation",
			action:  llmAction(func(a *Action) { a.LLM.Outputs.Relate[0].Relation = "nope" }),
			wantErr: `unknown relation "nope"`,
		},
		{
			name:    "bad apply",
			action:  llmAction(func(a *Action) { a.LLM.Apply = "now" }),
			wantErr: `apply must be "propose" (the default) or "auto"`,
		},
		{name: "auto apply", action: llmAction(func(a *Action) { a.LLM.Apply = LLMApplyAuto })},
		{
			name: "create only",
			action: llmAction(func(a *Action) {
				a.LLM.Outputs = LLMOutputs{Create: []LLMCreateOutput{{Type: "concept", Relation: "affects", Properties: []string{"title"}}}}
			}),
		},
		{
			name: "create incoming",
			action: llmAction(func(a *Action) {
				a.LLM.Outputs.Create = []LLMCreateOutput{{Type: "concept", Relation: "describes", Direction: "incoming",
					Properties: []string{"title"}}}
			}),
		},
		{
			name: "create wrong direction",
			action: llmAction(func(a *Action) {
				a.LLM.Outputs.Create = []LLMCreateOutput{{Type: "concept", Relation: "describes", Properties: []string{"title"}}}
			}),
			wantErr: `relation "describes" does not run from "ticket" to "concept"`,
		},
		{
			name: "create and relate one relation",
			action: llmAction(func(a *Action) {
				a.LLM.Outputs.Create = []LLMCreateOutput{{Type: "concept", Relation: "affects", Properties: []string{"title"}}}
			}),
			wantErr: `relation "affects" is also in outputs.relate`,
		},
		{
			name: "create misses a required property",
			meta: func(m *metamodel.Metamodel) {
				def := m.Entities["concept"]
				def.Properties["code"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeString, Required: true}
				m.Entities["concept"] = def
			},
			action: llmAction(func(a *Action) {
				a.LLM.Outputs = LLMOutputs{Create: []LLMCreateOutput{{Type: "concept", Relation: "affects", Properties: []string{"title"}}}}
			}),
			wantErr: `properties must include "code"`,
		},
		{
			name: "create required property with a default",
			meta: func(m *metamodel.Metamodel) {
				def := m.Entities["concept"]
				def.Properties["code"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeString, Required: true, Default: "C"}
				m.Entities["concept"] = def
			},
			action: llmAction(func(a *Action) {
				a.LLM.Outputs = LLMOutputs{Create: []LLMCreateOutput{{Type: "concept", Relation: "affects", Properties: []string{"title"}}}}
			}),
		},
		{
			name: "create without properties",
			action: llmAction(func(a *Action) {
				a.LLM.Outputs.Create = []LLMCreateOutput{{Type: "concept", Relation: "affects"}}
			}),
			wantErr: "properties needs at least one property",
		},
		{
			name: "create faced type",
			action: llmAction(func(a *Action) {
				a.LLM.Outputs.Create = []LLMCreateOutput{{Type: "document", Relation: "cites", Properties: []string{"title"}}}
			}),
			wantErr: `"document" has content states`,
		},
		{
			name: "create max over max_outgoing",
			action: llmAction(func(a *Action) {
				a.LLM.Outputs.Create = []LLMCreateOutput{{Type: "concept", Relation: "owner", Properties: []string{"title"}, Max: 2}}
			}),
			wantErr: "max 2 exceeds max_outgoing 1",
		},
		{
			name: "create computed property",
			action: llmAction(func(a *Action) {
				a.LLM.Outputs.Create = []LLMCreateOutput{{Type: "ticket", Relation: "describes", Direction: "incoming",
					Properties: []string{"score"}}}
			}),
			wantErr: `relation "describes" does not run from "ticket" to "ticket"`,
		},
		{
			name: "create listed twice",
			action: llmAction(func(a *Action) {
				c := LLMCreateOutput{Type: "concept", Relation: "affects", Properties: []string{"title"}}
				a.LLM.Outputs.Create = []LLMCreateOutput{c, c}
			}),
			wantErr: `outputs.create: relation "affects" is listed twice`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Actions: map[string]Action{"triage": tc.action}}
			meta := llmTestMeta()
			if tc.meta != nil {
				tc.meta(meta)
			}
			errs := validateActions(cfg, meta)
			joined := strings.Join(errs, "\n")
			if tc.wantErr == "" {
				if len(errs) > 0 {
					t.Fatalf("expected no errors, got: %s", joined)
				}
				return
			}
			if !strings.Contains(joined, tc.wantErr) {
				t.Fatalf("expected an error containing %q, got: %s", tc.wantErr, joined)
			}
		})
	}
}

func TestLLMUnknownKeys(t *testing.T) {
	data := []byte(`
actions:
  triage:
    llm:
      prompt: x
      output: {}
      inputs:
        propertes: [a]
        content: { max_char: 3 }
        related:
          - { relation: r, limt: 3 }
      outputs:
        relate:
          - { relation: r, candidate: "x" }
        create:
          - { relation: r, typ: x }
  plain:
    script: a.lua
`)
	got := strings.Join(llmUnknownKeys(data), "\n")
	for _, want := range []string{
		`actions.triage.llm: unknown key "output"`,
		`actions.triage.llm.inputs: unknown key "propertes"`,
		`actions.triage.llm.inputs.content: unknown key "max_char"`,
		`actions.triage.llm.inputs.related[0]: unknown key "limt"`,
		`actions.triage.llm.outputs.relate[0]: unknown key "candidate"`,
		`actions.triage.llm.outputs.create[0]: unknown key "typ"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestLLMDefaults(t *testing.T) {
	if got := (LLMContentInput{}).ContentChars(); got != DefaultLLMContentChars {
		t.Errorf("ContentChars = %d", got)
	}
	if got := (LLMRelatedInput{}).EffectiveLimit(); got != DefaultLLMRelatedLimit {
		t.Errorf("related limit = %d", got)
	}
	r := LLMRelateOutput{}
	if r.EffectiveLimit() != DefaultLLMCandidateLimit || r.EffectiveMax() != 1 {
		t.Errorf("relate defaults = %d, %d", r.EffectiveLimit(), r.EffectiveMax())
	}
	if (LLMRelatedInput{Direction: "incoming"}).Outgoing() {
		t.Error("incoming reported outgoing")
	}
	a := llmAction(nil)
	if a.Subject() != "ticket" {
		t.Errorf("Subject = %q", a.Subject())
	}
	if got := LLMCandidateConditionID("t", 2, "concept"); got != "t:relate[2]:concept" {
		t.Errorf("LLMCandidateConditionID = %q", got)
	}
}

func TestValidateListActions_LLM(t *testing.T) {
	tests := []struct {
		name     string
		listType string
		mut      func(a *Action)
		wantErr  string
	}{
		{name: "auto on the list type", listType: "ticket"},
		{
			name: "proposing", listType: "ticket",
			mut:     func(a *Action) { a.LLM.Apply = "" },
			wantErr: "a list runs an llm action only with apply: auto",
		},
		{name: "other type", listType: "concept", wantErr: `runs on "ticket", not on the list's type "concept"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := llmAction(func(a *Action) {
				a.LLM.Apply = LLMApplyAuto
				a.Label, a.Key = "Triage", "t"
				if tc.mut != nil {
					tc.mut(a)
				}
			})
			cfg := &Config{
				Actions: map[string]Action{"triage": a},
				Lists:   map[string]List{"l": {EntityType: tc.listType, Actions: []string{"triage"}}},
			}
			joined := strings.Join(validateActions(cfg, llmTestMeta()), "\n")
			if tc.wantErr == "" {
				if joined != "" {
					t.Fatalf("expected no errors, got: %s", joined)
				}
				return
			}
			if !strings.Contains(joined, tc.wantErr) {
				t.Fatalf("expected an error containing %q, got: %s", tc.wantErr, joined)
			}
		})
	}
}
