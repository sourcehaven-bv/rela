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
// It runs on the served page, after the row gate, the list condition and
// paging, so it adds no read. Each group costs one batched [matchPage] over
// the page, never a per-row lookup. The conditions judge the redacted rows
// only, as a list condition does: a hidden field binds Nil, so the group a
// row lands in carries no bit of its value. The result is per request and is
// not cached.
//
// A failed evaluation fails the list, as a failed list condition does: a
// list that silently puts every row in the fallback would misreport it.
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
		return true // no compiler wired: leave the rows ungrouped
	}
	// One compile per request, shared by every group.
	lookup, problems := a.viewConditions(s.Cfg, s.Meta)
	if len(problems) > 0 {
		slog.WarnContext(ctx, "dataentry: list groups not compiled; rows left ungrouped", "list", listID)
		return true
	}
	candidates := redactAll(ctx, rows, suggestionRedactor(a.affordances))
	placed := make([]bool, len(rows))
	for _, g := range gb.Groups {
		if g.Condition == "" {
			// The fallback takes every row not yet placed.
			for i := range rows {
				if !placed[i] {
					placed[i], data[i].Group = true, g.ID
				}
			}
			break
		}
		m := viewConditionFor(lookup, viewKindListGroup, dataentryconfig.ListGroupConditionID(listID, g.ID))
		if m == nil {
			// coverage-ignore: invariant: validation compiled every group condition
			slog.WarnContext(ctx, "dataentry: list group has no compiled condition; rows left ungrouped",
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
