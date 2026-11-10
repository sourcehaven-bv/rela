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
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// WebhookPath is the IdP webhook route. Its body is a signed JWT that names
// the tenant, so the host routes it by the body rather than by a header.
const WebhookPath = "/webhooks/idp"

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

	next, release, err := h.cfg.Acquire(r.Context(), id.OrgID)
	if err != nil {
		if !shell() {
			h.acquireFailed(w, r, id.OrgID, err)
		}
		return
	}
	defer release()
	next.ServeHTTP(w, r.WithContext(withVerified(r.Context(), raw, id)))
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
		// server does not host. Provisioning a tenant is TKT-TNPRV8.
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

func (h *Host) acquireFailed(w http.ResponseWriter, r *http.Request, orgID string, err error) {
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

func writeError(w http.ResponseWriter, r *http.Request, status int, code, title string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem{
		Type: "https://rela.dev/errors/" + code, Title: title, Status: status, Instance: r.URL.Path,
	})
}
