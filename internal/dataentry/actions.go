package dataentry

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// actionIDRegex defines the allowed format for action IDs at request time.
// Must match the regex used in dataentryconfig.validateActions.
var actionIDRegex = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

// actionTimeout is the maximum execution time for an action script.
// Tighter than the default Lua timeout because the action handler holds
// the workspace write lock for the entire script execution.
const actionTimeout = 5 * time.Second

// V1ActionResponse mirrors script.ActionResponse for API JSON output.
// Has both successful response fields and error fields with correlation ID.
type V1ActionResponse struct {
	Redirect      string `json:"redirect,omitempty"`
	Message       string `json:"message,omitempty"`
	MessageType   string `json:"message_type,omitempty"`
	Error         string `json:"error,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// handleV1Action executes a configured action script and returns the result.
// Endpoint: POST /api/v1/_action/{id}
//
// Note: Called under reloadLockMiddleware which holds RLock. Action scripts
// may mutate the workspace, so we release the read lock and acquire the
// write lock for the duration of script execution, then restore the read
// lock for the middleware's defer.
func (a *App) handleV1Action(w http.ResponseWriter, r *http.Request) {
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

	// We're under RLock from middleware. Read the action config first.
	action, ok := a.Cfg.Actions[id]
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "action_not_found", "Action not found", "")
		return
	}

	correlationID := newCorrelationID()

	// Swap to write lock for the duration of script execution.
	a.mu.RUnlock()
	a.mu.Lock()
	defer func() {
		a.mu.Unlock()
		a.mu.RLock()
	}()

	ctx := &actionScriptContext{
		ws:          a.ws,
		meta:        a.meta,
		projectRoot: a.ws.Paths().Root,
	}

	engine := script.NewEngine()
	resp, err := engine.ExecuteAction(action.Script, ctx, action.Params, actionTimeout)
	if err != nil {
		log.Printf("action %q failed [correlation=%s]: %v", id, correlationID, err)
		writeV1JSON(w, http.StatusInternalServerError, V1ActionResponse{
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

	writeV1JSON(w, http.StatusOK, V1ActionResponse{
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

// actionScriptContext implements metamodel.ScriptContext for action scripts.
// Actions don't have a triggering entity context — they're user-initiated.
type actionScriptContext struct {
	ws          *workspace.Workspace
	meta        *metamodel.Metamodel
	projectRoot string
}

func (c *actionScriptContext) GetWorkspace() interface{}     { return c.ws }
func (c *actionScriptContext) GetMeta() *metamodel.Metamodel { return c.meta }
func (c *actionScriptContext) GetProjectRoot() string        { return c.projectRoot }
func (c *actionScriptContext) GetEntity() *model.Entity      { return nil }
func (c *actionScriptContext) GetOldEntity() *model.Entity   { return nil }
