package dataentry

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
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
// writeMu for the entire script execution, blocking other mutations.
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

	s := a.State()
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

	// Serialize action script execution against other mutations and
	// against workspace reloads via writeMu.
	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	// Resolve entity if provided in the request.
	var entity *model.Entity
	if req.EntityID != "" {
		if e, ok := s.Graph.GetNode(req.EntityID); ok {
			entity = e
		}
	}

	ctx := &actionScriptContext{
		ws:          a.ws,
		meta:        s.Meta,
		projectRoot: a.ws.Paths().Root,
		entity:      entity,
	}

	engine := script.NewEngine()
	resp, err := engine.ExecuteAction(action.Script, ctx, action.Params, actionTimeout)
	if err != nil {
		slog.Warn("action failed", "action", id, "correlation", correlationID, "error", err)
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
// The entity field is optionally populated when the action is invoked with
// entity context (e.g., from a list action applied to selected rows).
type actionScriptContext struct {
	ws          *workspace.Workspace
	meta        *metamodel.Metamodel
	projectRoot string
	entity      *model.Entity
}

func (c *actionScriptContext) GetWorkspace() interface{}     { return c.ws }
func (c *actionScriptContext) GetMeta() *metamodel.Metamodel { return c.meta }
func (c *actionScriptContext) GetProjectRoot() string        { return c.projectRoot }
func (c *actionScriptContext) GetEntity() *model.Entity      { return c.entity }
func (c *actionScriptContext) GetOldEntity() *model.Entity   { return nil }
func (c *actionScriptContext) GetStdout() io.Writer          { return nil }
func (c *actionScriptContext) GetArgs() []string             { return nil }
func (c *actionScriptContext) GetOutputDir() string          { return "" }
