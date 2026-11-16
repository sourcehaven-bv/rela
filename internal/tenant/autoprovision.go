package tenant

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ProvisionMode says when a tenant-mode server creates a tenant by itself
// (TKT-TNPRV8). The operator chooses it; the default creates none.
type ProvisionMode string

// The provisioning modes.
const (
	// ProvisionOff creates no tenant: an operator runs `rela-server tenant
	// create`, and an unknown org is refused.
	ProvisionOff ProvisionMode = "off"
	// ProvisionWebhook creates a tenant when the IdP's verified
	// `organization.created` webhook names an org this server does not know.
	// An unknown org on a request is still refused.
	ProvisionWebhook ProvisionMode = "webhook"
	// ProvisionLazy is [ProvisionWebhook] plus a backstop: the first
	// verified request of an unknown org creates its tenant, so a missed
	// webhook heals itself.
	ProvisionLazy ProvisionMode = "lazy"
)

// EnvProvision is the environment variable that sets the provisioning mode
// when the -tenant-provision flag is not given.
const EnvProvision = "RELA_TENANT_PROVISION"

// ParseProvisionMode reads a mode name. The empty string is
// [ProvisionOff].
func ParseProvisionMode(s string) (ProvisionMode, error) {
	switch m := ProvisionMode(s); m {
	case "", ProvisionOff:
		return ProvisionOff, nil
	case ProvisionWebhook, ProvisionLazy:
		return m, nil
	}
	return "", fmt.Errorf("tenant: unknown provisioning mode %q; want off, webhook or lazy", s)
}

// Eager reports whether the mode acts on the `organization.created`
// webhook.
func (m ProvisionMode) Eager() bool { return m == ProvisionWebhook || m == ProvisionLazy }

// Lazy reports whether the mode creates a tenant on a verified request.
func (m ProvisionMode) Lazy() bool { return m == ProvisionLazy }

// ProvisionState is where an org's automatic provisioning stands.
type ProvisionState string

// The provisioning states.
const (
	// ProvisionNone means no provisioning is pending or failed for the org.
	ProvisionNone ProvisionState = ""
	// ProvisionRunning means a create runs now, in this process or another.
	ProvisionRunning ProvisionState = "provisioning"
	// ProvisionFailed means the last create failed; a new one starts after
	// [AutoProvisionOptions.RetryAfter].
	ProvisionFailed ProvisionState = "failed"
	// ProvisionThrottled means the rate limit refused a new create.
	ProvisionThrottled ProvisionState = "throttled"
)

// creator creates one tenant automatically. [Provisioner] implements it.
type creator interface {
	Provision(ctx context.Context, orgID string) (Tenant, error)
}

// AutoProvisionOptions bounds an [AutoProvisioner]. A zero field takes its
// default.
type AutoProvisionOptions struct {
	// Rate is how many creates may start per minute. Default 10.
	Rate int
	// Burst is how many creates may start at once after a quiet spell.
	// Default 3.
	Burst int
	// Concurrent is how many creates may run at the same time. Default 2.
	Concurrent int
	// RetryAfter is how long a failed org waits before a new create.
	// Default one minute.
	RetryAfter time.Duration
}

const (
	defaultProvisionRate       = 10
	defaultProvisionBurst      = 3
	defaultProvisionConcurrent = 2
	defaultProvisionRetryAfter = time.Minute
	// provisionInUseHold is how long an org whose create runs in another
	// process stays [ProvisionRunning] here, so polls do not start a create
	// that would only find the lock taken.
	provisionInUseHold = 10 * time.Second
	// maxProvisionJobs bounds the per-org state kept for failed creates.
	maxProvisionJobs = 4096
)

// provisionJob is one org's pending or failed create. until is zero while
// the create runs here; once it ended, the job's state holds until then.
type provisionJob struct {
	state ProvisionState
	until time.Time
	done  chan struct{}
}

// liveLocked reports whether j still decides orgID's state at now.
func (j *provisionJob) liveLocked(now time.Time) bool {
	return j.until.IsZero() || now.Before(j.until)
}

// AutoProvisioner creates tenants in the background for the IdP webhook and
// the lazy backstop (TKT-TNPRV8).
//
// # The trust assumption
//
// Creating a tenant for a verified org this server has never seen is an
// authorization decision: anyone who can mint a verified assertion or a
// signed webhook can create schemas. That is acceptable only while all of
// these hold:
//
//   - there is exactly one issuer, and the assertion and webhook audiences
//     are pinned (-jwt-issuer, -jwt-audience, -webhook-audience);
//   - the IdP (Pratique) puts `org_id` on every verified assertion and gives
//     an orgless session no assertion at all.
//
// So "verified org never seen" means a new org, not an attacker. An erased
// org is not one: [Provisioner.Provision] refuses it ([ErrTenantErased]),
// so a replayed webhook or a still-valid token cannot bring it back. The day a
// second issuer exists, this stops holding: switch the lazy path off
// ([ProvisionWebhook] or [ProvisionOff]) before adding one.
//
// # Bounds
//
// Creating a schema is expensive, so creates are single-flighted per org,
// at most [AutoProvisionOptions.Concurrent] run at once, and a token bucket
// bounds how many start. A failed org waits [AutoProvisionOptions.RetryAfter]
// before another attempt, so a create that keeps failing is not retried on
// every request.
//
// [AutoProvisioner.Start] returns at once and the create runs in its own
// goroutine, bounded by the provisioner's own timeout. Only a caller that
// must not acknowledge before the create ends, the IdP webhook, waits for
// it with [AutoProvisioner.Wait], under its own deadline. Several processes may provision the same org; the provisioner's
// exclusive lock and its status compare-and-set make all but one of them
// back off.
type AutoProvisioner struct {
	create     creator
	limiter    *rate.Limiter
	slots      chan struct{}
	retryAfter time.Duration
	now        func() time.Time

	// ctx is the lifetime of every create; Close cancels it. It is not a
	// request's context: a create outlives the request that started it.
	ctx    context.Context //nolint:containedctx // provisioner lifetime, cancelled by Close
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu   sync.Mutex
	jobs map[string]*provisionJob
}

// NewAutoProvisioner builds an auto-provisioner over p.
//
// Nil: rejected — a nil provisioner is an error.
func NewAutoProvisioner(p *Provisioner, opts AutoProvisionOptions) (*AutoProvisioner, error) {
	if p == nil {
		return nil, errors.New("tenant: auto-provisioner needs a provisioner")
	}
	return newAutoProvisioner(p, opts, time.Now), nil
}

func newAutoProvisioner(c creator, opts AutoProvisionOptions, now func() time.Time) *AutoProvisioner {
	if opts.Rate <= 0 {
		opts.Rate = defaultProvisionRate
	}
	if opts.Burst <= 0 {
		opts.Burst = defaultProvisionBurst
	}
	if opts.Concurrent <= 0 {
		opts.Concurrent = defaultProvisionConcurrent
	}
	if opts.RetryAfter <= 0 {
		opts.RetryAfter = defaultProvisionRetryAfter
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &AutoProvisioner{
		create:     c,
		limiter:    rate.NewLimiter(rate.Every(time.Minute/time.Duration(opts.Rate)), opts.Burst),
		slots:      make(chan struct{}, opts.Concurrent),
		retryAfter: opts.RetryAfter,
		now:        now,
		ctx:        ctx,
		cancel:     cancel,
		jobs:       make(map[string]*provisionJob),
	}
}

// RetryAfter is how long a failed org waits before another attempt.
func (a *AutoProvisioner) RetryAfter() time.Duration { return a.retryAfter }

// Start begins creating orgID's tenant unless a create for it runs or failed
// recently, and returns where provisioning stands. It never waits for the
// create.
//
// The caller has already found that orgID is unknown, or that its create
// did not finish ([ErrTenantProvisioning]). It must never call Start for a
// suspended or erasing tenant: a create refuses those, but the call would
// still count against the rate limit.
func (a *AutoProvisioner) Start(orgID string) ProvisionState {
	if err := checkOrgID(orgID); err != nil {
		return ProvisionFailed
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ctx.Err() != nil {
		return ProvisionFailed
	}
	if j, ok := a.jobs[orgID]; ok {
		if j.liveLocked(a.now()) {
			return j.state
		}
		delete(a.jobs, orgID)
	}
	if !a.limiter.Allow() {
		return ProvisionThrottled
	}
	a.pruneLocked()
	j := &provisionJob{state: ProvisionRunning, done: make(chan struct{})}
	a.jobs[orgID] = j
	a.wg.Add(1)
	go a.run(orgID, j)
	return ProvisionRunning
}

// State reports where orgID's provisioning stands without starting one.
// [ProvisionNone] means nothing is pending: the tenant exists, or nobody
// asked for it.
func (a *AutoProvisioner) State(orgID string) ProvisionState {
	a.mu.Lock()
	defer a.mu.Unlock()
	j, ok := a.jobs[orgID]
	if !ok || !j.liveLocked(a.now()) {
		return ProvisionNone
	}
	return j.state
}

// Wait waits until orgID's create running here ends or ctx is done, and
// returns where provisioning stands then. With no create running it returns
// at once.
func (a *AutoProvisioner) Wait(ctx context.Context, orgID string) ProvisionState {
	a.mu.Lock()
	j, ok := a.jobs[orgID]
	a.mu.Unlock()
	if ok {
		select {
		case <-j.done:
		case <-ctx.Done():
		}
	}
	return a.State(orgID)
}

// Close stops starting creates and waits for the running ones, which end
// when their context is cancelled.
func (a *AutoProvisioner) Close() {
	a.mu.Lock()
	a.cancel()
	a.mu.Unlock()
	a.wg.Wait()
}

// run creates orgID's tenant and records the outcome.
func (a *AutoProvisioner) run(orgID string, j *provisionJob) {
	defer a.wg.Done()
	defer close(j.done)
	select {
	case a.slots <- struct{}{}:
	case <-a.ctx.Done():
		a.finish(orgID, j, a.ctx.Err())
		return
	}
	defer func() { <-a.slots }()

	slog.Info("tenant: provisioning", "org_id", orgID)
	_, err := a.create.Provision(a.ctx, orgID)
	a.finish(orgID, j, err)
}

// finish records a create's outcome. A tenant that already exists ends the
// job: another process or an operator created it first, or it is suspended
// or erasing, which the request path refuses on its own.
func (a *AutoProvisioner) finish(orgID string, j *provisionJob, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case err == nil:
		slog.Info("tenant: provisioned", "org_id", orgID)
		delete(a.jobs, orgID)
		return
	case errors.Is(err, ErrTenantExists):
		slog.Info("tenant: provisioning found the tenant already recorded", "org_id", orgID, "error", err)
		delete(a.jobs, orgID)
		return
	case errors.Is(err, ErrTenantInUse):
		// Another process holds the tenant's lock: it is creating the
		// tenant now. Held as running, so polls meanwhile start nothing.
		slog.Info("tenant: provisioning runs in another process", "org_id", orgID)
		j.until = a.now().Add(provisionInUseHold)
		return
	case errors.Is(err, ErrTenantErased):
		slog.Warn("tenant: provisioning refused; the org was erased and only an operator may create it",
			"org_id", orgID)
	case errors.Is(err, ErrSchemaExists), errors.Is(err, ErrSchemaCollision):
		slog.Error("tenant: provisioning refused; an operator must resolve the schema",
			"org_id", orgID, "error", err)
	default:
		slog.Error("tenant: provisioning failed", "org_id", orgID, "error", err)
	}
	j.state = ProvisionFailed
	j.until = a.now().Add(a.retryAfter)
}

// pruneLocked drops ended jobs whose hold has passed, and more ended ones
// beyond [maxProvisionJobs], so the map stays bounded.
func (a *AutoProvisioner) pruneLocked() {
	if len(a.jobs) < maxProvisionJobs {
		return
	}
	now := a.now()
	for org, j := range a.jobs {
		if !j.liveLocked(now) {
			delete(a.jobs, org)
		}
	}
	for org, j := range a.jobs {
		if len(a.jobs) < maxProvisionJobs {
			return
		}
		if !j.until.IsZero() {
			delete(a.jobs, org)
		}
	}
}
