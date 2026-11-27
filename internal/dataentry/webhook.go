package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/readtime"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// errWebhookActionMissing is returned when the configured provisioning action id
// does not exist in the loaded config — an operator misconfiguration.
var errWebhookActionMissing = errors.New("dataentry: configured webhook action not found")

// errWebhookActionEntityBound is returned when the configured provisioning
// action has available_on. Such an action needs an entity and is gated on
// the invoking user; the webhook supplies neither (TKT-VVS16W).
var errWebhookActionEntityBound = errors.New(
	"dataentry: configured webhook action has available_on and can only run from a detail page")

// errWebhookActionElevated is returned when the configured provisioning
// action declares allow_acl_bypass. The webhook never elevates, so such a
// script would fail at rela.bypass_acl; refusing names the cause.
var errWebhookActionElevated = errors.New(
	"dataentry: configured webhook action has allow_acl_bypass, which the webhook does not grant")

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

// webhookClaimTTL is how long an id stays claimed by a delivery that is still
// running. It exceeds [webhookActionTimeout] by a wide margin, so a running
// delivery keeps its claim. It is short so that a claim whose release could
// not be saved lapses before the IdP stops retrying: the event then runs on a
// later retry instead of being lost.
const webhookClaimTTL = 2 * time.Minute

// webhookRetryAfter is the Retry-After, in seconds, of a webhook the receiver
// asks the IdP to deliver again.
const webhookRetryAfter = "30"

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
	org      webhookOrgHandler
}

// webhookOrgHandler handles a verified `organization.*` event (TKT-2R2RW5H).
// These events name a tenant and no user, so they do not go to the per-user
// action. An error answers 502 so the IdP retries, as an action failure does.
//
// ctx is the request context: it carries no principal, no ACL request and no
// deadline beyond the request's own. An implementation that writes stamps its
// principal and binds identity itself, as dispatchWebhookAction does, and
// bounds its own work. OrgID is signed by the IdP, but an implementation
// still checks that it names a tenant this deployment serves before acting.
type webhookOrgHandler interface {
	HandleOrgEvent(ctx context.Context, claims WebhookClaims) error
}

// logOrgEvents is the organization handler. It logs the event and
// acknowledges it, so the IdP does not retry an event the app does not act
// on. Provisioning does not happen here: in tenant mode the tenant host
// creates the tenant on `organization.created` before any tenant app exists
// for it (TKT-TNPRV8, package tenanthost). The org id names a tenant, not a person, so it is
// logged.
type logOrgEvents struct{}

// A deletion is logged at Warn as an erasure-needed record: the IdP does not
// redeliver an acknowledged event, so the log is the operator's trigger.
// Erasure is never run from the webhook (TKT-TNERAS). It is irreversible, it
// must not drop a schema a server still has open, and a signed event is a
// weaker authorization than an operator at the host.
func (logOrgEvents) HandleOrgEvent(_ context.Context, claims WebhookClaims) error {
	if claims.Event == "organization.deleted" {
		slog.Warn("idp webhook organization deletion: tenant erasure needed; check PROCEDURE-u020, "+
			"then run rela-server tenant erase",
			"event", claims.Event, "org_id", claims.OrgID)
		return nil
	}
	slog.Info("idp webhook organization event acknowledged; no handler acts on it yet",
		"event", claims.Event, "org_id", claims.OrgID)
	return nil
}

// webhookFamily is the group of events a webhook belongs to. The family
// decides which claims are required and where the event goes.
type webhookFamily int

const (
	// familyOther is every event outside the two named families, including
	// unknown ones. It goes to the action when it names a user, as every
	// event did before TKT-2R2RW5H. The action is the provider-specific
	// layer and may ignore an event, so the receiver keeps no list of event
	// names.
	familyOther webhookFamily = iota
	// familyMembership is `membership.*`: a user in a tenant.
	familyMembership
	// familyOrganization is `organization.*`: a tenant, with no user.
	familyOrganization
)

// classifyWebhookEvent returns the family of an event name.
func classifyWebhookEvent(event string) webhookFamily {
	switch {
	case strings.HasPrefix(event, "organization."):
		return familyOrganization
	case strings.HasPrefix(event, "membership."):
		return familyMembership
	default:
		return familyOther
	}
}

// String names the family in logs.
func (f webhookFamily) String() string {
	switch f {
	case familyMembership:
		return "membership"
	case familyOrganization:
		return "organization"
	default:
		return "other"
	}
}

// missingClaim returns the name of the first claim the family requires that
// claims lacks, or "" when none is missing.
func (f webhookFamily) missingClaim(claims WebhookClaims) string {
	var needUser, needOrg bool
	switch f {
	case familyMembership:
		needUser, needOrg = true, true
	case familyOrganization:
		needOrg = true
	case familyOther:
		needUser = true
	default:
		needUser, needOrg = true, true
	}
	switch {
	case needUser && claims.UserID == "":
		return "user_id"
	case needOrg && claims.OrgID == "":
		return "org_id"
	default:
		return ""
	}
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
	a.webhook = &webhookReceiver{
		verify: v, actionID: actionID, seen: newSeenSet(webhookDedupTTL, a.kv), roles: roles, org: logOrgEvents{},
	}
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
	rec := a.webhook
	mux.HandleFunc("POST /webhooks/idp", func(w http.ResponseWriter, r *http.Request) {
		rec.handle(w, r, func(ctx context.Context, claims WebhookClaims) error {
			return dispatchWebhookAction(ctx, a.write, rec, claims)
		})
	})
}

// handle verifies an inbound IdP webhook (a signed JWT body) and dispatches
// it. A verification failure answers 401 and nothing runs; a missing required
// claim answers 400. `organization.*` events go to the organization handler,
// others to dispatch.
//
// Dedup claims the webhook id (jti) in the [seenSet] before running. A
// redelivery of a handled id gets 200, a copy of a running one gets 503 with
// Retry-After, and a handler failure releases the claim. An id-less webhook
// is always processed.
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
		slog.Warn("idp webhook rejected", "error", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	family := classifyWebhookEvent(claims.Event)
	if missing := family.missingClaim(claims); missing != "" {
		slog.Warn("idp webhook missing required claim",
			"event", claims.Event, "id", claims.ID, "family", family, "missing", missing)
		http.Error(w, "missing "+missing, http.StatusBadRequest)
		return
	}

	if claims.ID != "" {
		claim, seenErr := rec.seen.claim(r.Context(), claims.ID)
		switch {
		case seenErr != nil:
			slog.Error("idp webhook: the dedup set is unavailable; asking the IdP to retry", "error", seenErr)
			retryWebhook(w, "dedup unavailable")
			return
		case claim == seenDone:
			slog.Info("idp webhook deduped", "id", claims.ID, "event", claims.Event)
			w.WriteHeader(http.StatusOK)
			return
		case claim == seenInFlight:
			slog.Info("idp webhook already in flight; asking the IdP to retry", "id", claims.ID, "event", claims.Event)
			retryWebhook(w, "already in flight")
			return
		}
	}

	if family == familyOrganization {
		err = rec.org.HandleOrgEvent(r.Context(), claims)
	} else {
		err = dispatch(r.Context(), claims)
	}
	if err != nil {
		if claims.ID != "" {
			if relErr := rec.seen.release(r.Context(), claims.ID); relErr != nil {
				slog.Error("idp webhook: releasing the claim failed; it lapses by itself",
					"id", claims.ID, "error", relErr)
			}
		}
		if family == familyOrganization {
			slog.Warn("idp webhook organization handler failed", "event", claims.Event, "error", err)
		} else {
			slog.Warn("idp webhook action failed", "action", rec.actionID, "event", claims.Event, "error", err)
		}
		http.Error(w, "handler failed", http.StatusBadGateway)
		return
	}
	if claims.ID != "" {
		if err = rec.seen.finish(r.Context(), claims.ID); err != nil {
			slog.Error("idp webhook: recording the id failed; asking the IdP to retry", "id", claims.ID, "error", err)
			retryWebhook(w, "dedup unavailable")
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// retryWebhook answers 503 with Retry-After, so the IdP delivers the webhook
// again.
func retryWebhook(w http.ResponseWriter, msg string) {
	w.Header().Set("Retry-After", webhookRetryAfter)
	http.Error(w, msg, http.StatusServiceUnavailable)
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
		slog.Error("idp webhook: configured action not found", "action", actionID)
		return errWebhookActionMissing
	}
	if action.AvailableOn != nil {
		slog.Error("idp webhook: configured action has available_on", "action", actionID)
		return errWebhookActionEntityBound
	}
	if action.AllowACLBypass.Enabled() {
		slog.Error("idp webhook: configured action has allow_acl_bypass", "action", actionID)
		return errWebhookActionElevated
	}

	ctx = principal.With(ctx, principal.Principal{
		User: idpWebhookUserPrefix + claims.Event,
		Tool: principal.ToolWebhookReceiver,
	})
	ctx, err := acl.BindRequest(readtime.Ensure(ctx, nowFunc), h.acl())
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
	slog.Info("idp webhook stored asserted roles", "event", claims.Event, "roles", len(resp.AssertedRoles))
	return nil
}

// --- dedup seen-set -------------------------------------------------------

// webhookSeenKey is the state.KV key that holds the seen-set. state.KV is the
// store's own runtime state, so on postgres it lives in the tenant's schema
// and outlives the App: evicting a tenant no longer resets the set, and a
// captured webhook cannot be replayed after a reopen (BUG-ZO33J9M). Every
// process serving the schema shares it too.
const webhookSeenKey = "webhooks/idp-seen.json"

// webhookSeenMax bounds the persisted set. The id space per TTL window is
// tiny (one per membership change), so this is only a ceiling on a flood of
// distinct signed webhooks; the oldest ids go first.
const webhookSeenMax = 4096

// seenState is where a webhook id stands in the [seenSet].
type seenState int

const (
	// seenNew: the id was not known, and the caller now holds its claim.
	seenNew seenState = iota
	// seenInFlight: another delivery holds the id's claim.
	seenInFlight
	// seenDone: a delivery of the id was handled within the TTL.
	seenDone
)

// seenEntry is one id in the [seenSet]. An entry that is not Done is a claim
// of a delivery still running.
type seenEntry struct {
	At   time.Time `json:"at"`
	Done bool      `json:"done,omitempty"`
}

// seenSet records the webhook ids that were handled within the TTL, and
// claims the ids a delivery is handling now (BUG-ZO33J9M). Every operation
// loads, sweeps and saves the set under one mutex, so the check and the claim
// are one step: two copies of one id that arrive together cannot both run.
// A done id expires after ttl, a claim after claimTTL.
//
// The kv copy carries the set across eviction and across processes. Two
// processes can still race between load and save and both claim one id; that
// leaves the at-least-once delivery the handlers already tolerate (the action
// upserts by sub). A kv read or write failure fails the call, and the
// webhook gets 503 so the IdP retries.
type seenSet struct {
	mu       sync.Mutex
	ttl      time.Duration
	claimTTL time.Duration
	at       map[string]seenEntry
	// kv persists the set. Nil: the set is in memory only (tests).
	kv state.KV
	// now is injectable for tests; nil ⇒ time.Now.
	now func() time.Time
}

func newSeenSet(ttl time.Duration, kv state.KV) *seenSet {
	return &seenSet{ttl: ttl, claimTTL: webhookClaimTTL, at: make(map[string]seenEntry), kv: kv}
}

func (s *seenSet) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// claim reports where id stands. When it was neither done nor claimed, it
// claims id for the caller and saves the claim before it returns [seenNew].
// It fails when the stored set cannot be read or written: a replay could
// then pass unnoticed.
func (s *seenSet) claim(ctx context.Context, id string) (seenState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(ctx); err != nil {
		return seenNew, err
	}
	now := s.clock()
	s.sweepLocked(now)
	if e, ok := s.at[id]; ok {
		if e.Done {
			return seenDone, nil
		}
		return seenInFlight, nil
	}
	s.at[id] = seenEntry{At: now}
	if err := s.saveLocked(ctx); err != nil {
		delete(s.at, id)
		return seenNew, err
	}
	return seenNew, nil
}

// finish marks a claimed id done, so a redelivery within the TTL is deduped.
func (s *seenSet) finish(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(ctx); err != nil {
		return err
	}
	now := s.clock()
	s.sweepLocked(now)
	s.at[id] = seenEntry{At: now, Done: true}
	return s.saveLocked(ctx)
}

// release drops the claim on id after its handler failed, so the IdP's
// retry runs. A done id is left as it is. When the release cannot be saved,
// the claim lapses after claimTTL.
func (s *seenSet) release(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(ctx); err != nil {
		return err
	}
	s.sweepLocked(s.clock())
	if e, ok := s.at[id]; !ok || e.Done {
		return nil
	}
	delete(s.at, id)
	return s.saveLocked(ctx)
}

func (s *seenSet) sweepLocked(now time.Time) {
	for k, e := range s.at {
		ttl := s.ttl
		if !e.Done {
			ttl = s.claimTTL
		}
		if now.Sub(e.At) > ttl {
			delete(s.at, k)
		}
	}
	for len(s.at) >= webhookSeenMax {
		var oldest string
		for k, e := range s.at {
			if oldest == "" || e.At.Before(s.at[oldest].At) {
				oldest = k
			}
		}
		delete(s.at, oldest)
	}
}

// loadLocked merges the persisted set into memory. The kv copy is
// authoritative for ids another process or an earlier App recorded or forgot,
// so it replaces the in-memory map when it reads. A missing set is empty; a
// read that fails is an error.
func (s *seenSet) loadLocked(ctx context.Context) error {
	if s.kv == nil {
		return nil
	}
	data, err := s.kv.Get(ctx, webhookSeenKey)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read the dedup set: %w", err)
	}
	var at map[string]seenEntry
	if err := json.Unmarshal(data, &at); err != nil {
		slog.Warn("idp webhook: the stored dedup set is unreadable; starting a new one", "error", err)
		at = nil
	}
	s.at = at
	if s.at == nil {
		s.at = make(map[string]seenEntry)
	}
	return nil
}

func (s *seenSet) saveLocked(ctx context.Context) error {
	if s.kv == nil {
		return nil
	}
	data, err := json.Marshal(s.at)
	if err == nil {
		err = s.kv.Put(ctx, webhookSeenKey, data)
	}
	if err != nil {
		return fmt.Errorf("save the dedup set: %w", err)
	}
	return nil
}
