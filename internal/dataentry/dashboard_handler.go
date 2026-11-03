package dataentry

import (
	"net/http"
	"strconv"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
)

// handleV1Dashboard returns the dashboard page config with the cards this
// principal may see (TKT-53KICM).
//
// `?name=<name>` selects a dashboard under `dashboards:`; without it (or with
// `name=default`) the default `dashboard:` is served. An unknown name is a
// 404 `dashboard_not_found`. Dashboard names are config, not data, so the
// distinct code discloses nothing (see "The configuration is not a secret;
// the data is" in the root CLAUDE.md).
//
// This is the per-principal counterpart to the `dashboard:` block that
// `/api/v1/_config` serves verbatim to everyone. The split is deliberate:
// `_config` is pinned principal-independent (see TestNavPermission_ConfigUnfiltered
// and "Do NOT filter /_config" in this package's CLAUDE.md), so the filtered
// view needs its own endpoint rather than a divergent `_config`.
//
// Filtering is a UX affordance and enforces nothing — see permitsGatedUIElement.
// A hidden card's query is still runnable through `/api/v1/_search`, where it
// returns exactly the ACL-scoped rows it always did.
//
//archplan:auth=required
func (h *viewsHandler) handleV1Dashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	// Non-nil so a project with no dashboard, an empty cards:, and an
	// all-filtered dashboard all serialize as [] rather than null — one
	// "render what you got" path in the SPA, and no 404 for a non-error.
	cards := make([]v1.DashboardCardEntry, 0)
	resp := v1.DashboardResponse{Cards: cards}

	name := r.URL.Query().Get("name")
	cfg, ok := h.schema().Cfg.DashboardByName(name)
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "dashboard_not_found",
			"Dashboard not found", "no dashboard named "+strconv.Quote(name)+" under dashboards:")
		return
	}
	if cfg == nil {
		writeV1JSON(w, http.StatusOK, resp)
		return
	}

	// Resolved ONCE for the whole request rather than per card, matching
	// handleV1Sidebar and resolveCommands.
	aclImpl := h.currentACL()
	for i, card := range cfg.Cards {
		if !permitsGatedUIElement(r.Context(), aclImpl, card.Permission) {
			continue
		}
		cards = append(cards, v1.DashboardCardEntry{DashboardCard: card, Index: i})
	}

	resp.Title = cfg.Title
	resp.Description = cfg.Description
	resp.Cards = cards

	writeV1JSON(w, http.StatusOK, resp)
}

// handleV1DashboardCard serves GET /api/v1/_dashboard/card: the value of one
// `list:` card, computed for this principal (TKT-2SGPJZ).
//
// `?name=` selects the dashboard as on /api/v1/_dashboard, and `?card=` is the
// card's config index, which /api/v1/_dashboard serves as `index`. The
// response echoes the card's title and display, so the SPA can drop a value
// computed for another card after a config reload moved the indexes.
//
// A card hidden by `permission:` answers 404 like an unknown index. That
// filter is a UX affordance (permitsGatedUIElement), so this is not a
// boundary either: the card's list answers the same rows on its own page.
// A `query:` card answers 400: the SPA computes it from /api/v1/_search.
//
// The rows are the list's rows for this principal (navStatusRows), each
// redacted once, and only then folded. A value hidden by `visible:` is left
// out of every group, cell and color that would need it, and the response
// is marked partial. Totals still count the row, since the list shows it.
// A `match:` scope decides membership on the stored row, as a list's own
// scope does, so a ratio discloses no more than the list with that scope.
//
// A package function taking the App, like handleV1NavStatus: App is at its
// plimsoll load line.
//
//archplan:auth=required
func handleV1DashboardCard(a *App, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	ctx := r.Context()
	s := a.State()
	q := r.URL.Query()

	name := q.Get("name")
	dash, ok := s.Cfg.DashboardByName(name)
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "dashboard_not_found",
			"Dashboard not found", "no dashboard named "+strconv.Quote(name)+" under dashboards:")
		return
	}
	index, err := strconv.Atoi(q.Get("card"))
	if dash == nil || err != nil || index < 0 || index >= len(dash.Cards) ||
		!permitsGatedUIElement(ctx, a.views.currentACL(), dash.Cards[index].Permission) {

		writeV1Error(w, r, http.StatusNotFound, "card_not_found",
			"Card not found", "no card at index "+strconv.Quote(q.Get("card"))+" on this dashboard")
		return
	}
	card := dash.Cards[index]
	if card.List == "" {
		writeV1Error(w, r, http.StatusBadRequest, "card_not_computed",
			"Card is not computed", "only a card with list: has a computed value; this one uses query:")
		return
	}

	value, err := dashboardCardValue(ctx, a, s, card)
	if err != nil {
		writeListPipelineError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusOK, value)
}
