package dataentry

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// actionIDRegex defines the allowed format for action IDs at request time.
// Must match the regex used in dataentryconfig.validateActions.
var actionIDRegex = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

// handleV1Action executes a configured action script and returns the result.
// Endpoint: POST /api/v1/_action/{id}
//
// The request body is optional. When provided, it may contain entity_id and
// entity_type to set the entity context for the script (used by list actions
// that invoke a script once per selected entity).
//
// The gates and the run itself are [actionOps.run], shared with the remote
// MCP tools (TKT-NW9QSN).
//
//archplan:auth=authz
func (h *writeHandler) handleV1Action(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/_action/")
	if !actionIDRegex.MatchString(id) {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_action_id",
			"Invalid action ID", "")
		return
	}

	s := h.schema()
	action, ok := s.Cfg.Actions[id]
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "action_not_found", "Action not found", "")
		return
	}

	// Read the body under the action's size cap and project whatever the
	// action's `request:` block opted into (TKT-EFMRQM). Absent a request:
	// block this reproduces the pre-existing behavior — entity_id out of the
	// body, everything else dropped — with the cap the endpoint never had.
	payload, err := readActionPayload(r, action)
	if err != nil {
		if errors.Is(err, errActionBodyTooLarge) {
			// 413 rather than 400: a producer that reads it can shrink the
			// payload, where a generic 400 leaves it retrying forever.
			writeV1Error(w, r, http.StatusRequestEntityTooLarge, "body_too_large",
				"Request body too large", "")
			return
		}
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body",
			"Invalid request body", err.Error())
		return
	}

	resp, fail := actionOps{h: h}.run(r.Context(), s, id, action, payload, nil)
	if fail != nil {
		// r is the unprovisioned request; no failure writer reads the
		// principal, so the answer is the one the provisioned request got.
		fail.writeHTTP(w, r)
		return
	}
	writeActionResponse(w, resp)
}

// newCorrelationID returns a short random hex string for log tracing.
func newCorrelationID() string {
	b := make([]byte, 6)
	// coverage-ignore-start: defensive: crypto/rand.Read never returns an error on supported platforms, so the
	// timestamp fallback is unreachable
	// in tests
	if _, err := rand.Read(b); err != nil {
		// Fallback to a timestamp if rand fails (extremely unlikely)
		return "ts" + time.Now().Format("150405.000")
	}
	// coverage-ignore-end
	return hex.EncodeToString(b)
}
