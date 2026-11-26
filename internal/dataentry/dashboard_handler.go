package dataentry

import (
	"context"
	"net/http"
	"strconv"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
)

// handleV1Dashboard returns the dashboard page config with the cards this
// principal may see. `?name=` selects a dashboard under `dashboards:`; an
// unknown name is a 404 `dashboard_not_found`, since names are config. This
// is the per-principal counterpart to the verbatim `/api/v1/_config`.
// Filtering is a UX affordance and enforces nothing (permitsGatedUIElement):
// a hidden card's query still returns its ACL-scoped rows via `_search`.
//
//archplan:auth=required
func (h *viewsHandler) handleV1Dashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

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
		resp.Validation = true
		writeV1JSON(w, http.StatusOK, resp)
		return
	}

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
	ctx := r.Context()
	resp.Validation = !cfg.IsEntityDashboard() && cfg.Validation.Shown(func(perm string) bool {
		return permitsGatedUIElement(ctx, aclImpl, perm)
	})

	writeV1JSON(w, http.StatusOK, resp)
}

// handleV1DashboardCard serves the value of one `list:` card for this
// principal; `?name=` selects the dashboard and `?card=` the card index. A
// card hidden by `permission:` answers 404 like an unknown index; a `query:`
// card answers 400. Rows are redacted once and only then folded, and a hidden
// value leaves the response marked partial. On an entity dashboard
// `?anchor=` is required and read through the gate first, so a hidden and a
// missing anchor give the same 404; a plain dashboard refuses it.
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
			"Card is not computed", "only a card with list: has a computed value")
		return
	}

	anchor, ok := dashboardCardAnchor(ctx, a, w, r, dash)
	if !ok {
		return
	}
	value, err := dashboardCardValue(ctx, a, s, card, anchor)
	if err != nil {
		writeListPipelineError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusOK, value)
}

// dashboardCardAnchor reads the `anchor` parameter of a card request and the
// anchor it names. It writes the error response itself and reports false
// when the request must stop. The anchor is nil on a plain dashboard.
func dashboardCardAnchor(
	ctx context.Context, a *App, w http.ResponseWriter, r *http.Request, dash *dataentryconfig.DashboardConfig,
) (*entityPkg.Entity, bool) {
	vals, given := r.URL.Query()[scopeAnchorParam]
	if !dash.IsEntityDashboard() {
		if given {
			writeV1Error(w, r, http.StatusBadRequest, "anchor_not_allowed",
				"Anchor not allowed", "this dashboard has no entity_type, so it counts for no entity")
			return nil, false
		}
		return nil, true
	}
	if len(vals) != 1 || vals[0] == "" {
		writeV1Error(w, r, http.StatusBadRequest, "anchor_required",
			"Anchor required", "this dashboard has entity_type "+strconv.Quote(dash.EntityType)+
				"; name the entity to count for exactly once in ?anchor=")
		return nil, false
	}
	anchor, err := readPageAnchor(ctx, a, dash.EntityType, vals[0])
	if err != nil {
		writeListPipelineError(w, r, err)
		return nil, false
	}
	return anchor, true
}
