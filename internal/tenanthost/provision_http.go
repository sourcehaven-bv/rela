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
// SPA polls quickly; a refusal at the cap or after a failure waits longer.
const (
	retryProvisioning = "2"
	retryBusy         = "10"
)

// tenantStatus is the body of [StatusPath] and of a 202 answer.
type tenantStatus struct {
	Status string `json:"status"`
}

func writeStatus(w http.ResponseWriter, code int, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if status == statusProvisioning {
		w.Header().Set("Retry-After", retryProvisioning)
	}
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(tenantStatus{Status: status})
}

// serveStatus answers the status of the caller's own org, orgID, which the
// host verified. An active tenant is opened to tell, as any request for it
// would. With lazy provisioning an unknown org starts its create here too.
func (h *Host) serveStatus(w http.ResponseWriter, r *http.Request, orgID string) {
	_, release, err := h.cfg.Acquire(r.Context(), orgID)
	switch {
	case err == nil:
		release()
		writeStatus(w, http.StatusOK, statusActive)
	case h.prov != nil && h.prov.lazy && provisionable(err):
		h.provisionLazily(w, r, orgID, http.StatusOK)
	case errors.Is(err, tenant.ErrTenantProvisioning):
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
	switch {
	case errors.Is(err, errProvisionElsewhere):
		writeStatus(w, code, statusProvisioning)
	case errors.Is(err, errProvisionBusy):
		slog.WarnContext(r.Context(), "tenant host: provisioning at its limit", "org_id", orgID)
		w.Header().Set("Retry-After", retryBusy)
		writeError(w, r, http.StatusServiceUnavailable, "provisioning_busy", "Tenant provisioning is busy")
	case errors.Is(err, errProvisionFailed):
		// The cause is in the log of the run; it can name the database.
		w.Header().Set("Retry-After", strconv.Itoa(int(failureHold/time.Second)))
		writeError(w, r, http.StatusServiceUnavailable, "provisioning_failed", "Tenant provisioning failed")
	case err != nil:
		slog.ErrorContext(r.Context(), "tenant host: provisioning not started", "org_id", orgID, "error", err)
		w.Header().Set("Retry-After", retryBusy)
		writeError(w, r, http.StatusServiceUnavailable, "tenant_unavailable", "Tenant unavailable")
	default:
		writeStatus(w, code, statusProvisioning)
	}
}

// provisionFromWebhook creates the tenant a verified `organization.created`
// names and waits a bounded time for it. Success is 200. A run still going
// is 202: the IdP does not retry, and lazy provisioning or the next event
// finishes it. A failure or a refusal at the cap asks the IdP to retry.
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
	case err != nil:
		http.Error(w, "provisioning failed", http.StatusBadGateway)
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
		http.Error(w, "provisioning failed", http.StatusBadGateway)
	}
}
