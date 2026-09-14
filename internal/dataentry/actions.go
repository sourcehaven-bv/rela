package dataentry

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// actionIDRegex defines the allowed format for action IDs at request time.
// Must match the regex used in dataentryconfig.validateActions.
var actionIDRegex = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

// actionTimeout is the maximum execution time for an action script.
// Tighter than the default Lua timeout because the action handler holds
// writeMu for the entire script execution, blocking other mutations.
const actionTimeout = 5 * time.Second

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

	correlationID := newCorrelationID()

	// Resolve the caller-supplied entity_id through the SCRIPT READER, before
	// taking writeMu and before it reaches the script.
	//
	// `entity_id` names an entity the script receives as the global `entity`
	// (script.Engine.ExecuteAction), so resolving it through the raw store made
	// this endpoint a read-side ACL bypass: any caller who may POST an action
	// could name any id and have its properties land in script scope, echoed
	// back through the action's own response.
	//
	// visibility.ScriptReader is the seam that answers all of this correctly,
	// and reusing it beats hand-rolling a gate here:
	//
	//   - it gates on the STORED type, so a caller cannot claim a type they may
	//     read to reach an id of a type they may not (BUG-ZWTDH9's defect);
	//   - it REDACTS `visible:`-hidden fields, which a row-level PermitsRead
	//     check does not — the script must not see values the caller cannot;
	//   - a denied entity comes back as store.ErrNotFound, indistinguishable
	//     from a genuine miss, so this endpoint is not an existence oracle.
	//
	// A denied or absent id therefore leaves ent nil and the action still RUNS,
	// exactly as when no entity_id was supplied. That is deliberate: entity_id
	// is an optional parameter, not the resource. Refusing the whole request
	// would break list actions whose script ignores `entity` — the SPA sends an
	// id for every selected row, so one unreadable row in a selection would
	// otherwise fail an operation the caller is fully entitled to perform.
	//
	// deps is hoisted so the reader that authorizes the lookup is the same one
	// the script runs with, and so luaDeps() builds one gate/reader/redactor
	// chain per request rather than two.
	deps := h.luaDeps()
	// TKT-YH52OM: the action's declared `capabilities:` block is the ONLY
	// source of ambient capability for this script. An action with no block
	// gets no http, no ai, no secrets and no write_file — this endpoint is
	// reachable by anyone who may POST an action, so it is not an
	// operator-shell surface and must not inherit a trusted default.
	deps.Capabilities = luaCapabilities(action.Capabilities)
	var ent *entity.Entity
	if payload.EntityID != "" {
		if e, getErr := deps.VisibleReader.GetEntity(r.Context(), payload.EntityID); getErr == nil {
			ent = e
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
	resp, err := h.engine().ExecuteActionRequest(r.Context(), action.Script, deps,
		script.ActionInvocation{
			TriggerEntity: ent,
			Params:        action.Params,
			Request:       payload.Request,
			Timeout:       actionTimeout,
			CorrelationID: correlationID,
		})
	if err != nil {
		// ERROR MAPPING (TKT-EFMRQM): a script FAILURE always produces rela's
		// error envelope, never a script-chosen status. A script that raised
		// has by definition not decided on a response — the status field lives
		// on a successful RETURN value, which a failed run never produced. A
		// script that wants to answer 4xx/5xx deliberately returns
		// {status = ...}, which is the success path below.
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

	writeActionResponse(w, resp)
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
