package metamodel

import (
	"fmt"
	"slices"
)

// validateAutomationCopies checks every automation's `on.copy:` scope. Each
// name must be a declared copy, and the copy must be able to raise the
// trigger: it writes a type and face the trigger accepts, and the trigger
// waits for an entity event rather than a relation event. Each failure is a
// load error, because a scope that can never match makes the automation dead
// without a word.
func validateAutomationCopies(m *Metamodel) []string {
	var errs []string
	for _, auto := range m.Automations {
		if len(auto.On.Copy) == 0 {
			continue
		}
		bad := func(format string, args ...any) {
			errs = append(errs, fmt.Sprintf("automation %q: "+format, append([]any{auto.Name}, args...)...))
		}
		if auto.On.RelationCreated != "" || auto.On.RelationRemoved != "" {
			bad("`on.copy` cannot be combined with `relation_created` or `relation_removed`; " +
				"a copy raises only entity events")
		}
		for _, name := range auto.On.Copy {
			def, ok := m.Copies[name]
			if !ok {
				bad("`on.copy: [%s]` names no declared copy", name)
				continue
			}
			to, err := ParseCopyTarget(def.To)
			if err != nil {
				continue // validateCopies reports the target
			}
			if len(auto.On.Entity) > 0 && !slices.Contains([]string(auto.On.Entity), to.Type) {
				bad("`on.copy: [%s]` writes %q, which `on.entity` does not name; the trigger would never fire",
					name, to.Type)
			}
			if len(auto.On.Faces) > 0 && !slices.Contains([]string(auto.On.Faces), to.Face) {
				bad("`on.copy: [%s]` writes face %q, which `on.faces` does not name; the trigger would never fire",
					name, to.Face)
			}
		}
	}
	return errs
}
