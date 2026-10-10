package metamodel

import (
	"fmt"
	"strings"
)

// validateAddToPile checks every `add_to_pile` automation action at load: the
// pile name is required, and the action stands alone in its list entry. A
// list entry that also sets, creates or runs a script would read as one step
// while doing two, so it is refused rather than silently doing both.
func validateAddToPile(m *Metamodel) []string {
	var errs []string
	for _, auto := range m.Automations {
		for i, act := range auto.Do {
			if act.AddToPile == nil {
				continue
			}
			if strings.TrimSpace(act.AddToPile.Pile) == "" {
				errs = append(errs, fmt.Sprintf(
					"automation %q: do[%d]: add_to_pile requires `pile:` (the pile name)", auto.Name, i))
			}
			if others := otherActionKinds(act); len(others) > 0 {
				errs = append(errs, fmt.Sprintf(
					"automation %q: do[%d]: add_to_pile cannot share a list entry with %s; "+
						"give each action its own `-` entry", auto.Name, i, strings.Join(others, ", ")))
			}
		}
	}
	return errs
}

// otherActionKinds names the action keys other than add_to_pile that act is
// carrying.
func otherActionKinds(act AutomationAction) []string {
	var kinds []string
	if act.Set != "" {
		kinds = append(kinds, "set")
	}
	if act.CreateRelation != nil {
		kinds = append(kinds, "create_relation")
	}
	if act.CreateEntity != nil {
		kinds = append(kinds, "create_entity")
	}
	if act.Lua != "" {
		kinds = append(kinds, "lua")
	}
	if act.LuaFile != "" {
		kinds = append(kinds, "lua_file")
	}
	return kinds
}
