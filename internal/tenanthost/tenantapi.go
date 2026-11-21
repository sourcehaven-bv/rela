package tenanthost

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// TenantsPath is the prefix of the tenant API (TKT-84CD0PY). An external
// provisioning job creates an org's tenant with `PUT TenantsPath+<org_id>`
// before the org's first visit, and reads its status with `GET`.
//
// # Authentication
//
// The caller sends `Authorization: Bearer <JWT>`. The JWT is verified like
// the IdP webhook: ES256, the identity issuer and JWKS, a required expiry,
// and an audience of its own ([Config.TenantAPI]). The audience must differ
// from the identity and webhook audiences, so an end user's assertion or a
// webhook body is never accepted here. The token's `org_id` claim must name
// the org in the path, so one token acts on one org only. An end user holds
// no such token: only the IdP's signing key can mint one.
//
// Creating a tenant goes through the host's provisioning runner, so it
// shares the start rate, the running cap and the failure holds with lazy
// and webhook provisioning.
const TenantsPath = "/api/v1/_tenants/"

// The extra tenant statuses the tenant API reports.
const statusErased = "erased"

// TenantAPIClaims is what the tenant API acts on from a verified token.
type TenantAPIClaims struct {
	// OrgID is the `org_id` claim, the only org the token may act on.
	OrgID string
	// ID is the `jti` claim, if any, for the log.
	ID string
}

// TenantAPIVerifier verifies the bearer token of a tenant API request. The
// production value is a jwtauth verifier pinned to the tenant API audience.
type TenantAPIVerifier interface {
	VerifyTenantAPI(ctx context.Context, raw string) (TenantAPIClaims, error)
}

// tenantAPIBody is the body of a tenant API answer.
type tenantAPIBody struct {
	OrgID  string `json:"org_id"`
	Status string `json:"status"`
}

func writeTenantAPI(w http.ResponseWriter, code int, orgID, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if status == statusProvisioning {
		w.Header().Set("Retry-After", retryProvisioning)
		w.Header().Set("Location", TenantsPath+orgID)
	}
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(tenantAPIBody{OrgID: orgID, Status: status})
}

// serveTenantAPI authenticates a request under [TenantsPath] and routes it
// by method. clean is the cleaned path.
func (h *Host) serveTenantAPI(w http.ResponseWriter, r *http.Request, clean string) {
	orgID := strings.TrimPrefix(clean, TenantsPath)
	if orgID == "" || strings.Contains(orgID, "/") {
		writeError(w, r, http.StatusNotFound, "not_found", "Not found")
		return
	}
	raw := stripBearer(r.Header.Get("Authorization"))
	if raw == "" {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, r, http.StatusUnauthorized, "unauthenticated", "Authentication required")
		return
	}
	claims, err := h.cfg.TenantAPI.VerifyTenantAPI(r.Context(), raw)
	if err != nil {
		// The reason stays in the log, as for an assertion.
		slog.InfoContext(r.Context(), "tenant api: token rejected", "method", r.Method, "error", err)
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, r, http.StatusUnauthorized, "unauthenticated", "Authentication required")
		return
	}
	if claims.OrgID != orgID {
		slog.WarnContext(r.Context(), "tenant api: token names another org",
			"path_org_id", orgID, "token_org_id", claims.OrgID, "id", claims.ID)
		writeError(w, r, http.StatusForbidden, "forbidden", "The token is not for this organization")
		return
	}
	// From here on the org id is the verified claim, equal to the path.
	orgID = claims.OrgID
	if _, err := tenant.SchemaFor(orgID); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_org_id", "The organization id cannot be a tenant")
		return
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		h.tenantAPIStatus(w, r, orgID)
	case http.MethodPut:
		slog.InfoContext(r.Context(), "tenant api: create requested", "org_id", orgID, "id", claims.ID)
		h.tenantAPICreate(w, r, orgID)
	default:
		w.Header().Set("Allow", "GET, HEAD, PUT")
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

// tenantAPIStatus answers the status of orgID's tenant. An active tenant is
// opened to tell, as [StatusPath] does. It never starts a create.
func (h *Host) tenantAPIStatus(w http.ResponseWriter, r *http.Request, orgID string) {
	_, release, err := h.cfg.Acquire(r.Context(), orgID)
	switch {
	case err == nil:
		release()
		writeTenantAPI(w, http.StatusOK, orgID, statusActive)
	case errors.Is(err, tenant.ErrTenantProvisioning):
		writeTenantAPI(w, http.StatusOK, orgID, statusProvisioning)
	case errors.Is(err, tenant.ErrTenantInactive):
		writeTenantAPI(w, http.StatusOK, orgID, statusSuspended)
	case errors.Is(err, tenant.ErrTenantErased):
		writeTenantAPI(w, http.StatusOK, orgID, statusErased)
	case errors.Is(err, tenant.ErrTenantAbsent) && h.prov.running(orgID):
		writeTenantAPI(w, http.StatusOK, orgID, statusProvisioning)
	case errors.Is(err, tenant.ErrUnknownTenant):
		writeError(w, r, http.StatusNotFound, "unknown_tenant", "No tenant for this organization")
	default:
		h.tenantAPIUnavailable(w, r, orgID, err)
	}
}

// tenantAPICreate creates orgID's tenant and waits a bounded time for it,
// as an `organization.created` webhook does. 201 means it was created now,
// 200 that it was already active, 202 that the create is still running.
func (h *Host) tenantAPICreate(w http.ResponseWriter, r *http.Request, orgID string) {
	_, release, err := h.cfg.Acquire(r.Context(), orgID)
	switch {
	case err == nil:
		release()
		writeTenantAPI(w, http.StatusOK, orgID, statusActive)
		return
	case provisionable(err):
		// Created below.
	case errors.Is(err, tenant.ErrTenantErased):
		writeError(w, r, http.StatusConflict, "tenant_erased",
			"The tenant was erased; only an operator can recreate it")
		return
	case errors.Is(err, tenant.ErrTenantInactive):
		writeError(w, r, http.StatusConflict, "tenant_suspended", "The tenant exists but is suspended")
		return
	default:
		h.tenantAPIUnavailable(w, r, orgID, err)
		return
	}

	run, err := h.prov.start(r.Context(), orgID)
	if err != nil {
		h.tenantAPIStartRefused(w, r, orgID, err)
		return
	}
	timer := time.NewTimer(h.prov.wait)
	defer timer.Stop()
	select {
	case <-run.done:
	case <-timer.C:
		writeTenantAPI(w, http.StatusAccepted, orgID, statusProvisioning)
		return
	case <-r.Context().Done():
		// The caller hung up; the run goes on.
		return
	}
	switch {
	case run.err == nil:
		writeTenantAPI(w, http.StatusCreated, orgID, statusActive)
	case errors.Is(run.err, tenant.ErrTenantInUse):
		writeTenantAPI(w, http.StatusAccepted, orgID, statusProvisioning)
	case errors.Is(run.err, tenant.ErrTenantExists):
		writeError(w, r, http.StatusConflict, "tenant_suspended", "The tenant exists but is not active")
	case errors.Is(run.err, ErrNeedsOperator):
		writeError(w, r, http.StatusServiceUnavailable, "provisioning_needs_operator",
			"Tenant provisioning needs an operator")
	default:
		w.Header().Set("Retry-After", (&heldFailureError{left: failureHold}).retryAfter())
		writeError(w, r, http.StatusServiceUnavailable, "provisioning_failed", "Tenant provisioning failed")
	}
}

// tenantAPIStartRefused answers a create whose run did not start. The
// causes stay in the log of the run.
func (h *Host) tenantAPIStartRefused(w http.ResponseWriter, r *http.Request, orgID string, err error) {
	var held *heldFailureError
	switch {
	case errors.Is(err, errProvisionElsewhere), errors.Is(err, errProvisionedRecently):
		writeTenantAPI(w, http.StatusAccepted, orgID, statusProvisioning)
	case errors.Is(err, errProvisionBusy):
		w.Header().Set("Retry-After", retryBusy)
		writeError(w, r, http.StatusServiceUnavailable, "provisioning_busy", "Tenant provisioning is busy")
	case errors.Is(err, ErrNeedsOperator):
		writeError(w, r, http.StatusServiceUnavailable, "provisioning_needs_operator",
			"Tenant provisioning needs an operator")
	case errors.As(err, &held):
		w.Header().Set("Retry-After", held.retryAfter())
		writeError(w, r, http.StatusServiceUnavailable, "provisioning_failed", "Tenant provisioning failed")
	default:
		h.tenantAPIUnavailable(w, r, orgID, err)
	}
}

// tenantAPIUnavailable answers a lookup that failed. The error is logged
// only: it can name the database.
func (h *Host) tenantAPIUnavailable(w http.ResponseWriter, r *http.Request, orgID string, err error) {
	slog.ErrorContext(r.Context(), "tenant api: tenant unavailable", "org_id", orgID, "error", err)
	w.Header().Set("Retry-After", retryBusy)
	writeError(w, r, http.StatusServiceUnavailable, "tenant_unavailable", "Tenant unavailable")
}
