// Package tenanthost is the front handler of a multi-tenant rela-server
// (TKT-D6MF3SW). It verifies the identity assertion once, reads the verified
// `org_id`, leases that tenant's resident application from the registry and
// serves the request on the tenant's own router.
//
// # Fail closed
//
// No request is ever served from a default store. A request without a
// verified org, with an org the resolver does not serve, or whose tenant
// cannot be opened is refused, and the refusal opens no store:
//
//   - no assertion, or one that does not verify: 401;
//   - a verified assertion with no `org_id`, or an org that is not an active
//     tenant (unknown, suspended or being erased): 403 `unknown_tenant`;
//   - the tenant lookup or the store open failed, or the org's derived
//     schema is recorded for another org: 503.
//
// # Provisioning
//
// With [Config.Provision] set, an org that has no tenant yet can become one
// (TKT-TNPRV8). The IdP's verified `organization.created` webhook starts it,
// and with [Config.LazyProvision] so does the first verified request of
// such an org. Neither holds the request open across the create: the answer
// is `202` with a `tenant_provisioning` problem whose Location is
// [StatusPath], which the SPA polls until it reports `ready`. The trust
// assumption this rests on is on [tenant.AutoProvisioner].
//
// The one exception is the embedded SPA shell and its static assets, which a
// browser needs before it can authenticate. They are served by
// [dataentry.StaticShell], which returns only bytes compiled into the binary,
// so a request routed there cannot read any tenant's data whatever its path.
// Which paths go there decides only whether a request without a tenant sees
// the shell or a 401; it is not a confidentiality boundary.
//
// # Isolation
//
// Isolation is delivered by the store a tenant's application reads: each
// tenant's store has a `search_path` pinned to its own schema. This package
// only chooses which tenant's application serves a request. `org_id` is not
// an ACL condition.
package tenanthost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// WebhookPath is the IdP webhook route. Its body is a signed JWT that names
// the tenant, so the host routes it by the body rather than by a header.
const WebhookPath = "/webhooks/idp"

// StatusPath reports where the caller's tenant stands: `ready`,
// `provisioning`, `failed` or `throttled`. The SPA polls it after a `202`.
const StatusPath = "/api/v1/_tenant/status"

// provisionPollAfter is the Retry-After a `202` asks the SPA to wait before
// it polls [StatusPath] again.
const provisionPollAfter = "2"

// webhookProvisionWait bounds how long an `organization.created` webhook
// waits for its create. The IdP is told 200 only once the tenant exists, so
// a create that fails later is delivered again rather than lost.
const webhookProvisionWait = 10 * time.Second

// webhookMaxBody caps the webhook body read before verification, matching
// the data-entry receiver's own cap.
const webhookMaxBody = 64 << 10

// AssertionVerifier verifies an identity assertion. The production value is
// the same adapter the data-entry JWT gate uses.
type AssertionVerifier interface {
	VerifyAssertion(ctx context.Context, raw string) (dataentry.AssertedIdentity, error)
}

// WebhookVerifier verifies a signed IdP webhook body.
type WebhookVerifier interface {
	VerifyWebhook(ctx context.Context, raw string) (dataentry.WebhookClaims, error)
}

// Provisioning creates tenants in the background. [tenant.AutoProvisioner]
// implements it.
type Provisioning interface {
	Start(orgID string) tenant.ProvisionState
	State(orgID string) tenant.ProvisionState
	Wait(ctx context.Context, orgID string) tenant.ProvisionState
	RetryAfter() time.Duration
}

// Acquire leases the resident application of the tenant orgID names. It
// returns that tenant's handler and a release func the host calls when the
// request ends. It must return [tenant.ErrUnknownTenant] (wrapped or not)
// for an org the tenant map does not name, and must open nothing for one.
type Acquire func(ctx context.Context, orgID string) (h http.Handler, release func(), err error)

// Config wires a [Host].
type Config struct {
	// Verifier verifies the assertion header. Required.
	Verifier AssertionVerifier
	// HeaderName is the header carrying the assertion. Required.
	HeaderName string
	// Acquire leases a tenant's application. Required.
	Acquire Acquire
	// Static serves the tenant-free shell. Required; production passes
	// [dataentry.StaticShell].
	Static http.Handler
	// Webhook verifies the IdP webhook body. Optional: nil leaves
	// [WebhookPath] an ordinary path that needs an assertion.
	Webhook WebhookVerifier
	// Provision creates the tenant of an `organization.created` webhook for
	// an org that has none. Optional: nil provisions nothing.
	Provision Provisioning
	// LazyProvision also creates a tenant for a verified request of an org
	// that has none. It needs Provision.
	LazyProvision bool
}

// Host is the front http.Handler of a multi-tenant server.
type Host struct {
	cfg Config
}

// New builds a Host.
//
// Nil: rejected — every required collaborator is checked here, so a
// misconfigured host fails at startup rather than on a request.
func New(cfg Config) (*Host, error) {
	switch {
	case cfg.Verifier == nil:
		return nil, errors.New("tenanthost: verifier is required")
	case cfg.HeaderName == "":
		return nil, errors.New("tenanthost: header name is required")
	case cfg.Acquire == nil:
		return nil, errors.New("tenanthost: acquire is required")
	case cfg.Static == nil:
		return nil, errors.New("tenanthost: static handler is required")
	case cfg.LazyProvision && cfg.Provision == nil:
		return nil, errors.New("tenanthost: lazy provisioning needs a provisioner")
	}
	return &Host{cfg: cfg}, nil
}

// ServeHTTP routes r to its tenant's application, or refuses it.
//
// A tenant-free path whose request has no usable tenant (no assertion, a bad
// one, no org, an unknown org, a tenant that cannot open) is served the
// static shell instead of the refusal, so the browser can load and show why
// its API calls are refused. A request with a tenant gets the tenant's own
// router for every path, including the shell with the operator's `custom/`
// injection.
func (h *Host) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	clean := cleanPath(r.URL.Path)
	if h.cfg.Webhook != nil && r.Method == http.MethodPost && clean == WebhookPath {
		h.serveWebhook(w, r)
		return
	}
	// shell serves the static shell instead of a refusal on a tenant-free
	// path, and reports whether it did.
	shell := func() bool {
		if !tenantFree(clean) {
			return false
		}
		h.cfg.Static.ServeHTTP(w, r)
		return true
	}

	raw := stripBearer(r.Header.Get(h.cfg.HeaderName))
	if raw == "" {
		if !shell() {
			slog.DebugContext(r.Context(), "tenant host: no assertion presented",
				"path", r.URL.Path, "method", r.Method)
			h.unauthenticated(w, r)
		}
		return
	}

	id, err := h.cfg.Verifier.VerifyAssertion(r.Context(), raw)
	if err != nil {
		if !shell() {
			// The reason stays in the log: the token is attacker input, and
			// echoing why it failed is an oracle.
			slog.InfoContext(r.Context(), "tenant host: assertion rejected",
				"path", r.URL.Path, "method", r.Method, "error", err)
			h.unauthenticated(w, r)
		}
		return
	}

	if id.OrgID == "" {
		if !shell() {
			slog.InfoContext(r.Context(), "tenant host: verified assertion carries no org_id",
				"path", r.URL.Path, "method", r.Method)
			writeError(w, r, http.StatusForbidden, "unknown_tenant", "No tenant for this organization")
		}
		return
	}

	status := r.Method == http.MethodGet && clean == StatusPath
	if status && h.cfg.Provision != nil {
		if st := h.cfg.Provision.State(id.OrgID); st != tenant.ProvisionNone {
			h.provisioning(w, r, st)
			return
		}
	}
	next, release, err := h.cfg.Acquire(r.Context(), id.OrgID)
	if err != nil {
		if !shell() {
			h.acquireFailed(w, r, id.OrgID, err)
		}
		return
	}
	defer release()
	if status {
		writeStatus(w, http.StatusOK, "ready")
		return
	}
	next.ServeHTTP(w, r.WithContext(withVerified(r.Context(), raw, id)))
}

// provisionable reports whether err from Acquire names an org that may be
// provisioned: one with no tenant, or one whose create did not finish. A
// suspended or erasing tenant never is.
func provisionable(err error) bool {
	return errors.Is(err, tenant.ErrTenantNotFound) || errors.Is(err, tenant.ErrTenantProvisioning)
}

// provisioning answers a request whose tenant is being provisioned, or
// whose provisioning could not start.
func (h *Host) provisioning(w http.ResponseWriter, r *http.Request, st tenant.ProvisionState) {
	switch st {
	case tenant.ProvisionRunning:
		w.Header().Set("Location", StatusPath)
		w.Header().Set("Retry-After", provisionPollAfter)
		if r.URL.Path == StatusPath {
			writeStatus(w, http.StatusAccepted, string(st))
			return
		}
		writeError(w, r, http.StatusAccepted, "tenant_provisioning", "Your organization is being set up")
	case tenant.ProvisionThrottled:
		w.Header().Set("Retry-After", "30")
		writeError(w, r, http.StatusServiceUnavailable, "tenant_provisioning_busy",
			"Too many organizations are being set up; try again shortly")
	default:
		w.Header().Set("Retry-After", h.retryAfter())
		writeError(w, r, http.StatusServiceUnavailable, "tenant_provisioning_failed",
			"Setting up your organization failed; try again later")
	}
}

// serveWebhook routes an IdP webhook by the org its verified body names.
// The tenant's own receiver verifies the body again: the webhook verifier
// keeps no replay state, so a second verification of the same body is
// harmless, and it keeps the receiver's checks in one place.
func (h *Host) serveWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, webhookMaxBody))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "read failed", http.StatusBadRequest)
		return
	}
	claims, err := h.cfg.Webhook.VerifyWebhook(r.Context(), string(body))
	if err != nil {
		slog.Warn("tenant host: idp webhook rejected", "error", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if claims.OrgID == "" {
		// Without an org there is no tenant to act in, and the host has no
		// default to fall back to. Logged because an IdP rarely retries a
		// 400.
		slog.Warn("tenant host: idp webhook carries no org_id", "event", claims.Event, "id", claims.ID)
		http.Error(w, "missing org_id", http.StatusBadRequest)
		return
	}

	next, release, err := h.cfg.Acquire(r.Context(), claims.OrgID)
	switch {
	case h.cfg.Provision != nil && claims.Event == "organization.created" && provisionable(err):
		h.provisionFromWebhook(w, r, claims)
		return
	case errors.Is(err, tenant.ErrTenantInactive):
		// A suspended tenant keeps its data, and a role removal or user
		// deletion sent now must still apply when it is resumed. Refused
		// with a retryable status so the IdP delivers it again, never
		// acknowledged and dropped.
		slog.Warn("tenant host: idp webhook for a tenant that is not served now; asking the IdP to retry",
			"org_id", claims.OrgID, "event", claims.Event, "id", claims.ID)
		w.Header().Set("Retry-After", "300")
		http.Error(w, "tenant not served now", http.StatusServiceUnavailable)
		return
	case errors.Is(err, tenant.ErrUnknownTenant):
		// Acknowledged so the IdP does not retry an event for an org this
		// server does not host. With provisioning on, organization.created
		// was handled above.
		if claims.Event == "organization.deleted" {
			// An org being erased, or one this server never had: worth an
			// operator's look, since an erasure may be in progress.
			slog.Warn("tenant host: idp webhook organization deletion for an org this server does not "+
				"serve; check that its erasure completed (PROCEDURE-u020, rela-server tenant erase)",
				"org_id", claims.OrgID, "id", claims.ID)
		} else {
			slog.Info("tenant host: idp webhook for an org this server does not host",
				"org_id", claims.OrgID, "event", claims.Event, "id", claims.ID)
		}
		w.WriteHeader(http.StatusOK)
		return
	case err != nil:
		slog.Error("tenant host: idp webhook tenant unavailable",
			"org_id", claims.OrgID, "event", claims.Event, "error", err)
		http.Error(w, "tenant unavailable", http.StatusBadGateway)
		return
	}
	defer release()
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	next.ServeHTTP(w, r)
}

// provisionFromWebhook provisions the org of a verified
// `organization.created` webhook. It waits up to [webhookProvisionWait] for
// the create and answers `200` once the tenant exists. A create still
// running, throttled or failed is `503` with Retry-After, so the IdP
// delivers the event again and a failed create is never acknowledged.
func (h *Host) provisionFromWebhook(w http.ResponseWriter, r *http.Request, claims dataentry.WebhookClaims) {
	st := h.cfg.Provision.Start(claims.OrgID)
	if st == tenant.ProvisionRunning {
		ctx, cancel := context.WithTimeout(r.Context(), webhookProvisionWait)
		st = h.cfg.Provision.Wait(ctx, claims.OrgID)
		cancel()
	}
	//nolint:gosec // G706: constant message; the verified org and webhook ids are escaped attributes
	slog.Info("tenant host: idp webhook organization.created; provisioning",
		"org_id", claims.OrgID, "id", claims.ID, "state", string(st))
	switch st {
	case tenant.ProvisionNone:
		w.WriteHeader(http.StatusOK)
	case tenant.ProvisionRunning:
		w.Header().Set("Retry-After", provisionPollAfter)
		http.Error(w, "provisioning", http.StatusServiceUnavailable)
	case tenant.ProvisionThrottled:
		w.Header().Set("Retry-After", "60")
		http.Error(w, "provisioning busy", http.StatusServiceUnavailable)
	default:
		w.Header().Set("Retry-After", h.retryAfter())
		http.Error(w, "provisioning failed", http.StatusServiceUnavailable)
	}
}

// retryAfter is the Retry-After for a failed create, at least one second.
func (h *Host) retryAfter() string {
	return strconv.Itoa(max(1, int(h.cfg.Provision.RetryAfter().Seconds())))
}

func (h *Host) acquireFailed(w http.ResponseWriter, r *http.Request, orgID string, err error) {
	if h.cfg.LazyProvision && provisionable(err) {
		st := h.cfg.Provision.Start(orgID)
		slog.InfoContext(r.Context(), "tenant host: verified org has no tenant; provisioning",
			"org_id", orgID, "path", r.URL.Path, "state", string(st))
		h.provisioning(w, r, st)
		return
	}
	if errors.Is(err, tenant.ErrUnknownTenant) {
		slog.InfoContext(r.Context(), "tenant host: org is not a tenant of this server",
			"org_id", orgID, "path", r.URL.Path)
		writeError(w, r, http.StatusForbidden, "unknown_tenant", "No tenant for this organization")
		return
	}
	// Logged only, never sent: an open error can name the database host,
	// user and database (pgx redacts the password).
	slog.ErrorContext(r.Context(), "tenant host: tenant unavailable",
		"org_id", orgID, "path", r.URL.Path, "error", err)
	w.Header().Set("Retry-After", "5")
	writeError(w, r, http.StatusServiceUnavailable, "tenant_unavailable", "Tenant unavailable")
}

func (h *Host) unauthenticated(w http.ResponseWriter, r *http.Request) {
	if strings.EqualFold(h.cfg.HeaderName, "Authorization") {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	writeError(w, r, http.StatusUnauthorized, "unauthenticated", "Authentication required")
}

// tenantPrefixes are the routes that serve tenant data or act on it. A
// request for one without a verified tenant is refused rather than shown the
// shell.
var tenantPrefixes = []string{"/api/", "/webhooks/", "/hooks/", "/_custom/", "/.well-known/"}

// tenantFree reports whether a request for clean may be served the static
// shell without a tenant. The bare `/api` is listed apart because the prefix
// form does not match it.
func tenantFree(clean string) bool {
	if clean == "/api" {
		return false
	}
	for _, p := range tenantPrefixes {
		if strings.HasPrefix(clean+"/", p) {
			return false
		}
	}
	return true
}

// cleanPath resolves dot segments and duplicate slashes, so `/static/../api`
// is judged as `/api`.
func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	return path.Clean(p)
}

// stripBearer removes an optional "Bearer " scheme, as the data-entry gate
// does. If the two ever differed, the memo would miss and the tenant gate
// would verify again: slower, not wrong.
func stripBearer(v string) string {
	v = strings.TrimSpace(v)
	const scheme = "bearer"
	if len(v) > len(scheme) && strings.EqualFold(v[:len(scheme)], scheme) {
		rest := v[len(scheme):]
		if trimmed := strings.TrimLeft(rest, " \t"); trimmed != rest {
			return strings.TrimSpace(trimmed)
		}
	}
	return v
}

// problem is the RFC 9457 body the data-entry API answers errors with.
type problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Instance string `json:"instance,omitempty"`
}

// tenantStatus is the body of [StatusPath].
type tenantStatus struct {
	Status string `json:"status"`
}

func writeStatus(w http.ResponseWriter, code int, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(tenantStatus{Status: status})
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, title string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem{
		Type: "https://rela.dev/errors/" + code, Title: title, Status: status, Instance: r.URL.Path,
	})
}
