package dataentry

import (
	"context"
)

// addToolbarActions adds the `action:<id>` keys of the toolbar buttons, and of
// batch `llm:` actions, of lists and kanbans over entityType whose `permission:`
// the principal holds to out, and returns out, allocating it when nil. A key is
// only ever true; absence means not offered. The verdict is the one the POST
// /_action/{id} gate gives, but it is a UI hint: the endpoint re-checks. Keys
// are per entity type, so lists over one type share verdicts.
//
// Nil cfg: accepted, adds nothing.
func addToolbarActions(
	ctx context.Context, cfg *Config, entityType string, out map[string]bool,
) map[string]bool {
	if cfg == nil {
		return out
	}
	if _, past := asOfFromContext(ctx); past {
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
			if !ok || action.AvailableOn != nil || !holdsActionPermission(ctx, action) {
				continue
			}
			add(id)
		}
	}
	return out
}
