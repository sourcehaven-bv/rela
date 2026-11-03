package dataentry

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// MCPActions runs the data-entry actions for the remote MCP tools
// (TKT-NW9QSN). [MCPActions.RunAction] is the POST /api/v1/_action/{id}
// route with the same gates, in the same order; only the transport differs.
// A refusal or a failed script is a [*script.ActionError] carrying the
// route's error code.
//
// The ctx must be the remote MCP request's: it carries the principal, the ACL
// request and the read gate that the `/api/` middleware stamps.
//
// A script error is always described with the degraded detail a non-loopback
// HTTP caller gets. The MCP endpoint requires a verified token, so it is never
// the loopback developer surface full detail is for.
type MCPActions struct {
	h *writeHandler
}

// ListActions returns the actions the ctx principal may run; see
// [actionOps.list].
func (a MCPActions) ListActions(ctx context.Context, address string) ([]script.ActionInfo, error) {
	list, fail := actionOps(a).list(ctx, address)
	if fail != nil {
		return nil, fail.actionError(ctx)
	}
	return list, nil
}

// RunAction is the action route. An action that declares `confirm:` runs
// only when in.Confirmed is set: the web app asks before it posts, and an
// agent must ask its user the same question. The question is returned only
// once every gate has passed.
//
// A request body reaches the script as `rela.request.body`, and
// `rela.request.raw` is its JSON re-serialization, not the caller's bytes.
func (a MCPActions) RunAction(
	ctx context.Context, id string, in script.ActionRunInput,
) (*script.ActionResponse, error) {
	if !actionIDRegex.MatchString(id) {
		return nil, failV1(http.StatusBadRequest, "invalid_action_id", "Invalid action ID").actionError(ctx)
	}
	s := a.h.schema()
	action, ok := s.Cfg.Actions[id]
	if !ok {
		return nil, failV1(http.StatusNotFound, "action_not_found", "Action not found").actionError(ctx)
	}
	payload, fail := mcpActionPayload(id, action, in)
	if fail != nil {
		return nil, fail.actionError(ctx)
	}
	var confirm func() *opFailure
	if action.Confirm.Enabled && !in.Confirmed {
		confirm = func() *opFailure {
			question := action.Confirm.Text
			if question == "" {
				question = "Run this action?"
			}
			return failV1(http.StatusConflict, "confirmation_required",
				"Ask the user, then run again with confirm=true: "+question)
		}
	}
	resp, fail := actionOps(a).run(ctx, s, id, action, payload, confirm)
	if fail != nil {
		return nil, fail.actionError(ctx)
	}
	return resp, nil
}

// mcpActionPayload is [readActionPayload] for a tool call: the address in
// place of the body's entity_id, and the request body only for an action
// whose `request:` block opts into one, under the same size cap. A body for
// any other action is refused rather than dropped, so a caller does not take
// a run that ignored its input for one that used it.
func mcpActionPayload(id string, action dataentryconfig.Action, in script.ActionRunInput) (actionPayload, *opFailure) {
	payload := actionPayload{EntityID: in.Address}
	if in.Body != nil && (action.Request == nil || !action.Request.Body) {
		return actionPayload{}, failV1(http.StatusBadRequest, "invalid_body", "This action accepts no request body")
	}
	if action.Request == nil {
		return payload, nil
	}

	req := &lua.Request{Method: http.MethodPost, Path: "/api/v1/_action/" + id}
	if in.Body != nil {
		raw, err := json.Marshal(in.Body)
		if err != nil {
			return actionPayload{}, failV1(http.StatusBadRequest, "invalid_body", "Invalid request body")
		}
		limit := DefaultActionMaxBodyBytes
		if action.Request.MaxBodyBytes > 0 {
			limit = action.Request.MaxBodyBytes
		}
		if int64(len(raw)) > limit {
			return actionPayload{}, failV1(http.StatusRequestEntityTooLarge, "body_too_large",
				"Request body too large")
		}
		req.ContentType = "application/json"
		req.Raw = string(raw)
		req.Body = in.Body
	}
	payload.Request = req
	return payload, nil
}

// mcpActions returns the action operations for the MCP host.
func mcpActions(h *writeHandler) *MCPActions {
	if h == nil {
		return nil
	}
	return &MCPActions{h: h}
}
