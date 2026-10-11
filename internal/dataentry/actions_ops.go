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
}

// run executes action id for the ctx principal: the permission gate, the
// entity resolution and the script, everything handleV1Action does after it
// has read the request.
//
// beforeScript, when not nil, runs after every gate has passed and before the
// script starts; a failure it returns refuses the run. The MCP tools ask for
// confirmation there, so a caller is never asked to confirm a run the gates
// would refuse anyway.
//
// Scripts run concurrently with each other and with every other write. Each
// write a script makes is individually protected by the manager, but a
// sequence of writes is not atomic.
func (o actionOps) run(
	ctx context.Context, s *Schema, id string, action dataentryconfig.Action, payload actionPayload,
	beforeScript func() *opFailure,
) (*script.ActionResponse, *opFailure) {
	h := o.h

	// permission: holds on every surface that can invoke the action
	// (TKT-VVS16W), so it is checked before anything else. The 403 names the
	// permission: which permission an action needs is config, not a secret,
	// and the operator debugging a missing button needs it.
	if !holdsActionPermission(ctx, action) {
		return nil, failV1(http.StatusForbidden, "permission_required",
			fmt.Sprintf("action %q requires the %q permission", id, action.Permission))
	}

	correlationID := newCorrelationID()

	// Resolve the caller-supplied entity_id through the SCRIPT READER before
	// it reaches the script.
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
	if h.versionTags != nil {
		deps.VersionTags = h.versionTags()
	}
	if h.oauth != nil {
		deps.OAuth = h.oauth()
	}
	var ent *entity.Entity
	if payload.EntityID != "" && action.AvailableOn == nil {
		if e, getErr := deps.VisibleReader.GetAddress(ctx, payload.EntityID); getErr == nil {
			ent = e
		}
	}

	// Provision (a no-op unless unmatched_principal: provision fired) so an
	// action-triggered write by an unmatched verified principal is covered
	// like CRUD.
	if h.provision != nil {
		ctx = h.provision(ctx)
	}

	if action.AvailableOn != nil {
		// The gate must judge the config the script runs under. A reload that
		// landed since the action was looked up may have changed the action's
		// scope or when, so refuse rather than check a stale copy.
		if h.schema() != s {
			return nil, failV1(http.StatusConflict, "config_reloaded",
				"The configuration was reloaded; retry the action")
		}
		bound, fail := o.boundEntity(ctx, s, id, action, payload.EntityID)
		if fail != nil {
			return nil, fail
		}
		ent = bound
	}

	if beforeScript != nil {
		if fail := beforeScript(); fail != nil {
			return nil, fail
		}
	}

	// Reuse App's long-lived engine so rela.cache state persists
	// across action invocations. Constructing a fresh engine per
	// request would reset the cache each time and defeat memoization.
	resp, err := h.engine().ExecuteActionRequest(ctx, action.Script, deps,
		script.ActionInvocation{
			TriggerEntity: ent,
			Params:        action.Params,
			Request:       payload.Request,
			Timeout:       lua.DefaultTimeout,
			CorrelationID: correlationID,
		})
	if err != nil {
		// ERROR MAPPING (TKT-EFMRQM): a script FAILURE always produces rela's
		// error envelope, never a script-chosen status. A script that raised
		// has by definition not decided on a response — the status field lives
		// on a successful RETURN value, which a failed run never produced. A
		// script that wants to answer 4xx/5xx deliberately returns
		// {status = ...}, which is the success path.
		if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
			// The caller went away; nothing failed that is worth a warning.
			f := o.failRun(err, correlationID)
			f.cause = err
			return nil, f
		}
		slog.Warn("action failed", "action", id, "correlation", correlationID, "error", err)
		return nil, o.failRun(err, correlationID)
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
	// Non-Lua failure (script-not-found, contract failure from
	// parseActionResponse, redirect validation, etc.) — keep the
	// existing minimal-detail shape.
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

// boundEntity resolves and authorizes the entity an entity-bound action
// (available_on, TKT-VVS16W) runs against.
//
// Unlike an ordinary action, entity_id is REQUIRED here and names an ADDRESS
// (`ID` or `ID@face`), resolved literally as the command entity path does.
// A missing, unparseable, unreadable or face-hidden entity is the same 404,
// so the endpoint is not an existence oracle. A readable entity the action
// does not apply to is a 403 naming the action: which actions exist is
// config, and the caller can already see everything the verdict read.
//
// The script receives the redacted entity, the same one `when` judged, with
// Redacted set so entity:is_redacted() tells hidden from unset.
func (o actionOps) boundEntity(
	ctx context.Context, s *Schema, id string, action dataentryconfig.Action, address string,
) (*entity.Entity, *opFailure) {
	e, fail := o.readableEntity(ctx, address)
	if fail != nil {
		return nil, fail
	}
	check := o.h.affordances.newDetailActionCheck(ctx, s, e)
	if !check.Allows(ctx, id, action) {
		return nil, failV1(http.StatusForbidden, "action_not_available",
			fmt.Sprintf("action %q is not available on this entity", id))
	}
	return check.Redacted(), nil
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
