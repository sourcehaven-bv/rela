package dataentry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/nextaction"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

const (
	// maxNextActionBodyBytes caps the feedback body. The payload is four
	// short strings; anything larger is a mistake or an attack.
	maxNextActionBodyBytes = 4 << 10

	// dismissHorizon is how long a dismissal suppresses a suggestion. Long
	// enough to read as "gone", finite so a dismissal cannot outlive the
	// config that produced it — a suppression with no expiry is the invisible
	// state per-entity muting was rejected for.
	dismissHorizon = 365 * 24 * time.Hour
)

// nextActionResponse is the wire shape for GET /api/v1/_next_action.
//
// A deliberately thin envelope: `suggestion` is null when nothing is owed,
// rather than the endpoint 404ing. "Nothing to suggest" is a normal, frequent
// answer for an advisory surface — a well-configured system is quiet most of
// the time — and an error status would push callers into treating silence as
// a failure.
type nextActionResponse struct {
	Suggestion *nextActionWire `json:"suggestion"`
}

// nextActionWire is one suggestion as the SPA sees it.
type nextActionWire struct {
	Source   string `json:"source"`
	Band     string `json:"band"`
	EntityID string `json:"entity_id,omitempty"`
	// Variant carries the source's key_props values. The client MUST echo it
	// back on feedback: it is part of the suggestion key, so a snooze stored
	// without it lands under a different key than the one Resolve checks and
	// silently fails to suppress anything.
	//
	// Opaque to the client — it exists to be returned verbatim, not parsed.
	Variant string                            `json:"variant,omitempty"`
	Message string                            `json:"message"`
	Actions []dataentryconfig.NextActionOffer `json:"actions,omitempty"`
	// PickOptions carries the render-time options for any pick_one
	// affordance, keyed by that offer's index in Actions (as a string,
	// because JSON object keys are strings).
	//
	// Sent alongside the offers rather than embedded in them so the config
	// shape stays exactly what the operator wrote — the SPA reads the offer
	// for its kind and this map for its live contents.
	PickOptions map[string][]pickOptionWire `json:"pick_options,omitempty"`
}

// pickOptionWire is one option in a pick_one affordance.
type pickOptionWire struct {
	EntityID string `json:"entity_id"`
	Label    string `json:"label"`
}

// nextActionFeedbackRequest is the body of POST /api/v1/_next_action.
type nextActionFeedbackRequest struct {
	// Source and EntityID identify which suggestion is being answered. They
	// are echoed from the GET rather than trusted blindly — see the handler.
	Source   string `json:"source"`
	EntityID string `json:"entity_id,omitempty"`
	Variant  string `json:"variant,omitempty"`

	// Kind is one of: "snooze", "dismiss", "mute", "unmute", "shown".
	Kind string `json:"kind"`
	// Duration applies to snooze, e.g. "1d". Ignored otherwise.
	Duration string `json:"duration,omitempty"`
}

// handleV1NextAction serves the advisory suggestion surface.
//
// GET resolves the one suggestion to show; POST records the user's response
// (snooze / dismiss / mute / shown).
//
// The two are separate verbs on purpose. Resolving is a READ: a caller that
// previews or discards the result must not start a cooldown, so the clock
// only moves when the client explicitly reports the suggestion was shown.
func (a *App) handleV1NextAction(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleV1NextActionGet(w, r)
	case http.MethodPost:
		a.handleV1NextActionPost(w, r)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

func (a *App) handleV1NextActionGet(w http.ResponseWriter, r *http.Request) {
	eng, ok := a.nextActionEngine()
	if !ok {
		// No sources configured: a valid, common state (the feature is
		// opt-in). An empty answer, not a 404 — the SPA renders nothing.
		writeV1JSON(w, http.StatusOK, nextActionResponse{})
		return
	}

	ctx := r.Context()
	sug, found, err := eng.Resolve(ctx, nextActionUser(ctx), time.Now())
	if err != nil {
		writeListPipelineError(w, r, err)
		return
	}
	if !found {
		writeV1JSON(w, http.StatusOK, nextActionResponse{})
		return
	}

	writeV1JSON(w, http.StatusOK, nextActionResponse{Suggestion: &nextActionWire{
		Source:      sug.Source,
		Band:        sug.Band,
		EntityID:    sug.EntityID,
		Variant:     sug.Key.Variant,
		Message:     sug.Message,
		Actions:     sug.Actions,
		PickOptions: pickOptionsWire(sug.PickOptions),
	}})
}

// pickOptionsWire converts the engine's index-keyed options to the wire
// shape. Nil in, nil out — the field is omitempty so a suggestion without a
// pick_one carries nothing.
func pickOptionsWire(in map[int][]nextaction.PickOption) map[string][]pickOptionWire {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]pickOptionWire, len(in))
	for idx, opts := range in {
		wire := make([]pickOptionWire, 0, len(opts))
		for _, o := range opts {
			wire = append(wire, pickOptionWire{EntityID: o.EntityID, Label: o.Label})
		}
		out[strconv.Itoa(idx)] = wire
	}
	return out
}

func (a *App) handleV1NextActionPost(w http.ResponseWriter, r *http.Request) {
	state := a.userState
	if state == nil {
		writeV1Error(w, r, http.StatusNotImplemented, "not_configured",
			"Next-action state is not configured", "")
		return
	}

	var req nextActionFeedbackRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxNextActionBodyBytes)).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "Malformed request body", "")
		return
	}
	if req.Source == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "source is required", "")
		return
	}
	// Only a CONFIGURED source may be addressed. Without this an arbitrary
	// string would create user-state rows keyed on nothing — unbounded growth
	// driven by request bodies, and a mute list full of ids that name no
	// source and so can never be surfaced for un-muting.
	src, known := a.Cfg().NextActions[req.Source]
	if !known {
		writeV1Error(w, r, http.StatusBadRequest, "unknown_source",
			"Unknown next-action source", req.Source)
		return
	}

	ctx := r.Context()
	user := nextActionUser(ctx)
	key := userstate.Key{
		User:     user,
		Source:   req.Source,
		EntityID: req.EntityID,
		Variant:  req.Variant,
	}
	// The SERVER decides the key shape, not the client. A source-scoped
	// source keys on the source alone, so the entity the client echoed back
	// (which it needs for the link, and which the GET does advertise) is
	// dropped here. Trusting the echo verbatim would store the deferral under
	// a key the engine never checks — a 204 that silently does nothing, which
	// is exactly how this was found.
	if src.ResolvedDeferScope() == dataentryconfig.DeferScopeSource {
		key.EntityID = ""
		key.Variant = ""
	}

	if err := a.applyNextActionFeedback(ctx, state, key, req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_feedback", err.Error(), "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// nextActionUser is the key this request's snoozes and mutes are stored
// under.
//
// An UNSTAMPED principal (no auth configured) yields principal.From's
// "unknown" sentinel, and every such request therefore shares one bucket of
// state. That is deliberate and correct for a single-user deployment — which
// is exactly when there is no auth — but it means a shared unauthenticated
// deployment gives everyone one another's snoozes.
//
// Not a confidentiality problem: the state holds no entity content, only
// which suggestions someone deferred. It IS a usability one, so it is named
// here rather than left to be discovered. Wiring any identity source (JWT,
// header, OS user) fixes it with no change to this code.
//
// Deliberately NOT translated to a synthesized per-session id: that would
// invent an identity the audit log and ACL do not share, and the project's
// rule is to never translate an unknown principal into a guessed one.
func nextActionUser(ctx context.Context) string {
	return principal.From(ctx).User
}

// applyNextActionFeedback records one user response.
func (a *App) applyNextActionFeedback(
	ctx context.Context, state userstate.Store, key userstate.Key, req nextActionFeedbackRequest,
) error {
	now := time.Now()
	switch req.Kind {
	case "snooze":
		d, err := dataentryconfig.ParseNextActionDuration(req.Duration)
		if err != nil {
			return fmt.Errorf("invalid snooze duration %q", req.Duration)
		}
		return state.SetSnooze(ctx, key, now.Add(d))

	case "dismiss":
		// Dismissal is a snooze with no end: suppressed until the suggestion
		// KEY changes, which is what key_props are for. Modeled on the
		// existing mechanism rather than a separate one, so there is a single
		// suppression path to reason about.
		return state.SetSnooze(ctx, key, now.Add(dismissHorizon))

	case "mute":
		return state.SetMuted(ctx, key.User, key.Source, true)

	case "unmute":
		return state.SetMuted(ctx, key.User, key.Source, false)

	case "shown":
		return state.MarkShown(ctx, key, now)

	default:
		return fmt.Errorf("unknown kind %q", req.Kind)
	}
}

// nextActionEngine builds an engine for this request, or reports that the
// feature is not configured.
//
// Constructed per call rather than held on App because the CandidateFunc
// closes over nothing request-scoped — the ACL gate is read from the ctx
// passed to Resolve — and an Engine is a thin value over config plus two
// collaborators. If this ever becomes hot, cache the engine, never the
// suggestions (see nextaction.Engine.Resolve on why results must not be
// cached across principals).
func (a *App) nextActionEngine() (*nextaction.Engine, bool) {
	// Snapshot the config once: State() reads an atomic pointer, and two
	// reads could observe different snapshots if a reload lands between them.
	cfg := a.Cfg()
	if cfg == nil || len(cfg.NextActions) == 0 || a.userState == nil {
		return nil, false
	}
	eng, err := nextaction.New(cfg, a.userState, a.nextActionCandidates(),
		nextaction.WithOptions(a.nextActionOptions()))
	if err != nil {
		return nil, false
	}
	return eng, true
}
