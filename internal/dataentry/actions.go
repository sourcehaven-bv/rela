package dataentry

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// actionIDRegex defines the allowed format for action IDs at request time.
// Must match the regex used in dataentryconfig.validateActions.
var actionIDRegex = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

// actionTimeout is the maximum execution time for an action script.
// Tighter than the default Lua timeout because the action handler holds
// writeMu for the entire script execution, blocking other mutations.
const actionTimeout = 5 * time.Second

// v1ActionRequest is the optional JSON body for action invocation.
// When entity_id is provided, the script context includes the entity.
type v1ActionRequest struct {
	EntityID   string `json:"entity_id"`
	EntityType string `json:"entity_type"`
}

// handleV1Action executes a configured action script and returns the result.
// Endpoint: POST /api/v1/_action/{id}
//
// The request body is optional. When provided, it may contain entity_id and
// entity_type to set the entity context for the script (used by list actions
// that invoke a script once per selected entity).
//
// Action scripts may mutate the workspace, so we serialize them via
// writeMu for the duration of script execution. Concurrent reloads,
// other mutations, and other action scripts wait for writeMu.
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

	// Parse optional entity context from request body.
	var req v1ActionRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeV1Error(w, r, http.StatusBadRequest, "invalid_body",
				"Invalid request body", err.Error())
			return
		}
	}

	correlationID := newCorrelationID()

	// Gate the caller-supplied entity_id BEFORE taking writeMu, and before it
	// reaches the script.
	//
	// `entity_id` names an entity the script receives as the global `entity`
	// (see script.Engine.ExecuteAction), so resolving it through the raw store
	// makes this endpoint a read-side ACL bypass: any caller who may POST an
	// action could name any id and have its full properties — `visible:`-hidden
	// fields included — placed in script scope and echoed back through the
	// action's own response. The gate answers for the CALLER, which is right:
	// the caller chose the id. Pinned by TestAction_EntityIDRespectsReadGate.
	//
	// Ordering matters twice over. Gating before writeMu means a denied caller
	// never acquires the process-wide write lock nor burns actionTimeout — an
	// unauthorized POST loop would otherwise serialize every writer in the
	// process. Same rule the document renderer follows: decide, then work.
	var ent *entity.Entity
	if req.EntityID != "" {
		// Look up first, then gate on the STORED type — never on
		// req.EntityType. Authorizing against a caller-supplied type is a
		// cross-type escalation: name a type you may read, an id of a type you
		// may not, and an AllowAll verdict on the claim grants the id. That is
		// the same defect BUG-ZWTDH9 fixed on the sync channel. The lookup
		// itself discloses nothing — the entity goes nowhere unless the gate
		// passes.
		stored, err := h.store.GetEntity(r.Context(), req.EntityID)
		if err != nil {
			// Absent id: fall through with ent nil, exactly as when no
			// entity_id was supplied. Deliberately NOT a 404 — that would make
			// this endpoint an existence oracle for ids the caller cannot read.
			stored = nil
		}
		if stored != nil {
			if !h.gateRead(w, r, stored.Type, req.EntityID) {
				return
			}
			ent = stored
		}
	}

	// Serialize action script execution against other mutations and
	// against workspace reloads via writeMu. Provision under the lock (a no-op
	// unless unmatched_principal: provision fired) so an action-triggered write
	// by an unmatched verified principal is covered like CRUD.
	r = h.enterWrite(r)
	defer h.writeMu.Unlock()

	// Reuse App's long-lived engine so rela.cache state persists
	// across action invocations. Constructing a fresh engine per
	// request would reset the cache each time and defeat memoization.
	resp, err := h.engine().ExecuteAction(r.Context(), action.Script, h.luaDeps(),
		ent, action.Params, actionTimeout, correlationID)
	if err != nil {
		slog.Warn("action failed", "action", id, "correlation", correlationID, "error", err)
		var se *lua.ScriptError
		if errors.As(err, &se) {
			writeV1ScriptError(w, se, h.fullScriptDetail(r), correlationID)
			return
		}
		// Non-Lua failure (script-not-found, contract failure from
		// parseActionResponse, redirect validation, etc.) — keep the
		// existing minimal-detail shape.
		writeV1JSON(w, http.StatusInternalServerError, v1.ActionResponse{
			Error:         "action_failed",
			Message:       "Action failed",
			CorrelationID: correlationID,
		})
		return
	}

	if resp == nil || (resp.Redirect == "" && resp.Message == "") {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	writeV1JSON(w, http.StatusOK, v1.ActionResponse{
		Redirect:    resp.Redirect,
		Message:     resp.Message,
		MessageType: resp.MessageType,
	})
}

// newCorrelationID returns a short random hex string for log tracing.
func newCorrelationID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		// Fallback to a timestamp if rand fails (extremely unlikely)
		return "ts" + time.Now().Format("150405.000")
	}
	return hex.EncodeToString(b)
}
