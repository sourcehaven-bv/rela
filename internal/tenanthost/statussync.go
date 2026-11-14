package tenanthost

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// eventOrgUpdated is the IdP event that makes the host sync a tenant's
// status. The event carries only the org id, so the host asks the IdP for
// the org's current status.
const eventOrgUpdated = "organization.updated"

// retryProvisioningSync is the Retry-After, in seconds, of an
// `organization.updated` for a tenant that is still being created.
const retryProvisioningSync = "30"

// StatusSyncer brings the status of orgID's tenant in line with the org's
// status at the IdP. It returns nil when it applied a change or found
// nothing to do, including for an org with no tenant. Any error means the
// IdP should send the event again. An error wrapping
// [tenant.ErrTenantProvisioning] means the tenant is still being created.
//
// The production value is a [*StatusSync].
type StatusSyncer interface {
	SyncStatus(ctx context.Context, orgID string) error
}

// ErrOrgNotFound reports an org the IdP does not know. An [OrgStatusSource]
// returns it.
var ErrOrgNotFound = errors.New("tenanthost: the IdP does not know the organization")

// OrgStatusSource reports whether orgID is suspended at the IdP. It returns
// [ErrOrgNotFound] for an org the IdP does not know; any other error means
// the IdP could not answer. The production value is an adapter over the
// operator API client in internal/idporg.
type OrgStatusSource interface {
	OrgSuspended(ctx context.Context, orgID string) (bool, error)
}

// IDPSuspension changes a tenant's status on the IdP's behalf. The
// production value is [tenant.Provisioner]; see its SuspendForIDP and
// ResumeForIDP for the rules.
type IDPSuspension interface {
	SuspendForIDP(ctx context.Context, orgID string) (bool, error)
	ResumeForIDP(ctx context.Context, orgID string) (bool, error)
}

// StatusSync is the [StatusSyncer] over the IdP's org status and the
// control table.
type StatusSync struct {
	src        OrgStatusSource
	apply      IDPSuspension
	revalidate func(ctx context.Context, orgID string)
}

// NewStatusSync builds a StatusSync. revalidate closes orgID's open tenant
// when it is no longer served; the production value is
// [tenant.Registry.RevalidateOrg]. Without it a suspension would take
// effect only at the registry's next revalidation pass.
//
// Nil: rejected — every collaborator is required.
func NewStatusSync(src OrgStatusSource, apply IDPSuspension,
	revalidate func(ctx context.Context, orgID string),
) (*StatusSync, error) {
	if src == nil || apply == nil || revalidate == nil {
		return nil, errors.New("tenanthost: status sync needs a status source, a suspension writer and a revalidator")
	}
	return &StatusSync{src: src, apply: apply, revalidate: revalidate}, nil
}

// SyncStatus reads orgID's status at the IdP and applies it. A suspended
// org suspends its tenant, which closes at once. An active org resumes a
// tenant only the IdP suspended. An org the IdP does not know changes
// nothing: erasure is an operator's decision (TKT-TNERAS).
func (s *StatusSync) SyncStatus(ctx context.Context, orgID string) error {
	suspended, err := s.src.OrgSuspended(ctx, orgID)
	if errors.Is(err, ErrOrgNotFound) {
		slog.Warn("tenant host: the IdP does not know an org it sent an update for; nothing changed",
			"org_id", orgID)
		return nil
	}
	if err != nil {
		return err
	}
	var changed bool
	if suspended {
		changed, err = s.apply.SuspendForIDP(ctx, orgID)
		if err == nil {
			// Also when nothing changed: a tenant already suspended may
			// still be open until the next revalidation pass.
			s.revalidate(ctx, orgID)
		}
	} else {
		changed, err = s.apply.ResumeForIDP(ctx, orgID)
	}
	switch {
	case errors.Is(err, tenant.ErrUnknownTenant):
		slog.Info("tenant host: org status update for an org with no tenant", "org_id", orgID)
		return nil
	case errors.Is(err, tenant.ErrSchemaCollision):
		// Sending the event again cannot help.
		slog.Error("tenant host: org status update for an org whose schema belongs to another org",
			"org_id", orgID, "error", err)
		return nil
	case errors.Is(err, tenant.ErrOperatorSuspended):
		slog.Info("tenant host: the IdP reactivated an org an operator suspended; it stays suspended "+
			"until `rela-server tenant resume`", "org_id", orgID)
		return nil
	case err != nil:
		return err
	case changed && suspended:
		slog.Info("tenant host: tenant suspended because the IdP suspended its org", "org_id", orgID)
	case changed:
		slog.Info("tenant host: tenant resumed because the IdP reactivated its org", "org_id", orgID)
	}
	return nil
}

// syncFromWebhook applies a verified `organization.updated`. It runs before
// the tenant is acquired, so it works for a suspended tenant too, and the
// event is not passed on: the tenant app's org handler only logs it.
func (h *Host) syncFromWebhook(w http.ResponseWriter, r *http.Request, claims dataentry.WebhookClaims) {
	err := h.cfg.StatusSync.SyncStatus(r.Context(), claims.OrgID)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, tenant.ErrTenantProvisioning):
		slog.Warn("tenant host: org status update for a tenant still being created; asking the IdP to retry",
			"org_id", claims.OrgID, "id", claims.ID)
		w.Header().Set("Retry-After", retryProvisioningSync)
		http.Error(w, "tenant is being created", http.StatusServiceUnavailable)
	default:
		// Logged only: the cause can name the IdP's address or the database.
		slog.Error("tenant host: org status update not applied; asking the IdP to retry",
			"org_id", claims.OrgID, "id", claims.ID, "error", err)
		http.Error(w, "org status unavailable", http.StatusBadGateway)
	}
}
