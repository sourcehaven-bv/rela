package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func actionElevationMeta() *metamodel.Metamodel {
	m := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"processing": {Aliases: []string{"proc"}, Properties: map[string]metamodel.PropertyDef{
			"status": {Type: "string"}, "submitted_on": {Type: "date"},
			"code":  {Type: "string", Unique: true},
			"label": {Type: "string", Computed: "status"},
			"scan":  {Type: metamodel.PropertyTypeFile},
		}},
	}}
	m.InitAliases()
	return m
}

// TestValidateActions_Elevation pins the load-time rules for
// allow_acl_bypass and elevated_updates on an action (TKT-JA5SABG).
func TestValidateActions_Elevation(t *testing.T) {
	scope := map[string][]string{"processing": {"status", "submitted_on"}}
	tests := []struct {
		name    string
		action  Action
		wantErr string
	}{
		{
			name: "valid",
			action: Action{Script: "s.lua", Permission: "submit",
				AllowACLBypass: metamodel.ACLBypassWrite, ElevatedUpdates: scope},
		},
		{
			name: "read is refused",
			action: Action{Script: "s.lua", Permission: "submit",
				AllowACLBypass: metamodel.ACLBypassRead, ElevatedUpdates: scope},
			wantErr: `allow_acl_bypass must be "write"`,
		},
		{
			name: "read+write is refused",
			action: Action{Script: "s.lua", Permission: "submit",
				AllowACLBypass: metamodel.ACLBypassReadWrite, ElevatedUpdates: scope},
			wantErr: `allow_acl_bypass must be "write"`,
		},
		{
			name: "needs a permission",
			action: Action{Script: "s.lua",
				AllowACLBypass: metamodel.ACLBypassWrite, ElevatedUpdates: scope},
			wantErr: "permission is required",
		},
		{
			name: "needs a script",
			action: Action{Set: map[string]string{"status": "x"}, Permission: "submit",
				AllowACLBypass: metamodel.ACLBypassWrite, ElevatedUpdates: scope},
			wantErr: "applies only to a script action",
		},
		{
			name:    "needs elevated_updates",
			action:  Action{Script: "s.lua", Permission: "submit", AllowACLBypass: metamodel.ACLBypassWrite},
			wantErr: "needs elevated_updates",
		},
		{
			name: "unknown type",
			action: Action{Script: "s.lua", Permission: "submit", AllowACLBypass: metamodel.ACLBypassWrite,
				ElevatedUpdates: map[string][]string{"nope": {"status"}}},
			wantErr: `unknown entity type "nope"`,
		},
		{
			name: "undeclared property",
			action: Action{Script: "s.lua", Permission: "submit", AllowACLBypass: metamodel.ACLBypassWrite,
				ElevatedUpdates: map[string][]string{"processing": {"owner"}}},
			wantErr: `property "owner" is not declared on entity type "processing"`,
		},
		{
			name: "alias key",
			action: Action{Script: "s.lua", Permission: "submit", AllowACLBypass: metamodel.ACLBypassWrite,
				ElevatedUpdates: map[string][]string{"proc": {"status"}}},
			wantErr: `uses alias "proc"; name the entity type "processing"`,
		},
		{
			name: "unique property",
			action: Action{Script: "s.lua", Permission: "submit", AllowACLBypass: metamodel.ACLBypassWrite,
				ElevatedUpdates: map[string][]string{"processing": {"code"}}},
			wantErr: `property "code" of "processing" is unique`,
		},
		{
			name: "computed property",
			action: Action{Script: "s.lua", Permission: "submit", AllowACLBypass: metamodel.ACLBypassWrite,
				ElevatedUpdates: map[string][]string{"processing": {"label"}}},
			wantErr: `property "label" of "processing" cannot be written`,
		},
		{
			name: "file property",
			action: Action{Script: "s.lua", Permission: "submit", AllowACLBypass: metamodel.ACLBypassWrite,
				ElevatedUpdates: map[string][]string{"processing": {"scan"}}},
			wantErr: `property "scan" of "processing" cannot be written`,
		},
		{
			name: "type without properties",
			action: Action{Script: "s.lua", Permission: "submit", AllowACLBypass: metamodel.ACLBypassWrite,
				ElevatedUpdates: map[string][]string{"processing": {}}},
			wantErr: `lists no properties for entity type "processing"`,
		},
		{
			name:    "elevated_updates without allow_acl_bypass",
			action:  Action{Script: "s.lua", Permission: "submit", ElevatedUpdates: scope},
			wantErr: "has elevated_updates but no allow_acl_bypass",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Actions: map[string]Action{"submit": tc.action}}
			joined := strings.Join(validateActions(cfg, actionElevationMeta()), "\n")
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

// TestAction_ElevationYAML pins the YAML keys of the two fields.
func TestAction_ElevationYAML(t *testing.T) {
	var a Action
	src := "script: s.lua\nallow_acl_bypass: write\nelevated_updates:\n  processing: [status]\n"
	if err := yaml.Unmarshal([]byte(src), &a); err != nil {
		t.Fatal(err)
	}
	if a.AllowACLBypass != metamodel.ACLBypassWrite || len(a.ElevatedUpdates["processing"]) != 1 {
		t.Errorf("decoded %+v", a)
	}
}
