package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// The action operations, separated from the POST /api/v1/_action/{id} route
// (TKT-NW9QSN) so the remote MCP tools run the same gates in the same order.
// Each operation takes a ctx and returns an [opFailure] instead of writing a
// response; the route writes the failure as it always did, and [MCPActions]
// renders it as a [script.ActionError].
//
// The ops hang off their own type rather than writeHandler, which is at its
// method load line.
type actionOps struct {
	h *writeHandler
	// background is set on a run of an `apply: auto` action from the job
	// queue or a flow step, which applies the proposal itself. Unset, such
	// an action is queued instead.
	background bool
}

// run executes action id for the ctx principal: the permission gate, the
// entity resolution and the script, everything handleV1Action does after it
// has read the request.
//
// beforeScript, when not nil, runs after every gate has passed and before the
// script starts; a failure it returns refuses the run.
//
// Scripts run concurrently with each other and with every other write. Each
// write a script makes is individually protected by the manager, but a
// sequence of writes is not atomic.
//
// The bound entity is resolved through the redacting reader on its stored
// type, and gates judge the same config the script runs under.
func (o actionOps) run(
	ctx context.Context, s *Schema, id string, action dataentryconfig.Action, payload actionPayload,
	beforeScript func() *opFailure,
) (*script.ActionResponse, *opFailure) {
	h := o.h

	if !holdsActionPermission(ctx, action) {
		return nil, failV1(http.StatusForbidden, "permission_required",
			fmt.Sprintf("action %q requires the %q permission", id, action.Permission))
	}

	if fail := checkBatchPayload(id, action, payload); fail != nil {
		return nil, fail
	}

	correlationID := newCorrelationID()

	deps := h.luaDeps()
	deps.Capabilities = luaCapabilities(action.Capabilities)
	addOptionalScriptDeps(h, &deps)
	if action.AllowACLBypass.Enabled() && h.schema() != s {
		return nil, failV1(http.StatusConflict, "config_reloaded",
			"The configuration was reloaded; retry the action")
	}
	var ent, boundRaw *entity.Entity
	if payload.EntityID != "" && action.AvailableOn == nil {
		if e, getErr := deps.VisibleReader.GetAddress(ctx, payload.EntityID); getErr == nil {
			ent = e
		}
	}

	if h.provision != nil {
		ctx = h.provision(ctx)
	}

	if payload.Batch {
		return o.runLLMBatch(ctx, s, id, action, payload.EntityIDs, deps)
	}

	if action.AvailableOn != nil {
		if h.schema() != s {
			return nil, failV1(http.StatusConflict, "config_reloaded",
				"The configuration was reloaded; retry the action")
		}
		raw, bound, fail := o.boundEntity(ctx, s, id, action, payload.EntityID)
		if fail != nil {
			return nil, fail
		}
		ent, boundRaw = bound, raw
	}
	if fail := grantElevatedUpdates(h.elevatedUpdates, &deps, id, action, boundRaw); fail != nil {
		return nil, fail
	}

	if beforeScript != nil {
		if fail := beforeScript(); fail != nil {
			return nil, fail
		}
	}
	if action.LLM != nil {
		return o.llm(ctx, s, id, action, boundRaw, ent, deps)
	}

	resp, err := h.engine().ExecuteActionRequest(ctx, action.Script, deps,
		script.ActionInvocation{
			TriggerEntity: ent,
			Params:        action.Params,
			Request:       payload.Request,
			Timeout:       lua.DefaultTimeout,
			CorrelationID: correlationID,
		})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
			f := o.failRun(err, correlationID)
			f.cause = err
			return nil, f
		}
		slog.Warn("action failed", "action", id, "correlation", correlationID, "error", err)
		return nil, o.failRun(err, correlationID)
	}
	if resp != nil && resp.AssertedRoles != nil {
		slog.Warn("action returned asserted_roles outside the IdP webhook", "action", id, "correlation", correlationID)
		return nil, o.failRun(errors.New("asserted_roles is honored only by the IdP webhook action"), correlationID)
	}
	return resp, nil
}

// failRun is the failure of a script that ran and failed. Its cause is
// already logged by run, so it carries none.
func (o actionOps) failRun(err error, correlationID string) *opFailure {
	var se *lua.ScriptError
	if errors.As(err, &se) {
		f := &opFailure{
			status: http.StatusUnprocessableEntity, code: "script_error", title: "Script error",
			detail: scriptErrorDetail(se), correlationID: correlationID,
		}
		f.write = func(w http.ResponseWriter, r *http.Request) {
			writeV1ScriptError(w, se, o.h.fullScriptDetail(r), correlationID)
		}
		return f
	}
	f := &opFailure{status: http.StatusInternalServerError, code: "action_failed", title: "Action failed"}
	f.correlationID = correlationID
	f.write = func(w http.ResponseWriter, _ *http.Request) {
		writeV1JSON(w, http.StatusInternalServerError, v1.ActionResponse{
			Error:         "action_failed",
			Message:       "Action failed",
			CorrelationID: correlationID,
		})
	}
	return f
}

// scriptErrorDetail renders the degraded envelope of a script error, the one
// a non-loopback HTTP caller gets: never the source, stack or captured output
// that only a loopback caller may see.
func scriptErrorDetail(se *lua.ScriptError) string {
	env := buildScriptErrorEnvelope(se, false, "")
	detail := env.Lua.Message
	if env.Script.Path != "" {
		loc := env.Script.Path
		if env.Lua.Line > 0 {
			loc += ":" + strconv.Itoa(env.Lua.Line)
		}
		detail = loc + ": " + detail
	}
	if env.Script.EntityID != "" {
		detail += " (entity " + env.Script.EntityID + ")"
	}
	return detail
}

// boundEntity resolves and authorizes the entity an entity-bound action runs
// against. entity_id is required and is an address (`ID` or `ID@face`). A
// missing, unparseable, unreadable or face-hidden entity is the same 404, so
// the endpoint is no existence oracle. A readable entity the action does not
// apply to is a 403 naming the action. The script gets the redacted entity
// with Redacted set; the raw row is returned for the elevated-update pin and
// never reaches the script.
func (o actionOps) boundEntity(
	ctx context.Context, s *Schema, id string, action dataentryconfig.Action, address string,
) (raw, redacted *entity.Entity, fail *opFailure) {
	e, fail := o.readableEntity(ctx, address)
	if fail != nil {
		return nil, nil, fail
	}
	check := o.h.affordances.newDetailActionCheck(ctx, s, e)
	if !check.Allows(ctx, id, action) {
		return nil, nil, failV1(http.StatusForbidden, "action_not_available",
			fmt.Sprintf("action %q is not available on this entity", id))
	}
	return e, check.Redacted(), nil
}

// readableEntity resolves address for the ctx principal. A missing or
// unreadable entity is the one 404 `entity_not_found`.
func (o actionOps) readableEntity(ctx context.Context, address string) (*entity.Entity, *opFailure) {
	notFound := failV1(http.StatusNotFound, "entity_not_found", "Entity not found")
	if address == "" {
		return nil, notFound
	}
	e, found, err := o.h.visible.untypedAddress(ctx, address)
	if err != nil {
		return nil, failGate(err)
	}
	if !found {
		return nil, notFound
	}
	return e, nil
}

// list returns the actions the ctx principal may run, sorted by id.
//
// Without an address these are the actions that are not entity-bound and
// whose permission the principal holds: what the POST route would run. With
// one they are the entity-bound actions the detail page offers on that
// entity, judged by the same check as the `action:<id>` affordance.
//
// It reads with the unprovisioned ctx, as the affordance does: listing is a
// read, and only a write provisions.
func (o actionOps) list(ctx context.Context, address string) ([]script.ActionInfo, *opFailure) {
	s := o.h.schema()
	var allows func(id string, action dataentryconfig.Action) bool
	if address == "" {
		allows = func(_ string, action dataentryconfig.Action) bool {
			return action.AvailableOn == nil && holdsActionPermission(ctx, action)
		}
	} else {
		e, fail := o.readableEntity(ctx, address)
		if fail != nil {
			return nil, fail
		}
		check := o.h.affordances.newDetailActionCheck(ctx, s, e)
		allows = func(id string, action dataentryconfig.Action) bool {
			return check.Allows(ctx, id, action)
		}
	}

	var out []script.ActionInfo
	for id, action := range s.Cfg.Actions {
		if allows(id, action) {
			out = append(out, actionInfo(id, action))
		}
	}
	slices.SortFunc(out, func(a, b script.ActionInfo) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
	return out, nil
}

// actionInfo describes action id to a caller.
func actionInfo(id string, action dataentryconfig.Action) script.ActionInfo {
	info := script.ActionInfo{
		ID:            id,
		Label:         action.Label,
		Description:   action.Description,
		EntityBound:   action.AvailableOn != nil,
		RequestScoped: action.Request != nil && action.Request.Body,
	}
	if action.Confirm.Enabled {
		info.Confirm = true
		info.ConfirmText = action.Confirm.Text
	}
	return info
}

// actionError describes f to a transport other than HTTP. A cancelled request
// returns the context error itself.
func (f *opFailure) actionError(ctx context.Context) error {
	if errors.Is(f.cause, context.Canceled) {
		return f.cause
	}
	f.logInternal(ctx, "actions: operation failed")
	return &script.ActionError{Code: f.code, Title: f.title, Detail: f.detail, CorrelationID: f.correlationID}
}

// addOptionalScriptDeps fills the action script's optional collaborators:
// each is set only when the server wired it.
func addOptionalScriptDeps(h *writeHandler, deps *lua.WriteDeps) {
	if h.versionTags != nil {
		deps.VersionTags = h.versionTags()
	}
	if h.oauth != nil {
		deps.OAuth = h.oauth()
	}
	if h.attachRender != nil {
		deps.AttachRender = h.attachRender()
	}
	if h.attachFile != nil {
		deps.AttachFile = h.attachFile()
	}
	if h.records != nil {
		deps.Records = h.records()
	}
}
