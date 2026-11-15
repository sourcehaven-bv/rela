package dataentryconfig

import (
	"encoding/json"
	"strings"
	"testing"
)

func batchLLMAction(mut func(a *Action)) Action {
	return llmAction(func(a *Action) {
		a.Label = "Triage"
		a.LLM.Inputs.Related = nil
		a.LLM.Outputs.Relate = nil
		a.LLM.Batch = &LLMBatch{}
		if mut != nil {
			mut(a)
		}
	})
}

func TestValidateActions_LLMBatch(t *testing.T) {
	tests := []struct {
		name    string
		action  Action
		wantErr string
	}{
		{name: "valid", action: batchLLMAction(nil)},
		{name: "max at limit", action: batchLLMAction(func(a *Action) {
			a.LLM.Batch.Max = MaxLLMBatchMax
			a.LLM.Inputs.Content.MaxChars = MaxLLMBatchContentChars / MaxLLMBatchMax
		})},
		{
			name:    "max too large",
			action:  batchLLMAction(func(a *Action) { a.LLM.Batch.Max = MaxLLMBatchMax + 1 }),
			wantErr: "max must be between",
		},
		{
			name:    "negative max",
			action:  batchLLMAction(func(a *Action) { a.LLM.Batch.Max = -1 }),
			wantErr: "max must be between",
		},
		{
			name: "related input",
			action: batchLLMAction(func(a *Action) {
				a.LLM.Inputs.Related = []LLMRelatedInput{{Relation: "describes", Direction: "incoming"}}
			}),
			wantErr: "a batch action cannot have inputs.related",
		},
		{
			name: "relate output",
			action: batchLLMAction(func(a *Action) {
				a.LLM.Outputs.Relate = []LLMRelateOutput{{Relation: "affects"}}
			}),
			wantErr: "a batch action cannot have outputs.relate",
		},
		{
			name: "content budget",
			action: batchLLMAction(func(a *Action) {
				a.LLM.Batch.Max = MaxLLMBatchMax
				a.LLM.Inputs.Content.MaxChars = MaxLLMContentChars
			}),
			wantErr: "lower one of them",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Actions: map[string]Action{"triage": tc.action}}
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

func TestValidateKanbanActions(t *testing.T) {
	cfg := &Config{Actions: map[string]Action{
		"batch":   batchLLMAction(nil),
		"single":  llmAction(func(a *Action) { a.Label = "One" }),
		"nolabel": batchLLMAction(func(a *Action) { a.Label = "" }),
		"script":  {Label: "S", Script: "s.lua"},
	}}
	tests := []struct {
		name    string
		kanban  Kanban
		wantErr string
	}{
		{name: "batch action", kanban: Kanban{EntityType: "ticket", Actions: []string{"batch"}}},
		{name: "unknown", kanban: Kanban{EntityType: "ticket", Actions: []string{"nope"}}, wantErr: "unknown action"},
		{name: "twice", kanban: Kanban{EntityType: "ticket", Actions: []string{"batch", "batch"}}, wantErr: "listed twice"},
		{name: "not batch", kanban: Kanban{EntityType: "ticket", Actions: []string{"single"}}, wantErr: "with a batch: block"},
		{name: "script", kanban: Kanban{EntityType: "ticket", Actions: []string{"script"}}, wantErr: "with a batch: block"},
		{name: "other type", kanban: Kanban{EntityType: "concept", Actions: []string{"batch"}}, wantErr: `runs on "ticket", not on "concept"`},
		{name: "no label", kanban: Kanban{EntityType: "ticket", Actions: []string{"nolabel"}}, wantErr: "needs a label"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			joined := strings.Join(validateKanbanActions("board", tc.kanban, cfg), "\n")
			if tc.wantErr == "" && joined != "" {
				t.Fatalf("unexpected errors: %s", joined)
			}
			if !strings.Contains(joined, tc.wantErr) {
				t.Fatalf("expected %q, got: %s", tc.wantErr, joined)
			}
		})
	}
}

func TestLLMBatch_JSON(t *testing.T) {
	a := batchLLMAction(nil)
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	llm, ok := got["llm"].(map[string]any)
	if !ok || len(llm) != 1 {
		t.Fatalf("llm = %v, want only batch", got["llm"])
	}
	if b := llm["batch"].(map[string]any); b["max"] != float64(DefaultLLMBatchMax) {
		t.Errorf("batch = %v", b)
	}
	if strings.Contains(string(raw), "Triage the ticket.") {
		t.Errorf("prompt serialized: %s", raw)
	}
	unknown := llmUnknownKeys([]byte("actions:\n  t:\n    llm:\n      batch: { mx: 3 }\n"))
	if len(unknown) != 1 || !strings.Contains(unknown[0], `llm.batch: unknown key "mx"`) {
		t.Errorf("unknown batch key: %v", unknown)
	}
}

func TestValidateLists_BatchAction(t *testing.T) {
	actions := map[string]Action{
		"batch":  batchLLMAction(nil),
		"single": llmAction(func(a *Action) { a.Label = "One" }),
	}
	tests := []struct {
		name, entityType, action, wantErr string
	}{
		{name: "batch action", entityType: "ticket", action: "batch"},
		{name: "single-row llm action", entityType: "ticket", action: "single", wantErr: `"single"`},
		{name: "other type", entityType: "concept", action: "batch", wantErr: `runs on "ticket", not on "concept"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Actions: actions, Lists: map[string]List{
				"l": {EntityType: tc.entityType, Actions: []string{tc.action}},
			}}
			var errs []string
			for _, e := range validateActions(cfg, llmTestMeta()) {
				if strings.Contains(e, `list "l"`) && !strings.Contains(e, "key") {
					errs = append(errs, e)
				}
			}
			joined := strings.Join(errs, "\n")
			if tc.wantErr == "" && joined != "" {
				t.Fatalf("unexpected errors: %s", joined)
			}
			if !strings.Contains(joined, tc.wantErr) {
				t.Fatalf("expected %q, got: %s", tc.wantErr, joined)
			}
		})
	}
}
