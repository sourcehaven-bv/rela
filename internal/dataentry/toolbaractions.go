package dataentry

import (
	"context"
)

// addToolbarActions adds the `action:<id>` keys of a list's toolbar buttons
// to out, the collection `_actions` map of entityType: one per action in the
// `toolbar_actions:` of a list over entityType whose `permission:` the
// principal holds. It returns out, allocating it when nil.
//
// Like the detail-page keys (detailActionKeyPrefix), a key is only ever
// emitted as true and absence means "not offered". The verdict is the one
// holdsActionPermission gives the POST /_action/{id} gate, so a rendered
// button and the enforced check cannot drift. It is still a UI hint: the
// action endpoint re-checks the permission on every run.
//
// The keys are per entity type, not per list, because the collection
// endpoint does not know which list asked. Two lists over one type therefore
// see each other's verdicts, which is harmless: each list renders only the
// ids in its own toolbar_actions.
//
// Batch `llm:` actions (TKT-I2SRNWR) named in a list's `actions:` or a
// kanban's `actions:` over entityType get a key the same way. Their gate
// also judges each row's available_on and when, but those are per row: the
// key says only that the caller may start a run.
//
// Nil cfg: accepted, adds nothing.
func addToolbarActions(
	ctx context.Context, cfg *Config, entityType string, out map[string]bool,
) map[string]bool {
	if cfg == nil {
		return out
	}
	add := func(id string) {
		if out == nil {
			out = map[string]bool{}
		}
		out[detailActionKeyPrefix+id] = true
	}
	addBatch := func(ids []string) {
		for _, id := range ids {
			action, ok := cfg.Actions[id]
			if ok && action.LLM != nil && action.LLM.Batch != nil && holdsActionPermission(ctx, action) {
				add(id)
			}
		}
	}
	for _, kanban := range cfg.Kanbans {
		if kanban.EntityType == entityType {
			addBatch(kanban.Actions)
		}
	}
	for _, list := range cfg.Lists {
		if list.EntityType != entityType {
			continue
		}
		addBatch(list.Actions)
		for _, id := range list.ToolbarActions {
			action, ok := cfg.Actions[id]
			// An entity-bound action is refused at load; skipping it here
			// keeps a hand-built config from offering a button the gate refuses.
			if !ok || action.AvailableOn != nil || !holdsActionPermission(ctx, action) {
				continue
			}
			add(id)
		}
	}
	return out
}
