package tenanthost

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// ProvisionMode says when the host creates a tenant for a verified org that
// has none (TKT-TNPRV8). It is the operator's `-tenant-provision` switch.
type ProvisionMode string

// The provisioning modes. Off is the default and fails closed.
const (
	// ProvisionOff creates no tenant. An unknown org is a 403, and the IdP
	// webhook acknowledges and drops events for it.
	ProvisionOff ProvisionMode = "off"
	// ProvisionWebhook creates a tenant from a verified
	// `organization.created` webhook only. Requests for an unknown org stay
	// a 403.
	ProvisionWebhook ProvisionMode = "webhook"
	// ProvisionLazy does what ProvisionWebhook does, and also creates a
	// tenant when a request with a verified but unknown org arrives. That
	// request is answered 202, so a missed webhook heals itself.
	ProvisionLazy ProvisionMode = "lazy"
)

// ParseProvisionMode reads a `-tenant-provision` value. An empty value is
// [ProvisionOff].
func ParseProvisionMode(s string) (ProvisionMode, error) {
	switch m := ProvisionMode(s); m {
	case "":
		return ProvisionOff, nil
	case ProvisionOff, ProvisionWebhook, ProvisionLazy:
		return m, nil
	}
	return "", fmt.Errorf("tenanthost: provisioning mode %q is not one of off, webhook, lazy", s)
}

// Provisioner creates the tenant of orgID: its control row, schema and
// migrations. It must be idempotent: a call for an org whose tenant exists
// returns nil, and a call for one whose create failed part way finishes it.
// It returns [tenant.ErrTenantInUse] when another process holds the
// tenant's lock, which the host reports as still running.
//
// The production value is an adapter over [tenant.Provisioner.Create].
type Provisioner interface {
	Provision(ctx context.Context, orgID string) error
}

// ProvisionConfig enables provisioning on a [Host]. The zero value is
// [ProvisionOff].
type ProvisionConfig struct {
	// Mode says when to provision. Empty is [ProvisionOff].
	Mode ProvisionMode
	// Provisioner creates a tenant. Required unless Mode is off.
	Provisioner Provisioner
	// PerMinute caps how many provisioning runs start per minute, across
	// all orgs. Zero takes [DefaultProvisionPerMinute].
	PerMinute int
	// MaxRunning caps how many provisioning runs are in progress at once.
	// Zero takes [DefaultProvisionMaxRunning].
	MaxRunning int
	// RunTimeout bounds one run. Zero takes [DefaultProvisionRunTimeout].
	RunTimeout time.Duration
	// WebhookWait is how long an `organization.created` webhook waits for
	// its run before answering 202. Zero takes [DefaultProvisionWebhookWait].
	WebhookWait time.Duration
}

// The provisioning defaults.
const (
	DefaultProvisionPerMinute   = 10
	DefaultProvisionMaxRunning  = 2
	DefaultProvisionRunTimeout  = 5 * time.Minute
	DefaultProvisionWebhookWait = 10 * time.Second
)

// failureHold is how long a failed run is reported before a request may
// start another one. It spaces out retries of a create that keeps failing.
const failureHold = 30 * time.Second

// errProvisionBusy reports that a run could not start because the rate or
// the running cap is reached.
var errProvisionBusy = errors.New("tenanthost: provisioning is at its limit")

// errProvisionFailed reports that the org's last run failed less than
// [failureHold] ago.
var errProvisionFailed = errors.New("tenanthost: provisioning failed")

// provisioning single-flights provisioning runs by org id and bounds how
// many start.
//
// # Trust assumption
//
// Provisioning on a verified but unknown org is an authorization decision:
// anyone who can mint an assertion, or a signed webhook, can make this
// server create schemas. That is acceptable only because of what holds
// today. There is one issuer. The audience is pinned, and the webhook has
// its own audience. Pratique sets `org_id` on every verified assertion, and
// an orgless session receives no assertion at all. So an org this server
// has not seen is new, not malicious. This stops holding the day a second
// issuer is trusted, which is why [ProvisionOff] is the default and the
// rate and running caps exist.
type provisioning struct {
	p          Provisioner
	lazy       bool
	limiter    *rate.Limiter
	maxRunning int
	timeout    time.Duration
	wait       time.Duration
	now        func() time.Time

	// stop closes when the host closes, which cancels every run.
	stop chan struct{}
	wg   sync.WaitGroup

	mu     sync.Mutex
	closed bool
	runs   map[string]*run
	failed map[string]failure
}

// run is one provisioning run. done closes when it ends; err is set before.
type run struct {
	done chan struct{}
	err  error
}

type failure struct {
	err error
	at  time.Time
}

// newProvisioning builds the runner for cfg. It returns nil for
// [ProvisionOff]. cfg is already validated.
func newProvisioning(cfg ProvisionConfig) *provisioning {
	if cfg.Mode == "" || cfg.Mode == ProvisionOff {
		return nil
	}
	perMinute := orDefault(cfg.PerMinute, DefaultProvisionPerMinute)
	return &provisioning{
		p:          cfg.Provisioner,
		lazy:       cfg.Mode == ProvisionLazy,
		limiter:    rate.NewLimiter(rate.Every(time.Minute/time.Duration(perMinute)), perMinute),
		maxRunning: orDefault(cfg.MaxRunning, DefaultProvisionMaxRunning),
		timeout:    orDefault(cfg.RunTimeout, DefaultProvisionRunTimeout),
		wait:       orDefault(cfg.WebhookWait, DefaultProvisionWebhookWait),
		now:        time.Now,
		stop:       make(chan struct{}),
		runs:       map[string]*run{},
		failed:     map[string]failure{},
	}
}

func orDefault[T int | time.Duration](v, def T) T {
	if v <= 0 {
		return def
	}
	return v
}

// validateProvision refuses a provisioning config the host cannot act on.
// webhook is whether the host receives the IdP webhook.
func validateProvision(cfg ProvisionConfig, webhook bool) error {
	mode, err := ParseProvisionMode(string(cfg.Mode))
	if err != nil {
		return err
	}
	if mode == ProvisionOff {
		return nil
	}
	if cfg.Provisioner == nil {
		return fmt.Errorf("tenanthost: provisioning mode %s needs a provisioner", mode)
	}
	if mode == ProvisionWebhook && !webhook {
		return errors.New("tenanthost: provisioning mode webhook needs the IdP webhook " +
			"(-webhook-audience and -webhook-action)")
	}
	if cfg.PerMinute < 0 || cfg.MaxRunning < 0 {
		return errors.New("tenanthost: provisioning limits must not be negative")
	}
	return nil
}

// start returns orgID's run, starting one when none is in progress. A
// caller already checked orgID with [tenant.SchemaFor]. The run keeps ctx's
// values but not its cancellation, so a client that disconnects does not
// abort a migration.
//
// It refuses with errProvisionFailed while the org's last run failed less
// than failureHold ago, and with errProvisionBusy when the rate or the
// running cap is reached or the host is closing.
func (p *provisioning) start(ctx context.Context, orgID string) (*run, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if r, ok := p.runs[orgID]; ok {
		return r, nil
	}
	now := p.now()
	for org, f := range p.failed {
		if now.Sub(f.at) >= failureHold {
			delete(p.failed, org)
		}
	}
	if f, ok := p.failed[orgID]; ok {
		return nil, fmt.Errorf("%w: %w", errProvisionFailed, f.err)
	}
	if p.closed || len(p.runs) >= p.maxRunning || !p.limiter.Allow() {
		return nil, errProvisionBusy
	}
	r := &run{done: make(chan struct{})}
	p.runs[orgID] = r
	p.wg.Add(1)
	go p.execute(context.WithoutCancel(ctx), orgID, r)
	return r, nil
}

func (p *provisioning) execute(ctx context.Context, orgID string, r *run) {
	defer p.wg.Done()
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	go func() {
		select {
		case <-p.stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	err := p.p.Provision(ctx, orgID)
	cancel()
	switch {
	case err == nil:
		slog.Info("tenant host: tenant provisioned", "org_id", orgID)
	case errors.Is(err, tenant.ErrTenantInUse):
		slog.Info("tenant host: tenant is being provisioned by another process", "org_id", orgID)
	default:
		slog.Error("tenant host: provisioning failed", "org_id", orgID, "error", err)
	}
	p.mu.Lock()
	delete(p.runs, orgID)
	if err != nil && !errors.Is(err, tenant.ErrTenantInUse) {
		p.failed[orgID] = failure{err: err, at: p.now()}
	}
	r.err = err
	p.mu.Unlock()
	close(r.done)
}

// close stops new runs, cancels the ones in progress and waits for them.
// A cancelled create is finished by the next one.
func (p *provisioning) close() {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.stop)
	}
	p.mu.Unlock()
	p.wg.Wait()
}

// provisionable reports whether err is a refusal that creating the tenant
// answers: no control row, or a create that has not finished.
func provisionable(err error) bool {
	return errors.Is(err, tenant.ErrTenantAbsent) || errors.Is(err, tenant.ErrTenantProvisioning)
}
