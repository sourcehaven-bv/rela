package dataentryconfig

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestValidateActions_Style pins the values `style:` accepts, and that
// style and order need available_on.
func TestValidateActions_Style(t *testing.T) {
	onNote := &ActionScope{EntityTypes: []string{"note"}}
	tests := []struct {
		name    string
		action  Action
		wantErr string
	}{
		{name: "absent", action: Action{Script: "r.lua"}},
		{name: "danger", action: Action{Script: "r.lua", Style: ActionStyleDanger, AvailableOn: onNote}},
		{name: "order", action: Action{Script: "r.lua", Order: 10, AvailableOn: onNote}},
		{
			name:    "unknown",
			action:  Action{Script: "r.lua", Style: "primary", AvailableOn: onNote},
			wantErr: `actions: "archive" style "primary" is not known`,
		},
		{
			name:    "wrong case",
			action:  Action{Script: "r.lua", Style: "Danger", AvailableOn: onNote},
			wantErr: `style "Danger" is not known`,
		},
		{
			name:    "style without available_on",
			action:  Action{Script: "r.lua", Style: ActionStyleDanger},
			wantErr: `actions: "archive" has style or order but no available_on`,
		},
		{
			name:    "order without available_on",
			action:  Action{Script: "r.lua", Order: 10},
			wantErr: `has style or order but no available_on`,
		},
		{
			name: "checked on an llm action too",
			action: Action{Style: "loud", LLM: &LLMAction{}, Permission: "x",
				AvailableOn: &ActionScope{EntityTypes: []string{"note"}}},
			wantErr: `style "loud" is not known`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Actions: map[string]Action{"archive": tc.action}}
			joined := strings.Join(validateActions(cfg, actionScopeMeta()), "\n")
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

// TestAction_StyleAndOrderServed pins that `style:` and `order:` decode from
// YAML and reach the SPA, which sorts and draws the entity page's actions.
func TestAction_StyleAndOrderServed(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantJSON string
	}{
		{name: "absent", yaml: "script: a.lua", wantJSON: `{"script":"a.lua"}`},
		{name: "both", yaml: "script: a.lua\nstyle: danger\norder: 90",
			wantJSON: `{"script":"a.lua","style":"danger","order":90}`},
		{name: "negative order", yaml: "script: a.lua\norder: -1",
			wantJSON: `{"script":"a.lua","order":-1}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var a Action
			if err := yaml.Unmarshal([]byte(tc.yaml), &a); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			got, err := json.Marshal(a)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tc.wantJSON {
				t.Errorf("json = %s, want %s", got, tc.wantJSON)
			}
		})
	}
}

// TestAction_OrderRejectsFraction pins that a fractional or out-of-range
// order is a decode error naming its line, rather than a silent truncation.
func TestAction_OrderRejectsFraction(t *testing.T) {
	tests := []struct {
		name    string
		order   string
		wantErr string
	}{
		{name: "fraction", order: "1.5", wantErr: "order must be a whole number, got 1.5 (line 2)"},
		{name: "huge", order: "1e300", wantErr: "order must be a whole number"},
		{name: "not a number", order: "first", wantErr: "cannot unmarshal"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var a Action
			err := yaml.Unmarshal([]byte("script: a.lua\norder: "+tc.order), &a)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
}
