package tenanthost

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// eventOrgCreated is the IdP event that starts provisioning.
const eventOrgCreated = "organization.created"

// The tenant statuses [StatusPath] and a 202 answer with.
const (
	statusActive       = "active"
	statusProvisioning = "provisioning"
	statusSuspended    = "suspended"
)

// Retry-After values, in seconds. A provisioning run takes seconds, so the
// client polls quickly; a refusal at the cap waits longer. A webhook whose
// create needs an operator waits longest: until an operator acts, each
// retry meets the failure hold and starts no run.
const (
	retryProvisioning  = "2"
	retryBusy          = "10"
	retryNeedsOperator = "300"
)

// tenantStatus is the body of [StatusPath] and of a 202 answer. The plan
// fields are set for an active tenant when the tenant API is on: they are
// what the control plane stored, shown to the tenant's own users.
type tenantStatus struct {
	Status       string              `json:"status"`
	ReadOnly     bool                `json:"read_only,omitempty"`
	Plan         string              `json:"plan,omitempty"`
	Entitlements tenant.Entitlements `json:"entitlements,omitempty"`
}

func writeStatus(w http.ResponseWriter, code int, status string) {
	writeStatusBody(w, code, tenantStatus{Status: status})
}

func writeStatusBody(w http.ResponseWriter, code int, body tenantStatus) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if body.Status == statusProvisioning {
		w.Header().Set("Retry-After", retryProvisioning)
	}
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// activeStatus is the [StatusPath] body of orgID's active tenant. With the
// tenant API on it adds the access state and plan from the control row; a
// failed lookup leaves them out rather than failing the answer.
func (h *Host) activeStatus(r *http.Request, orgID string) tenantStatus {
	body := tenantStatus{Status: statusActive}
	if h.cfg.TenantAPI == nil {
		return body
	}
	rec, err := h.cfg.TenantAPI.Control.Lookup(r.Context(), orgID)
	if err != nil {
		slog.WarnContext(r.Context(), "tenant host: plan lookup failed", "org_id", orgID, "error", err)
		return body
	}
	body.ReadOnly, body.Plan, body.Entitlements = rec.ReadOnly, rec.Plan, rec.Entitlements
	return body
}

// writeProvisioning answers a request for a tenant being created. A read
// gets code with the provisioning status. Any other method gets 503
// `tenant_provisioning`: a 202 would read as success, and nothing was
// written.
func writeProvisioning(w http.ResponseWriter, r *http.Request, code int) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Retry-After", retryProvisioning)
		writeError(w, r, http.StatusServiceUnavailable, "tenant_provisioning", "Tenant is being created")
		return
	}
	writeStatus(w, code, statusProvisioning)
}

// serveStatus answers the status of the caller's own org, orgID, which the
// host verified. An active tenant is opened to tell, as any request for it
// would. With lazy provisioning an unknown org starts its create here too.
func (h *Host) serveStatus(w http.ResponseWriter, r *http.Request, orgID string) {
	_, release, err := h.cfg.Acquire(r.Context(), orgID)
	switch {
	case err == nil:
		release()
		writeStatusBody(w, http.StatusOK, h.activeStatus(r, orgID))
	case h.prov != nil && h.prov.lazy && provisionable(err):
		h.provisionLazily(w, r, orgID, http.StatusOK)
	case errors.Is(err, tenant.ErrTenantProvisioning):
		if h.prov != nil {
			h.prov.noteStuck(r.Context(), orgID)
		}
		writeStatus(w, http.StatusOK, statusProvisioning)
	case errors.Is(err, tenant.ErrTenantInactive):
		writeStatus(w, http.StatusOK, statusSuspended)
	default:
		h.acquireFailed(w, r, orgID, err)
	}
}

// provisionLazily starts the create of orgID's tenant and answers code with
// a provisioning status at once. It never waits for the run: a migration
// can outlast a client's timeout.
//
// It relies on the trust assumption written at [provisioning].
func (h *Host) provisionLazily(w http.ResponseWriter, r *http.Request, orgID string, code int) {
	if _, err := tenant.SchemaFor(orgID); err != nil {
		slog.InfoContext(r.Context(), "tenant host: org id cannot be a tenant", "error", err)
		writeError(w, r, http.StatusForbidden, "unknown_tenant", "No tenant for this organization")
		return
	}
	_, err := h.prov.start(r.Context(), orgID)
	var held *heldFailureError
	switch {
	case errors.Is(err, errProvisionElsewhere), errors.Is(err, errProvisionedRecently):
		writeProvisioning(w, r, code)
	case errors.Is(err, errProvisionBusy):
		slog.WarnContext(r.Context(), "tenant host: provisioning at its limit", "org_id", orgID)
		w.Header().Set("Retry-After", retryBusy)
		writeError(w, r, http.StatusServiceUnavailable, "provisioning_busy", "Tenant provisioning is busy")
	case errors.Is(err, ErrNeedsOperator):
		// No Retry-After: a retry cannot help until an operator acts. The
		// cause and the fix are in the log of the run.
		writeError(w, r, http.StatusServiceUnavailable, "provisioning_needs_operator",
			"Tenant provisioning needs an operator")
	case errors.As(err, &held):
		// The cause is in the log of the run; it can name the database.
		w.Header().Set("Retry-After", held.retryAfter())
		writeError(w, r, http.StatusServiceUnavailable, "provisioning_failed", "Tenant provisioning failed")
	case err != nil:
		slog.ErrorContext(r.Context(), "tenant host: provisioning not started", "org_id", orgID, "error", err)
		w.Header().Set("Retry-After", retryBusy)
		writeError(w, r, http.StatusServiceUnavailable, "tenant_unavailable", "Tenant unavailable")
	default:
		writeProvisioning(w, r, code)
	}
}

// provisionFromWebhook creates the tenant a verified `organization.created`
// names and waits a bounded time for it. Success is 200. A run still going
// is 202: the IdP does not retry, and lazy provisioning or the next event
// finishes it. A failure, a refusal at the cap and a create that needs an
// operator ask the IdP to retry. A create that finds the tenant existing
// but not active is 200 with a body that says so.
//
// It relies on the trust assumption written at [provisioning].
func (h *Host) provisionFromWebhook(w http.ResponseWriter, r *http.Request, claims dataentry.WebhookClaims) {
	if _, err := tenant.SchemaFor(claims.OrgID); err != nil {
		// Acknowledged like any event for an org this server cannot host.
		slog.Warn("tenant host: idp webhook org id cannot be a tenant", "id", claims.ID, "error", err)
		w.WriteHeader(http.StatusOK)
		return
	}
	run, err := h.prov.start(r.Context(), claims.OrgID)
	switch {
	case errors.Is(err, errProvisionBusy):
		slog.Warn("tenant host: idp webhook provisioning at its limit; asking the IdP to retry",
			"org_id", claims.OrgID, "id", claims.ID)
		w.Header().Set("Retry-After", retryBusy)
		http.Error(w, "provisioning busy", http.StatusServiceUnavailable)
		return
	case errors.Is(err, errProvisionElsewhere):
		// Another server is creating the tenant, as for a run that found
		// it so below.
		w.WriteHeader(http.StatusAccepted)
		return
	case errors.Is(err, errProvisionedRecently):
		// Not acknowledged: this request read the tenant as absent, and
		// the retry after the hold reads it again (see successHold).
		w.Header().Set("Retry-After", strconv.Itoa(int(successHold/time.Second)))
		http.Error(w, "tenant provisioned moments ago; retry", http.StatusServiceUnavailable)
		return
	case err != nil:
		webhookProvisionFailed(w, err)
		return
	}
	timer := time.NewTimer(h.prov.wait)
	defer timer.Stop()
	select {
	case <-run.done:
	case <-timer.C:
		w.WriteHeader(http.StatusAccepted)
		return
	case <-r.Context().Done():
		// The IdP hung up; the run goes on.
		return
	}
	switch {
	case run.err == nil:
		w.WriteHeader(http.StatusOK)
	case errors.Is(run.err, tenant.ErrTenantInUse):
		w.WriteHeader(http.StatusAccepted)
	default:
		webhookProvisionFailed(w, run.err)
	}
}

// webhookProvisionFailed answers a webhook whose create failed with err.
// The cause stays in the log of the run.
func webhookProvisionFailed(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNeedsOperator):
		// Not acknowledged: the IdP keeps the event until an operator has
		// fixed the cause. The failure hold keeps the retries cheap; each
		// one meets the hold and starts no run.
		w.Header().Set("Retry-After", retryNeedsOperator)
		http.Error(w, "not provisioned: operator action is needed; see the server log",
			http.StatusServiceUnavailable)
	case errors.Is(err, tenant.ErrTenantExists):
		// Not a dropped event: the org has a tenant, and its status is
		// governed by status sync and operators, not by this create.
		http.Error(w, "not provisioned: the tenant exists but is not active", http.StatusOK)
	default:
		http.Error(w, "provisioning failed", http.StatusBadGateway)
	}
}
