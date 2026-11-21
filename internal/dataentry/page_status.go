package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// viewKindPageDone keys a steps-page tab's compiled done_when in the
// view-condition lookup, matching conditionlint.ViewConditionPageDone. The id
// is dataentryconfig.PageDoneConditionID.
const viewKindPageDone = "page_done"

// The query parameters of GET /api/v1/_page_status. Both are ids: the page
// names its own tabs and their conditions, so a caller cannot ask for an
// expression of its own.
const (
	pageStatusPageParam   = "page"
	pageStatusEntityParam = "entity"
)

// handleV1PageStatus serves GET /api/v1/_page_status?page=<id>[&entity=<id>]:
// for each step of a steps page that declares `done_when:` and that this
// principal sees, whether the step is done (TKT-MWZF0UO). An entity page
// needs `entity`, its anchor, and judges each step over the rows the tab
// keeps for that anchor.
//
// Served apart from /api/v1/_sidebar for the reason /api/v1/_nav_status is:
// the sidebar is the same for every principal, and whether a step is done
// depends on the rows, so on who asks.
//
// Each step is judged over the rows its list shows THIS principal: gated,
// redacted, then judged (configuredListRows). A row the principal cannot see
// never counts for or against a step, so the answer reveals nothing the list
// itself does not. A hidden or missing anchor is the uniform 404 an entity
// GET gives.
//
// Cost per judged step: one gated list read, one batch of `related(...)`
// answers when done_when traverses, and on an entity page one anchor read
// and one relation query to narrow to the anchor. Nothing is read per row.
//
// `?world=` is admitted (worldCapablePath): the list reads take the world
// on ctx, so a world-bound page is judged over the faces it shows.
//
// A package function taking the App rather than a method: App is at its
// plimsoll load line.
func handleV1PageStatus(a *App, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	ctx := r.Context()
	s := a.State()
	query := r.URL.Query()
	pageID, anchor := query.Get(pageStatusPageParam), query.Get(pageStatusEntityParam)
	page, ok := s.Cfg.Pages[pageID]
	if !ok {
		// The page ids are config, which is not a secret (CLAUDE.md), so
		// naming the unknown one is fine.
		writeV1Error(w, r, http.StatusNotFound, "page_not_found", "Page not found",
			fmt.Sprintf("no page %q is configured", pageID))
		return
	}
	switch {
	case page.IsEntityPage() && anchor == "":
		writeV1Error(w, r, http.StatusBadRequest, "bad_request", "entity is required on an entity page", "")
		return
	case !page.IsEntityPage() && anchor != "":
		writeV1Error(w, r, http.StatusBadRequest, "bad_request", "entity is only for an entity page", "")
		return
	}

	steps := make(map[string]v1.PageStepStatus)
	for _, tab := range visiblePageTabs(ctx, a.views.currentACL(), page) {
		if tab.DoneWhen == "" {
			continue
		}
		done, err := pageStepDone(ctx, a, s, pageID, tab, anchor)
		switch {
		case errors.Is(err, errPageAnchorNotFound):
			// Every scoped tab reads the same anchor, so the first refusal
			// answers for the page.
			writeListPipelineError(w, r, err)
			return
		case err != nil:
			// One step that cannot be judged must not blank the others, and
			// no check is the honest answer for it. The check is advisory.
			slog.Log(ctx, sidebarSkipLevel(err), "page status: step skipped",
				"page", pageID, "tab", tab.ID, "list", tab.List, "error", err)
			continue
		}
		steps[tab.ID] = v1.PageStepStatus{Done: done}
	}
	// Per principal; writeV1JSON marks every v1 response no-store.
	writeV1JSON(w, http.StatusOK, v1.PageStatusResponse{Steps: steps})
}

// pageStepDone reports whether a step is done: its list shows this principal
// at least one row, and done_when keeps every row it shows. A step with no
// rows is not done, since a workshop step whose list is still empty has not
// started.
func pageStepDone(
	ctx context.Context, a *App, s *Schema, pageID string, tab dataentryconfig.PageTab, anchor string,
) (bool, error) {
	listCfg, ok := s.Cfg.Lists[tab.List]
	if !ok {
		return false, fmt.Errorf("unknown list %q", tab.List)
	}
	m := viewCondition(a.viewConditions, s, viewKindPageDone, dataentryconfig.PageDoneConditionID(pageID, tab.ID))
	if m == nil {
		// Judging without the condition would mark every non-empty step
		// done, which is a wrong answer rather than a missing one.
		return false, errors.New("done_when is not compiled")
	}
	query := navStatusListQuery(listCfg)
	if anchor != "" {
		// The tab's page scope, resolved from config by resolvePageScope
		// inside the list read, exactly as the tab's own list request is.
		query[scopePageParam] = []string{pageID}
		query[scopeTabParam] = []string{tab.ID}
		query[scopeAnchorParam] = []string{anchor}
	}
	ctx, rows, err := configuredListRows(ctx, a, s, tab.List, listCfg, query, nil)
	if err != nil || len(rows) == 0 {
		return false, err
	}
	// done_when redacts each row before judging it; configuredListRows
	// primed the rows already when the list has a condition of its own.
	if listCfg.Condition == "" {
		ctx = primeVerdicts(ctx, a.fieldResolver, rows)
	}
	kept, err := applyViewCondition(ctx, rows, m, suggestionRedactor(a.affordances), a.Services().Store)
	if err != nil {
		return false, err
	}
	return len(kept) == len(rows), nil
}
