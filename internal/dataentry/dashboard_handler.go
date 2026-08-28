package dataentry

import (
	"net/http"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// handleV1Dashboard returns the dashboard page config with the cards this
// principal may see (TKT-53KICM).
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
func (h *viewsHandler) handleV1Dashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	// Non-nil so a project with no dashboard, an empty cards:, and an
	// all-filtered dashboard all serialize as [] rather than null — one
	// "render what you got" path in the SPA, and no 404 for a non-error.
	cards := make([]dataentryconfig.DashboardCard, 0)
	resp := v1.DashboardResponse{Cards: cards}

	cfg := h.schema().Cfg.Dashboard
	if cfg == nil {
		writeV1JSON(w, http.StatusOK, resp)
		return
	}

	// Resolved ONCE for the whole request rather than per card, matching
	// handleV1Sidebar and resolveCommands.
	aclImpl := h.currentACL()
	for _, card := range cfg.Cards {
		if !permitsGatedUIElement(r.Context(), aclImpl, card.Permission) {
			continue
		}
		cards = append(cards, card)
	}

	resp.Title = cfg.Title
	resp.Description = cfg.Description
	resp.Cards = cards

	writeV1JSON(w, http.StatusOK, resp)
}
