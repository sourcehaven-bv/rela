package dataentry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// errWebhookActionMissing is returned when the configured provisioning action id
// does not exist in the loaded config — an operator misconfiguration.
var errWebhookActionMissing = errors.New("dataentry: configured webhook action not found")

// errWebhookActionEntityBound is returned when the configured provisioning
// action has available_on. Such an action needs an entity and is gated on
// the invoking user; the webhook supplies neither (TKT-VVS16W).
var errWebhookActionEntityBound = errors.New(
	"dataentry: configured webhook action has available_on and can only run from a detail page")

// WebhookClaims is the verified subset of an inbound webhook the receiver acts
// on. It mirrors jwtauth.WebhookClaims but is declared HERE so the dataentry
// package needn't import jwtauth (the inward-pointing layering rule — the same
// reason the JWT gate takes a local assertionVerifier interface). The wiring
// layer, which may import both, adapts the concrete verifier to this shape.
type WebhookClaims struct {
	Event  string // the event name, e.g. "membership.created"
	UserID string // the subject the event concerns
	OrgID  string // the tenant the event concerns
	ID     string // the webhook id (jti), for replay dedup
}

// webhookMaxBody caps the JWT body the receiver reads before verifying it, so a
// malicious oversized body can't exhaust memory pre-auth. A webhook JWT is a few
// hundred bytes; 64 KiB is generous headroom.
const webhookMaxBody = 64 << 10 // 64 KiB

// webhookActionTimeout bounds the provisioning action (which fetches the user
// from the IdP over HTTP, then upserts). Tighter than lua.DefaultTimeout so a
// slow IdP answers the webhook sender before it gives up and redelivers.
const webhookActionTimeout = 15 * time.Second

// webhookDedupTTL is how long a processed webhook id is remembered so a redelivery
// (IdPs retry on a slow/failed response) is a no-op rather than a duplicate
// provision. The window need only exceed the IdP's retry span.
const webhookDedupTTL = 10 * time.Minute

// webhookVerifier verifies a signed webhook JWT and projects the claims the
// receiver acts on. Declared at the call site (returning the local WebhookClaims
// so this package doesn't import jwtauth) — the wiring layer adapts the concrete
// *jwtauth.WebhookVerifier to it. Also lets the handler be tested with a stub.
type webhookVerifier interface {
	VerifyWebhook(ctx context.Context, raw string) (WebhookClaims, error)
}

// webhookReceiver holds the wiring for the inbound-IdP webhook: the verifier, the
// action it dispatches to, and a small dedup cache. Nil on the App until
// SetWebhookReceiver is called, in which case the route is not mounted.
type webhookReceiver struct {
	verify   webhookVerifier
	actionID string
	seen     *seenSet
	roles    webhookRoleStore
}

// webhookRoleStore stores the IdP role names a webhook action returned for
// the verified subject (TKT-IZL6NX8). *appbuild.IDPRoleWriter satisfies it.
// An empty roles list removes the subject's record. Hold locks one subject
// across the action and Set, so an older fetch never overwrites a newer one.
type webhookRoleStore interface {
	Hold(ctx context.Context, subject string) (release func(), err error)
	Set(ctx context.Context, subject, orgID string, roles []string) error
}

// errWebhookRolesUnavailable is returned when the action returned
// asserted_roles but no role store is wired: storing nothing silently would
// leave a revoked role in force.
var errWebhookRolesUnavailable = errors.New("dataentry: webhook action returned asserted_roles but no role store is wired")

// errWebhookActionError is returned when the action reported an error
// (message_type "error"), with or without asserted_roles. Nothing is stored,
// and the 502 makes the IdP retry: acknowledging it would lose the event, and
// a lost event can be a lost revocation.
var errWebhookActionError = errors.New("dataentry: webhook action reported an error; nothing stored")

// SetWebhookReceiver enables the POST /webhooks/idp endpoint: a verified IdP
// callback that dispatches to the named action (which fetches authoritative user
// data and upserts a person entity). Must be called before NewRouter. A nil
// verifier or empty actionID leaves the receiver disabled and the route
// unmounted — matching the inert-when-unconfigured shape of the other optional
// wiring (SetPrincipalResolver, SetSecurityConfig).
//
// roles stores the `asserted_roles` the action returns (TKT-IZL6NX8).
//
// Nil: roles is accepted; an action that then returns asserted_roles fails
// with a 502 rather than having its roles dropped.
func (a *App) SetWebhookReceiver(v webhookVerifier, actionID string, roles webhookRoleStore) {
	if v == nil || actionID == "" {
		return
	}
	a.webhook = &webhookReceiver{verify: v, actionID: actionID, seen: newSeenSet(webhookDedupTTL), roles: roles}
}

// registerWebhookRoutes mounts the webhook endpoint when a receiver is
// configured. Called from NewRouter. The route lives OUTSIDE /api/ on purpose:
// it is not the browser API surface, and it authenticates itself by verifying a
// signed JWT body rather than trusting a proxy header or a session cookie. That
// makes it CSRF-immune by construction (a browser cannot forge an ES256
// signature), so it needs neither the same-origin gate nor the CSRF-exempt
// heuristic the calendar feeds and CalDAV rely on.
func (a *App) registerWebhookRoutes(mux *http.ServeMux) {
	if a.webhook == nil {
		return
	}
	// The handler lives on the receiver; the App only supplies the dispatch
	// closure (which routes through the writeHandler: engine, schema). This
	// keeps the HTTP-flow logic off the (already large) App type.
	rec := a.webhook
	mux.HandleFunc("POST /webhooks/idp", func(w http.ResponseWriter, r *http.Request) {
		rec.handle(w, r, func(ctx context.Context, claims WebhookClaims) error {
			return dispatchWebhookAction(ctx, a.write, rec, claims)
		})
	})
}

// handle verifies an inbound IdP webhook and dispatches it via dispatch. The
// request body is the signed JWT (the IdP sends Content-Type: application/jwt).
// On any verification failure it answers 401 and dispatch is never called. A
// redelivered webhook (same id) is acknowledged with 200 without re-dispatching.
//
//archplan:auth=none reason="the body is a JWT signed by the IdP; verified before dispatch"
func (rec *webhookReceiver) handle(
	w http.ResponseWriter, r *http.Request, dispatch func(context.Context, WebhookClaims) error,
) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, webhookMaxBody))
	if err != nil {
		http.Error(w, "read failed", http.StatusBadRequest)
		return
	}

	claims, err := rec.verify.VerifyWebhook(r.Context(), string(raw))
	if err != nil {
		// Do not echo the reason — a probing client learns nothing beyond "rejected".
		slog.Warn("idp webhook rejected", "error", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// A membership event with no subject is unusable — the action has nothing to
	// fetch. Treat it as a bad request rather than dispatching a no-op.
	if claims.UserID == "" {
		http.Error(w, "missing user_id", http.StatusBadRequest)
		return
	}

	// Dedup on the webhook id (jti). An id-less webhook can't be deduped, so it is
	// always processed — at-least-once is acceptable because the action is
	// idempotent (it upserts by sub).
	if claims.ID != "" && !rec.seen.add(claims.ID) {
		slog.Info("idp webhook deduped", "id", claims.ID, "event", claims.Event)
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := dispatch(r.Context(), claims); err != nil {
		// The action failed (IdP fetch error, script error, ...). Return 502 so the
		// IdP retries; drop the id from the seen-set so the retry isn't deduped.
		if claims.ID != "" {
			rec.seen.forget(claims.ID)
		}
		slog.Warn("idp webhook action failed", "action", rec.actionID, "event", claims.Event, "error", err)
		http.Error(w, "action failed", http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// dispatchWebhookAction runs the provisioning action with the webhook's claims as
// params, under the webhook-receiver principal. It runs concurrently with other
// writes, like handleV1Action; an upsert keyed on a `unique:` property stays
// single because the manager checks uniqueness and writes inside one store.Tx.
// A free function taking the writeHandler explicitly (it is dispatch wiring,
// not a route handler).
func dispatchWebhookAction(ctx context.Context, h *writeHandler, rec *webhookReceiver, claims WebhookClaims) error {
	actionID := rec.actionID
	s := h.schema()
	action, ok := s.Cfg.Actions[actionID]
	if !ok {
		// Misconfiguration: the configured action id doesn't exist. Fail loud in
		// the log; the 502 the caller sends will surface it operationally too.
		slog.Error("idp webhook: configured action not found", "action", actionID)
		return errWebhookActionMissing
	}
	// The webhook runs as the webhook-receiver principal, not a user, so an
	// action's `permission:` does not apply here: the verified callback is the
	// authorization. An entity-bound action cannot run here at all.
	if action.AvailableOn != nil {
		slog.Error("idp webhook: configured action has available_on", "action", actionID)
		return errWebhookActionEntityBound
	}

	// Stamp the webhook-receiver principal so the provisioned entity is attributed
	// to the callback, not to the default data-entry "unknown". The action path
	// reads principal.From(ctx).
	ctx = principal.With(ctx, principal.Principal{
		User: "webhook:" + claims.Event,
		Tool: principal.ToolWebhookReceiver,
	})
	// The webhook route is outside /api/, so attachACLRequest did not run:
	// resolve the receiver's identity here, once, as every request edge does,
	// and attach its read gate.
	ctx, err := acl.BindRequest(ctx, h.acl())
	if err != nil {
		return fmt.Errorf("idp webhook: resolve identity: %w", err)
	}
	ctx, err = attachBoundReadGate(ctx, h.acl())
	if err != nil {
		return fmt.Errorf("idp webhook: read gate: %w", err)
	}

	params := map[string]string{
		"event":   claims.Event,
		"user_id": claims.UserID,
		"org_id":  claims.OrgID,
	}

	// TKT-YH52OM: the webhook path resolves the SAME `actions:` entry as the
	// HTTP endpoint, so it honors the same `capabilities:` declaration. This
	// is the surface that most needs it to work: an IdP-sync action legitimately
	// calls out over http with a named secret (see examples/idp-sync.lua), and
	// it now must say so in config rather than receiving the whole secrets file
	// by default.
	// Hold the subject from before the action fetches the roles until they
	// are stored. Two deliveries for one user then store in fetch order, so
	// a slow older fetch cannot undo a revocation a newer one stored.
	if rec.roles != nil {
		release, holdErr := rec.roles.Hold(ctx, claims.UserID)
		if holdErr != nil {
			return fmt.Errorf("idp webhook: hold subject: %w", holdErr)
		}
		defer release()
	}

	deps := h.luaDeps()
	deps.Capabilities = luaCapabilities(action.Capabilities)
	resp, err := h.engine().ExecuteAction(ctx, action.Script, deps,
		nil, params, webhookActionTimeout, newCorrelationID())
	if err != nil {
		return err
	}
	if resp != nil && resp.MessageType == "error" {
		return errWebhookActionError
	}
	return storeAssertedRoles(ctx, rec.roles, claims, resp)
}

// storeAssertedRoles stores the `asserted_roles` the action returned, for the
// subject and org of the VERIFIED webhook claims (TKT-IZL6NX8). The script
// chooses the role names, which it fetched from the IdP; it cannot choose
// whose roles they are. A response without the key changes nothing. The
// caller has already turned an error response into a 502.
func storeAssertedRoles(
	ctx context.Context, roles webhookRoleStore, claims WebhookClaims, resp *script.ActionResponse,
) error {
	if resp == nil || resp.AssertedRoles == nil {
		return nil
	}
	if roles == nil {
		return errWebhookRolesUnavailable
	}
	if err := roles.Set(ctx, claims.UserID, claims.OrgID, resp.AssertedRoles); err != nil {
		return fmt.Errorf("idp webhook: store asserted roles: %w", err)
	}
	// The IdP subject is personal data; the audit record names the bound
	// entity instead.
	slog.Info("idp webhook stored asserted roles", "event", claims.Event, "roles", len(resp.AssertedRoles))
	return nil
}

// --- dedup seen-set -------------------------------------------------------

// seenSet is a small TTL set of recently-processed webhook ids. It bounds memory
// by evicting expired entries on each add; the id space per window is tiny (one
// per membership change), so no size cap is needed beyond the TTL sweep.
type seenSet struct {
	mu  sync.Mutex
	ttl time.Duration
	at  map[string]time.Time
	// now is injectable for tests; nil ⇒ time.Now.
	now func() time.Time
}

func newSeenSet(ttl time.Duration) *seenSet {
	return &seenSet{ttl: ttl, at: make(map[string]time.Time)}
}

func (s *seenSet) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// add records id and reports whether it was NEW (true) or already present within
// the TTL (false). Expired entries are swept on each call.
func (s *seenSet) add(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	for k, t := range s.at {
		if now.Sub(t) > s.ttl {
			delete(s.at, k)
		}
	}
	if t, ok := s.at[id]; ok && now.Sub(t) <= s.ttl {
		return false
	}
	s.at[id] = now
	return true
}

// forget drops id so a retried-after-failure webhook isn't deduped.
func (s *seenSet) forget(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.at, id)
}
