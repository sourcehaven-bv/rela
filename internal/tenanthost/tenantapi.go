package tenanthost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// TenantsPath is the prefix of the tenant API (TKT-84CD0PY, TKT-59HWQ76).
// A control plane manages an org's tenant under `TenantsPath+<org_id>`:
//
//   - `PUT` creates the tenant, before the org's first visit;
//   - `GET` reads its state, plan and entitlements;
//   - `PATCH` sets its state (active, read_only or suspended), its plan
//     and its entitlements;
//   - `DELETE` archives it: it is no longer served and its data stays;
//   - `GET .../usage` reads its users and storage.
//
// # Authentication
//
// The server does not verify the control plane's credentials itself. The
// edge in front of it does, and forwards an assertion in the same header,
// signed with the same key, as an end user's ([Config.Verifier]). The
// assertion must name an operator principal: `principal_type` is
// `operator`, it carries no `org_id`, and its `roles` hold
// [TenantAPIConfig.Role]. An end user's assertion always carries an org,
// so it is refused here, and an operator's assertion carries none, so the
// host refuses it on every other path. One operator principal acts on
// every org.
//
// Creating a tenant goes through the host's provisioning runner, so it
// shares the start rate, the running cap and the failure holds with lazy
// and webhook provisioning.
const TenantsPath = "/api/v1/_tenants/"

// usageSuffix ends the usage route under [TenantsPath].
const usageSuffix = "/usage"

// operatorPrincipal is the `principal_type` of an operator principal.
const operatorPrincipal = "operator"

// patchMaxBody caps a PATCH body. A plan with its entitlements is well
// under a kilobyte.
const patchMaxBody = 16 << 10

// TenantControl reads and changes tenants for the tenant API. The wiring
// implements it over [tenant.ClusterResolver] and [tenant.Provisioner],
// recording each change in the lifecycle log and closing a tenant that is
// no longer served. actor is the operator principal's subject.
type TenantControl interface {
	// Lookup returns the org's control record, as
	// [tenant.ClusterResolver.Lookup] does.
	Lookup(ctx context.Context, orgID string) (tenant.Record, error)
	// Apply makes the change, as [tenant.Provisioner.Apply] does.
	Apply(ctx context.Context, orgID, actor string, c tenant.Change) (tenant.Record, error)
	// Archive archives the org's tenant, as [tenant.Provisioner.Archive]
	// does.
	Archive(ctx context.Context, orgID, actor string) (tenant.Record, error)
	// Usage reads the org's usage, as [tenant.ClusterResolver.Usage]
	// does.
	Usage(ctx context.Context, orgID string) (tenant.Usage, error)
}

// TenantAPIConfig enables the tenant API.
type TenantAPIConfig struct {
	// Role is the role an operator principal must hold. Required.
	Role string
	// Control reads and changes tenants. Required.
	Control TenantControl
}

func (c *TenantAPIConfig) validate() error {
	switch {
	case c == nil:
		return nil
	case c.Role == "":
		return errors.New("tenanthost: the tenant API needs a role")
	case c.Control == nil:
		return errors.New("tenanthost: the tenant API needs tenant control")
	}
	return nil
}

// The states the tenant API reports beyond those [StatusPath] reports.
const (
	stateReadOnly = "read_only"
	stateArchived = "archived"
	stateErasing  = "erasing"
	stateErased   = "erased"
)

// tenantAPIBody is the body of a tenant API answer about one tenant.
type tenantAPIBody struct {
	OrgID        string              `json:"org_id"`
	State        string              `json:"state"`
	SuspendedBy  string              `json:"suspended_by,omitempty"`
	ReadOnly     bool                `json:"read_only"`
	Plan         string              `json:"plan"`
	Entitlements tenant.Entitlements `json:"entitlements"`
	ArchivedAt   *time.Time          `json:"archived_at,omitempty"`
}

// stateOf names rec's state as the tenant API reports it.
func stateOf(rec tenant.Record) string {
	switch {
	case rec.Status == tenant.StatusActive && rec.ReadOnly:
		return stateReadOnly
	case rec.Archived():
		return stateArchived
	case rec.Status == tenant.StatusErasing:
		return stateErasing
	case rec.Status == tenant.StatusErased:
		return stateErased
	}
	return string(rec.Status)
}

// bodyOf is the answer about rec.
func bodyOf(rec tenant.Record) tenantAPIBody {
	b := tenantAPIBody{
		OrgID: rec.OrgID, State: stateOf(rec), SuspendedBy: string(rec.SuspendedBy),
		ReadOnly: rec.ReadOnly, Plan: rec.Plan, Entitlements: rec.Entitlements,
	}
	if b.Entitlements == nil {
		b.Entitlements = tenant.Entitlements{}
	}
	if rec.Archived() {
		at := rec.ArchivedAt.UTC()
		b.ArchivedAt = &at
	}
	return b
}

// provisioningBody is the answer about a tenant whose create has not
// recorded it yet.
func provisioningBody(orgID string) tenantAPIBody {
	return tenantAPIBody{OrgID: orgID, State: statusProvisioning, Entitlements: tenant.Entitlements{}}
}

func writeTenantAPI(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if b, ok := body.(tenantAPIBody); ok && b.State == statusProvisioning {
		w.Header().Set("Retry-After", retryProvisioning)
		w.Header().Set("Location", TenantsPath+b.OrgID)
	}
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// serveTenantAPI authenticates a request under [TenantsPath] and routes it
// by path and method. clean is the cleaned path.
func (h *Host) serveTenantAPI(w http.ResponseWriter, r *http.Request, clean string) {
	rest := strings.TrimPrefix(clean, TenantsPath)
	orgID, usage := strings.CutSuffix(rest, usageSuffix)
	if orgID == "" || strings.Contains(orgID, "/") {
		writeError(w, r, http.StatusNotFound, "not_found", "Not found")
		return
	}
	id, ok := h.authorizeTenantAPI(w, r)
	if !ok {
		return
	}
	if _, err := tenant.SchemaFor(orgID); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_org_id", "The organization id cannot be a tenant")
		return
	}
	if usage {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
			return
		}
		h.tenantAPIUsage(w, r, orgID)
		return
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		h.tenantAPIStatus(w, r, orgID)
	case http.MethodPut:
		slog.InfoContext(r.Context(), "tenant api: create requested", "org_id", orgID, "actor", id.Subject)
		h.tenantAPICreate(w, r, orgID)
	case http.MethodPatch:
		h.tenantAPIPatch(w, r, orgID, id.Subject)
	case http.MethodDelete:
		h.tenantAPIArchive(w, r, orgID, id.Subject)
	default:
		w.Header().Set("Allow", "GET, HEAD, PUT, PATCH, DELETE")
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

// authorizeTenantAPI verifies the request's assertion and checks that it
// names an operator principal holding the tenant API's role. It answers
// the refusal itself and reports whether the request may go on.
func (h *Host) authorizeTenantAPI(w http.ResponseWriter, r *http.Request) (dataentry.AssertedIdentity, bool) {
	raw := stripBearer(r.Header.Get(h.cfg.HeaderName))
	if raw == "" {
		h.unauthenticated(w, r)
		return dataentry.AssertedIdentity{}, false
	}
	id, err := h.cfg.Verifier.VerifyAssertion(r.Context(), raw)
	if err != nil {
		// The reason stays in the log, as for any assertion.
		slog.InfoContext(r.Context(), "tenant api: assertion rejected", "method", r.Method, "error", err)
		h.unauthenticated(w, r)
		return dataentry.AssertedIdentity{}, false
	}
	if id.PrincipalType != operatorPrincipal || id.OrgID != "" || !slices.Contains(id.Roles, h.cfg.TenantAPI.Role) {
		slog.WarnContext(r.Context(), "tenant api: principal may not manage tenants",
			"subject", id.Subject, "principal_type", id.PrincipalType, "org_id", id.OrgID)
		writeError(w, r, http.StatusForbidden, "forbidden", "This principal may not manage tenants")
		return dataentry.AssertedIdentity{}, false
	}
	return id, true
}

// tenantAPIStatus answers the state of orgID's tenant from its control
// row. It opens nothing and never starts a create.
func (h *Host) tenantAPIStatus(w http.ResponseWriter, r *http.Request, orgID string) {
	rec, err := h.cfg.TenantAPI.Control.Lookup(r.Context(), orgID)
	switch {
	case err == nil:
		writeTenantAPI(w, http.StatusOK, bodyOf(rec))
	case errors.Is(err, tenant.ErrTenantAbsent) && h.prov.running(orgID):
		writeTenantAPI(w, http.StatusOK, provisioningBody(orgID))
	default:
		h.tenantAPIFailed(w, r, orgID, err)
	}
}

// tenantAPICreate creates orgID's tenant and waits a bounded time for it,
// as an `organization.created` webhook does. 201 means it was created now,
// 200 that it already existed and is served, 202 that the create is still
// running.
func (h *Host) tenantAPICreate(w http.ResponseWriter, r *http.Request, orgID string) {
	rec, err := h.cfg.TenantAPI.Control.Lookup(r.Context(), orgID)
	switch {
	case err == nil && rec.Status == tenant.StatusActive:
		writeTenantAPI(w, http.StatusOK, bodyOf(rec))
		return
	case (err == nil && rec.Status == tenant.StatusProvisioning) || errors.Is(err, tenant.ErrTenantAbsent):
		// Created below.
	case err == nil:
		h.tenantAPIConflict(w, r, rec)
		return
	default:
		h.tenantAPIFailed(w, r, orgID, err)
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
		writeTenantAPI(w, http.StatusAccepted, provisioningBody(orgID))
		return
	case <-r.Context().Done():
		// The caller hung up; the run goes on.
		return
	}
	switch {
	case run.err == nil:
		h.tenantAPIAfterCreate(w, r, orgID, http.StatusCreated)
	case errors.Is(run.err, tenant.ErrTenantInUse):
		writeTenantAPI(w, http.StatusAccepted, provisioningBody(orgID))
	case errors.Is(run.err, tenant.ErrTenantExists):
		h.tenantAPIAfterCreate(w, r, orgID, http.StatusOK)
	case errors.Is(run.err, ErrNeedsOperator):
		writeError(w, r, http.StatusServiceUnavailable, "provisioning_needs_operator",
			"Tenant provisioning needs an operator")
	default:
		w.Header().Set("Retry-After", (&heldFailureError{left: failureHold}).retryAfter())
		writeError(w, r, http.StatusServiceUnavailable, "provisioning_failed", "Tenant provisioning failed")
	}
}

// tenantAPIAfterCreate answers a create that ended, with the row as it is
// now: code when the tenant is served, a conflict when another change
// landed meanwhile.
func (h *Host) tenantAPIAfterCreate(w http.ResponseWriter, r *http.Request, orgID string, code int) {
	rec, err := h.cfg.TenantAPI.Control.Lookup(r.Context(), orgID)
	switch {
	case err != nil:
		h.tenantAPIFailed(w, r, orgID, err)
	case rec.Status == tenant.StatusActive:
		writeTenantAPI(w, code, bodyOf(rec))
	default:
		h.tenantAPIConflict(w, r, rec)
	}
}

// tenantAPIConflict refuses to create a tenant that exists but is not
// served.
func (h *Host) tenantAPIConflict(w http.ResponseWriter, r *http.Request, rec tenant.Record) {
	switch stateOf(rec) {
	case stateArchived:
		writeError(w, r, http.StatusConflict, "tenant_archived",
			"The tenant is archived; only an operator can resume or erase it")
	case stateErasing, stateErased:
		writeError(w, r, http.StatusConflict, "tenant_erased",
			"The tenant was erased; only an operator can recreate it")
	default:
		writeError(w, r, http.StatusConflict, "tenant_suspended", "The tenant exists but is suspended")
	}
}

// tenantPatch is the body of a PATCH. An absent or null field is left as
// it is; `"entitlements": {}` clears the entitlements and `"plan": ""` the
// plan.
type tenantPatch struct {
	State        *string              `json:"state"`
	Plan         *string              `json:"plan"`
	Entitlements *tenant.Entitlements `json:"entitlements"`
}

// tenantAPIPatch changes orgID's state, plan or entitlements.
func (h *Host) tenantAPIPatch(w http.ResponseWriter, r *http.Request, orgID, actor string) {
	change, err := readPatch(w, r)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_change", "The change is not valid", err.Error())
		return
	}
	rec, err := h.cfg.TenantAPI.Control.Apply(r.Context(), orgID, actor, change)
	if err != nil {
		h.tenantAPIChangeFailed(w, r, orgID, err)
		return
	}
	writeTenantAPI(w, http.StatusOK, bodyOf(rec))
}

// readPatch reads and checks a PATCH body.
func readPatch(w http.ResponseWriter, r *http.Request) (tenant.Change, error) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, patchMaxBody))
	if err != nil {
		return tenant.Change{}, errors.New("the body could not be read or is too large")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	var p tenantPatch
	if err := dec.Decode(&p); err != nil {
		return tenant.Change{}, errors.New("the body must be a JSON object with state, plan or entitlements")
	}
	if dec.More() {
		return tenant.Change{}, errors.New("the body holds more than one JSON value")
	}
	if p.State == nil && p.Plan == nil && p.Entitlements == nil {
		return tenant.Change{}, errors.New("the body changes nothing; send state, plan or entitlements")
	}
	c := tenant.Change{Plan: p.Plan, Entitlements: p.Entitlements}
	if p.State != nil {
		st, err := tenant.ParseAccessState(*p.State)
		if err != nil {
			return tenant.Change{}, err
		}
		c.State = &st
	}
	return c, c.Validate()
}

// tenantAPIArchive archives orgID's tenant.
func (h *Host) tenantAPIArchive(w http.ResponseWriter, r *http.Request, orgID, actor string) {
	rec, err := h.cfg.TenantAPI.Control.Archive(r.Context(), orgID, actor)
	if err != nil {
		h.tenantAPIChangeFailed(w, r, orgID, err)
		return
	}
	writeTenantAPI(w, http.StatusOK, bodyOf(rec))
}

// tenantUsageBody is the body of the usage route.
type tenantUsageBody struct {
	OrgID          string    `json:"org_id"`
	Users          int       `json:"users"`
	ActiveUsers30d int       `json:"active_users_30d"`
	StorageBytes   int64     `json:"storage_bytes"`
	MeasuredAt     time.Time `json:"measured_at"`
}

// tenantAPIUsage answers orgID's usage.
func (h *Host) tenantAPIUsage(w http.ResponseWriter, r *http.Request, orgID string) {
	u, err := h.cfg.TenantAPI.Control.Usage(r.Context(), orgID)
	if err != nil {
		h.tenantAPIChangeFailed(w, r, orgID, err)
		return
	}
	writeTenantAPI(w, http.StatusOK, tenantUsageBody{
		OrgID: orgID, Users: u.Users, ActiveUsers30d: u.ActiveUsers,
		StorageBytes: u.StorageBytes, MeasuredAt: u.MeasuredAt,
	})
}

// tenantAPIChangeFailed answers a PATCH, DELETE or usage read that was
// refused.
func (h *Host) tenantAPIChangeFailed(w http.ResponseWriter, r *http.Request, orgID string, err error) {
	switch {
	case errors.Is(err, tenant.ErrArchived):
		writeError(w, r, http.StatusConflict, "tenant_archived",
			"The tenant is archived; only an operator can resume or erase it")
	case errors.Is(err, tenant.ErrHeldSuspended):
		writeError(w, r, http.StatusConflict, "tenant_held",
			"An operator or the IdP suspended the tenant; only they can resume it")
	case errors.Is(err, tenant.ErrTenantProvisioning):
		w.Header().Set("Retry-After", retryProvisioning)
		writeError(w, r, http.StatusConflict, "tenant_provisioning", "Tenant is being created")
	case errors.Is(err, tenant.ErrWrongStatus):
		writeError(w, r, http.StatusConflict, "wrong_status", "The tenant cannot change in its current state")
	default:
		h.tenantAPIFailed(w, r, orgID, err)
	}
}

// tenantAPIStartRefused answers a create whose run did not start. The
// causes stay in the log of the run.
func (h *Host) tenantAPIStartRefused(w http.ResponseWriter, r *http.Request, orgID string, err error) {
	var held *heldFailureError
	switch {
	case errors.Is(err, errProvisionElsewhere), errors.Is(err, errProvisionedRecently):
		writeTenantAPI(w, http.StatusAccepted, provisioningBody(orgID))
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
		h.tenantAPIFailed(w, r, orgID, err)
	}
}

// tenantAPIFailed answers an unknown org with 404 and any other failure
// with 503. The error is logged only: it can name the database.
func (h *Host) tenantAPIFailed(w http.ResponseWriter, r *http.Request, orgID string, err error) {
	if errors.Is(err, tenant.ErrUnknownTenant) {
		writeError(w, r, http.StatusNotFound, "unknown_tenant", "No tenant for this organization")
		return
	}
	slog.ErrorContext(r.Context(), "tenant api: tenant unavailable", "org_id", orgID, "error", err)
	w.Header().Set("Retry-After", retryBusy)
	writeError(w, r, http.StatusServiceUnavailable, "tenant_unavailable", "Tenant unavailable")
}
