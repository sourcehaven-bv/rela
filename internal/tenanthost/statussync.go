package tenanthost

import (
	"context"
	"errors"
	"fmt"
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

// eventOrgDeleted is the IdP event that makes the host suspend the org's
// tenant. It never erases: erasure stays an operator's decision
// (TKT-TNERAS).
const eventOrgDeleted = "organization.deleted"

// retryProvisioningSync is the Retry-After, in seconds, of an
// `organization.updated` for a tenant that is still being created.
const retryProvisioningSync = "30"

// retryNoStatusSync is the Retry-After, in seconds, of a status event that
// arrives while status sync is not configured.
const retryNoStatusSync = "300"

// StatusSyncer brings the status of orgID's tenant in line with the org's
// status at the IdP. Its methods return nil when they applied a change or
// found nothing to do, including for an org with no tenant. Any error
// means the IdP should send the event again. An error wrapping
// [tenant.ErrTenantProvisioning] means the tenant is still being created.
//
// The production value is a [*StatusSync].
type StatusSyncer interface {
	// SyncStatus handles an `organization.updated`.
	SyncStatus(ctx context.Context, orgID string) error
	// SyncDeleted handles an `organization.deleted`.
	SyncDeleted(ctx context.Context, orgID string) error
}

// ErrOrgNotFound reports an org the IdP does not know. An [OrgStatusSource]
// returns it.
var ErrOrgNotFound = errors.New("tenanthost: the IdP does not know the organization")

// ErrInvalidOrgID reports an org id the IdP cannot be asked about. An
// [OrgStatusSource] returns it. Unlike [ErrOrgNotFound] it says nothing
// about the org, so it never changes the tenant.
var ErrInvalidOrgID = errors.New("tenanthost: the org id cannot name an IdP organization")

// OrgStatusSource reports whether orgID is suspended at the IdP. It returns
// [ErrOrgNotFound] for an org the IdP does not know and [ErrInvalidOrgID]
// for an id it cannot ask about; any other error means the IdP could not
// answer. The production value is an adapter over the operator API client
// in internal/idporg.
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
// org suspends its tenant, which closes at once. So does an org the IdP no
// longer knows: a deleted org is a deactivation, and its data stays until
// an operator erases it (TKT-TNERAS). An active org resumes a tenant only
// the IdP suspended. An erased tenant never changes.
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
	gone := errors.Is(err, ErrOrgNotFound)
	switch {
	case gone:
		suspended = true
	case err != nil:
		return err
	}
	var changed bool
	if suspended {
		changed, err = s.suspend(ctx, orgID)
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
	case errors.Is(err, tenant.ErrOperatorSuspended):
		slog.Info("tenant host: the IdP reactivated an org an operator suspended; it stays suspended "+
			"until `rela-server tenant resume`", "org_id", orgID)
		return nil
	case err != nil:
		return syncFailed(orgID, err)
	case changed && gone:
		slog.Warn("tenant host: tenant suspended because the IdP no longer knows its org; "+
			"its data stays until an operator erases it (rela-server tenant erase)", "org_id", orgID)
	case changed && suspended:
		slog.Info("tenant host: tenant suspended because the IdP suspended its org", "org_id", orgID)
	case changed:
		slog.Info("tenant host: tenant resumed because the IdP reactivated its org", "org_id", orgID)
	}
	return nil
}

// SyncDeleted suspends orgID's tenant because the IdP deleted the org, and
// closes it at once. It never erases: the data stays until an operator
// erases it (TKT-TNERAS). An org with no tenant, and a tenant already
// suspended, erasing or erased, change nothing. It takes the org's turn
// like [StatusSync.SyncStatus]. An id that cannot name a tenant is
// [ErrInvalidOrgID], as for a status update the IdP cannot be asked about.
func (s *StatusSync) SyncDeleted(ctx context.Context, orgID string) error {
	if _, err := tenant.SchemaFor(orgID); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidOrgID, err)
	}
	release, err := s.turns.take(ctx, orgID)
	if err != nil {
		return err
	}
	defer release()
	changed, err := s.suspend(ctx, orgID)
	switch {
	case errors.Is(err, tenant.ErrUnknownTenant):
		// An org being erased, or one this server never had: worth an
		// operator's look, since an erasure may be in progress.
		slog.Warn("tenant host: idp webhook organization deletion for an org this server does not "+
			"serve; check that its erasure completed (PROCEDURE-u020, rela-server tenant erase)",
			"org_id", orgID)
		return nil
	case err != nil:
		return syncFailed(orgID, err)
	case changed:
		slog.Warn("tenant host: tenant suspended because the IdP deleted its org; "+
			"its data stays until an operator erases it (rela-server tenant erase)", "org_id", orgID)
	}
	return nil
}

// suspend suspends orgID's tenant for the IdP and closes it when that
// succeeded. It closes it also when nothing changed: a tenant already
// suspended may still be open until the next revalidation pass.
func (s *StatusSync) suspend(ctx context.Context, orgID string) (bool, error) {
	changed, err := s.apply.SuspendForIDP(ctx, orgID)
	if err == nil {
		s.revalidateNow(ctx, orgID)
	}
	return changed, err
}

// syncFailed logs a sync that cannot apply because orgID's derived schema
// is recorded for another org, which only an operator resolves, and returns
// err. The caller answers with an error status in every case, so the IdP
// keeps the event until the cause is fixed.
func syncFailed(orgID string, err error) error {
	if errors.Is(err, tenant.ErrSchemaCollision) {
		slog.Error("tenant host: org status update for an org whose schema belongs to another org; "+
			"see `rela-server tenant show <org_id>`", "org_id", orgID, "error", err)
	}
	return err
}

// recheck reads orgID's status at the IdP again after its tenant was
// resumed, and suspends the tenant again when the org is suspended or
// unknown now. The IdP may have suspended or deleted the org between this
// sync's read and its resume, and a sync in another process then found the
// tenant still suspended and changed nothing. It reports whether the IdP
// now says the org is not active, and whether this call changed the tenant
// to suspended. An error means the IdP should send the event again, so
// that the next sync decides.
func (s *StatusSync) recheck(ctx context.Context, orgID string) (suspended, changed bool, err error) {
	suspended, err = s.src.OrgSuspended(ctx, orgID)
	switch {
	case errors.Is(err, ErrOrgNotFound):
	case err != nil:
		return false, false, err
	case !suspended:
		return false, false, nil
	}
	changed, err = s.suspend(ctx, orgID)
	if err != nil {
		return true, false, err
	}
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

// syncFromWebhook applies a verified `organization.updated` or
// `organization.deleted`. It runs before the tenant is acquired, so it
// works for a suspended tenant too, and the event is not passed on: the
// tenant app's org handler only logs it.
//
// Without status sync the event cannot be applied, so it is refused with a
// retryable status rather than acknowledged: a deactivation that is
// answered 200 and not applied leaves the org's users their access.
// cmd/rela-server refuses to start with the IdP webhook and without status
// sync, so this answer means a wiring fault.
func (h *Host) syncFromWebhook(w http.ResponseWriter, r *http.Request, claims dataentry.WebhookClaims) {
	if h.cfg.StatusSync == nil {
		slog.Error("tenant host: org status event received but status sync is not configured "+
			"(-tenant-idp-org-api); asking the IdP to retry",
			"org_id", claims.OrgID, "event", claims.Event, "id", claims.ID)
		w.Header().Set("Retry-After", retryNoStatusSync)
		http.Error(w, "org status sync is not configured", http.StatusServiceUnavailable)
		return
	}
	var err error
	if claims.Event == eventOrgDeleted {
		err = h.cfg.StatusSync.SyncDeleted(r.Context(), claims.OrgID)
	} else {
		err = h.cfg.StatusSync.SyncStatus(r.Context(), claims.OrgID)
	}
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, tenant.ErrTenantProvisioning):
		slog.Warn("tenant host: org status event for a tenant still being created; asking the IdP to retry",
			"org_id", claims.OrgID, "event", claims.Event, "id", claims.ID)
		w.Header().Set("Retry-After", retryProvisioningSync)
		http.Error(w, "tenant is being created", http.StatusServiceUnavailable)
	case errors.Is(err, tenant.ErrSchemaCollision), errors.Is(err, ErrInvalidOrgID):
		// Retrying cannot help until an operator acts, but acknowledging
		// would drop a deactivation. syncFailed logged the collision.
		slog.Error("tenant host: org status event not applied; it needs an operator",
			"org_id", claims.OrgID, "event", claims.Event, "id", claims.ID, "error", err)
		http.Error(w, "org status event not applied: operator action is needed", http.StatusInternalServerError)
	default:
		// Logged only: the cause can name the IdP's address or the database.
		slog.Error("tenant host: org status event not applied; asking the IdP to retry",
			"org_id", claims.OrgID, "event", claims.Event, "id", claims.ID, "error", err)
		http.Error(w, "org status unavailable", http.StatusBadGateway)
	}
}
