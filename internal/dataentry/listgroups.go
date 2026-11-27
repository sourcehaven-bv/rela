package dataentry

import (
	"log/slog"
	"net/http"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
)

// viewKindListGroup is a computed group's `condition:` (TKT-QS024L0), keyed
// by dataentryconfig.ListGroupConditionID.
const viewKindListGroup = "list_groups"

// serveListGroups stamps `_group` on each row of a list whose group_by is
// computed: the id of the first group whose condition the row matches. data
// is parallel to rows.
//
// It runs on the served, redacted page: one batched [matchPage] per group,
// never cached. A failed evaluation fails the list rather than misreport it.
// A group whose condition did not compile leaves every row ungrouped; the
// compile problem is logged once per config load.
func serveListGroups(
	w http.ResponseWriter, r *http.Request, a *App, query map[string][]string,
	rows []*entityPkg.Entity, data []v1.Entity,
) bool {
	listID := queryGet(query, listIDParam)
	s := a.State()
	if listID == "" || s == nil || s.Cfg == nil || len(rows) == 0 {
		return true
	}
	gb := s.Cfg.Lists[listID].GroupBy
	if !gb.Computed() {
		return true
	}
	ctx := r.Context()
	if a.viewConditions == nil {
		return true
	}
	lookup, _ := a.viewConditions(s.Cfg, s.Meta)
	candidates := redactAll(ctx, rows, suggestionRedactor(a.affordances))
	placed := make([]bool, len(rows))
	for _, g := range gb.Groups {
		if g.Condition == "" {
			placeUnplaced(placed, data, g.ID)
			break
		}
		m := viewConditionFor(lookup, viewKindListGroup, dataentryconfig.ListGroupConditionID(listID, g.ID))
		if m == nil {
			slog.DebugContext(ctx, "dataentry: list group condition does not compile; rows left ungrouped",
				"list", listID, "group", g.ID)
			clearGroups(data)
			return true
		}
		verdicts, err := matchPage(ctx, candidates, m, a.Services().Store)
		if err != nil {
			slog.ErrorContext(ctx, "dataentry: evaluating list group", "list", listID, "group", g.ID, "err", err)
			writeJSONError(w, http.StatusInternalServerError, "evaluating list groups failed")
			return false
		}
		for i, ok := range verdicts {
			if ok && !placed[i] {
				placed[i], data[i].Group = true, g.ID
			}
		}
	}
	return true
}

func clearGroups(data []v1.Entity) {
	for i := range data {
		data[i].Group = ""
	}
}

// placeUnplaced assigns groupID to every row not yet placed.
func placeUnplaced(placed []bool, data []v1.Entity, groupID string) {
	for i := range data {
		if !placed[i] {
			placed[i], data[i].Group = true, groupID
		}
	}
}
