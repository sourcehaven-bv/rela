package dataentry

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
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
	for _, ns := range sidebarNavStatusEntries(ctx, aclImpl, s.Cfg, r.URL.Query().Get("space")) {
		status, ok, err := navStatusFor(ctx, a, s, ns)
		if err != nil {
			slog.Log(ctx, sidebarSkipLevel(err), "nav status: entry skipped",
				"entry", ns.Entry.Label, "list", ns.List, "error", err)
			continue
		}
		if ok {
			items[ns.Key] = status
		}
	}

	writeV1JSON(w, http.StatusOK, v1.NavStatusResponse{Items: items})
}

// navStatusFor evaluates one entry's rules in order and returns the first
// whose count is above zero, or ok=false when none is.
//
// When the store can answer the list and every rule, each rule is one
// scoped count ([navStatusInStore]). Otherwise the list's rows are read ONCE
// and every rule counts over that slice, so a second rule costs an
// evaluation per row, not another read.
func navStatusFor(
	ctx context.Context, a *App, s *Schema, ns dataentryconfig.NavStatusEntry,
) (status v1.NavStatus, ok bool, err error) {
	listCfg, found := s.Cfg.Lists[ns.List]
	if !found {
		return v1.NavStatus{}, false, fmt.Errorf("unknown list %q", ns.List)
	}
	if status, ok, handled, err := navStatusInStore(ctx, a, s, ns, listCfg); handled {
		return status, ok, err
	}
	return navStatusInGo(ctx, a, s, ns, listCfg)
}

// navStatusInGo is the Go path of [navStatusFor]: the list's rows read once,
// every rule judged over them.
func navStatusInGo(
	ctx context.Context, a *App, s *Schema, ns dataentryconfig.NavStatusEntry, listCfg dataentryconfig.List,
) (status v1.NavStatus, ok bool, err error) {
	ctx, rows, err := navStatusRows(ctx, a, s, ns.List, listCfg)
	if err != nil {
		return v1.NavStatus{}, false, err
	}
	if len(rows) == 0 {
		return v1.NavStatus{}, false, nil
	}
	rulesJudge := slices.ContainsFunc(ns.Entry.Status, func(r dataentryconfig.NavStatusRule) bool {
		return r.Condition != ""
	})
	if rulesJudge && listCfg.Condition == "" {
		ctx = primeVerdicts(ctx, a.fieldResolver, rows)
	}
	for i, rule := range ns.Entry.Status {
		m := viewCondition(a.viewConditions, s, viewKindNavStatus,
			dataentryconfig.NavStatusConditionID(ns.Key, i))
		if rule.Condition != "" && m == nil {
			return v1.NavStatus{}, false, fmt.Errorf("status[%d]: condition is not compiled", i)
		}
		kept, err := applyViewCondition(ctx, rows, m, suggestionRedactor(a.affordances), a.Services().Store)
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

// navStatusInStore answers an entry's rules with one scoped count each when the
// store can: the list's request takes the pushdown and every rule's condition
// lowers as a list condition does ([lowerViewCondition]). Each count is the
// list's ACL-scoped query narrowed by the rule. handled is false when any part
// does not lower, before anything is counted; the caller then takes the Go path.
// A write between two counts can change which rule wins; markers are advisory,
// so this is accepted.
func navStatusInStore(
	ctx context.Context, a *App, s *Schema, ns dataentryconfig.NavStatusEntry, listCfg dataentryconfig.List,
) (status v1.NavStatus, ok, handled bool, err error) {
	typeName := listCfg.EntityType
	scope, err := viewQueryScope(a.queryScopes, s.Cfg, s.Meta, typeName, listCfg.QueryScope)
	if err != nil {
		return v1.NavStatus{}, false, false, nil //nolint:nilerr // the Go path returns this error
	}
	if ctx, err = bindQueryIdentity(ctx, scope); err != nil {
		return v1.NavStatus{}, false, false, nil //nolint:nilerr // the Go path returns this error
	}
	gated, err := gateAppListQuery(ctx, a, s, typeName, navStatusListQuery(listCfg))
	if err != nil {
		return v1.NavStatus{}, false, false, nil //nolint:nilerr // the Go path returns this error
	}
	n := listNarrowing{
		cond:  viewCondition(a.viewConditions, s, viewKindList, ns.List),
		scope: scope, state: s, fields: a.fieldResolver,
	}
	if listCfg.Condition != "" && n.cond == nil {
		return v1.NavStatus{}, false, false, nil
	}
	plan, empty, planned := n.pushdownPlan(ctx, typeName, gated, 1, 1)
	if !planned {
		return v1.NavStatus{}, false, false, nil
	}
	if empty {
		return v1.NavStatus{}, false, true, nil
	}
	counts := make([]*listPlan, len(ns.Entry.Status))
	for i, rule := range ns.Entry.Status {
		ruled := plan
		if rule.Condition != "" {
			m := viewCondition(a.viewConditions, s, viewKindNavStatus, dataentryconfig.NavStatusConditionID(ns.Key, i))
			if m == nil {
				return v1.NavStatus{}, false, false, nil
			}
			frag, denied, lowered := lowerViewCondition(ctx, s.Meta, m, a.fieldResolver, typeName)
			if !lowered {
				return v1.NavStatus{}, false, false, nil
			}
			if denied {
				continue
			}
			if ruled, lowered = plan.narrowed(frag); !lowered {
				return v1.NavStatus{}, false, false, nil
			}
		}
		counts[i] = &ruled
	}
	for i, rule := range ns.Entry.Status {
		if counts[i] == nil {
			continue
		}
		count, err := scopedTotal(ctx, a.Services().Store, counts[i].query)
		if err != nil {
			return v1.NavStatus{}, false, true, fmt.Errorf("status[%d]: %w: %w", i, counts[i].errClass, err)
		}
		if count > 0 {
			return v1.NavStatus{
				Tone:  rule.Tone,
				Label: strings.ReplaceAll(rule.Label, navStatusCountVar, strconv.Itoa(count)),
				Count: count,
			}, true, true, nil
		}
	}
	return v1.NavStatus{}, false, true, nil
}

// navStatusRows returns exactly the rows the list shows this principal: its
// type under the ACL row gate and the request's world, its query scope, its
// static `filters:` and its `condition:`. It is built from the list's config;
// static filters are encoded as the SPA sends them so they run through the list
// endpoint's own filter pass. Rows are gated and redacted before anything counts
// over them. The returned ctx carries the query identity and, when the list has
// a condition, the verdict prime of the rows.
func navStatusRows(
	ctx context.Context, a *App, s *Schema, listID string, listCfg dataentryconfig.List,
) (context.Context, []*entityPkg.Entity, error) {
	return configuredListRows(ctx, a, s, listID, listCfg, navStatusListQuery(listCfg), nil)
}

// listRowsFor is navStatusRows with an optional sort and an optional reach.
// A non-empty sort orders the rows, in the list endpoint's sort syntax. A
// non-nil reach narrows the read to those ids, as a page scope does; see
// gatedListQuery.reach.
func listRowsFor(
	ctx context.Context, a *App, s *Schema, listID string, listCfg dataentryconfig.List,
	sort string, reach map[string]bool,
) (context.Context, []*entityPkg.Entity, error) {
	query := navStatusListQuery(listCfg)
	if sort != "" {
		query["sort"] = []string{sort}
	}
	return configuredListRows(ctx, a, s, listID, listCfg, query, reach)
}

// configuredListRows is [navStatusRows] over a prepared list query: the
// list's static filters, plus any narrowing the caller adds, such as the
// page scope of an entity page tab (pagescope.go) or a reach. The query
// only narrows: every row still passes the row gate, the world, the query
// scope and the list's condition.
func configuredListRows(
	ctx context.Context, a *App, s *Schema, listID string, listCfg dataentryconfig.List,
	query map[string][]string, reach map[string]bool,
) (context.Context, []*entityPkg.Entity, error) {
	typeName := listCfg.EntityType
	scope, err := viewQueryScope(a.queryScopes, s.Cfg, s.Meta, typeName, listCfg.QueryScope)
	if err != nil {
		return ctx, nil, err
	}
	if ctx, err = bindQueryIdentity(ctx, scope); err != nil {
		return ctx, nil, err
	}
	gated, err := gateAppListQuery(ctx, a, s, typeName, query)
	if err != nil {
		return ctx, nil, err
	}
	gated.reach = reach
	rows, _, err := a.lists.scopedSortedEntitiesScoped(ctx, typeName, gated, scope)
	if err != nil {
		return ctx, nil, err
	}
	cond := viewCondition(a.viewConditions, s, viewKindList, listID)
	if listCfg.Condition != "" && cond == nil {
		return ctx, nil, fmt.Errorf("list %q: condition is not compiled", listID)
	}
	if cond != nil {
		ctx = primeVerdicts(ctx, a.fieldResolver, rows)
	}
	rows, err = applyViewCondition(ctx, rows, cond, suggestionRedactor(a.affordances), a.Services().Store)
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
