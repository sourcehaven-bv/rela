package mcp

import (
	"context"
	"errors"
	"fmt"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/script"
)

// ActionRunner lists and runs the data-entry actions (`actions:` in
// data-entry.yaml) for list_actions and run_action (TKT-NW9QSN).
//
// The remote wiring passes the data-entry API's own operations, so a tool
// runs the same permission, entity and `available_on` gates as
// POST /api/v1/_action/{id}. A refusal or a failed script is a
// [*script.ActionError]. The ctx carries the caller; the runner reads the
// principal and the read gate from it.
type ActionRunner interface {
	// ListActions returns the actions the caller may run: the ones not bound
	// to an entity when address is empty, else the ones the entity's detail
	// page offers.
	ListActions(ctx context.Context, address string) ([]script.ActionInfo, error)
	RunAction(ctx context.Context, id string, in script.ActionRunInput) (*script.ActionResponse, error)
}

// actionHandler serves the action tools.
type actionHandler struct {
	runner ActionRunner
}

func selActions(h handlerSet) actionHandler { return h.actions }

// registerActionTools registers list_actions and run_action when
// [Deps.Actions] is set. Only the remote server sets it: an action runs as
// the signed-in caller, and stdio `rela mcp` has none. A free function, like
// [addTool], to keep Server under its plimsoll load line.
func registerActionTools(s *Server) {
	if s.deps().Actions == nil {
		return
	}
	addTool(s, toolListActions(), actionTool(s, actionHandler.handleListActions))
	addTool(s, toolRunAction(), actionTool(s, actionHandler.handleRunAction))
}

// actionTool binds an action handler. A reload can drop the runner after the
// tools were registered; the tools then answer that actions are unavailable
// instead of calling a nil interface.
func actionTool(
	s *Server, method func(actionHandler, context.Context, *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error),
) mcpgo.ToolHandler {
	return bind(s, selActions, func(
		h actionHandler, ctx context.Context, req *mcpgo.CallToolRequest,
	) (*mcpgo.CallToolResult, error) {
		if h.runner == nil {
			return errorResult("actions are not available on this server"), nil
		}
		return method(h, ctx, req)
	})
}

const actionAddressDescription = "Entity ID, or ID@face for an entity with content states."

func toolListActions() *mcpgo.Tool {
	return newTool("list_actions",
		withDescription("List the data-entry actions you may run. Without id: the actions not bound to an "+
			"entity. With id: the actions offered on that entity. Each has its id, label, description, "+
			"whether it asks for confirmation (and the question), whether it is entity-bound, and whether "+
			"it accepts a request body."),
		withString("id", description(actionAddressDescription)),
	)
}

func toolRunAction() *mcpgo.Tool {
	return newTool("run_action",
		withDescription("Run a data-entry action, as its button in the web app does. An action that asks "+
			"for confirmation runs only with confirm=true: call without it to read the question, ask the "+
			"user, then call again. Result: {message, message_type, redirect}, or {status, content_type, "+
			"body} for an action that answers with a body; a status of 400 or more is an error result."),
		withString("action", required(), description("Action ID, as returned by list_actions")),
		withString("id", description(actionAddressDescription+
			" Required for an entity-bound action; otherwise passed to the script as its entity.")),
		withObject("request", description("Request body, for an action that accepts one")),
		withBoolean("confirm", description("true once the user has confirmed the action")),
	)
}

func (h actionHandler) handleListActions(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	args := newToolRequest(request)
	address, err := optionalString(args, "id")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	var addr string
	if address != nil {
		addr = trimID(*address)
	}
	list, err := h.runner.ListActions(ctx, addr)
	if err != nil {
		return actionOpResult(err), nil
	}
	out := make([]actionView, len(list))
	for i, a := range list {
		out[i] = actionView{
			ID: a.ID, Label: a.Label, Description: a.Description,
			Confirm: a.Confirm, ConfirmText: a.ConfirmText,
			EntityBound: a.EntityBound, AcceptsRequest: a.RequestScoped,
		}
	}
	return jsonResult(struct {
		Actions []actionView `json:"actions"`
	}{Actions: out}), nil
}

// actionView is one list_actions entry.
type actionView struct {
	ID             string `json:"id"`
	Label          string `json:"label,omitempty"`
	Description    string `json:"description,omitempty"`
	Confirm        bool   `json:"confirm,omitempty"`
	ConfirmText    string `json:"confirm_text,omitempty"`
	EntityBound    bool   `json:"entity_bound,omitempty"`
	AcceptsRequest bool   `json:"accepts_request,omitempty"`
}

func (h actionHandler) handleRunAction(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	args := newToolRequest(request)
	id, err := args.RequireString("action")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	var in script.ActionRunInput
	address, err := optionalString(args, "id")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	if address != nil {
		in.Address = trimID(*address)
	}
	if raw, present := args.GetArguments()["request"]; present && raw != nil {
		body, ok := raw.(map[string]any)
		if !ok {
			return errorResult(`argument "request" is not an object`), nil
		}
		in.Body = body
	}
	confirmed, err := optionalBool(args, "confirm")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	in.Confirmed = confirmed != nil && *confirmed

	// The action id is passed as given: the runner refuses an id with
	// spaces, as the HTTP route does.
	resp, err := h.runner.RunAction(ctx, id, in)
	if err != nil {
		return actionOpResult(err), nil
	}
	result := jsonResult(actionResultView(resp))
	// A rich response with an error status is the script's own refusal; flag
	// it so a client that reads only isError does not take it for success.
	if resp != nil && resp.IsRich() && resp.Status >= 400 {
		result.IsError = true
	}
	return result, nil
}

// actionResult is run_action's result: the SPA shape or the rich shape,
// whichever the script returned (see [script.ActionResponse]).
type actionResult struct {
	Message     string `json:"message,omitempty"`
	MessageType string `json:"message_type,omitempty"`
	Redirect    string `json:"redirect,omitempty"`
	Status      int    `json:"status,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Body        string `json:"body,omitempty"`
}

func actionResultView(resp *script.ActionResponse) actionResult {
	if resp == nil {
		return actionResult{}
	}
	if resp.IsRich() {
		return actionResult{Status: resp.Status, ContentType: resp.ContentType, Body: resp.Body}
	}
	return actionResult{Message: resp.Message, MessageType: resp.MessageType, Redirect: resp.Redirect}
}

// actionOpResult answers a refused or failed action operation.
func actionOpResult(err error) *mcpgo.CallToolResult {
	var opErr *script.ActionError
	if errors.As(err, &opErr) {
		return errorResult(opErr.Error())
	}
	return errorResult(fmt.Sprintf("action failed: %v", err))
}
