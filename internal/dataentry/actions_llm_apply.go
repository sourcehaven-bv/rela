package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/actionproposal"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// An `llm:` action with `apply: auto` writes its proposal itself, in the
// background (TKT-WQ925LL). The SPA route queues the run and answers at once;
// the job runs the same gates and the same model call as a proposing run, as
// the identity that asked, and then writes what the model proposed through
// the entity manager, where that identity's ACL, validation and audit apply.
// A flow step runs it the same way, as the flow's run_as.

// LLMActionQueue queues a background run of an `apply: auto` action. The
// job runs as who: its user, and the client attenuation (principal type and
// scopes) that narrows that user's grants, so a run cannot write more than
// the request that queued it. It reads in world, the request's world, so
// address resolves to the face the user saw.
type LLMActionQueue interface {
	EnqueueLLMAction(ctx context.Context, action, address, world string, who principal.Principal) error
}

// SetLLMActionQueue installs the queue the action route puts `apply: auto`
// runs on. Without one such a run answers 503.
//
// Nil: rejected, because the route would answer 503 with nothing saying why.
func SetLLMActionQueue(a *App, q LLMActionQueue) error {
	if a == nil || q == nil {
		return errors.New("dataentry: SetLLMActionQueue: app and queue are required")
	}
	a.write.llmJobs = q
	return nil
}

// enqueueLLM queues an `apply: auto` run of action id on subject for the ctx
// principal and answers with a message saying so.
func (o actionOps) enqueueLLM(ctx context.Context, id string, subject *entityPkg.Entity) (
	*script.ActionResponse, *opFailure,
) {
	if subject == nil {
		return nil, failV1(http.StatusNotFound, "entity_not_found", "Entity not found")
	}
	if o.h.llmJobs == nil {
		return nil, failV1(http.StatusServiceUnavailable, "background_unavailable",
			fmt.Sprintf("action %q applies its changes in the background, which this server does not run", id))
	}
	who := principal.From(ctx)
	if who.User == "" || principal.IsRelaIdentity(who.User) {
		return nil, failV1(http.StatusForbidden, "identity_required",
			fmt.Sprintf("action %q runs in the background as you, and this request has no user", id))
	}
	world := worldFromContext(ctx).name
	if err := o.h.llmJobs.EnqueueLLMAction(ctx, id, subject.Ref().String(), world, who); err != nil {
		f := failV1(http.StatusServiceUnavailable, "enqueue_failed", "The action could not be queued")
		f.cause = err
		return nil, f
	}
	return &script.ActionResponse{
		Message:     "The AI changes are being made in the background.",
		MessageType: "info",
	}, nil
}

// llmApplied is what applyProposal wrote.
type llmApplied struct {
	set, stale       []string
	related, created int
}

// applyProposal writes p for the ctx principal: the same writes the SPA's
// proposal dialog makes when every change is ticked.
//
// A property whose value changed since the proposal is skipped, so a
// proposal never overwrites an edit made while the model was answering. The
// writes are not one transaction: each is protected by the manager, and a
// failure leaves the writes before it in place, as the dialog does.
func (o actionOps) applyProposal(ctx context.Context, p *actionproposal.Proposal) (llmApplied, error) {
	var out llmApplied
	if p.IsEmpty() {
		return out, nil
	}
	row, fail := o.readableEntity(ctx, p.Entity)
	if fail != nil {
		return out, errors.New(fail.title)
	}
	props := map[string]any{}
	for _, v := range p.Set {
		if !sameJSON(row.Properties[v.Property], v.Current) {
			out.stale = append(out.stale, v.Property)
			continue
		}
		var val any
		if err := json.Unmarshal(v.Proposed, &val); err != nil {
			return out, fmt.Errorf("property %q: %w", v.Property, err)
		}
		props[v.Property] = val
		out.set = append(out.set, v.Property)
	}
	if len(props) > 0 {
		if err := o.patchProposed(ctx, row, props); err != nil {
			return out, err
		}
	}
	for _, r := range p.Relate {
		key := entityPkg.RelationKey{From: row.ID, FromFace: row.Face, Type: r.Relation, To: r.To}
		_, err := o.h.manager.CreateRelation(ctx, key, entityPkg.RelationOptions{})
		switch {
		case errors.Is(err, entitymanager.ErrRelationAlreadyExists):
			continue
		case err != nil:
			return out, fmt.Errorf("relation %q to %s: %w", r.Relation, r.To, err)
		}
		out.related++
	}
	for _, c := range p.Create {
		if err := o.createProposed(ctx, row, c); err != nil {
			return out, err
		}
		out.created++
	}
	return out, nil
}

// patchProposed writes props to row, judged by the same affordance check as
// a PATCH.
func (o actionOps) patchProposed(ctx context.Context, row *entityPkg.Entity, props map[string]any) error {
	if denial := o.h.affordances.validateFieldWrite(ctx, row, props, nil); denial != nil {
		return fmt.Errorf("%s: %s", denial.Path, denial.Reason)
	}
	grant, err := o.h.manager.AuthorizeUpdate(ctx, row)
	if err != nil {
		return err
	}
	patch := entityPkg.Patch{Properties: props, ExpectedVersion: string(store.VersionOf(row))}
	_, err = o.h.manager.PatchGranted(ctx, grant, entityPkg.FormatStateRef(row.ID, row.Face), patch)
	return err
}

// createProposed creates c and relates it to row, judged by the same
// affordance check as a create.
func (o actionOps) createProposed(ctx context.Context, row *entityPkg.Entity, c actionproposal.NewEntity) error {
	props := make(map[string]any, len(c.Properties))
	for _, v := range c.Properties {
		var val any
		if err := json.Unmarshal(v.Proposed, &val); err != nil {
			return fmt.Errorf("new %s %q: %w", c.Type, v.Property, err)
		}
		props[v.Property] = val
	}
	candidate := &entityPkg.Entity{Type: c.Type, Properties: props}
	if denial := o.h.affordances.validateCreateFieldWrite(ctx, candidate, props); denial != nil {
		return fmt.Errorf("new %s: %s: %s", c.Type, denial.Path, denial.Reason)
	}
	res, err := o.h.manager.CreateEntity(ctx, candidate, entityPkg.CreateOptions{})
	if err != nil {
		return fmt.Errorf("new %s: %w", c.Type, err)
	}
	key := entityPkg.RelationKey{From: row.ID, FromFace: row.Face, Type: c.Relation, To: res.Entity.ID}
	if !c.Outgoing {
		key = entityPkg.RelationKey{From: res.Entity.ID, FromFace: res.Entity.Face, Type: c.Relation, To: row.ID}
	}
	if _, err := o.h.manager.CreateRelation(ctx, key, entityPkg.RelationOptions{}); err != nil {
		if _, delErr := o.h.manager.DeleteEntity(ctx, res.Entity.ID, true); delErr != nil {
			return fmt.Errorf("relation %q to new %s %s: %w; the new %s could not be removed: %w",
				c.Relation, c.Type, res.Entity.ID, err, c.Type, delErr)
		}
		return fmt.Errorf("relation %q to new %s: %w", c.Relation, c.Type, err)
	}
	return nil
}

// message describes a to the caller.
func (a llmApplied) message() string {
	var parts []string
	if n := len(a.set); n > 0 {
		parts = append(parts, plural(n, "property", "properties"))
	}
	if a.related > 0 {
		parts = append(parts, plural(a.related, "relation", "relations"))
	}
	if a.created > 0 {
		parts = append(parts, plural(a.created, "new record", "new records"))
	}
	msg := "The AI proposed no changes."
	if len(parts) > 0 {
		msg = "Applied " + strings.Join(parts, ", ") + "."
	}
	if len(a.stale) > 0 {
		msg += " Skipped " + strings.Join(a.stale, ", ") + ", which changed while the AI was answering."
	}
	return msg
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// sameJSON reports whether v and raw are the same value once both are JSON.
func sameJSON(v any, raw json.RawMessage) bool {
	enc, err := json.Marshal(v)
	if err != nil {
		return false
	}
	var a, b any
	if json.Unmarshal(enc, &a) != nil {
		return false
	}
	if len(raw) == 0 {
		raw = json.RawMessage("null")
	}
	if json.Unmarshal(raw, &b) != nil {
		return false
	}
	return reflect.DeepEqual(a, b)
}

// LLMActionRunner runs `apply: auto` actions for a host outside the HTTP
// request: the job queue and flow steps.
type LLMActionRunner struct{ a *App }

// NewLLMActionRunner returns the runner over a.
//
// Nil: rejected, because the runner would fail at its first job.
func NewLLMActionRunner(a *App) (LLMActionRunner, error) {
	if a == nil || a.write == nil {
		return LLMActionRunner{}, errors.New("dataentry: NewLLMActionRunner: app is required")
	}
	return LLMActionRunner{a: a}, nil
}

// RunLLMAction runs the `apply: auto` action id on address, in world (empty
// is the declared default world), as the identity bound on ctx and applies
// its proposal. retry
// reports whether another attempt may succeed where this one failed: a
// refusal or a bad configuration fails every attempt the same way, an
// unavailable provider may not.
func (r LLMActionRunner) RunLLMAction(ctx context.Context, id, address, world string) (retry bool, err error) {
	ctx, err = bindBackgroundRequest(ctx, r.a, world)
	if err != nil {
		return false, err
	}
	h := r.a.write
	s := h.schema()
	action, ok := s.Cfg.Actions[id]
	if !ok || !autoLLM(action) {
		return false, fmt.Errorf("action %q is not an llm action with apply: auto", id)
	}
	ops := actionOps{h: h, background: true}
	resp, fail := ops.run(ctx, s, id, action, actionPayload{EntityID: address}, nil)
	if fail != nil {
		if errors.Is(fail.cause, context.Canceled) {
			return true, fail.cause
		}
		fail.logInternal(ctx, "llm action: background run failed")
		return retryableStatus(fail.status), fmt.Errorf("action %q on %s: %s: %s", id, address, fail.title, fail.detail)
	}
	slog.Info("llm action applied", "action", id, "entity", address, "result", resp.Message)
	return false, nil
}

// autoLLM reports whether action is an `llm:` action that applies its own
// proposal.
func autoLLM(action dataentryconfig.Action) bool {
	return action.LLM != nil && action.LLM.Auto()
}

// retryableStatus reports whether a run that failed with status may succeed
// on another attempt.
func retryableStatus(status int) bool {
	switch status {
	case http.StatusConflict, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// bindBackgroundRequest gives ctx what an API request carries and a
// background context lacks. The read gate comes first and fails closed:
// under a declarative policy it is built from the Request bound on ctx, and
// with none bound the call is refused. Then the world: world, or the default
// world when empty. A named world needs the identity's read grant, checked
// again here. A world already bound on ctx wins.
func bindBackgroundRequest(ctx context.Context, a *App, world string) (context.Context, error) {
	if _, declarative := a.acl.(*acl.Declarative); declarative {
		bound, err := attachBoundReadGate(withACLEnforced(ctx), a.acl)
		if err != nil {
			slog.Warn("background run: no identity to read as", "err", err)
			return ctx, errors.New("no identity is bound to this run")
		}
		ctx = bound
	}
	if _, bound := ctx.Value(worldCtxKey{}).(worldHandle); bound {
		return ctx, nil
	}
	ctx, ok, err := nextActionSourceWorld(ctx, a.worlds, effectiveDefaultWorld(a),
		dataentryconfig.NextActionSource{SourceWorld: world})
	if err != nil || !ok {
		slog.Warn("background run: world unavailable", "world", world, "err", err)
		return ctx, fmt.Errorf("world %q is not available to this run", world)
	}
	return ctx, nil
}
