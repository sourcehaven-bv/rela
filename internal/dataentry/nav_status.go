package dataentry

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
)

// viewKindNavStatus keys a navigation status rule's compiled condition in the
// view-condition lookup, matching conditionlint.ViewConditionNavStatus. The
// id is dataentryconfig.NavStatusConditionID.
const viewKindNavStatus = "nav_status"

// navStatusCountVar is the one value a status label may interpolate. Nothing
// else is, so a label can never carry entity content.
const navStatusCountVar = "{count}"

// handleV1NavStatus serves GET /api/v1/_nav_status: the status each sidebar
// entry flags for this principal, keyed by the entry's status key. With
// `spaces:` configured, `?space=<id>` limits it to the entries of the space
// the sidebar resolves for the same id.
//
// Served apart from /api/v1/_sidebar on purpose. The sidebar is the same for
// every principal (docs/acl-security.md, "Sidebar menu structure is
// principal-independent"); a count is about data, so it depends on who asks.
// Keeping the counts here keeps that property of the sidebar true.
//
// A package function taking the App rather than a method: App is at its
// plimsoll load line, and this needs only what it names.
func handleV1NavStatus(a *App, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	ctx := r.Context()
	s := a.State()
	aclImpl := a.views.currentACL()

	items := make(map[string]v1.NavStatus)
	// The same filter the sidebar applies: an entry this principal does not
	// see gets no status either.
	for _, ns := range sidebarNavStatusEntries(ctx, aclImpl, s.Cfg, r.URL.Query().Get("space")) {
		status, ok, err := navStatusFor(ctx, a, s, ns)
		if err != nil {
			// One entry that cannot be counted must not blank every other
			// marker, and a missing marker is the honest answer for it.
			slog.WarnContext(ctx, "nav status: entry skipped",
				"entry", ns.Entry.Label, "list", ns.List, "error", err)
			continue
		}
		if ok {
			items[ns.Key] = status
		}
	}

	// Per principal; writeV1JSON already marks every v1 response no-store,
	// which is what keeps a shared cache from holding one principal's counts.
	writeV1JSON(w, http.StatusOK, v1.NavStatusResponse{Items: items})
}

// navStatusFor evaluates one entry's rules in order and returns the first
// whose count is above zero, or ok=false when none is.
//
// The list's rows are read ONCE and every rule counts over that slice, so a
// second rule costs an evaluation per row, not another read.
func navStatusFor(
	ctx context.Context, a *App, s *Schema, ns dataentryconfig.NavStatusEntry,
) (status v1.NavStatus, ok bool, err error) {
	listCfg, found := s.Cfg.Lists[ns.List]
	if !found {
		return v1.NavStatus{}, false, fmt.Errorf("unknown list %q", ns.List)
	}
	ctx, rows, err := navStatusRows(ctx, a, s, ns.List, listCfg)
	if err != nil {
		return v1.NavStatus{}, false, err
	}
	if len(rows) == 0 {
		return v1.NavStatus{}, false, nil
	}
	for i, rule := range ns.Entry.Status {
		m := viewCondition(a.viewConditions, s, viewKindNavStatus,
			dataentryconfig.NavStatusConditionID(ns.Key, i))
		if rule.Condition != "" && m == nil {
			// Counting without the condition would flag every row the list
			// shows under this rule's tone, which is a wrong answer rather
			// than a missing one.
			return v1.NavStatus{}, false, fmt.Errorf("status[%d]: condition is not compiled", i)
		}
		kept, err := applyViewCondition(ctx, rows, m, a.redactedForSuggestion, a.Services().Store)
		if err != nil {
			return v1.NavStatus{}, false, fmt.Errorf("status[%d]: %w", i, err)
		}
		if n := len(kept); n > 0 {
			return v1.NavStatus{
				Tone:  rule.Tone,
				Label: strings.ReplaceAll(rule.Label, navStatusCountVar, strconv.Itoa(n)),
				Count: n,
			}, true, nil
		}
	}
	return v1.NavStatus{}, false, nil
}

// navStatusRows returns exactly the rows the list shows this principal: its
// entity type under the ACL row gate and the request's world, its query scope
// (or the type's default), its static `filters:` and its `condition:`.
//
// Built from the list's CONFIG rather than from request parameters, since
// there is no list request to copy them from. The static filters are encoded
// the way the SPA sends them (listBaseParams in listParams.ts) so they run
// through the list endpoint's own filter pass, not a second implementation.
//
// Rows are gated and redacted before anything is counted over them, the order
// the root CLAUDE.md requires of an aggregate. The returned ctx carries the
// query identity, for rule conditions that name current_user.
func navStatusRows(
	ctx context.Context, a *App, s *Schema, listID string, listCfg dataentryconfig.List,
) (context.Context, []*entityPkg.Entity, error) {
	typeName := listCfg.EntityType
	scope, err := viewQueryScope(a.queryScopes, s.Cfg, s.Meta, typeName, listCfg.QueryScope)
	if err != nil {
		return ctx, nil, err
	}
	// Bound up front rather than only when a narrowing needs it: a rule
	// condition may name current_user even where the list itself does not.
	if ctx, err = bindQueryIdentity(ctx, scope); err != nil {
		return ctx, nil, err
	}
	rows, err := scopedSortedEntitiesScoped(ctx, a, typeName, navStatusListQuery(listCfg), scope)
	if err != nil {
		return ctx, nil, err
	}
	cond := viewCondition(a.viewConditions, s, viewKindList, listID)
	if listCfg.Condition != "" && cond == nil {
		return ctx, nil, fmt.Errorf("list %q: condition is not compiled", listID)
	}
	rows, err = applyViewCondition(ctx, rows, cond, a.redactedForSuggestion, a.Services().Store)
	if err != nil {
		return ctx, nil, err
	}
	return ctx, rows, nil
}

// navStatusFilterOps maps a configured filter operator to the list API's, as
// OPERATOR_MAP in frontend/src/utils/filters.ts does. An unknown operator
// passes through unchanged, as it does there.
var navStatusFilterOps = map[string]string{
	"!=": "ne", "=": "eq", "==": "eq",
	">": "gt", ">=": "gte", "<": "lt", "<=": "lte",
	"~": "contains", "in": "in",
}

// navStatusListQuery encodes a list's static `filters:` as the list endpoint's
// `filter[<prop>][<op>]` parameters, mirroring listBaseParams: an entry with
// no operator or no value is skipped, and repeated keys join with a comma.
func navStatusListQuery(listCfg dataentryconfig.List) map[string][]string {
	query := map[string][]string{}
	for _, f := range listCfg.Filters {
		if f.Operator == "" || f.Value == "" {
			continue
		}
		op, known := navStatusFilterOps[f.Operator]
		if !known {
			op = f.Operator
		}
		key := "filter[" + f.Property + "][" + op + "]"
		if existing := query[key]; len(existing) > 0 {
			query[key] = []string{existing[0] + "," + f.Value}
		} else {
			query[key] = []string{f.Value}
		}
	}
	return query
}
