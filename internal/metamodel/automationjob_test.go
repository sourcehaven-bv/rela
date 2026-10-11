package metamodel

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidateBackgroundActions(t *testing.T) {
	bg := AutomationAction{LuaFile: "push.lua", Background: true}
	tests := []struct {
		name  string
		autos []AutomationDef
		want  string // substring of the single error; "" means valid
	}{
		{"valid", []AutomationDef{{Name: "a", Do: []AutomationAction{bg}}}, ""},
		{"run_as and retry", []AutomationDef{{Name: "a", Do: []AutomationAction{
			{LuaFile: "p.lua", Background: true, RunAs: "bot", Retry: JobRetryNever}}}}, ""},
		{"no lua_file", []AutomationDef{{Name: "a", Do: []AutomationAction{{Background: true}}}}, "needs `lua_file`"},
		{"inline lua", []AutomationDef{{Name: "a", Do: []AutomationAction{
			{LuaFile: "p.lua", Lua: "x()", Background: true}}}}, "only `lua_file`"},
		{"bypass", []AutomationDef{{Name: "a", Do: []AutomationAction{
			{LuaFile: "p.lua", Background: true, AllowACLBypass: ACLBypassRead}}}}, "allow_acl_bypass"},
		{"run_as without background", []AutomationDef{{Name: "a", Do: []AutomationAction{
			{LuaFile: "p.lua", RunAs: "bot"}}}}, "only with `background: true`"},
		{"bad run_as", []AutomationDef{{Name: "a", Do: []AutomationAction{
			{LuaFile: "p.lua", Background: true, RunAs: " bot"}}}}, "invalid run_as"},
		{"control char run_as", []AutomationDef{{Name: "a", Do: []AutomationAction{
			{LuaFile: "p.lua", Background: true, RunAs: "b\x01ot"}}}}, "invalid run_as"},
		{"unnamed", []AutomationDef{{Do: []AutomationAction{bg}}}, "named automation"},
		{"duplicate name", []AutomationDef{{Name: "a", Do: []AutomationAction{bg}}, {Name: "a"}}, "unique automation name"},
		{"same file twice", []AutomationDef{{Name: "a", Do: []AutomationAction{bg, bg}}}, "twice"},
		{"updated with property", []AutomationDef{{Name: "a", On: AutomationTrigger{Updated: true, Property: "x"}}},
			"`on.updated`"},
		{"updated with becomes", []AutomationDef{{Name: "a", On: AutomationTrigger{Updated: true, Becomes: "x"}}},
			"`on.updated`"},
		{"updated with foreground script", []AutomationDef{{Name: "a", On: AutomationTrigger{Updated: true},
			Do: []AutomationAction{{Lua: "x = 1"}}}}, "must be `background: true`"},
		{"updated with background script", []AutomationDef{{Name: "a", On: AutomationTrigger{Updated: true},
			Do: []AutomationAction{bg}}}, ""},
		{"reserved run_as", []AutomationDef{{Name: "a", Do: []AutomationAction{
			{LuaFile: "p.lua", Background: true, RunAs: "system:scheduler"}}}}, "invalid run_as"},
		{"automation run_as", []AutomationDef{{Name: "a", Do: []AutomationAction{
			{LuaFile: "p.lua", Background: true, RunAs: "system:automation"}}}}, ""},
		{"integration run_as", []AutomationDef{{Name: "a", Do: []AutomationAction{
			{LuaFile: "p.lua", Background: true, RunAs: "integration:basecamp"}}}}, ""},
		{"bad integration run_as", []AutomationDef{{Name: "a", Do: []AutomationAction{
			{LuaFile: "p.lua", Background: true, RunAs: "integration:Base Camp"}}}}, "invalid run_as"},
		{"tokens on a synchronous action", []AutomationDef{{Name: "a", Do: []AutomationAction{
			{LuaFile: "p.lua", Capabilities: Capabilities{Tokens: []string{"basecamp"}}}}}},
			"`capabilities.tokens` applies only with `background: true`"},
		{"tokens on a background action", []AutomationDef{{Name: "a", Do: []AutomationAction{
			{LuaFile: "p.lua", Background: true, Capabilities: Capabilities{Tokens: []string{"basecamp"}}}}}}, ""},
		{"same file, other spelling", []AutomationDef{{Name: "a", Do: []AutomationAction{
			bg, {LuaFile: "./" + bg.LuaFile, Background: true}}}}, "twice"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateBackgroundActions(&Metamodel{Automations: tc.autos})
			if tc.want == "" {
				if len(errs) != 0 {
					t.Fatalf("errors = %v, want none", errs)
				}
				return
			}
			if len(errs) != 1 || !strings.Contains(errs[0], tc.want) {
				t.Fatalf("errors = %v, want one containing %q", errs, tc.want)
			}
		})
	}
}

func TestJobRetry_Unmarshal(t *testing.T) {
	for in, want := range map[string]JobRetry{"never": JobRetryNever, "Bounded": JobRetryBounded, "persistent": JobRetryPersistent} {
		var r JobRetry
		if err := yaml.Unmarshal([]byte(`"`+in+`"`), &r); err != nil || r != want {
			t.Errorf("%q: got %q, %v", in, r, err)
		}
	}
	var r JobRetry
	if err := yaml.Unmarshal([]byte("forever"), &r); err == nil {
		t.Error("unknown retry accepted")
	}
}
