package automation

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// NotifyAction is a compiled `notify` action (see metamodel.NotifyAction).
type NotifyAction struct {
	// Pile is the declared pile key.
	Pile string
	// Reason is a key of the pile's reasons, or empty.
	Reason string
	// Via is the relation path as written; Type picks the last hop's type.
	Via  []string
	Type string
	// Property holds the recipient on the entity the path reaches.
	Property string
	// hops is Via resolved for each trigger entity type, filled at load.
	hops map[string][]NotifyHop
}

// NotifyHop is one resolved hop of a notify path: the canonical relation and
// whether it is walked backwards (TO -> FROM).
type NotifyHop struct {
	Relation string
	Incoming bool
	// Target is the entity type the hop lands on. Only entities of the
	// last hop's Target are recipients' holders.
	Target string
}

// NotificationToSend is one planned `notify` action. The cascade finds the
// recipients, then pushes the triggering entity onto each one's declared
// pile; nothing is written while planning.
type NotificationToSend struct {
	// Pile is the declared pile key; Reason a key of its reasons, or empty.
	Pile   string
	Reason string
	// Hops lead from the triggering entity to the entities that hold the
	// recipient in Property. With no hops, Owners already holds the values
	// of Property on the triggering entity.
	Hops     []NotifyHop
	Property string
	Owners   []string
	// AutomationName is the originating automation, for the log line.
	AutomationName string
}

// resolveNotifyPaths resolves every notify path for each type its automation
// triggers on, and checks that the recipient property exists where the path
// ends. A path that cannot resolve is a load error: a notification that can
// never reach anyone must not load silently.
func resolveNotifyPaths(meta *metamodel.Metamodel, automations []Automation) error {
	if meta == nil {
		return nil
	}
	for ai := range automations {
		auto := &automations[ai]
		for di := range auto.Do {
			n := auto.Do[di].Notify
			if n == nil {
				continue
			}
			n.hops = make(map[string][]NotifyHop, len(auto.On.Entity))
			for _, from := range auto.On.Entity {
				target, hops, err := resolveNotifyPath(meta, from, n)
				if err != nil {
					return fmt.Errorf("automation %q: notify from %q: %w", auto.Name, from, err)
				}
				if def, ok := meta.GetEntityDef(target); ok {
					if _, has := def.Properties[n.Property]; !has {
						return fmt.Errorf("automation %q: notify: %q has no property %q to hold the recipient",
							auto.Name, target, n.Property)
					}
				}
				n.hops[from] = hops
			}
		}
	}
	return nil
}

func resolveNotifyPath(meta *metamodel.Metamodel, from string, n *NotifyAction) (string, []NotifyHop, error) {
	if len(n.Via) == 0 {
		return from, nil, nil
	}
	resolved, err := predicatefns.ResolveTraversal(meta, from, predicate.TraversalSpec{
		Subject: "entity", Path: n.Via, EntityType: n.Type,
	})
	if err != nil {
		return "", nil, err
	}
	hops := make([]NotifyHop, len(resolved))
	for i, h := range resolved {
		hops[i] = NotifyHop{Relation: h.Relation, Incoming: h.Incoming, Target: h.Target}
	}
	return resolved[len(resolved)-1].Target, hops, nil
}

// planNotify appends a notify action to result.Notifications. A path from a
// type the load did not resolve (an automation without `on.entity:` and a
// `via:`, which validation refuses) is skipped.
func (e *Engine) planNotify(n NotifyAction, event Event, result *Result, automationName string) {
	planned := NotificationToSend{
		Pile: n.Pile, Reason: n.Reason, Property: n.Property, AutomationName: automationName,
	}
	if len(n.Via) == 0 {
		planned.Owners = PropertyOwners(event.Entity.Properties[n.Property])
		if len(planned.Owners) == 0 {
			return
		}
	} else {
		hops, ok := n.hops[event.Entity.Type]
		if !ok {
			return
		}
		planned.Hops = hops
	}
	result.Notifications = append(result.Notifications, planned)
}

// PropertyOwners reads the recipients from a property value: a string or a
// list of strings, trimmed, empty values and duplicates dropped.
func PropertyOwners(v any) []string {
	var raw []string
	switch t := v.(type) {
	case string:
		raw = []string{t}
	case []string:
		raw = t
	case []any:
		for _, x := range t {
			if s, ok := x.(string); ok {
				raw = append(raw, s)
			}
		}
	}
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s != "" && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}
