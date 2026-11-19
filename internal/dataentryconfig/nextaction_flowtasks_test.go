package dataentryconfig

import (
	"strings"
	"testing"
)

func TestValidateNextActions_FlowTasks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		src     NextActionSource
		wantErr string
	}{
		{name: "valid", src: NextActionSource{Source: "flow_tasks", Flows: []string{"signup"}}},
		{name: "valid without flows", src: NextActionSource{Source: "flow_tasks", Cooldown: "1h"}},
		{name: "unknown source", src: NextActionSource{Source: "tasks"}, wantErr: `unknown source "tasks"`},
		{
			name:    "source and query",
			src:     NextActionSource{Source: "flow_tasks", Query: "type:x"},
			wantErr: "query and source are mutually exclusive",
		},
		{name: "flows without source", src: NextActionSource{Query: "type:quip", Flows: []string{"a"}},
			wantErr: "flows only applies to a flow_tasks source"},
		{name: "empty flow", src: NextActionSource{Source: "flow_tasks", Flows: []string{""}},
			wantErr: "flows[0] is empty"},
		{name: "duplicate flow", src: NextActionSource{Source: "flow_tasks", Flows: []string{"a", "a"}},
			wantErr: `flows[1]: duplicate flow "a"`},
		{name: "condition", src: NextActionSource{Source: "flow_tasks", Condition: "true"},
			wantErr: "condition does not apply to a flow_tasks source"},
		{name: "key_props", src: NextActionSource{Source: "flow_tasks", KeyProps: []string{"status"}},
			wantErr: "key_props does not apply"},
		{name: "source_world", src: NextActionSource{Source: "flow_tasks", SourceWorld: "default"},
			wantErr: "source_world does not apply"},
		{name: "pick", src: NextActionSource{Source: "flow_tasks", Pick: PickFirst},
			wantErr: "pick does not apply"},
		{
			name: "pick_one",
			src: NextActionSource{Source: "flow_tasks", Actions: []NextActionOffer{
				{PickOne: &NextActionPickOne{Query: "type:quip", Action: "mark-done"}},
			}},
			wantErr: "actions[0]: pick_one does not apply",
		},
		{
			name: "set",
			src: NextActionSource{Source: "flow_tasks", Actions: []NextActionOffer{
				{Snooze: []string{"1d"}}, {Set: map[string]string{"status": "done"}},
			}},
			wantErr: "actions[1]: set does not apply",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := validNextActionConfig()
			tc.src.Band = "blocking"
			tc.src.Suggest = "Complete {step} for {subject}"
			cfg.NextActions["tasks"] = tc.src
			errs := validateNextActions(cfg, nil)
			joined := strings.Join(errs, "\n")
			if tc.wantErr == "" {
				if len(errs) > 0 {
					t.Fatalf("unexpected errors: %v", errs)
				}
				return
			}
			if !strings.Contains(joined, tc.wantErr) {
				t.Fatalf("want an error containing %q, got:\n%s", tc.wantErr, joined)
			}
		})
	}
}

func TestNextActionSource_IsFlowTasks(t *testing.T) {
	t.Parallel()
	if !(NextActionSource{Source: NextActionSourceFlowTasks}).IsFlowTasks() {
		t.Fatal("flow_tasks source not recognized")
	}
	if (NextActionSource{Query: "type:x"}).IsFlowTasks() {
		t.Fatal("query source read as flow_tasks")
	}
}
