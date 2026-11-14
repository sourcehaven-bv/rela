package tenanthost

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

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
//
// Two `organization.updated` events for one org can arrive together. If
// each read the IdP and then applied what it read, an older answer could
// land last: one sync reads active, another reads suspended and suspends,
// and then the first resumes. So syncs of one org run one at a time in this
// process, and each reads the IdP only once it holds the org's turn. Other
// processes are not serialized by that, so a resume that changed the
// status reads the IdP once more and suspends again if the org is now
// suspended. Across processes a tenant is then served against the IdP's
// word only between that resume and the second read.
type StatusSync struct {
	src        OrgStatusSource
	apply      IDPSuspension
	revalidate func(ctx context.Context, orgID string)
	turns      orgTurns
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

// orgTurns lets one caller at a time hold an org's turn. An org's entry
// lives only while someone holds or waits for it.
type orgTurns struct {
	mu sync.Mutex
	m  map[string]*orgTurn
}

type orgTurn struct {
	// ch holds one token while the turn is taken.
	ch      chan struct{}
	waiters int
}

// take waits for orgID's turn and returns the function that gives it back.
// It gives up with ctx's error when ctx ends first.
func (t *orgTurns) take(ctx context.Context, orgID string) (func(), error) {
	t.mu.Lock()
	if t.m == nil {
		t.m = map[string]*orgTurn{}
	}
	e, ok := t.m[orgID]
	if !ok {
		e = &orgTurn{ch: make(chan struct{}, 1)}
		t.m[orgID] = e
	}
	e.waiters++
	t.mu.Unlock()
	leave := func() {
		t.mu.Lock()
		e.waiters--
		if e.waiters == 0 {
			delete(t.m, orgID)
		}
		t.mu.Unlock()
	}
	select {
	case e.ch <- struct{}{}:
		return func() { <-e.ch; leave() }, nil
	case <-ctx.Done():
		leave()
		return nil, ctx.Err()
	}
}

// SyncStatus reads orgID's status at the IdP and applies it. A suspended
// org suspends its tenant, which closes at once. An active org resumes a
// tenant only the IdP suspended. An org the IdP does not know changes
// nothing: erasure is an operator's decision (TKT-TNERAS). Neither does an
// erased tenant change.
//
// Syncs of one org take turns, and a resume is checked against the IdP
// again; see [StatusSync] for why.
func (s *StatusSync) SyncStatus(ctx context.Context, orgID string) error {
	release, err := s.turns.take(ctx, orgID)
	if err != nil {
		return err
	}
	defer release()
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
			s.revalidateNow(ctx, orgID)
		}
	} else {
		changed, err = s.apply.ResumeForIDP(ctx, orgID)
		if err == nil && changed {
			var nowSuspended, resuspended bool
			nowSuspended, resuspended, err = s.recheck(ctx, orgID)
			switch {
			case err != nil:
			case resuspended:
				slog.Info("tenant host: the IdP suspended the org again while its tenant was being resumed; "+
					"suspended it again", "org_id", orgID)
				return nil
			case nowSuspended:
				// Another process suspended it again first.
				slog.Info("tenant host: the IdP suspended the org again while its tenant was being resumed; "+
					"it is suspended", "org_id", orgID)
				return nil
			}
		}
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

// recheck reads orgID's status at the IdP again after its tenant was
// resumed, and suspends the tenant again when the org is suspended now.
// The IdP may have suspended the org between this sync's read and its
// resume, and a sync in another process then found the tenant still
// suspended and changed nothing. It reports whether the IdP now says the
// org is suspended, and whether this call changed the tenant to suspended.
// An error means the IdP should send the event again, so that the next
// sync decides.
func (s *StatusSync) recheck(ctx context.Context, orgID string) (suspended, changed bool, err error) {
	suspended, err = s.src.OrgSuspended(ctx, orgID)
	switch {
	case errors.Is(err, ErrOrgNotFound):
		return false, false, nil
	case err != nil:
		return false, false, err
	case !suspended:
		return false, false, nil
	}
	changed, err = s.apply.SuspendForIDP(ctx, orgID)
	if err != nil {
		return true, false, err
	}
	s.revalidateNow(ctx, orgID)
	return true, changed, nil
}

// revalidateTimeout bounds one revalidation after a status change.
const revalidateTimeout = 10 * time.Second

// revalidateNow closes orgID's open tenant when it is no longer served. It
// runs after the status changed, so the webhook hanging up must not cancel
// it.
func (s *StatusSync) revalidateNow(ctx context.Context, orgID string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), revalidateTimeout)
	defer cancel()
	s.revalidate(ctx, orgID)
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
