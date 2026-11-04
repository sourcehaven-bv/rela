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
			others := otherActionKinds(act)
			if act.Notify != nil {
				others = append(others, "notify")
			}
			if len(others) > 0 {
				errs = append(errs, fmt.Sprintf(
					"automation %q: do[%d]: add_to_pile cannot share a list entry with %s; "+
						"give each action its own `-` entry", auto.Name, i, strings.Join(others, ", ")))
			}
		}
	}
	return errs
}

// validateNotify checks every `notify` automation action at load: the pile
// is a declared pile with read state, the reason is one of its reasons, the
// recipient property is named, and the action stands alone in its list
// entry. The relation path is resolved by the automation engine, which owns
// the traversal rules.
func validateNotify(m *Metamodel) []string {
	var errs []string
	for _, auto := range m.Automations {
		for i, act := range auto.Do {
			n := act.Notify
			if n == nil {
				continue
			}
			at := fmt.Sprintf("automation %q: do[%d]: notify", auto.Name, i)
			errs = append(errs, notifyErrors(m, n, at, len(auto.On.Entity) > 0)...)
			others := otherActionKinds(act)
			if act.AddToPile != nil {
				others = append(others, "add_to_pile")
			}
			if len(others) > 0 {
				errs = append(errs, fmt.Sprintf(
					"%s cannot share a list entry with %s; give each action its own `-` entry",
					at, strings.Join(others, ", ")))
			}
		}
	}
	return errs
}

// notifyErrors checks one notify action's pile, reason and recipient.
func notifyErrors(m *Metamodel, n *NotifyAction, at string, hasOnEntity bool) []string {
	var errs []string
	decl, ok := m.Piles[n.Pile]
	switch {
	case strings.TrimSpace(n.Pile) == "":
		errs = append(errs, at+" requires `pile:` (a key of the top-level `piles:`)")
	case !ok:
		errs = append(errs, fmt.Sprintf("%s: pile %q is not declared under the top-level `piles:`", at, n.Pile))
	case !decl.ReadState:
		errs = append(errs, fmt.Sprintf("%s: pile %q needs `read_state: true` to take notifications", at, n.Pile))
	}
	if ok && n.Reason != "" {
		if _, known := decl.Reasons[n.Reason]; !known {
			errs = append(errs, fmt.Sprintf("%s: reason %q is not one of pile %q's reasons", at, n.Reason, n.Pile))
		}
	}
	if strings.TrimSpace(n.To.Property) == "" {
		errs = append(errs, at+" requires `to.property:` (the property that holds the recipient)")
	}
	if len(n.To.Via) > 0 && !hasOnEntity {
		errs = append(errs, at+": `to.via:` needs the automation's `on.entity:` to resolve the path")
	}
	if n.To.Type != "" && len(n.To.Via) == 0 {
		errs = append(errs, at+": `to.type:` applies only with `to.via:`")
	}
	return errs
}

// otherActionKinds names the action keys other than add_to_pile and notify
// that act is carrying.
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
